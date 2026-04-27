package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/installer"
)

func TestScaffoldProject(t *testing.T) {
	dir := t.TempDir()

	id, err := installer.ScaffoldProject(dir)
	if err != nil {
		t.Fatalf("ScaffoldProject: %v", err)
	}
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d (val=%q)", len(id), id)
	}

	if _, err := os.Stat(filepath.Join(dir, ".claude-context")); err != nil {
		t.Error(".claude-context/ not created")
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".claude-context", ".gitignore"))
	if !strings.Contains(string(gi), "_session.json") {
		t.Errorf(".gitignore missing _session.json, got: %q", string(gi))
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude-context", ".local-id")); err != nil {
		t.Error(".local-id not created")
	}
}

func TestScaffoldProjectIdempotent(t *testing.T) {
	dir := t.TempDir()
	id1, err1 := installer.ScaffoldProject(dir)
	id2, err2 := installer.ScaffoldProject(dir)
	if err1 != nil || err2 != nil {
		t.Fatalf("ScaffoldProject errors: %v, %v", err1, err2)
	}
	if id1 != id2 {
		t.Errorf("re-scaffold changed ID: %q → %q", id1, id2)
	}
}
