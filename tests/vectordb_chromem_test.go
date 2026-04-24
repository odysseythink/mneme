package tests

import (
	"context"
	"testing"

	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func TestChromemStoreInitialize(t *testing.T) {
	store := vectordb.NewChromemStore(t.TempDir())
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()
}

func TestChromemStoreInsertAndSearch(t *testing.T) {
	store := vectordb.NewChromemStore(t.TempDir())
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

func TestChromemStoreSearchFiltersLowSimilarity(t *testing.T) {
	store := vectordb.NewChromemStore(t.TempDir())
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Insert a vector pointing in one direction
	if err := store.InsertVector(ctx, pkg.Vector{
		ID:        1,
		Embedding: []float32{1.0, 0.0, 0.0, 0.0},
		Text:      "unrelated code",
		FilePath:  "other.go",
		Language:  "go",
		StartLine: 1,
		EndLine:   1,
	}); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	// Search with an orthogonal query (similarity = 0)
	results, err := store.Search(ctx, []float32{0.0, 1.0, 0.0, 0.0}, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (orthogonal vectors filtered out), got %d", len(results))
	}
}
