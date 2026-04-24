package tests

import (
	"os"
	"testing"

	"github.com/ranwei/claude-context/pkg/config"
)

func TestConfigBackendDefaults(t *testing.T) {
	os.Unsetenv("DB_BACKEND")
	os.Unsetenv("QDRANT_URL")
	os.Unsetenv("QDRANT_COLLECTION")
	os.Unsetenv("CHROMEM_PATH")

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
}

func TestConfigBackendFromEnv(t *testing.T) {
	os.Setenv("DB_BACKEND", "qdrant")
	os.Setenv("QDRANT_URL", "http://myserver:6333")
	os.Setenv("QDRANT_COLLECTION", "my-collection")
	os.Setenv("CHROMEM_PATH", "/tmp/chromem")
	defer func() {
		os.Unsetenv("DB_BACKEND")
		os.Unsetenv("QDRANT_URL")
		os.Unsetenv("QDRANT_COLLECTION")
		os.Unsetenv("CHROMEM_PATH")
	}()

	cfg := config.FromEnv()

	if cfg.DBBackend != "qdrant" {
		t.Errorf("expected DBBackend='qdrant', got %q", cfg.DBBackend)
	}
	if cfg.QdrantURL != "http://myserver:6333" {
		t.Errorf("expected QdrantURL='http://myserver:6333', got %q", cfg.QdrantURL)
	}
	if cfg.ChromemPath != "/tmp/chromem" {
		t.Errorf("expected ChromemPath='/tmp/chromem', got %q", cfg.ChromemPath)
	}
}
