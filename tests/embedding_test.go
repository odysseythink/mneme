package tests

import (
	"context"
	"testing"

	"github.com/ranwei/claude-context/pkg/embedding"
)

type MockEmbeddingProvider struct {
	CallCount      int
	BatchCallCount int
}

func (m *MockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.CallCount++
	emb := make([]float32, 1536)
	for i := range emb {
		emb[i] = 0.1
	}
	return emb, nil
}

func (m *MockEmbeddingProvider) BatchGenerateEmbedding(ctx context.Context, texts []string) ([][]float32, error) {
	m.BatchCallCount++
	result := make([][]float32, len(texts))
	for i := range texts {
		emb := make([]float32, 1536)
		for j := range emb {
			emb[j] = 0.1
		}
		result[i] = emb
	}
	return result, nil
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

func TestBatchEmbeddingCallCount(t *testing.T) {
	mockProvider := &MockEmbeddingProvider{}
	client := embedding.NewCachedClient(mockProvider)

	ctx := context.Background()

	// Call BatchGenerateEmbedding with 3 texts
	texts := []string{"hello", "world", "test"}
	embeddings, err := client.BatchGenerateEmbedding(ctx, texts)
	if err != nil {
		t.Fatalf("BatchGenerateEmbedding failed: %v", err)
	}

	// Verify the underlying provider's batch method was called once
	if mockProvider.BatchCallCount != 1 {
		t.Fatalf("Expected 1 batch call to provider, got %d", mockProvider.BatchCallCount)
	}

	// Verify all 3 embeddings are returned
	if len(embeddings) != 3 {
		t.Fatalf("Expected 3 embeddings, got %d", len(embeddings))
	}

	// Verify each embedding has content
	for i, emb := range embeddings {
		if len(emb) == 0 {
			t.Fatalf("Embedding %d is empty", i)
		}
	}
}

func TestBatchEmbeddingCaching(t *testing.T) {
	mockProvider := &MockEmbeddingProvider{}
	client := embedding.NewCachedClient(mockProvider)

	ctx := context.Background()

	texts := []string{"hello", "world", "test"}

	// First call
	embeddings1, err := client.BatchGenerateEmbedding(ctx, texts)
	if err != nil {
		t.Fatalf("First BatchGenerateEmbedding failed: %v", err)
	}

	if mockProvider.BatchCallCount != 1 {
		t.Fatalf("Expected 1 batch call after first call, got %d", mockProvider.BatchCallCount)
	}

	// Second call with same texts
	embeddings2, err := client.BatchGenerateEmbedding(ctx, texts)
	if err != nil {
		t.Fatalf("Second BatchGenerateEmbedding failed: %v", err)
	}

	// Should still have only 1 batch call (all from cache)
	if mockProvider.BatchCallCount != 1 {
		t.Fatalf("Expected 1 batch call total (cache hit), got %d", mockProvider.BatchCallCount)
	}

	// Verify results are identical
	if len(embeddings1) != len(embeddings2) {
		t.Fatal("Embedding counts differ")
	}
	for i := range embeddings1 {
		if len(embeddings1[i]) != len(embeddings2[i]) {
			t.Fatalf("Embedding %d lengths differ", i)
		}
	}
}

func TestBatchEmbeddingPartialCache(t *testing.T) {
	mockProvider := &MockEmbeddingProvider{}
	client := embedding.NewCachedClient(mockProvider)

	ctx := context.Background()

	// First call with 2 texts
	texts1 := []string{"hello", "world"}
	_, err := client.BatchGenerateEmbedding(ctx, texts1)
	if err != nil {
		t.Fatalf("First BatchGenerateEmbedding failed: %v", err)
	}

	if mockProvider.BatchCallCount != 1 {
		t.Fatalf("Expected 1 batch call, got %d", mockProvider.BatchCallCount)
	}

	// Second call with 3 texts (2 in cache, 1 new)
	texts2 := []string{"hello", "world", "test"}
	embeddings, err := client.BatchGenerateEmbedding(ctx, texts2)
	if err != nil {
		t.Fatalf("Second BatchGenerateEmbedding failed: %v", err)
	}

	// Should have 2 batch calls (only "test" was fetched)
	if mockProvider.BatchCallCount != 2 {
		t.Fatalf("Expected 2 batch calls (1 uncached text), got %d", mockProvider.BatchCallCount)
	}

	// Verify all 3 embeddings are returned
	if len(embeddings) != 3 {
		t.Fatalf("Expected 3 embeddings, got %d", len(embeddings))
	}
}
