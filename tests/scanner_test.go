package tests

import (
	"os"
	"testing"

	"github.com/ranwei/claude-context/pkg/scanner"
	"github.com/ranwei/claude-context/pkg/state"
)

func TestWalkFilesGitRepo(t *testing.T) {
	wd, _ := os.Getwd()
	root, ok := state.FindGitRoot(wd)
	if !ok {
		t.Skip("not running inside a git repo")
	}
	paths, err := scanner.Walk(root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(paths) == 0 {
		t.Error("expected at least one file from Walk")
	}
	for _, p := range paths {
		if len(p) == 0 {
			t.Error("Walk returned empty path")
		}
		// All paths must be relative (no leading slash)
		if p[0] == '/' {
			t.Errorf("Walk returned absolute path: %s", p)
		}
	}
}
