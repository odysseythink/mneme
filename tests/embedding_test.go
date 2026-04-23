package tests

import (
	"context"
	"testing"

	"github.com/ranwei/claude-context/pkg/embedding"
)

type MockEmbeddingProvider struct {
	CallCount int
}

func (m *MockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.CallCount++
	emb := make([]float32, 1536)
	for i := range emb {
		emb[i] = 0.1
	}
	return emb, nil
}

func TestCachedClientCache(t *testing.T) {
	mockProvider := &MockEmbeddingProvider{}
	client := embedding.NewCachedClient(mockProvider)

	ctx := context.Background()

	emb1, err := client.GenerateEmbedding(ctx, "hello world")
	if err != nil {
		t.Fatalf("GenerateEmbedding failed: %v", err)
	}

	if mockProvider.CallCount != 1 {
		t.Fatalf("Expected 1 call to provider, got %d", mockProvider.CallCount)
	}

	emb2, err := client.GenerateEmbedding(ctx, "hello world")
	if err != nil {
		t.Fatalf("GenerateEmbedding failed: %v", err)
	}

	if mockProvider.CallCount != 1 {
		t.Fatalf("Expected cache hit (1 call), got %d calls", mockProvider.CallCount)
	}

	if len(emb1) != len(emb2) {
		t.Fatal("Embeddings have different lengths")
	}
}
