package vectordb

import (
	"fmt"

	"github.com/ranwei/mneme/pkg"
	"github.com/ranwei/mneme/pkg/config"
)

// NewStoreFromConfig selects and constructs the vector store backend from config.
// Valid DB_BACKEND values: "duckdb" (default), "qdrant", "chromem".
func NewStoreFromConfig(cfg *config.Config) (pkg.Store, error) {
	switch cfg.DBBackend {
	case "qdrant":
		return NewQdrantStore(cfg.QdrantURL, cfg.QdrantCollection), nil
	case "chromem":
		return NewChromemStore(cfg.ChromemPath), nil
	case "duckdb", "":
		return NewStore(), nil
	default:
		return nil, fmt.Errorf("unknown DB_BACKEND: %q (valid: duckdb, qdrant, chromem)", cfg.DBBackend)
	}
}
