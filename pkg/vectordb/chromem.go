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

// fileHashEntry tracks the content hash of an indexed file for incremental updates.
type fileHashEntry struct {
	FileHash     string `json:"file_hash"`
	CodebaseHash string `json:"codebase_hash"`
}

type ChromemStore struct {
	db          *chromem.DB
	collections map[string]*chromem.Collection // codebaseHash → collection
	colMu       sync.Mutex
	path        string
	hashesPath  string
	mu          sync.RWMutex
	fileHashes  map[string]fileHashEntry // filePath → entry
}

func NewChromemStore(path string) *ChromemStore {
	return &ChromemStore{
		path:        path,
		collections: make(map[string]*chromem.Collection),
		fileHashes:  make(map[string]fileHashEntry),
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

	s.hashesPath = filepath.Join(s.path, "file_hashes.json")
	if err := s.loadFileHashes(); err != nil {
		mlog.Warningf("chromem: failed to load file hashes index, starting fresh: %v", err)
		s.fileHashes = make(map[string]fileHashEntry)
	}

	return nil
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

	// Update file hash index only when the hash changes (once per file, not per chunk).
	if vec.FileHash != "" {
		s.mu.Lock()
		existing, ok := s.fileHashes[vec.FilePath]
		if !ok || existing.FileHash != vec.FileHash {
			s.fileHashes[vec.FilePath] = fileHashEntry{
				FileHash:     vec.FileHash,
				CodebaseHash: vec.CodebaseHash,
			}
			s.mu.Unlock()
			if err := s.saveFileHashes(); err != nil {
				mlog.Warningf("chromem: failed to save file hashes index: %v", err)
			}
		} else {
			s.mu.Unlock()
		}
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for filePath, entry := range s.fileHashes {
		if entry.CodebaseHash == codebaseHash {
			result[filePath] = entry.FileHash
		}
	}
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
	delete(s.fileHashes, filePath)
	s.mu.Unlock()

	return s.saveFileHashes()
}

func (s *ChromemStore) Close() error {
	return nil // chromem flushes synchronously on each write
}

func (s *ChromemStore) loadFileHashes() error {
	data, err := os.ReadFile(s.hashesPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.fileHashes)
}

func (s *ChromemStore) saveFileHashes() error {
	s.mu.RLock()
	data, err := json.Marshal(s.fileHashes)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(s.hashesPath, data, 0644)
}
