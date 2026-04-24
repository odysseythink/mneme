package context

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/odysseythink/mlog"
	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/splitter"
)

type Indexer struct {
	store    pkg.Store
	embedding *embedding.CachedClient
	splitter  *splitter.ASTSplitter
	model    string
}

func NewIndexer(store pkg.Store, emb *embedding.CachedClient, s *splitter.ASTSplitter, model string) *Indexer {
	if s == nil {
		s = splitter.NewSplitter()
	}
	return &Indexer{
		store:    store,
		embedding: emb,
		splitter:  s,
		model:    model,
	}
}

func (idx *Indexer) Index(ctx context.Context, codebasePath string) (pkg.IndexResult, error) {
	mlog.Infof("indexer: starting index path=%q model=%q", codebasePath, idx.model)

	result := pkg.IndexResult{
		CodebasePath:   codebasePath,
		EmbeddingModel: idx.model,
		FailedFiles:    []string{},
	}

	startTime := time.Now()

	codebaseHash := hashPath(codebasePath)

	// Load stored file hashes for incremental update (skip unchanged files)
	storedHashes, err := idx.store.GetFileHashes(ctx, codebaseHash)
	if err != nil {
		mlog.Warningf("failed to load stored file hashes, will do full reindex: %v", err)
		storedHashes = map[string]string{}
	}

	fileMap := make(map[string]bool)
	err = filepath.Walk(codebasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if shouldSkipDir(path) {
			return filepath.SkipDir
		}

		ext := filepath.Ext(path)
		if isCodeFile(ext) {
			fileMap[path] = true
		}
		return nil
	})

	if err != nil {
		return result, fmt.Errorf("failed to walk filesystem: %w", err)
	}

	result.FilesProcessed = len(fileMap)

	var chunks []pkg.Vector
	for filePath := range fileMap {
		content, err := os.ReadFile(filePath)
		if err != nil {
			mlog.Warningf("failed to read %s: %v", filePath, err)
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}
		currentHash := hashContent(content)

		// Skip files whose content hasn't changed
		if storedHashes[filePath] == currentHash {
			result.FilesSkipped++
			continue
		}

		// Delete stale vectors for files that were previously indexed
		if _, existed := storedHashes[filePath]; existed {
			if err := idx.store.DeleteByFilePath(ctx, filePath, codebaseHash); err != nil {
				mlog.Warningf("failed to delete stale vectors for %s: %v", filePath, err)
			}
		}

		lang := getLanguageFromExt(filepath.Ext(filePath))
		fileChunks, err := idx.splitter.Split(filePath, lang)
		if err != nil {
			mlog.Warningf("failed to split %s: %v", filePath, err)
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		for i := range fileChunks {
			fileChunks[i].FileHash = currentHash
		}
		chunks = append(chunks, fileChunks...)
	}

	mlog.Infof("indexer: %d chunks to embed (batchSize=10)", len(chunks))

	// DashScope text-embedding models limit batch size to 10 per request.
	batchSize := 10
	for batchStart := 0; batchStart < len(chunks); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(chunks) {
			batchEnd = len(chunks)
		}

		batch := chunks[batchStart:batchEnd]

		// Collect texts for batch embedding
		texts := make([]string, len(batch))
		for i := range batch {
			texts[i] = batch[i].Text
		}

		mlog.V(2).Infof("indexer: embedding batch [%d, %d)", batchStart, batchEnd)

		// Get embeddings for entire batch
		embeddings, err := idx.embedding.BatchGenerateEmbedding(ctx, texts)
		if err != nil {
			mlog.Warningf("indexer: batch [%d,%d) embed failed: %v", batchStart, batchEnd, err)
			if result.LastError == "" {
				result.LastError = err.Error()
			}
			fileMap := make(map[string]bool)
			for i := range batch {
				if !fileMap[batch[i].FilePath] {
					result.FailedFiles = append(result.FailedFiles, batch[i].FilePath)
					fileMap[batch[i].FilePath] = true
				}
			}
			continue
		}

		// Store all embeddings
		for i := range batch {
			batch[i].Embedding = embeddings[i]
			batch[i].CodebaseHash = codebaseHash

			if err := idx.store.InsertVector(ctx, batch[i]); err != nil {
				mlog.Warningf("failed to insert vector for %s: %v", batch[i].FilePath, err)
				// Track files that failed during storage
				if !contains(result.FailedFiles, batch[i].FilePath) {
					result.FailedFiles = append(result.FailedFiles, batch[i].FilePath)
				}
				continue
			}

			result.VectorsStored++
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

func shouldSkipDir(path string) bool {
	skipDirs := []string{".git", "node_modules", "vendor", "__pycache__", ".idea", ".vscode", "dist", "build"}
	for _, dir := range skipDirs {
		if strings.Contains(path, "/"+dir+"/") || strings.HasSuffix(path, "/"+dir) {
			return true
		}
	}
	return false
}

func isCodeFile(ext string) bool {
	supportedExts := map[string]bool{
		".go":   true,
		".py":   true,
		".js":   true,
		".ts":   true,
		".jsx":  true,
		".tsx":  true,
		".java": true,
		".cpp":  true,
		".cc":   true,
		".cxx":  true,
		".h":    true,
		".c":    true,
		".cs":   true,
		".rs":   true,
	}
	return supportedExts[strings.ToLower(ext)]
}

func getLanguageFromExt(ext string) string {
	langMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "js",
		".ts":   "ts",
		".jsx":  "js",
		".tsx":  "ts",
		".java": "java",
		".cpp":  "cpp",
		".cc":   "cpp",
		".cxx":  "cpp",
		".h":    "cpp",
		".c":    "c",
		".cs":   "csharp",
		".rs":   "rust",
	}
	if lang, ok := langMap[strings.ToLower(ext)]; ok {
		return lang
	}
	return "unknown"
}

func hashPath(path string) string {
	hash := md5.Sum([]byte(path))
	return fmt.Sprintf("%x", hash)
}

func hashContent(data []byte) string {
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
