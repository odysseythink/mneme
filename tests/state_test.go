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
