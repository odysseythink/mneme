package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestReadBuglogEmpty(t *testing.T) {
	dir := t.TempDir()
	entries, err := state.ReadBuglog(dir)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for missing file, got %v", entries)
	}
}

func TestAppendAndReadBuglogEntries(t *testing.T) {
	dir := t.TempDir()
	e1 := state.BuglogEntry{
		Source:      "manual",
		File:        "pkg/foo.go",
		Description: "prefer := over var",
		BadCode:     "var x = someFunc()",
	}
	e2 := state.BuglogEntry{
		Source:      "auto",
		File:        "pkg/bar.go",
		Description: "possible nil deref",
		BadCode:     "session.StopCount++",
	}
	if err := state.AppendBuglogEntry(dir, e1); err != nil {
		t.Fatalf("AppendBuglogEntry e1: %v", err)
	}
	if err := state.AppendBuglogEntry(dir, e2); err != nil {
		t.Fatalf("AppendBuglogEntry e2: %v", err)
	}

	entries, err := state.ReadBuglog(dir)
	if err != nil {
		t.Fatalf("ReadBuglog: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Description != "prefer := over var" {
		t.Errorf("entries[0].Description = %q", entries[0].Description)
	}
	if entries[0].ID == "" {
		t.Errorf("expected ID to be auto-set, got empty")
	}
	if entries[0].CreatedAt == "" {
		t.Errorf("expected CreatedAt to be auto-set, got empty")
	}
	if entries[1].BadCode != "session.StopCount++" {
		t.Errorf("entries[1].BadCode = %q", entries[1].BadCode)
	}
}

func TestWriteBuglogClear(t *testing.T) {
	dir := t.TempDir()
	e := state.BuglogEntry{Source: "manual", Description: "test bug", BadCode: "bad code here now"}
	state.AppendBuglogEntry(dir, e)

	if err := state.WriteBuglog(dir, nil); err != nil {
		t.Fatalf("WriteBuglog(nil): %v", err)
	}
	entries, _ := state.ReadBuglog(dir)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(entries))
	}
	if _, err := os.Stat(filepath.Join(dir, ".mneme", "buglog.json")); err != nil {
		t.Errorf("buglog.json should exist after WriteBuglog: %v", err)
	}
}

func TestReadBuglogCorrupt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".mneme", "buglog.json")
	os.MkdirAll(filepath.Dir(p), 0755)
	os.WriteFile(p, []byte("{bad json"), 0644)

	entries, err := state.ReadBuglog(dir)
	if err != nil {
		t.Errorf("corrupt file should return nil error, got %v", err)
	}
	if entries != nil {
		t.Errorf("corrupt file should return nil entries, got %v", entries)
	}
}
