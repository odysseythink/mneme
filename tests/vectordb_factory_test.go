package tests

import (
	"testing"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/vectordb"
)

func TestFactoryDefaultsToDuckDB(t *testing.T) {
	cfg := &config.Config{DBBackend: ""}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if _, ok := store.(*vectordb.DuckDBStore); !ok {
		t.Errorf("expected *vectordb.DuckDBStore, got %T", store)
	}
}

func TestFactoryExplicitDuckDB(t *testing.T) {
	cfg := &config.Config{DBBackend: "duckdb"}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*vectordb.DuckDBStore); !ok {
		t.Errorf("expected *vectordb.DuckDBStore, got %T", store)
	}
}

func TestFactoryChromemBackend(t *testing.T) {
	cfg := &config.Config{DBBackend: "chromem", ChromemPath: t.TempDir()}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*vectordb.ChromemStore); !ok {
		t.Errorf("expected *vectordb.ChromemStore, got %T", store)
	}
}

func TestFactoryQdrantBackend(t *testing.T) {
	cfg := &config.Config{
		DBBackend:        "qdrant",
		QdrantURL:        "http://localhost:6333",
		QdrantCollection: "test",
	}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*vectordb.QdrantStore); !ok {
		t.Errorf("expected *vectordb.QdrantStore, got %T", store)
	}
}

func TestFactoryUnknownBackendReturnsError(t *testing.T) {
	cfg := &config.Config{DBBackend: "postgres"}
	_, err := vectordb.NewStoreFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for unknown backend, got nil")
	}
}
