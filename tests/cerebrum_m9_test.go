package tests

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/state"
)

func TestScanTextEnglish(t *testing.T) {
	hits := cerebrum.ScanText("you should not edit this file")
	got := phrases(hits)
	want := []string{"should not"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanTextChinese(t *testing.T) {
	hits := cerebrum.ScanText("请避免修改这个文件")
	got := phrases(hits)
	want := []string{"避免"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanTextMixedAndCaseInsensitive(t *testing.T) {
	hits := cerebrum.ScanText("Actually you should NOT do that — 不要 retry")
	got := phrases(hits)
	sort.Strings(got)
	want := []string{"actually", "should not", "不要"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanTextNoMatch(t *testing.T) {
	hits := cerebrum.ScanText("looks great, thanks")
	if len(hits) != 0 {
		t.Errorf("expected 0 hits, got %v", hits)
	}
}

func phrases(hits []cerebrum.TriggerHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Phrase
	}
	return out
}

func TestCandidateID(t *testing.T) {
	a := cerebrum.NewCandidateID("should not", "edit pkg/foo/bar.go")
	b := cerebrum.NewCandidateID("should not", "edit pkg/foo/bar.go")
	c := cerebrum.NewCandidateID("instead", "edit pkg/foo/bar.go")
	if a != b {
		t.Errorf("same inputs should produce same ID: %s vs %s", a, b)
	}
	if a == c {
		t.Errorf("different phrase should produce different ID")
	}
	if len(a) != 16 {
		t.Errorf("ID length: got %d, want 16", len(a))
	}
}

func TestSavePendingDedup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	c := cerebrum.Candidate{
		ID:        "abc123",
		Trigger:   cerebrum.Trigger{Phrase: "should not"},
		DraftRule: state.CerebrumRule{Pattern: "foo", Message: "x"},
		QueuedAt:  time.Now().UTC().Format(time.RFC3339),
		HitCount:  1,
	}

	if err := cerebrum.SavePending(root, []cerebrum.Candidate{c}); err != nil {
		t.Fatalf("save: %v", err)
	}
	c.HitCount = 1
	if err := cerebrum.AppendPending(root, c); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := cerebrum.LoadPending(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1", len(got))
	}
	if got[0].HitCount != 2 {
		t.Errorf("HitCount after re-add: got %d, want 2", got[0].HitCount)
	}
}

func TestRejectedTTLFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	if err := cerebrum.AppendRejected(root, "id-recent", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := cerebrum.AppendRejected(root, "id-old", time.Now().UTC().Add(-100*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	suppressed, err := cerebrum.LoadRejected(root, 90, time.Now().UTC())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !suppressed["id-recent"] {
		t.Errorf("id-recent should be suppressed")
	}
	if suppressed["id-old"] {
		t.Errorf("id-old (100d > 90d ttl) should not be suppressed")
	}
}

func TestLearnEnglishMatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	cs, err := cerebrum.Learn("golden/m9/transcripts/english.jsonl", root, "s1", now)
	if err != nil {
		t.Fatalf("learn: %v", err)
	}
	if len(cs) < 2 {
		t.Fatalf("expected ≥2 candidates (actually + should not), got %d", len(cs))
	}
	var foundShouldNot bool
	for _, c := range cs {
		if c.Trigger.Phrase == "should not" {
			foundShouldNot = true
			if c.DraftRule.Pattern != `pkg/foo/bar\.go` {
				t.Errorf("DraftRule.Pattern: got %q, want %q (escaped file path)",
					c.DraftRule.Pattern, `pkg/foo/bar\.go`)
			}
			if c.Trigger.Turn != 3 {
				t.Errorf("Turn: got %d, want 3", c.Trigger.Turn)
			}
		}
	}
	if !foundShouldNot {
		t.Errorf("expected 'should not' candidate, got: %+v", cs)
	}
}

func TestLearnNoMatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	cs, err := cerebrum.Learn("golden/m9/transcripts/no_match.jsonl", root, "s2", time.Now())
	if err != nil {
		t.Fatalf("learn: %v", err)
	}
	if len(cs) != 0 {
		t.Errorf("expected 0 candidates, got %d: %+v", len(cs), cs)
	}
}

func TestLearnPreemptedByExistingRule(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	state.AppendCerebrumRule(root, state.CerebrumRule{
		Comment: "manual",
		Pattern: `pkg/foo/bar\.go`,
		Message: "do not touch",
	})

	cs, err := cerebrum.Learn("golden/m9/transcripts/english.jsonl", root, "s3", time.Now())
	if err != nil {
		t.Fatalf("learn: %v", err)
	}
	for _, c := range cs {
		if c.DraftRule.Pattern == `pkg/foo/bar\.go` {
			t.Errorf("expected pre-empted candidate to be dropped: %+v", c)
		}
	}
}
