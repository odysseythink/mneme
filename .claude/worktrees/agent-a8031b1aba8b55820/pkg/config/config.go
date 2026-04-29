package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	ConfigSource      string // path to the config file that was loaded, or "" if none
}

// fileConfig mirrors Config but uses yaml tags and pointer fields so we can
// distinguish "not set in file" from "explicitly set to empty string".
type fileConfig struct {
	EmbeddingAPIKey   *string `yaml:"embedding_api_key"`
	EmbeddingProvider *string `yaml:"embedding_provider"`
	EmbeddingModel    *string `yaml:"embedding_model"`
	DBPath            *string `yaml:"db_path"`
	LogLevel          *string `yaml:"log_level"`
	DBBackend         *string `yaml:"db_backend"`
	QdrantURL         *string `yaml:"qdrant_url"`
	QdrantCollection  *string `yaml:"qdrant_collection"`
	ChromemPath       *string `yaml:"chromem_path"`
}

// FromEnv builds Config with priority: env var > config file > built-in default.
// The config file path is read from CONFIG_FILE env var, defaulting to
// ~/.claude-context/config.yaml. Missing file is silently ignored.
func FromEnv() *Config {
	homeDir, _ := os.UserHomeDir()

	defaultConfigPath := filepath.Join(homeDir, ".claude-context", "config.yaml")
	configPath := expandHome(getEnvOrDefault("CONFIG_FILE", defaultConfigPath), homeDir)

	file, fileLoaded := loadFileConfig(configPath)

	configSource := ""
	if fileLoaded {
		configSource = configPath
	}

	return &Config{
		EmbeddingAPIKey:   resolve(os.Getenv("EMBEDDING_API_KEY"), file.EmbeddingAPIKey, ""),
		EmbeddingProvider: resolve(os.Getenv("EMBEDDING_PROVIDER"), file.EmbeddingProvider, "siliconflow"),
		EmbeddingModel:    resolve(os.Getenv("EMBEDDING_MODEL"), file.EmbeddingModel, "BAAI/bge-large-zh-v1.5"),
		DBPath:            expandHome(resolve(os.Getenv("DB_PATH"), file.DBPath, filepath.Join(homeDir, ".claude-context", "db.duckdb")), homeDir),
		LogLevel:          resolve(os.Getenv("LOG_LEVEL"), file.LogLevel, "info"),
		DBBackend:         resolve(os.Getenv("DB_BACKEND"), file.DBBackend, "duckdb"),
		QdrantURL:         resolve(os.Getenv("QDRANT_URL"), file.QdrantURL, "http://localhost:6333"),
		QdrantCollection:  resolve(os.Getenv("QDRANT_COLLECTION"), file.QdrantCollection, "claude-context"),
		ChromemPath:       expandHome(resolve(os.Getenv("CHROMEM_PATH"), file.ChromemPath, filepath.Join(homeDir, ".claude-context", "chromem")), homeDir),
		ConfigSource:      configSource,
	}
}

// resolve returns the first non-empty value in priority order:
// env var → file value → built-in default.
func resolve(envVal string, fileVal *string, defaultVal string) string {
	if envVal != "" {
		return envVal
	}
	if fileVal != nil && *fileVal != "" {
		return *fileVal
	}
	return defaultVal
}

// loadFileConfig reads and parses the YAML config file.
// Returns (config, true) on success, (empty, false) if file is missing or malformed.
func loadFileConfig(path string) (fileConfig, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, false // missing file is not an error
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		fmt.Fprintf(os.Stderr, "claude-context: warning: malformed config file %s: %v (using defaults)\n", path, err)
		return fileConfig{}, false
	}
	return fc, true
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
