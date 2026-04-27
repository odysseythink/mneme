package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/hook"
	"github.com/ranwei/claude-context/pkg/state"
)

func TestReadCerebrumEmpty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)
	rules, err := state.ReadCerebrum(dir)
	if err != nil {
		t.Fatalf("ReadCerebrum on missing file: %v", err)
	}
	if rules != nil {
		t.Errorf("expected nil rules, got %v", rules)
	}
}

func TestAppendAndReadRules(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	r1 := state.CerebrumRule{Comment: "no var", Pattern: `\bvar\s+\w+\s*=`, Message: "prefer :="}
	r2 := state.CerebrumRule{Pattern: `fmt\.Println\(`, Message: "use structured logger"}

	if err := state.AppendCerebrumRule(dir, r1); err != nil {
		t.Fatalf("AppendCerebrumRule r1: %v", err)
	}
	if err := state.AppendCerebrumRule(dir, r2); err != nil {
		t.Fatalf("AppendCerebrumRule r2: %v", err)
	}

	rules, err := state.ReadCerebrum(dir)
	if err != nil {
		t.Fatalf("ReadCerebrum: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Pattern != `\bvar\s+\w+\s*=` {
		t.Errorf("rule[0].Pattern = %q", rules[0].Pattern)
	}
	if rules[0].Message != "prefer :=" {
		t.Errorf("rule[0].Message = %q", rules[0].Message)
	}
	if rules[0].Comment != "no var" {
		t.Errorf("rule[0].Comment = %q", rules[0].Comment)
	}
	if rules[1].Pattern != `fmt\.Println\(` {
		t.Errorf("rule[1].Pattern = %q", rules[1].Pattern)
	}
}

func TestWriteCerebrumRoundtrip(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	rules := []state.CerebrumRule{
		{Comment: "a", Pattern: "pat1", Message: "msg1"},
		{Comment: "b", Pattern: "pat2", Message: "msg2"},
		{Comment: "c", Pattern: "pat3", Message: "msg3"},
	}
	if err := state.WriteCerebrum(dir, rules); err != nil {
		t.Fatalf("WriteCerebrum: %v", err)
	}
	got, err := state.ReadCerebrum(dir)
	if err != nil {
		t.Fatalf("ReadCerebrum: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(got))
	}
	for i, r := range rules {
		if got[i].Pattern != r.Pattern {
			t.Errorf("rule[%d].Pattern = %q, want %q", i, got[i].Pattern, r.Pattern)
		}
		if got[i].Message != r.Message {
			t.Errorf("rule[%d].Message = %q, want %q", i, got[i].Message, r.Message)
		}
	}
}

func TestHeaderWrittenOnce(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	r := state.CerebrumRule{Pattern: "x", Message: "y"}
	state.AppendCerebrumRule(dir, r)
	state.AppendCerebrumRule(dir, r)

	data, _ := os.ReadFile(filepath.Join(dir, ".claude-context", "cerebrum.md"))
	count := strings.Count(string(data), "<!-- claude-context cerebrum v1 -->")
	if count != 1 {
		t.Errorf("header appears %d times, want 1", count)
	}
}

func TestExtractAddedLinesEdit(t *testing.T) {
	payload := `{
		"session_id": "s1",
		"transcript_path": "/tmp/t",
		"cwd": "/tmp",
		"hook_event_name": "PreToolUse",
		"tool_name": "Edit",
		"tool_input": {
			"file_path": "/tmp/foo.go",
			"old_string": "line1\nline2",
			"new_string": "line1\nline2\nnewline3\nnewline4"
		}
	}`
	ev, err := hook.ParseEvent(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	added := ev.ExtractAddedLines()
	if len(added) != 2 {
		t.Fatalf("expected 2 added lines, got %d: %v", len(added), added)
	}
	if added[3] != "newline3" {
		t.Errorf("line 3 = %q, want %q", added[3], "newline3")
	}
	if added[4] != "newline4" {
		t.Errorf("line 4 = %q, want %q", added[4], "newline4")
	}
}

func TestExtractAddedLinesWrite(t *testing.T) {
	payload := `{
		"session_id": "s1",
		"transcript_path": "/tmp/t",
		"cwd": "/tmp",
		"hook_event_name": "PreToolUse",
		"tool_name": "Write",
		"tool_input": {
			"file_path": "/tmp/new.go",
			"content": "package main\n\nfunc main() {}"
		}
	}`
	ev, _ := hook.ParseEvent(strings.NewReader(payload))
	added := ev.ExtractAddedLines()
	if len(added) != 2 {
		t.Fatalf("expected 2 added lines, got %d: %v", len(added), added)
	}
}

func TestExtractAddedLinesNoChange(t *testing.T) {
	payload := `{
		"session_id": "s1",
		"transcript_path": "/tmp/t",
		"cwd": "/tmp",
		"hook_event_name": "PreToolUse",
		"tool_name": "Edit",
		"tool_input": {
			"file_path": "/tmp/foo.go",
			"old_string": "same\ncontent",
			"new_string": "same\ncontent"
		}
	}`
	ev, _ := hook.ParseEvent(strings.NewReader(payload))
	added := ev.ExtractAddedLines()
	if len(added) != 0 {
		t.Errorf("expected 0 added lines, got %d: %v", len(added), added)
	}
}
