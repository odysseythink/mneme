// tests/bench_test.go
package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/hook"
	"github.com/ranwei/claude-context/pkg/scanner"
	"github.com/ranwei/claude-context/pkg/state"
)

func BenchmarkParseEvent(b *testing.B) {
	payload := `{"session_id":"bench-session","transcript_path":"/tmp/t","cwd":"/tmp","hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"/tmp/auth.go"},"tool_use_id":"toolu_bench"}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := strings.NewReader(payload)
		if _, err := hook.ParseEvent(r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLedgerIncrement(b *testing.B) {
	dir := b.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.IncrementSafe(dir, "hook_fired.pre-read")
	}
}

func BenchmarkScanProject(b *testing.B) {
	wd, _ := os.Getwd()
	root, ok := state.FindGitRoot(wd)
	if !ok {
		b.Skip("not running inside a git repo")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanner.ScanProject(root)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadAnatomy(b *testing.B) {
	dir := b.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	entries := make([]state.AnatomyEntry, 50)
	for i := range entries {
		entries[i] = state.AnatomyEntry{
			Path:        filepath.Join("pkg", "x", fmt.Sprintf("file%d.go", i)),
			Description: "example file description",
			EstTokens:   100,
			Language:    "go",
		}
	}
	if err := state.WriteAnatomy(dir, entries); err != nil {
		b.Fatalf("WriteAnatomy: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := state.ReadAnatomy(dir); err != nil {
			b.Fatal(err)
		}
	}
}
