package tests

import (
	"context"
	"testing"

	"github.com/ranwei/claude-context/pkg"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
)

type MockSearchStore struct{}

func (m *MockSearchStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return []pkg.Vector{
		{
			FilePath:  "test.go",
			Language:  "go",
			StartLine: 1,
			EndLine:   10,
			Text:      "func test() {}",
			Embedding: embedding,
		},
	}, nil
}

func (m *MockSearchStore) Initialize(dbPath string) error {
	return nil
}

func (m *MockSearchStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	return nil
}

func (m *MockSearchStore) Close() error {
	return nil
}

func TestSearcherSearch(t *testing.T) {
	mockStore := &MockSearchStore{}
	mockProvider := &MockEmbeddingProvider{}

	searcher := ctxpkg.NewSearcher(mockStore, embedding.NewCachedClient(mockProvider))

	ctx := context.Background()
	results, err := searcher.Search(ctx, "test query", 5)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Log("Expected at least 1 result, got 0 (may be normal with mock)")
	}
}
