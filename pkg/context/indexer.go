package context

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/splitter"
)

type Indexer struct {
	store          pkg.Store
	embedding      *embedding.CachedClient
	splitter       *splitter.ASTSplitter
	embeddingModel string
}

func NewIndexer(store pkg.Store, emb *embedding.CachedClient, s *splitter.ASTSplitter) *Indexer {
	if s == nil {
		s = splitter.NewSplitter()
	}
	return &Indexer{
		store:          store,
		embedding:      emb,
		splitter:       s,
		embeddingModel: "BAAI/bge-large-zh-v1.5",
	}
}

func (idx *Indexer) Index(ctx context.Context, codebasePath string) (pkg.IndexResult, error) {
	result := pkg.IndexResult{
		CodebasePath:   codebasePath,
		EmbeddingModel: idx.embeddingModel,
	}

	startTime := time.Now()

	codebaseHash := hashPath(codebasePath)

	fileMap := make(map[string]bool)
	err := filepath.Walk(codebasePath, func(path string, info os.FileInfo, err error) error {
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
		lang := getLanguageFromExt(filepath.Ext(filePath))

		fileChunks, err := idx.splitter.Split(filePath, lang)
		if err != nil {
			fmt.Printf("Warning: failed to split %s: %v\n", filePath, err)
			continue
		}

		chunks = append(chunks, fileChunks...)
	}

	for i := range chunks {
		emb, err := idx.embedding.GenerateEmbedding(ctx, chunks[i].Text)
		if err != nil {
			fmt.Printf("Warning: failed to embed chunk: %v\n", err)
			continue
		}

		chunks[i].Embedding = emb
		chunks[i].CodebaseHash = codebaseHash

		if err := idx.store.InsertVector(ctx, chunks[i]); err != nil {
			fmt.Printf("Warning: failed to insert vector: %v\n", err)
			continue
		}

		result.VectorsStored++
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
		".c":    "c",
		".cs":   "cs",
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
