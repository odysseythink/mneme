//go:build duckdb

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
