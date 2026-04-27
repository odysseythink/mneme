package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
