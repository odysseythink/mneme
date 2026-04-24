package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	EmbeddingAPIKey   string
	EmbeddingProvider string
	EmbeddingModel    string
	DBPath            string
	LogLevel          string
	DBBackend         string
	QdrantURL         string
	QdrantCollection  string
	ChromemPath       string
}

func FromEnv() *Config {
	homeDir, _ := os.UserHomeDir()

	return &Config{
		EmbeddingAPIKey:   os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingProvider: getEnvOrDefault("EMBEDDING_PROVIDER", "siliconflow"),
		EmbeddingModel:    getEnvOrDefault("EMBEDDING_MODEL", "BAAI/bge-large-zh-v1.5"),
		DBPath:            expandHome(getEnvOrDefault("DB_PATH", filepath.Join(homeDir, ".claude-context", "db.duckdb")), homeDir),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
		DBBackend:         getEnvOrDefault("DB_BACKEND", "duckdb"),
		QdrantURL:         getEnvOrDefault("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection:  getEnvOrDefault("QDRANT_COLLECTION", "claude-context"),
		ChromemPath:       expandHome(getEnvOrDefault("CHROMEM_PATH", filepath.Join(homeDir, ".claude-context", "chromem")), homeDir),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path, homeDir string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	if path == "~" {
		return homeDir
	}
	return path
}
