package tests

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestAcquireLockAndRelease(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "test.lock")

	release, err := state.AcquireLock(lp, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	release()
}

func TestAcquireLockTimeout(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "test.lock")

	release1, err := state.AcquireLock(lp, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer release1()

	// Second lock on same path should time out
	_, err = state.AcquireLock(lp, 50*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error for second lock on same path")
	}
}

func TestLedgerIncrementOnce(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	state.IncrementSafe(dir, "hook_fired.pre-read")

	l, err := state.ReadLedger(dir)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if l.Totals.HookFired["pre-read"] != 1 {
		t.Errorf("hook_fired.pre-read = %d, want 1", l.Totals.HookFired["pre-read"])
	}
}

func TestLedgerIncrementRMW(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.IncrementSafe(dir, "hook_fired.pre-read")
		}()
	}
	wg.Wait()

	l, _ := state.ReadLedger(dir)
	if l.Totals.HookFired["pre-read"] != n {
		t.Errorf("concurrent increments: got %d, want %d (lost updates)", l.Totals.HookFired["pre-read"], n)
	}
}

func TestLedgerIncrementTopLevel(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	state.IncrementSafe(dir, "hook_errors")
	state.IncrementSafe(dir, "stdin_parse_failures")

	l, _ := state.ReadLedger(dir)
	if l.Totals.HookErrors != 1 {
		t.Errorf("hook_errors = %d, want 1", l.Totals.HookErrors)
	}
	if l.Totals.StdinParseFailures != 1 {
		t.Errorf("stdin_parse_failures = %d, want 1", l.Totals.StdinParseFailures)
	}
}

func TestSessionUpsert(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	s := state.Session{SessionID: "sess-abc-123", ClaudeCodeModel: "claude-opus-4-7"}
	if err := state.UpsertSession(dir, s); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	id, _ := state.ReadOrCreateLocalID(dir)
	path := filepath.Join(state.GlobalProjectDir(id), "_session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("_session.json not found: %v", err)
	}
	if !strings.Contains(string(data), "sess-abc-123") {
		t.Errorf("session_id not in file: %s", data)
	}
}

func TestSessionJSONUpsertIdempotent(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude-context"), 0755)

	s := state.Session{SessionID: "sess-same", ClaudeCodeModel: "claude-opus-4-7"}
	if err := state.UpsertSession(dir, s); err != nil {
		t.Fatalf("first UpsertSession: %v", err)
	}
	if err := state.UpsertSession(dir, s); err != nil {
		t.Fatalf("second UpsertSession: %v", err)
	}

	id, _ := state.ReadOrCreateLocalID(dir)
	data, _ := os.ReadFile(filepath.Join(state.GlobalProjectDir(id), "_session.json"))
	if strings.Count(string(data), "sess-same") != 1 {
		t.Errorf("expected session_id exactly once, got: %s", data)
	}
}
