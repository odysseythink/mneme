package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestWriteAndReadOrigin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	id := "test-project-id"
	projectRoot := "/abs/path/to/project"

	if err := state.WriteOrigin(id, projectRoot); err != nil {
		t.Fatalf("WriteOrigin: %v", err)
	}

	got, err := state.ReadOrigin(id)
	if err != nil {
		t.Fatalf("ReadOrigin: %v", err)
	}
	if got != projectRoot {
		t.Errorf("ReadOrigin = %q, want %q", got, projectRoot)
	}

	path := filepath.Join(home, ".mneme", "projects", id, "origin")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("origin file not at expected path: %v", err)
	}
}

func TestReadOriginMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := state.ReadOrigin("does-not-exist")
	if err == nil {
		t.Errorf("expected error for missing origin, got nil; value=%q", got)
	}
}

func TestListOriginsEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := state.ListOrigins()
	if err != nil {
		t.Fatalf("ListOrigins on empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestListOriginsMultiple(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	state.WriteOrigin("id-a", "/path/a")
	state.WriteOrigin("id-b", "/path/b")
	state.WriteOrigin("id-c", "/path/c")

	got, err := state.ListOrigins()
	if err != nil {
		t.Fatalf("ListOrigins: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 origins, got %d", len(got))
	}
	if got["id-a"] != "/path/a" || got["id-b"] != "/path/b" || got["id-c"] != "/path/c" {
		t.Errorf("unexpected map content: %v", got)
	}
}
