//go:build qdrant_integration

package tests

import (
	"context"
	"os"
	"testing"

	"github.com/ranwei/mneme/pkg"
	"github.com/ranwei/mneme/pkg/vectordb"
)

func qdrantURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("QDRANT_URL")
	if url == "" {
		t.Skip("QDRANT_URL not set; skipping Qdrant integration tests")
	}
	return url
}

func TestQdrantStoreInitialize(t *testing.T) {
	store := vectordb.NewQdrantStore(qdrantURL(t), "test-init")
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()
}

func TestQdrantStoreInsertAndSearch(t *testing.T) {
	store := vectordb.NewQdrantStore(qdrantURL(t), "test-search")
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	emb := []float32{0.1, 0.2, 0.3, 0.4}

	vec := pkg.Vector{
		ID:           1,
		Embedding:    emb,
		Text:         "func hello() {}",
		FilePath:     "main.go",
		Language:     "go",
		StartLine:    1,
		EndLine:      3,
		CodebaseHash: "abc123",
	}

	if err := store.InsertVector(ctx, vec); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	results, err := store.Search(ctx, emb, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}
	if results[0].Text != "func hello() {}" {
		t.Errorf("expected text 'func hello() {}', got %q", results[0].Text)
	}
	if results[0].Embedding == nil {
		t.Error("expected Embedding to be populated, got nil")
	}
}

func TestQdrantStoreSearchFiltersLowSimilarity(t *testing.T) {
	store := vectordb.NewQdrantStore(qdrantURL(t), "test-filter")
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	if err := store.InsertVector(ctx, pkg.Vector{
		ID:        1,
		Embedding: []float32{1.0, 0.0, 0.0, 0.0},
		Text:      "unrelated",
		FilePath:  "other.go",
		Language:  "go",
		StartLine: 1,
		EndLine:   1,
	}); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	results, err := store.Search(ctx, []float32{0.0, 1.0, 0.0, 0.0}, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (orthogonal vectors filtered out), got %d", len(results))
	}
}
