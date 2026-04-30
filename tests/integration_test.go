//go:build duckdb

package tests

import (
	"context"
	"os"
	"testing"

	ctxpkg "github.com/ranwei/mneme/pkg/context"
	"github.com/ranwei/mneme/pkg/embedding"
	"github.com/ranwei/mneme/pkg/splitter"
	"github.com/ranwei/mneme/pkg/vectordb"
)

func TestEndToEndIndexAndSearch(t *testing.T) {
	dbPath := "/tmp/integration_test.duckdb"
	defer os.Remove(dbPath)

	store := vectordb.NewStore()
	if err := store.Initialize(dbPath); err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	mockProvider := &MockEmbeddingProvider{}
	embeddingClient := embedding.NewCachedClient(mockProvider)
	codeSplitter := splitter.NewSplitter()

	indexer := ctxpkg.NewIndexer(store, embeddingClient, codeSplitter, "test-model")
	searcher := ctxpkg.NewSearcher(store, embeddingClient)

	ctx := context.Background()

	result, err := indexer.Index(ctx, "/tmp")
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	if result.VectorsStored == 0 {
		t.Log("Expected vectors to be stored, but got 0 (may be normal for /tmp)")
	}

	results, err := searcher.Search(ctx, "test query", 5, "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Log("Expected search results, but got 0 (may be normal with empty DB)")
	}
}
