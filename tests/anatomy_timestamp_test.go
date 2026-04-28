package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

func TestReadAnatomyGeneratedTime(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	os.WriteFile(filepath.Join(dir, ".mneme", ".local-id"), []byte("test-ts-uuid"), 0644)

	entries := []state.AnatomyEntry{
		{Path: "main.go", Description: "entry point", EstTokens: 10, Language: "go"},
	}
	state.WriteAnatomy(dir, entries)

	before := time.Now().Add(-time.Second)
	ts, err := state.ReadAnatomyGeneratedTime(dir)
	if err != nil {
		t.Fatalf("ReadAnatomyGeneratedTime: %v", err)
	}
	if ts.Before(before) {
		t.Errorf("expected generated time >= %v, got %v", before, ts)
	}
}

func TestReadAnatomyGeneratedTimeMissing(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	_, err := state.ReadAnatomyGeneratedTime(dir)
	if err == nil {
		t.Error("expected error when anatomy.md missing, got nil")
	}
}
