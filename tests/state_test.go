package tests

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
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
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

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
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

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
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

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
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

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
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

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

func TestWriteReadAnatomy(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	entries := []state.AnatomyEntry{
		{Path: "main.go", Description: "entry point", EstTokens: 20, Language: "go"},
		{Path: "pkg/foo.go", Description: "foo package", EstTokens: 15, Language: "go"},
		{Path: "README.md", Description: "Project docs", EstTokens: 50, Language: "markdown"},
	}
	if err := state.WriteAnatomy(dir, entries); err != nil {
		t.Fatalf("WriteAnatomy: %v", err)
	}

	got, err := state.ReadAnatomy(dir)
	if err != nil {
		t.Fatalf("ReadAnatomy: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got["main.go"].Description != "entry point" {
		t.Errorf("main.go Description = %q", got["main.go"].Description)
	}
	if got["main.go"].Language != "go" {
		t.Errorf("main.go Language = %q", got["main.go"].Language)
	}
	if got["main.go"].EstTokens != 20 {
		t.Errorf("main.go EstTokens = %d, want 20", got["main.go"].EstTokens)
	}
	if got["pkg/foo.go"].Description != "foo package" {
		t.Errorf("pkg/foo.go Description = %q", got["pkg/foo.go"].Description)
	}
	if got["README.md"].Description != "Project docs" {
		t.Errorf("README.md Description = %q", got["README.md"].Description)
	}
}

func TestReadAnatomyMissing(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	got, err := state.ReadAnatomy(dir)
	if err != nil {
		t.Fatalf("ReadAnatomy on missing file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestAnatomyGolden(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	entries := []state.AnatomyEntry{
		{Path: "main.go", Description: "entry point", EstTokens: 20, Language: "go"},
		{Path: "cmd/sub.go", Description: "sub command", EstTokens: 15, Language: "go"},
		{Path: "README.md", Description: "Project documentation", EstTokens: 100, Language: "markdown"},
		{Path: "requirements.txt", Description: "Python dependencies", EstTokens: 10, Language: "config"},
	}
	if err := state.WriteAnatomy(dir, entries); err != nil {
		t.Fatalf("WriteAnatomy: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".mneme", "anatomy.md"))
	if err != nil {
		t.Fatalf("read anatomy.md: %v", err)
	}

	// Strip dynamic "<!-- generated: ... -->" line before comparing.
	var filtered []string
	for _, l := range strings.Split(string(got), "\n") {
		if strings.HasPrefix(l, "<!-- generated:") {
			continue
		}
		filtered = append(filtered, l)
	}
	actual := strings.Join(filtered, "\n")

	golden, err := os.ReadFile(filepath.Join("golden", "anatomy_sample.md"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if actual != string(golden) {
		t.Errorf("anatomy output mismatch\ngot:\n%s\nwant:\n%s", actual, string(golden))
	}
}

func TestAppendSessionReadFirstTime(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	s := state.Session{SessionID: "sess-reads-1", ClaudeCodeModel: "claude-opus-4-7"}
	if err := state.UpsertSession(dir, s); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	alreadyRead, err := state.AppendSessionRead(dir, "pkg/foo.go")
	if err != nil {
		t.Fatalf("AppendSessionRead: %v", err)
	}
	if alreadyRead {
		t.Error("expected alreadyRead=false on first read")
	}

	id, _ := state.ReadOrCreateLocalID(dir)
	data, _ := os.ReadFile(filepath.Join(state.GlobalProjectDir(id), "_session.json"))
	if !strings.Contains(string(data), "pkg/foo.go") {
		t.Errorf("path not persisted in _session.json: %s", data)
	}
}

func TestAppendSessionReadSecondTime(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	state.UpsertSession(dir, state.Session{SessionID: "sess-reads-2"})

	if _, err := state.AppendSessionRead(dir, "main.go"); err != nil {
		t.Fatalf("first AppendSessionRead: %v", err)
	}
	alreadyRead, err := state.AppendSessionRead(dir, "main.go")
	if err != nil {
		t.Fatalf("second AppendSessionRead: %v", err)
	}
	if !alreadyRead {
		t.Error("expected alreadyRead=true on second read")
	}

	id, _ := state.ReadOrCreateLocalID(dir)
	data, _ := os.ReadFile(filepath.Join(state.GlobalProjectDir(id), "_session.json"))
	if strings.Count(string(data), "main.go") != 1 {
		t.Errorf("expected path exactly once in session JSON, got: %s", data)
	}
}

func TestAppendSessionReadNewSession(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	state.UpsertSession(dir, state.Session{SessionID: "sess-A"})
	state.AppendSessionRead(dir, "foo.go")

	state.UpsertSession(dir, state.Session{SessionID: "sess-B"})

	alreadyRead, err := state.AppendSessionRead(dir, "foo.go")
	if err != nil {
		t.Fatalf("AppendSessionRead after new session: %v", err)
	}
	if alreadyRead {
		t.Error("expected alreadyRead=false after new session started")
	}
}

func TestLedgerIncrementNewCounters(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	state.IncrementSafe(dir, "anatomy_hits")
	state.IncrementSafe(dir, "anatomy_hits")
	state.IncrementSafe(dir, "repeat_reads")
	state.IncrementSafe(dir, "scan_count")

	l, err := state.ReadLedger(dir)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if l.Totals.AnatomyHits != 2 {
		t.Errorf("anatomy_hits = %d, want 2", l.Totals.AnatomyHits)
	}
	if l.Totals.RepeatReads != 1 {
		t.Errorf("repeat_reads = %d, want 1", l.Totals.RepeatReads)
	}
	if l.Totals.ScanCount != 1 {
		t.Errorf("scan_count = %d, want 1", l.Totals.ScanCount)
	}
}

func TestReadSessionNone(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	s, err := state.ReadSession(dir)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if s != nil {
		t.Errorf("expected nil session, got %+v", s)
	}
}

func TestAppendTurnEditNoSession(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	err := state.AppendTurnEdit(dir, state.TurnEdit{File: "foo.go", Category: "feature", LineDelta: 10})
	if err != nil {
		t.Errorf("AppendTurnEdit with no session should not error: %v", err)
	}
}

func TestAppendTurnEditAndRead(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	if err := state.UpsertSession(dir, state.Session{SessionID: "sess-1", ClaudeCodeModel: "claude-opus-4-7"}); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	edit := state.TurnEdit{File: "pkg/foo.go", Category: "feature", LineDelta: 20}
	if err := state.AppendTurnEdit(dir, edit); err != nil {
		t.Fatalf("AppendTurnEdit: %v", err)
	}
	s, err := state.ReadSession(dir)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if len(s.TurnEdits) != 1 {
		t.Fatalf("expected 1 TurnEdit, got %d", len(s.TurnEdits))
	}
	if s.TurnEdits[0].File != "pkg/foo.go" {
		t.Errorf("file = %q, want %q", s.TurnEdits[0].File, "pkg/foo.go")
	}
}

func TestAggregateTurn(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	state.UpsertSession(dir, state.Session{SessionID: "sess-2", ClaudeCodeModel: "claude-opus-4-7"})
	state.AppendTurnEdit(dir, state.TurnEdit{File: "a.go", Category: "bugfix", LineDelta: 3})
	state.AppendTurnEdit(dir, state.TurnEdit{File: "b.go", Category: "test", LineDelta: 5})

	if err := state.AggregateTurn(dir); err != nil {
		t.Fatalf("AggregateTurn: %v", err)
	}

	s, _ := state.ReadSession(dir)
	if len(s.TurnEdits) != 0 {
		t.Errorf("TurnEdits should be cleared after aggregate, got %d", len(s.TurnEdits))
	}
	if len(s.TurnSummaries) != 1 {
		t.Fatalf("expected 1 TurnSummary, got %d", len(s.TurnSummaries))
	}
	if len(s.TurnSummaries[0].Edits) != 2 {
		t.Errorf("expected 2 edits in summary, got %d", len(s.TurnSummaries[0].Edits))
	}
	if s.StopCount != 1 {
		t.Errorf("StopCount = %d, want 1", s.StopCount)
	}
}

func TestLedgerEditPattern(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	state.IncrementSafe(dir, "edit_pattern.feature")
	state.IncrementSafe(dir, "edit_pattern.feature")
	state.IncrementSafe(dir, "edit_pattern.bugfix")

	l, err := state.ReadLedger(dir)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if l.Totals.EditPatterns["feature"] != 2 {
		t.Errorf("feature = %d, want 2", l.Totals.EditPatterns["feature"])
	}
	if l.Totals.EditPatterns["bugfix"] != 1 {
		t.Errorf("bugfix = %d, want 1", l.Totals.EditPatterns["bugfix"])
	}
}

func TestAppendMemoryRowCreatesHeader(t *testing.T) {
	home := t.TempDir()
	row := state.MemoryRow{
		StartedAt:     "2026-04-27T14:32:00Z",
		TurnCount:     3,
		FileSummary:   []state.FileStat{{File: "auth.go", Categories: []string{"bugfix×1"}}},
		PatternCounts: map[string]int{"bugfix": 1},
		Summary:       "Fixed 1 bug.",
	}
	if err := state.AppendMemoryRow(home, row); err != nil {
		t.Fatalf("AppendMemoryRow: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".claude", "mneme-memory.md"))
	if err != nil {
		t.Fatalf("memory.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "<!-- mneme memory v1 -->") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "## 2026-04-27T14:32:00Z (3 turns)") {
		t.Error("missing session header line")
	}
	if !strings.Contains(content, "Patterns: bugfix×1") {
		t.Error("missing patterns line")
	}
	if !strings.Contains(content, "Summary: Fixed 1 bug.") {
		t.Error("missing summary line")
	}
}

func TestAppendMemoryRowHeaderOnce(t *testing.T) {
	home := t.TempDir()
	row := state.MemoryRow{StartedAt: "2026-04-27T10:00:00Z", TurnCount: 1, PatternCounts: map[string]int{}}
	state.AppendMemoryRow(home, row)
	state.AppendMemoryRow(home, row)

	data, _ := os.ReadFile(filepath.Join(home, ".claude", "mneme-memory.md"))
	count := strings.Count(string(data), "<!-- mneme memory v1 -->")
	if count != 1 {
		t.Errorf("header appears %d times, want 1", count)
	}
}

func TestReadMemoryParsesRows(t *testing.T) {
	home := t.TempDir()
	rows := []state.MemoryRow{
		{StartedAt: "2026-04-27T10:00:00Z", TurnCount: 5, PatternCounts: map[string]int{"feature": 2, "bugfix": 1}},
		{StartedAt: "2026-04-27T14:00:00Z", TurnCount: 3, PatternCounts: map[string]int{"refactor": 1}},
	}
	for _, r := range rows {
		state.AppendMemoryRow(home, r)
	}

	got, err := state.ReadMemory(home)
	if err != nil {
		t.Fatalf("ReadMemory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0].TurnCount != 5 {
		t.Errorf("row[0].TurnCount = %d, want 5", got[0].TurnCount)
	}
	if got[1].PatternCounts["refactor"] != 1 {
		t.Errorf("row[1] refactor = %d, want 1", got[1].PatternCounts["refactor"])
	}
}

func TestReadMemoryEmpty(t *testing.T) {
	home := t.TempDir()
	rows, err := state.ReadMemory(home)
	if err != nil {
		t.Fatalf("ReadMemory on missing file: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows for missing file, got %v", rows)
	}
}
