// tests/bench_test.go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/hook"
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
