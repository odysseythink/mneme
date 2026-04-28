package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/state"
)

func TestLogger_WritesLine(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonLogsDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC) }
	lg, err := daemon.NewFileLogger(home, "info", clock)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()

	lg.Info("http", "method=GET path=/health status=200")

	path := state.DaemonLogPath(home, "20260428")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "INFO") || !strings.Contains(got, "/health") {
		t.Errorf("log content = %q, missing INFO or /health", got)
	}
}

func TestLogger_RotatesOnDateChange(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonLogsDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	cur := time.Date(2026, 4, 28, 23, 59, 59, 0, time.UTC)
	clock := func() time.Time { return cur }
	lg, err := daemon.NewFileLogger(home, "info", clock)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()

	lg.Info("test", "before-midnight")
	cur = time.Date(2026, 4, 29, 0, 0, 1, 0, time.UTC)
	lg.Info("test", "after-midnight")

	d1 := mustReadFile(t, state.DaemonLogPath(home, "20260428"))
	d2 := mustReadFile(t, state.DaemonLogPath(home, "20260429"))
	if !strings.Contains(d1, "before-midnight") {
		t.Errorf("first file missing before-midnight content: %q", d1)
	}
	if !strings.Contains(d2, "after-midnight") {
		t.Errorf("second file missing after-midnight content: %q", d2)
	}
}

func TestLogger_LevelFilter(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonLogsDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC) }
	lg, err := daemon.NewFileLogger(home, "warn", clock)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()

	lg.Debug("test", "debug-msg")
	lg.Info("test", "info-msg")
	lg.Warn("test", "warn-msg")

	got := mustReadFile(t, state.DaemonLogPath(home, "20260428"))
	if strings.Contains(got, "debug-msg") || strings.Contains(got, "info-msg") {
		t.Errorf("warn-level logger emitted lower level: %q", got)
	}
	if !strings.Contains(got, "warn-msg") {
		t.Errorf("warn-level logger dropped warn: %q", got)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	return string(data)
}
