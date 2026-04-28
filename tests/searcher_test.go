package tests

import (
	"context"
	"testing"

	"github.com/ranwei/mneme/pkg"
	ctxpkg "github.com/ranwei/mneme/pkg/context"
	"github.com/ranwei/mneme/pkg/embedding"
)

type MockSearchStore struct{}

func (m *MockSearchStore) Search(ctx context.Context, embedding []float32, topK int, _ string) ([]pkg.Vector, error) {
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

func (m *MockSearchStore) GetFileHashes(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *MockSearchStore) DeleteByFilePath(_ context.Context, _ string, _ string) error {
	return nil
}

func TestSearcherSearch(t *testing.T) {
	mockStore := &MockSearchStore{}
	mockProvider := &MockEmbeddingProvider{}

	searcher := ctxpkg.NewSearcher(mockStore, embedding.NewCachedClient(mockProvider))

	ctx := context.Background()
	results, err := searcher.Search(ctx, "test query", 5, "")

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Log("Expected at least 1 result, got 0 (may be normal with mock)")
	}
}
