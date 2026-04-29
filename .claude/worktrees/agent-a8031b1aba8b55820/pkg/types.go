package pkg

import (
	"context"
	"time"
)

type Vector struct {
	ID           int64
	Embedding    []float32
	Text         string
	FilePath     string
	Language     string
	StartLine    int
	EndLine      int
	CodebaseHash string
	FileHash     string // MD5 of the source file content, used for incremental updates
	IndexedAt    time.Time
}

type IndexResult struct {
	FilesProcessed int
	FilesSkipped   int // files skipped because content hash is unchanged
	VectorsStored  int
	Duration       time.Duration
	EmbeddingModel string
	CodebasePath   string
	FailedFiles    []string
	LastError      string // first embedding/storage error seen during indexing, for diagnostics
}

type SearchResult struct {
	FileLocation string
	Language     string
	Code         string
	Similarity   float32
}

type Indexer interface {
	Index(ctx context.Context, codebasePath string) (IndexResult, error)
}

type Searcher interface {
	Search(ctx context.Context, query string, topK int, codebasePath string) ([]SearchResult, error)
}

type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	BatchGenerateEmbedding(ctx context.Context, texts []string) ([][]float32, error)
}

type Store interface {
	Initialize(dbPath string) error
	InsertVector(ctx context.Context, vec Vector) error
	// Search queries vectors. codebaseHash="" searches all indexed codebases.
	Search(ctx context.Context, embedding []float32, topK int, codebaseHash string) ([]Vector, error)
	Close() error
	// GetFileHashes returns filePath→fileHash for all files indexed under the given codebaseHash.
	GetFileHashes(ctx context.Context, codebaseHash string) (map[string]string, error)
	// DeleteByFilePath removes all vectors for the given file within the given codebase.
	DeleteByFilePath(ctx context.Context, filePath string, codebaseHash string) error
}

type Splitter interface {
	Split(filePath string, language string) ([]Vector, error)
}
