package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/claude-context/pkg/state"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := state.AtomicWrite(path, []byte("hello")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
}

func TestAtomicWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	state.AtomicWrite(path, []byte("first"))
	state.AtomicWrite(path, []byte("second"))
	data, _ := os.ReadFile(path)
	if string(data) != "second" {
		t.Errorf("got %q, want second", string(data))
	}
}

func TestAtomicWriteNoTmpResidue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	state.AtomicWrite(path, []byte("data"))

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "test.txt" {
			t.Errorf("unexpected residue file: %s", e.Name())
		}
	}
}

func TestFindGitRootInRepo(t *testing.T) {
	wd, _ := os.Getwd()
	root, ok := state.FindGitRoot(wd)
	if !ok {
		t.Skip("test not running inside a git repo")
	}
	if root == "" {
		t.Error("expected non-empty root")
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Errorf("root %q has no .git dir", root)
	}
}

func TestFindGitRootNonGit(t *testing.T) {
	dir := t.TempDir()
	_, ok := state.FindGitRoot(dir)
	if ok {
		t.Error("expected FindGitRoot to return false for non-git dir")
	}
}

func TestLocalIDGeneration(t *testing.T) {
	dir := t.TempDir()
	id, err := state.ReadOrCreateLocalID(dir)
	if err != nil {
		t.Fatalf("ReadOrCreateLocalID: %v", err)
	}
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d (val=%q)", len(id), id)
	}
}

func TestLocalIDStable(t *testing.T) {
	dir := t.TempDir()
	id1, _ := state.ReadOrCreateLocalID(dir)
	id2, _ := state.ReadOrCreateLocalID(dir)
	if id1 != id2 {
		t.Errorf("ID changed across calls: %q → %q", id1, id2)
	}
}

func TestGlobalProjectDir(t *testing.T) {
	d := state.GlobalProjectDir("test-uuid-123")
	if d == "" {
		t.Error("expected non-empty dir")
	}
	if !filepath.IsAbs(d) {
		t.Errorf("expected absolute path, got %q", d)
	}
}
