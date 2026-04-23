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
	IndexedAt    time.Time
}

type IndexResult struct {
	FilesProcessed int
	VectorsStored  int
	Duration       time.Duration
	EmbeddingModel string
	CodebasePath   string
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
	Search(ctx context.Context, query string, topK int) ([]SearchResult, error)
}

type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

type Store interface {
	Initialize(dbPath string) error
	InsertVector(ctx context.Context, vec Vector) error
	Search(ctx context.Context, embedding []float32, topK int) ([]Vector, error)
	Close() error
}

type Splitter interface {
	Split(filePath string, language string) ([]Vector, error)
}
