package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	EmbeddingAPIKey   string
	EmbeddingProvider string
	EmbeddingModel    string
	DBPath            string
	LogLevel          string
}

func FromEnv() *Config {
	homeDir, _ := os.UserHomeDir()

	return &Config{
		EmbeddingAPIKey:   os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingProvider: getEnvOrDefault("EMBEDDING_PROVIDER", "siliconflow"),
		EmbeddingModel:    getEnvOrDefault("EMBEDDING_MODEL", "BAAI/bge-large-zh-v1.5"),
		DBPath:            getEnvOrDefault("DB_PATH", filepath.Join(homeDir, ".claude-context", "db.duckdb")),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
