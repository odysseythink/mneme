package tests

import (
	"context"
	"os"
	"testing"

	"github.com/ranwei/mneme/pkg"
	"github.com/ranwei/mneme/pkg/vectordb"
)

func TestStoreInitialize(t *testing.T) {
	dbPath := "/tmp/test_vectors.duckdb"
	defer os.Remove(dbPath)

	store := vectordb.NewStore()
	err := store.Initialize(dbPath)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("DB file was not created")
	}
}

func TestStoreInsertAndSearch(t *testing.T) {
	dbPath := "/tmp/test_vectors.duckdb"
	defer os.Remove(dbPath)

	store := vectordb.NewStore()
	store.Initialize(dbPath)
	defer store.Close()

	ctx := context.Background()

	vec := pkg.Vector{
		Embedding:    []float32{0.1, 0.2, 0.3, 0.4},
		Text:         "func hello() {}",
		FilePath:     "main.go",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		CodebaseHash: "abc123",
	}

	err := store.InsertVector(ctx, vec)
	if err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	results, err := store.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4}, 1, "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected at least 1 result")
	}

	if results[0].Text != "func hello() {}" {
		t.Fatalf("Expected text 'func hello() {}', got %s", results[0].Text)
	}
}

func TestStoreSearchReturnsEmbeddings(t *testing.T) {
	dbPath := "/tmp/test_vectors_emb.duckdb"
	defer os.Remove(dbPath)

	store := vectordb.NewStore()
	store.Initialize(dbPath)
	defer store.Close()

	ctx := context.Background()

	inserted := pkg.Vector{
		Embedding:    []float32{0.1, 0.2, 0.3, 0.4},
		Text:         "func hello() {}",
		FilePath:     "main.go",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		CodebaseHash: "abc123",
	}

	if err := store.InsertVector(ctx, inserted); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	results, err := store.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4}, 1, "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected at least 1 result")
	}
	if results[0].Embedding == nil {
		t.Fatal("Store.Search() returned nil embedding — similarity scoring will always be 0")
	}
}
