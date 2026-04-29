package tests

import (
	"os"
	"testing"

	"github.com/ranwei/claude-context/pkg/config"
)

func TestConfigBackendDefaults(t *testing.T) {
	// Save and restore any pre-existing values
	for _, key := range []string{"DB_BACKEND", "QDRANT_URL", "QDRANT_COLLECTION", "CHROMEM_PATH"} {
		key := key
		orig, exists := os.LookupEnv(key)
		os.Unsetenv(key)
		t.Cleanup(func() {
			if exists {
				os.Setenv(key, orig)
			} else {
				os.Unsetenv(key)
			}
		})
	}
	// Point CONFIG_FILE at a nonexistent path so the user's ~/.claude-context/config.yaml
	// does not interfere with built-in default assertions.
	t.Setenv("CONFIG_FILE", "/nonexistent/config.yaml")

	cfg := config.FromEnv()

	if cfg.DBBackend != "duckdb" {
		t.Errorf("expected DBBackend='duckdb', got %q", cfg.DBBackend)
	}
	if cfg.QdrantURL != "http://localhost:6333" {
		t.Errorf("expected QdrantURL='http://localhost:6333', got %q", cfg.QdrantURL)
	}
	if cfg.QdrantCollection != "claude-context" {
		t.Errorf("expected QdrantCollection='claude-context', got %q", cfg.QdrantCollection)
	}
	if cfg.ChromemPath == "" {
		t.Error("expected ChromemPath to be non-empty default")
	}
}

func TestConfigBackendFromEnv(t *testing.T) {
	t.Setenv("DB_BACKEND", "qdrant")
	t.Setenv("QDRANT_URL", "http://myserver:6333")
	t.Setenv("QDRANT_COLLECTION", "my-collection")
	t.Setenv("CHROMEM_PATH", "/tmp/chromem")

	cfg := config.FromEnv()

	if cfg.DBBackend != "qdrant" {
		t.Errorf("expected DBBackend='qdrant', got %q", cfg.DBBackend)
	}
	if cfg.QdrantURL != "http://myserver:6333" {
		t.Errorf("expected QdrantURL='http://myserver:6333', got %q", cfg.QdrantURL)
	}
	if cfg.QdrantCollection != "my-collection" {
		t.Errorf("expected QdrantCollection='my-collection', got %q", cfg.QdrantCollection)
	}
	if cfg.ChromemPath != "/tmp/chromem" {
		t.Errorf("expected ChromemPath='/tmp/chromem', got %q", cfg.ChromemPath)
	}
}
