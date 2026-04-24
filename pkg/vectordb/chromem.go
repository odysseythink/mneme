package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	chromem "github.com/philippgille/chromem-go"
	"github.com/odysseythink/mlog"
	"github.com/ranwei/claude-context/pkg"
)

type ChromemStore struct {
	db          *chromem.DB
	collections map[string]*chromem.Collection // codebaseHash → collection
	colMu       sync.Mutex
	path        string

	// fileHashes stores per-codebase file hash maps.
	// Outer key: codebaseHash. Inner key: filePath. Value: fileHash (MD5 of content).
	// Each codebase is persisted to its own hashes_{codebaseHash}.json file so the
	// file size stays proportional to the individual codebase, not the total.
	fileHashes  map[string]map[string]string // codebaseHash → (filePath → fileHash)
	loadedHashes map[string]bool             // which codebases have been loaded from disk
	mu          sync.RWMutex
}

func NewChromemStore(path string) *ChromemStore {
	return &ChromemStore{
		path:         path,
		collections:  make(map[string]*chromem.Collection),
		fileHashes:   make(map[string]map[string]string),
		loadedHashes: make(map[string]bool),
	}
}

func (s *ChromemStore) Initialize(_ string) error {
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return fmt.Errorf("failed to create chromem dir: %w", err)
	}

	db, err := chromem.NewPersistentDB(s.path, false)
	if err != nil {
		return fmt.Errorf("failed to open chromem db: %w", err)
	}
	s.db = db

	// Load any collections that were persisted in a previous session.
	for name, col := range db.ListCollections() {
		if strings.HasPrefix(name, "vectors_") {
			hash := strings.TrimPrefix(name, "vectors_")
			s.collections[hash] = col
		}
	}

	// Migrate legacy file_hashes.json to per-codebase files if it exists.
	s.migrateLegacyHashes()

	return nil
}

// hashesFilePath returns the path for a codebase-specific hash file.
func (s *ChromemStore) hashesFilePath(codebaseHash string) string {
	return filepath.Join(s.path, "hashes_"+codebaseHash+".json")
}

// loadHashesForCodebase loads (once) the hash map for a specific codebase from disk.
// Must be called with s.mu held for writing, or before s.mu is needed.
func (s *ChromemStore) loadHashesForCodebase(codebaseHash string) {
	if s.loadedHashes[codebaseHash] {
		return
	}
	s.loadedHashes[codebaseHash] = true

	data, err := os.ReadFile(s.hashesFilePath(codebaseHash))
	if os.IsNotExist(err) {
		s.fileHashes[codebaseHash] = make(map[string]string)
		return
	}
	if err != nil {
		mlog.Warningf("chromem: failed to read hashes for codebase %s: %v", codebaseHash, err)
		s.fileHashes[codebaseHash] = make(map[string]string)
		return
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		mlog.Warningf("chromem: failed to parse hashes for codebase %s: %v", codebaseHash, err)
		s.fileHashes[codebaseHash] = make(map[string]string)
		return
	}
	s.fileHashes[codebaseHash] = m
}

// saveHashesForCodebase persists the hash map for a codebase to its dedicated file.
// Must be called with s.mu.RLock held (reads fileHashes but not loadedHashes).
func (s *ChromemStore) saveHashesForCodebase(codebaseHash string) error {
	hashes := s.fileHashes[codebaseHash]
	data, err := json.Marshal(hashes)
	if err != nil {
		return err
	}
	return os.WriteFile(s.hashesFilePath(codebaseHash), data, 0644)
}

// migrateLegacyHashes converts the old monolithic file_hashes.json to per-codebase files.
func (s *ChromemStore) migrateLegacyHashes() {
	legacyPath := filepath.Join(s.path, "file_hashes.json")
	data, err := os.ReadFile(legacyPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		mlog.Warningf("chromem: failed to read legacy hashes file for migration: %v", err)
		return
	}

	// Legacy format: filePath → {file_hash, codebase_hash}
	type legacyEntry struct {
		FileHash     string `json:"file_hash"`
		CodebaseHash string `json:"codebase_hash"`
	}
	var legacy map[string]legacyEntry
	if err := json.Unmarshal(data, &legacy); err != nil {
		mlog.Warningf("chromem: failed to parse legacy hashes file: %v", err)
		return
	}

	// Group by codebase hash.
	grouped := make(map[string]map[string]string)
	for filePath, entry := range legacy {
		if grouped[entry.CodebaseHash] == nil {
			grouped[entry.CodebaseHash] = make(map[string]string)
		}
		grouped[entry.CodebaseHash][filePath] = entry.FileHash
	}

	// Write per-codebase files and mark as loaded.
	s.mu.Lock()
	for cbHash, hashes := range grouped {
		s.fileHashes[cbHash] = hashes
		s.loadedHashes[cbHash] = true
		data, err := json.Marshal(hashes)
		if err != nil {
			mlog.Warningf("chromem: migration: failed to marshal hashes for %s: %v", cbHash, err)
			continue
		}
		if err := os.WriteFile(s.hashesFilePath(cbHash), data, 0644); err != nil {
			mlog.Warningf("chromem: migration: failed to write hashes for %s: %v", cbHash, err)
			continue
		}
	}
	s.mu.Unlock()

	// Remove the legacy file only after successful migration.
	if err := os.Remove(legacyPath); err != nil {
		mlog.Warningf("chromem: failed to remove legacy hashes file after migration: %v", err)
	} else {
		mlog.Infof("chromem: migrated legacy file_hashes.json to %d per-codebase files", len(grouped))
	}
}

// getOrCreateCollection returns the collection for the given codebase hash,
// creating it if it doesn't exist yet.
func (s *ChromemStore) getOrCreateCollection(codebaseHash string) (*chromem.Collection, error) {
	s.colMu.Lock()
	defer s.colMu.Unlock()

	if col, ok := s.collections[codebaseHash]; ok {
		return col, nil
	}

	name := "vectors_" + codebaseHash
	col, err := s.db.GetOrCreateCollection(name, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create collection %s: %w", name, err)
	}
	s.collections[codebaseHash] = col
	return col, nil
}

func (s *ChromemStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	col, err := s.getOrCreateCollection(vec.CodebaseHash)
	if err != nil {
		return err
	}

	id := fmt.Sprintf("%s:%d:%d", vec.FilePath, vec.StartLine, vec.EndLine)
	doc := chromem.Document{
		ID:        id,
		Embedding: vec.Embedding,
		Metadata: map[string]string{
			"text":       vec.Text,
			"file_path":  vec.FilePath,
			"language":   vec.Language,
			"start_line": strconv.Itoa(vec.StartLine),
			"end_line":   strconv.Itoa(vec.EndLine),
			"file_hash":  vec.FileHash,
			"indexed_at": vec.IndexedAt.Format(time.RFC3339),
		},
	}
	if err := col.AddDocument(ctx, doc); err != nil {
		return err
	}

	if vec.FileHash == "" {
		return nil
	}

	// Update per-codebase hash file only when the hash actually changes (once per file).
	s.mu.Lock()
	s.loadHashesForCodebase(vec.CodebaseHash)
	existing := s.fileHashes[vec.CodebaseHash][vec.FilePath]
	if existing == vec.FileHash {
		s.mu.Unlock()
		return nil
	}
	s.fileHashes[vec.CodebaseHash][vec.FilePath] = vec.FileHash
	s.mu.Unlock()

	s.mu.RLock()
	saveErr := s.saveHashesForCodebase(vec.CodebaseHash)
	s.mu.RUnlock()
	if saveErr != nil {
		mlog.Warningf("chromem: failed to save hashes for codebase %s: %v", vec.CodebaseHash, saveErr)
	}
	return nil
}

func (s *ChromemStore) Search(ctx context.Context, embedding []float32, topK int, codebaseHash string) ([]pkg.Vector, error) {
	if codebaseHash != "" {
		return s.searchCollection(ctx, embedding, topK, codebaseHash)
	}

	// No codebase specified: search all known collections and merge results.
	s.colMu.Lock()
	cols := make(map[string]*chromem.Collection, len(s.collections))
	for h, c := range s.collections {
		cols[h] = c
	}
	s.colMu.Unlock()

	type ranked struct {
		vec        pkg.Vector
		similarity float32
	}
	var all []ranked

	for hash, col := range cols {
		count := col.Count()
		n := topK
		if n > count {
			n = count
		}
		if n == 0 {
			continue
		}
		results, err := col.QueryEmbedding(ctx, embedding, n, nil, nil)
		if err != nil {
			mlog.Warningf("chromem: search in collection for hash %s failed: %v", hash, err)
			continue
		}
		for _, r := range results {
			v, ok := chromemResultToVector(r, hash)
			if ok {
				all = append(all, ranked{vec: v, similarity: r.Similarity})
			}
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].similarity > all[j].similarity })
	if len(all) > topK {
		all = all[:topK]
	}

	vectors := make([]pkg.Vector, 0, len(all))
	for _, r := range all {
		if r.similarity >= 0.5 {
			vectors = append(vectors, r.vec)
		}
	}
	return vectors, nil
}

func (s *ChromemStore) searchCollection(ctx context.Context, embedding []float32, topK int, codebaseHash string) ([]pkg.Vector, error) {
	col, err := s.getOrCreateCollection(codebaseHash)
	if err != nil {
		return nil, err
	}

	count := col.Count()
	nResults := topK
	if nResults > count {
		nResults = count
	}
	if nResults == 0 {
		return []pkg.Vector{}, nil
	}

	results, err := col.QueryEmbedding(ctx, embedding, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem search failed: %w", err)
	}

	var vectors []pkg.Vector
	for _, r := range results {
		if r.Similarity < 0.5 {
			continue
		}
		v, ok := chromemResultToVector(r, codebaseHash)
		if ok {
			vectors = append(vectors, v)
		}
	}
	return vectors, nil
}

// chromemResultToVector converts a chromem Result to a pkg.Vector.
// Returns false if required metadata fields are missing or unparseable.
func chromemResultToVector(r chromem.Result, codebaseHash string) (pkg.Vector, bool) {
	startLine, err := strconv.Atoi(r.Metadata["start_line"])
	if err != nil {
		mlog.Warningf("chromem: skipping result with unparseable start_line %q: %v", r.Metadata["start_line"], err)
		return pkg.Vector{}, false
	}
	endLine, err := strconv.Atoi(r.Metadata["end_line"])
	if err != nil {
		mlog.Warningf("chromem: skipping result with unparseable end_line %q: %v", r.Metadata["end_line"], err)
		return pkg.Vector{}, false
	}
	indexedAt, err := time.Parse(time.RFC3339, r.Metadata["indexed_at"])
	if err != nil {
		mlog.Warningf("chromem: skipping result with unparseable indexed_at %q: %v", r.Metadata["indexed_at"], err)
		return pkg.Vector{}, false
	}

	return pkg.Vector{
		Embedding:    r.Embedding,
		Text:         r.Metadata["text"],
		FilePath:     r.Metadata["file_path"],
		Language:     r.Metadata["language"],
		StartLine:    startLine,
		EndLine:      endLine,
		CodebaseHash: codebaseHash,
		FileHash:     r.Metadata["file_hash"],
		IndexedAt:    indexedAt,
	}, true
}

func (s *ChromemStore) GetFileHashes(_ context.Context, codebaseHash string) (map[string]string, error) {
	s.mu.Lock()
	s.loadHashesForCodebase(codebaseHash)
	hashes := s.fileHashes[codebaseHash]
	// Return a copy to avoid races with the caller.
	result := make(map[string]string, len(hashes))
	for k, v := range hashes {
		result[k] = v
	}
	s.mu.Unlock()
	return result, nil
}

func (s *ChromemStore) DeleteByFilePath(ctx context.Context, filePath string, codebaseHash string) error {
	col, err := s.getOrCreateCollection(codebaseHash)
	if err != nil {
		return err
	}

	if err := col.Delete(ctx, map[string]string{"file_path": filePath}, nil); err != nil {
		return fmt.Errorf("chromem delete failed: %w", err)
	}

	s.mu.Lock()
	s.loadHashesForCodebase(codebaseHash)
	delete(s.fileHashes[codebaseHash], filePath)
	s.mu.Unlock()

	s.mu.RLock()
	saveErr := s.saveHashesForCodebase(codebaseHash)
	s.mu.RUnlock()
	return saveErr
}

func (s *ChromemStore) Close() error {
	return nil // chromem flushes synchronously on each write
}
