package vectordb

import (
	"fmt"

	"github.com/ranwei/mneme/pkg"
	"github.com/ranwei/mneme/pkg/config"
)

// NewStoreFromConfig selects and constructs the vector store backend from config.
// Valid DB_BACKEND values: "chromem" (default), "qdrant", "duckdb" (requires -tags duckdb).
func NewStoreFromConfig(cfg *config.Config) (pkg.Store, error) {
	switch cfg.DBBackend {
	case "qdrant":
		return NewQdrantStore(cfg.QdrantURL, cfg.QdrantCollection), nil
	case "chromem", "":
		return NewChromemStore(cfg.ChromemPath), nil
	case "duckdb":
		s := NewStore()
		if s == nil {
			return nil, fmt.Errorf("duckdb backend not compiled; rebuild with -tags duckdb")
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown DB_BACKEND: %q (valid: duckdb, qdrant, chromem)", cfg.DBBackend)
	}
}
