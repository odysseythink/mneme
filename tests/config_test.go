package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/config"
)

func TestConfigFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
embedding_provider: qwen
embedding_model: text-embedding-v4
db_backend: chromem
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Clear relevant env vars, point CONFIG_FILE at temp file
	for _, k := range []string{"EMBEDDING_PROVIDER", "EMBEDDING_MODEL", "DB_BACKEND", "EMBEDDING_API_KEY"} {
		t.Setenv(k, "")
	}
	t.Setenv("CONFIG_FILE", cfgPath)

	cfg := config.FromEnv()

	if cfg.EmbeddingProvider != "qwen" {
		t.Errorf("EmbeddingProvider: got %q, want %q", cfg.EmbeddingProvider, "qwen")
	}
	if cfg.EmbeddingModel != "text-embedding-v4" {
		t.Errorf("EmbeddingModel: got %q, want %q", cfg.EmbeddingModel, "text-embedding-v4")
	}
	if cfg.DBBackend != "chromem" {
		t.Errorf("DBBackend: got %q, want %q", cfg.DBBackend, "chromem")
	}
}

func TestEnvOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
embedding_provider: siliconflow
embedding_model: BAAI/bge-large-zh-v1.5
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_FILE", cfgPath)
	t.Setenv("EMBEDDING_PROVIDER", "qwen")
	t.Setenv("EMBEDDING_MODEL", "text-embedding-v4")

	cfg := config.FromEnv()

	if cfg.EmbeddingProvider != "qwen" {
		t.Errorf("EmbeddingProvider: got %q, want %q (env should win)", cfg.EmbeddingProvider, "qwen")
	}
	if cfg.EmbeddingModel != "text-embedding-v4" {
		t.Errorf("EmbeddingModel: got %q, want %q (env should win)", cfg.EmbeddingModel, "text-embedding-v4")
	}
}

func TestMissingConfigFileUsesDefaults(t *testing.T) {
	t.Setenv("CONFIG_FILE", "/nonexistent/path/config.yaml")
	for _, k := range []string{"EMBEDDING_PROVIDER", "EMBEDDING_MODEL", "DB_BACKEND"} {
		t.Setenv(k, "")
	}

	cfg := config.FromEnv()

	if cfg.EmbeddingProvider != "siliconflow" {
		t.Errorf("EmbeddingProvider: got %q, want %q (built-in default)", cfg.EmbeddingProvider, "siliconflow")
	}
	if cfg.DBBackend != "duckdb" {
		t.Errorf("DBBackend: got %q, want %q (built-in default)", cfg.DBBackend, "duckdb")
	}
}

func TestFromEnv_DaemonDefaults(t *testing.T) {
	t.Setenv("CONFIG_FILE", "/nonexistent/file")
	t.Setenv("DAEMON_CRON_ENABLED", "")
	t.Setenv("DAEMON_DASHBOARD_PORT", "")
	t.Setenv("DAEMON_DASHBOARD_PORT_ENABLED", "")
	t.Setenv("DAEMON_SHUTDOWN_TIMEOUT_SECONDS", "")
	t.Setenv("DAEMON_LOG_RETENTION_DAYS", "")

	cfg := config.FromEnv()

	if !cfg.DaemonCronEnabled {
		t.Errorf("DaemonCronEnabled = false, want true (default)")
	}
	if cfg.DaemonDashboardPort != 18801 {
		t.Errorf("DaemonDashboardPort = %d, want 18801 (default)", cfg.DaemonDashboardPort)
	}
	if cfg.DaemonDashboardPortEnabled {
		t.Errorf("DaemonDashboardPortEnabled = true, want false (default)")
	}
	if cfg.DaemonShutdownTimeoutSeconds != 10 {
		t.Errorf("DaemonShutdownTimeoutSeconds = %d, want 10 (default)", cfg.DaemonShutdownTimeoutSeconds)
	}
	if cfg.DaemonLogRetentionDays != 14 {
		t.Errorf("DaemonLogRetentionDays = %d, want 14 (default)", cfg.DaemonLogRetentionDays)
	}
}

func TestFromEnv_DaemonEnvOverride(t *testing.T) {
	t.Setenv("CONFIG_FILE", "/nonexistent/file")
	t.Setenv("DAEMON_CRON_ENABLED", "false")
	t.Setenv("DAEMON_DASHBOARD_PORT", "19999")
	t.Setenv("DAEMON_DASHBOARD_PORT_ENABLED", "true")
	t.Setenv("DAEMON_SHUTDOWN_TIMEOUT_SECONDS", "30")
	t.Setenv("DAEMON_LOG_RETENTION_DAYS", "7")

	cfg := config.FromEnv()

	if cfg.DaemonCronEnabled {
		t.Errorf("DaemonCronEnabled = true, want false")
	}
	if cfg.DaemonDashboardPort != 19999 {
		t.Errorf("DaemonDashboardPort = %d, want 19999", cfg.DaemonDashboardPort)
	}
	if !cfg.DaemonDashboardPortEnabled {
		t.Errorf("DaemonDashboardPortEnabled = false, want true")
	}
	if cfg.DaemonShutdownTimeoutSeconds != 30 {
		t.Errorf("DaemonShutdownTimeoutSeconds = %d, want 30", cfg.DaemonShutdownTimeoutSeconds)
	}
	if cfg.DaemonLogRetentionDays != 7 {
		t.Errorf("DaemonLogRetentionDays = %d, want 7", cfg.DaemonLogRetentionDays)
	}
}
