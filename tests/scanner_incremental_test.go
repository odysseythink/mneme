package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/claude-context/pkg/scanner"
	"github.com/ranwei/claude-context/pkg/state"
)

func TestScanProjectIncrementalSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)
	os.WriteFile(filepath.Join(dir, ".claude-context", ".local-id"), []byte("test-inc-uuid"), 0644)

	// Write the file BEFORE the since timestamp
	goFile := filepath.Join(dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n"), 0644)

	// since is now (after file was written) — file should appear unchanged
	time.Sleep(10 * time.Millisecond)
	since := time.Now()

	existing := map[string]state.AnatomyEntry{
		"main.go": {Path: "main.go", Description: "cached entry", EstTokens: 3, Language: "go"},
	}

	entries, err := scanner.ScanProjectIncremental(dir, []string{"main.go"}, since, existing)
	if err != nil {
		t.Fatalf("ScanProjectIncremental: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description != "cached entry" {
		t.Errorf("expected cached description %q, got %q", "cached entry", entries[0].Description)
	}
}

func TestScanProjectIncrementalRescansChanged(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)
	os.WriteFile(filepath.Join(dir, ".claude-context", ".local-id"), []byte("test-inc-uuid2"), 0644)

	// since is in the past
	since := time.Now().Add(-10 * time.Second)

	// Write the file AFTER since — it's newer
	goFile := filepath.Join(dir, "main.go")
	os.WriteFile(goFile, []byte("package main\nfunc main() {}\n"), 0644)

	existing := map[string]state.AnatomyEntry{
		"main.go": {Path: "main.go", Description: "stale entry", EstTokens: 1, Language: "go"},
	}

	entries, err := scanner.ScanProjectIncremental(dir, []string{"main.go"}, since, existing)
	if err != nil {
		t.Fatalf("ScanProjectIncremental: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description == "stale entry" {
		t.Errorf("expected fresh extraction, still got stale cached entry")
	}
}

func TestScanProjectIncrementalNewFile(t *testing.T) {
	dir := t.TempDir()
	// since is in the past — new file not in cache must be extracted
	since := time.Now().Add(-10 * time.Second)
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0644)

	existing := map[string]state.AnatomyEntry{} // empty cache

	entries, err := scanner.ScanProjectIncremental(dir, []string{"new.go"}, since, existing)
	if err != nil {
		t.Fatalf("ScanProjectIncremental: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for new file, got %d", len(entries))
	}
}
