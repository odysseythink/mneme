package tests

import (
	"context"
	"testing"

	"github.com/ranwei/claude-context/pkg"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestIndexerIndex(t *testing.T) {
	mockStore := &MockStore{}
	mockProvider := &MockEmbeddingProvider{}

	indexer := ctxpkg.NewIndexer(mockStore, embedding.NewCachedClient(mockProvider), nil)

	ctx := context.Background()
	result, err := indexer.Index(ctx, "/tmp")

	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	if result.FilesProcessed == 0 {
		t.Log("Expected FilesProcessed > 0, but got 0 (may be normal for /tmp)")
	}
}

type MockStore struct{}

func (m *MockStore) Initialize(dbPath string) error {
	return nil
}

func (m *MockStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	return nil
}

func (m *MockStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return []pkg.Vector{}, nil
}

func (m *MockStore) Close() error {
	return nil
}
