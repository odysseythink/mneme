# M9 — Intelligence Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the four sub-systems specified in `docs/superpowers/specs/2026-04-28-m9-intelligence-loop-design.md` — cerebrum learner (transcript-driven, human-reviewed), weekly waste report, suggestions engine with TTL dismissal, and the envcheck library.

**Architecture:** Four independent `pkg/` packages, each library-callable. Stop hook posts to the M8 daemon's `/cerebrum/learn` endpoint and falls back to in-process; daemon cron runs the weekly waste report and the daily suggestions refresh.

**Tech Stack:** Go 1.25.7, `github.com/gofrs/flock`, `github.com/robfig/cron/v3` (via M8), `gopkg.in/yaml.v3`. Testing via Go's `testing` plus golden fixtures under `tests/golden/m9/`.

**Hard prerequisite — M8 daemon must ship before M9.** The cerebrum-learn endpoint, the three new cron tasks, and the route registration all touch M8 types (`daemon.RouteDeps`, `daemon.Manifest`, `daemon.LookupTask`, `daemon.Scheduler`). If M8 has not shipped, Tasks 8, 12, and 17 cannot complete; the rest of the plan still works (CLI surfaces and library packages are M8-independent).

**Soft prerequisite — M7 origin file convention.** The daemon-side per-project workers need `~/.mneme/projects/<id>/origin` to resolve project-id → project root. Without M7, the workers degrade gracefully: they fall back to using `state.GlobalProjectDir(id)` as the project root for the cerebrum-pending writes (sufficient for tests, surfaces as a degraded run in production until M7 lands). Task 8 implements this stub resolver.

---

## File Structure

| Path | Type | Purpose |
|---|---|---|
| `pkg/config/config.go` | modify | Add `CerebrumLearningEnabled`, `WasteThresholdPercent`, `SuggestionsDismissTTLDays`, `CerebrumRejectTTLDays` fields + env + yaml |
| `pkg/state/ledger_history.go` | create | `LedgerSnapshot`, `AppendLedgerHistory`, `ReadLedgerHistory`, 1 MiB rolling trim |
| `cmd/hook_stop.go` | modify | Append `LedgerSnapshot` after `IncrementSafe`; POST `/cerebrum/learn` with in-process fallback |
| `pkg/cerebrum/triggers.go` | create | Hardcoded English + Chinese keyword list; `ScanText` returns matched phrases with offsets |
| `pkg/cerebrum/pending.go` | create | `Candidate` type, ID hash, `LoadPending`/`SavePending`/`LoadRejected`/`AppendRejected` (lock-protected) |
| `pkg/cerebrum/learner.go` | create | Transcript JSONL parser, `Learn(transcriptPath, projectRoot, sessionID, now)`, dedup, pre-emption, reject-TTL filter |
| `cmd/cmd_cerebrum.go` | modify | Add `cerebrum review` interactive + `--list --json` |
| `pkg/daemon/cerebrum_handler.go` | create | `POST /cerebrum/learn` handler, project-id resolver, in-memory queue, worker loop |
| `pkg/daemon/routes.go` | modify (M8 file) | Register `/cerebrum/learn` |
| `pkg/daemon/tasks.go` | modify (M8 file) | Add task wrappers for `weekly-waste-report` and `suggestions-refresh` to registry |
| `pkg/daemon/manifest.go` | modify (M8 file) | Seed manifest with the two new cron entries |
| `pkg/waste/report.go` | create | `ReportInput`, `ReportOutput`, `GenerateReport`, `RenderMarkdown`, `WriteReport` |
| `cmd/cmd_report.go` | create | `mneme report waste` CLI |
| `pkg/suggestions/dismissed.go` | create | `Dismissed` record, reader/writer, TTL filter |
| `pkg/suggestions/generators.go` | create | 4 generators: `stale_anatomy`, `unread_file`, `co_read`, `stale_rule` |
| `pkg/suggestions/engine.go` | create | `Suggestion` type, `Refresh`, ID hash, persistence |
| `cmd/cmd_suggestions.go` | create | `mneme suggestions list/dismiss/refresh` CLI |
| `cmd/main.go` | modify | Add `report` and `suggestions` dispatch cases |
| `pkg/envcheck/envcheck.go` | create | `Result`, `Detect`, Chrome path + package mgr + framework |
| `tests/cerebrum_test.go` | create | Golden-fixture tests for triggers, pending I/O, Learn |
| `tests/cerebrum_handler_test.go` | create | `httptest` test for `/cerebrum/learn` |
| `tests/waste_report_test.go` | create | Golden markdown report tests |
| `tests/suggestions_test.go` | create | Generator + dismissal TTL tests |
| `tests/envcheck_test.go` | create | Lockfile + framework detection tests |
| `tests/ledger_history_test.go` | create | Append, read, 1 MiB trim tests |
| `tests/integration/cerebrum_learn_test.go` | create | `//go:build integration` end-to-end |
| `tests/golden/m9/` | create | Transcript fixtures, expected pending/report/suggestions JSON |

---

## Task 1: Config keys for M9

**Files:**
- Modify: `pkg/config/config.go`
- Test: `tests/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `tests/config_test.go`:

```go
func TestM9ConfigDefaults(t *testing.T) {
	t.Setenv("CEREBRUM_LEARNING_ENABLED", "")
	t.Setenv("WASTE_THRESHOLD_PERCENT", "")
	t.Setenv("SUGGESTIONS_DISMISS_TTL_DAYS", "")
	t.Setenv("CEREBRUM_REJECT_TTL_DAYS", "")
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))

	cfg := config.FromEnv()
	if !cfg.CerebrumLearningEnabled {
		t.Errorf("CerebrumLearningEnabled default: got false, want true")
	}
	if cfg.WasteThresholdPercent != 15 {
		t.Errorf("WasteThresholdPercent default: got %d, want 15", cfg.WasteThresholdPercent)
	}
	if cfg.SuggestionsDismissTTLDays != 30 {
		t.Errorf("SuggestionsDismissTTLDays default: got %d, want 30", cfg.SuggestionsDismissTTLDays)
	}
	if cfg.CerebrumRejectTTLDays != 90 {
		t.Errorf("CerebrumRejectTTLDays default: got %d, want 90", cfg.CerebrumRejectTTLDays)
	}
}

func TestM9ConfigEnvOverride(t *testing.T) {
	t.Setenv("CEREBRUM_LEARNING_ENABLED", "false")
	t.Setenv("WASTE_THRESHOLD_PERCENT", "25")
	t.Setenv("SUGGESTIONS_DISMISS_TTL_DAYS", "7")
	t.Setenv("CEREBRUM_REJECT_TTL_DAYS", "180")
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))

	cfg := config.FromEnv()
	if cfg.CerebrumLearningEnabled {
		t.Errorf("env override learning_enabled=false not applied")
	}
	if cfg.WasteThresholdPercent != 25 {
		t.Errorf("env override threshold: got %d", cfg.WasteThresholdPercent)
	}
	if cfg.SuggestionsDismissTTLDays != 7 {
		t.Errorf("env override dismiss_ttl: got %d", cfg.SuggestionsDismissTTLDays)
	}
	if cfg.CerebrumRejectTTLDays != 180 {
		t.Errorf("env override reject_ttl: got %d", cfg.CerebrumRejectTTLDays)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestM9Config -v`
Expected: FAIL with `cfg.CerebrumLearningEnabled undefined` etc.

- [ ] **Step 3: Add fields and helpers**

In `pkg/config/config.go`, add to `Config`:

```go
	CerebrumLearningEnabled    bool
	WasteThresholdPercent      int
	SuggestionsDismissTTLDays  int
	CerebrumRejectTTLDays      int
```

Add to `fileConfig`:

```go
	CerebrumLearningEnabled    *bool `yaml:"cerebrum_learning_enabled"`
	WasteThresholdPercent      *int  `yaml:"waste_threshold_percent"`
	SuggestionsDismissTTLDays  *int  `yaml:"suggestions_dismiss_ttl_days"`
	CerebrumRejectTTLDays      *int  `yaml:"cerebrum_reject_ttl_days"`
```

Add helpers (place after `resolve`):

```go
// resolveBool returns env > file > default. Env values "1", "true", "yes" → true;
// "0", "false", "no" → false; anything else falls through.
func resolveBool(envVal string, fileVal *bool, defaultVal bool) bool {
	switch strings.ToLower(envVal) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	}
	if fileVal != nil {
		return *fileVal
	}
	return defaultVal
}

// resolveInt returns env > file > default. Non-integer env values fall through.
func resolveInt(envVal string, fileVal *int, defaultVal int) int {
	if envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil {
			return n
		}
	}
	if fileVal != nil {
		return *fileVal
	}
	return defaultVal
}
```

Add `"strconv"` to the import block.

In `FromEnv()` return struct, append:

```go
		CerebrumLearningEnabled:   resolveBool(os.Getenv("CEREBRUM_LEARNING_ENABLED"), file.CerebrumLearningEnabled, true),
		WasteThresholdPercent:     resolveInt(os.Getenv("WASTE_THRESHOLD_PERCENT"), file.WasteThresholdPercent, 15),
		SuggestionsDismissTTLDays: resolveInt(os.Getenv("SUGGESTIONS_DISMISS_TTL_DAYS"), file.SuggestionsDismissTTLDays, 30),
		CerebrumRejectTTLDays:     resolveInt(os.Getenv("CEREBRUM_REJECT_TTL_DAYS"), file.CerebrumRejectTTLDays, 90),
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestM9Config -v`
Expected: PASS for both new tests; existing `TestConfig*` still pass.

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go tests/config_test.go
git commit -m "feat(m9): add cerebrum/waste/suggestions config keys"
```

---

## Task 2: Ledger history append-only log

**Files:**
- Create: `pkg/state/ledger_history.go`
- Test: `tests/ledger_history_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/ledger_history_test.go`:

```go
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

func TestAppendLedgerHistory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	root := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(filepath.Join(root, ".mneme"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadOrCreateLocalID(root); err != nil {
		t.Fatal(err)
	}

	snap := state.LedgerSnapshot{
		TS:        "2026-04-28T10:00:00Z",
		SessionID: "s1",
		Totals:    state.LedgerTotals{HookFired: map[string]int{"pre-read": 5}},
	}
	if err := state.AppendLedgerHistory(root, snap); err != nil {
		t.Fatalf("append: %v", err)
	}

	id, _ := state.ReadOrCreateLocalID(root)
	histPath := filepath.Join(state.GlobalProjectDir(id), "ledger-history.jsonl")
	data, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `"session_id":"s1"`) {
		t.Errorf("missing session_id: %s", data)
	}

	var got state.LedgerSnapshot
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Totals.HookFired["pre-read"] != 5 {
		t.Errorf("HookFired[pre-read]: got %d, want 5", got.Totals.HookFired["pre-read"])
	}
}

func TestReadLedgerHistorySince(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	old := state.LedgerSnapshot{TS: "2026-01-01T00:00:00Z", SessionID: "old"}
	new := state.LedgerSnapshot{TS: "2026-04-25T00:00:00Z", SessionID: "new"}
	state.AppendLedgerHistory(root, old)
	state.AppendLedgerHistory(root, new)

	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	snaps, err := state.ReadLedgerHistory(root, since)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(snaps) != 1 || snaps[0].SessionID != "new" {
		t.Errorf("got %d snaps, want 1 with session_id=new: %+v", len(snaps), snaps)
	}
}

func TestLedgerHistoryRollingTrim(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	// Append 1500 entries (~150 KB each via padding) to push file over 1 MiB.
	pad := strings.Repeat("x", 1024)
	for i := 0; i < 1500; i++ {
		snap := state.LedgerSnapshot{
			TS:        time.Now().UTC().Add(-time.Duration(1500-i) * time.Hour).Format(time.RFC3339),
			SessionID: "s" + string(rune(i)) + pad[:0], // pad excluded; we want size growth
			Totals:    state.LedgerTotals{HookFired: map[string]int{pad: i}},
		}
		state.AppendLedgerHistory(root, snap)
	}

	id, _ := state.ReadOrCreateLocalID(root)
	histPath := filepath.Join(state.GlobalProjectDir(id), "ledger-history.jsonl")
	info, _ := os.Stat(histPath)
	if info.Size() > 2*1024*1024 {
		t.Errorf("file size after trim: %d bytes (want < 2 MiB)", info.Size())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestAppendLedgerHistory -v`
Expected: FAIL with `state.LedgerSnapshot undefined`.

- [ ] **Step 3: Implement**

Create `pkg/state/ledger_history.go`:

```go
package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ledgerHistoryMaxBytes = 1 << 20 // 1 MiB

type LedgerSnapshot struct {
	TS        string       `json:"ts"`
	SessionID string       `json:"session_id"`
	Totals    LedgerTotals `json:"totals"`
}

// AppendLedgerHistory appends a snapshot line to the project's ledger-history.jsonl.
// If the file grows beyond ledgerHistoryMaxBytes after the append, the writer drops
// the oldest 25% of lines.
func AppendLedgerHistory(projectRoot string, snap LedgerSnapshot) error {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return err
	}
	dir := GlobalProjectDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "ledger-history.lock")
	release, err := AcquireLock(lockPath, lockTimeout)
	if err != nil {
		return err
	}
	defer release()

	path := filepath.Join(dir, "ledger-history.jsonl")
	line, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if info, err := os.Stat(path); err == nil && info.Size() > ledgerHistoryMaxBytes {
		return trimLedgerHistory(path)
	}
	return nil
}

// ReadLedgerHistory returns snapshots with TS >= since. Lines older than `since`
// or unparsable are skipped silently.
func ReadLedgerHistory(projectRoot string, since time.Time) ([]LedgerSnapshot, error) {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(GlobalProjectDir(id), "ledger-history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []LedgerSnapshot
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var snap LedgerSnapshot
		if err := json.Unmarshal([]byte(line), &snap); err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, snap.TS)
		if err != nil {
			continue
		}
		if ts.Before(since) {
			continue
		}
		out = append(out, snap)
	}
	return out, sc.Err()
}

func trimLedgerHistory(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 8 {
		return nil
	}
	keep := lines[len(lines)/4:] // drop oldest 25%
	out := strings.Join(keep, "\n") + "\n"
	return AtomicWrite(path, []byte(out))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestAppendLedgerHistory -v && go test ./tests -run TestReadLedgerHistorySince -v && go test ./tests -run TestLedgerHistoryRollingTrim -v`
Expected: PASS all three.

- [ ] **Step 5: Commit**

```bash
git add pkg/state/ledger_history.go tests/ledger_history_test.go
git commit -m "feat(m9): ledger-history.jsonl append-only log with rolling trim"
```

---

## Task 3: Wire ledger-history append into stop hook

**Files:**
- Modify: `cmd/hook_stop.go`
- Test: `tests/hook_test.go`

- [ ] **Step 1: Write the failing test**

Add to `tests/hook_test.go`:

```go
func TestStopHookAppendsLedgerHistory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	root := filepath.Join(tmp, "proj")
	mustMkdir(t, filepath.Join(root, ".mneme"))
	state.ReadOrCreateLocalID(root)

	// Pre-existing session so AggregateTurn has something to do.
	state.UpsertSession(root, state.Session{SessionID: "s1"})

	ev := hook.Event{
		SessionID:     "s1",
		HookEventName: "stop",
		Cwd:           root,
	}
	body, _ := json.Marshal(ev)

	// Run stop hook subprocess.
	cmd := exec.Command(testBinary(t), "hook", "stop")
	cmd.Env = append(os.Environ(), "HOME="+tmp, "MNEME_DEBUG=0")
	cmd.Stdin = bytes.NewReader(body)
	if err := cmd.Run(); err != nil {
		t.Fatalf("hook run: %v", err)
	}

	id, _ := state.ReadOrCreateLocalID(root)
	histPath := filepath.Join(state.GlobalProjectDir(id), "ledger-history.jsonl")
	if _, err := os.Stat(histPath); err != nil {
		t.Fatalf("ledger-history.jsonl missing: %v", err)
	}
	data, _ := os.ReadFile(histPath)
	if !strings.Contains(string(data), `"session_id":"s1"`) {
		t.Errorf("expected session_id=s1 in history: %s", data)
	}
}
```

If `testBinary` and `mustMkdir` helpers don't already exist in `tests/hook_test.go`, use the same patterns the existing `hook_test.go` tests use (compile a binary in a `TestMain` or via `go build`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestStopHookAppendsLedgerHistory -v`
Expected: FAIL — file does not exist.

- [ ] **Step 3: Modify `cmd/hook_stop.go`**

```go
package main

import (
	"io"
	"os"
	"time"

	"github.com/ranwei/mneme/pkg/hook"
	"github.com/ranwei/mneme/pkg/state"
)

func runStop(stdin io.Reader) {
	defer recoverAndLog("stop")
	ev := parseOrExit(stdin, "stop")
	if ev.IsRecursiveStop() {
		os.Exit(0)
	}
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.stop")

	if err := state.AggregateTurn(root); err != nil {
		hook.WriteStderr("stop: aggregate turn: " + err.Error())
	}

	if l, err := state.ReadLedger(root); err == nil {
		snap := state.LedgerSnapshot{
			TS:        time.Now().UTC().Format(time.RFC3339),
			SessionID: ev.SessionID,
			Totals:    l.Totals,
		}
		if err := state.AppendLedgerHistory(root, snap); err != nil {
			hook.WriteStderr("stop: ledger history: " + err.Error())
		}
	}

	exitHook("stop", root)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests -run TestStopHookAppendsLedgerHistory -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/hook_stop.go tests/hook_test.go
git commit -m "feat(m9): stop hook appends per-session ledger snapshot"
```

---

## Task 4: Cerebrum trigger keyword scanner

**Files:**
- Create: `pkg/cerebrum/triggers.go`
- Test: `tests/cerebrum_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/cerebrum_test.go`:

```go
package tests

import (
	"reflect"
	"sort"
	"testing"

	"github.com/ranwei/mneme/pkg/cerebrum"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestScanText -v`
Expected: FAIL with `pkg cerebrum not found`.

- [ ] **Step 3: Implement**

Create `pkg/cerebrum/triggers.go`:

```go
package cerebrum

import (
	"strings"
)

// TriggerPhrases lists hardcoded user-message keywords that flag a possible
// "user corrected Claude" moment. Match is case-insensitive substring.
var TriggerPhrases = []string{
	// English
	"corrected",
	"should not",
	"shouldn't",
	"instead",
	"wrong",
	"don't",
	"do not",
	"actually",
	"revert",
	// Chinese
	"避免",
	"不要",
	"不应",
	"错误",
	"改回",
}

type TriggerHit struct {
	Phrase string // lowercase canonical phrase from TriggerPhrases
	Offset int    // byte offset of match in the input
}

// ScanText returns one TriggerHit per first occurrence of each phrase.
// Matches are case-insensitive on ASCII letters; Chinese phrases match
// as raw substring.
func ScanText(s string) []TriggerHit {
	lower := strings.ToLower(s)
	var hits []TriggerHit
	seen := make(map[string]bool)
	for _, p := range TriggerPhrases {
		if seen[p] {
			continue
		}
		idx := strings.Index(lower, strings.ToLower(p))
		if idx < 0 {
			continue
		}
		hits = append(hits, TriggerHit{Phrase: p, Offset: idx})
		seen[p] = true
	}
	return hits
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestScanText -v`
Expected: PASS all four.

- [ ] **Step 5: Commit**

```bash
git add pkg/cerebrum/triggers.go tests/cerebrum_test.go
git commit -m "feat(m9): cerebrum trigger phrase scanner"
```

---

## Task 5: Cerebrum pending file I/O

**Files:**
- Create: `pkg/cerebrum/pending.go`
- Test: `tests/cerebrum_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `tests/cerebrum_test.go`:

```go
import (
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

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
	// Re-add same ID — HitCount should bump, not duplicate.
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestCandidateID -v && go test ./tests -run TestSavePendingDedup -v && go test ./tests -run TestRejectedTTLFilter -v`
Expected: FAIL — none of those types exist yet.

- [ ] **Step 3: Implement**

Create `pkg/cerebrum/pending.go`:

```go
package cerebrum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const pendingLockTimeout = 5 * time.Second

type Trigger struct {
	Phrase    string `json:"phrase"`
	UserMsg   string `json:"user_msg"`
	PriorAsst string `json:"prior_asst"`
	Turn      int    `json:"turn"`
}

type Candidate struct {
	ID         string             `json:"id"`
	Trigger    Trigger            `json:"trigger"`
	DraftRule  state.CerebrumRule `json:"draft_rule"`
	Confidence float64            `json:"confidence"`
	QueuedAt   string             `json:"queued_at"`
	HitCount   int                `json:"hit_count"`
}

type rejectedRecord struct {
	ID         string `json:"id"`
	RejectedAt string `json:"rejected_at"`
}

// NewCandidateID returns a stable 16-char hex hash of phrase + priorAsst.
func NewCandidateID(phrase, priorAsst string) string {
	h := sha256.Sum256([]byte("v1|" + phrase + "|" + priorAsst))
	return hex.EncodeToString(h[:8])
}

func pendingPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "cerebrum-pending.json"), nil
}

func rejectedPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "cerebrum-rejected.json"), nil
}

// LoadPending reads cerebrum-pending.json. Returns (nil, nil) if missing.
func LoadPending(projectRoot string) ([]Candidate, error) {
	p, err := pendingPath(projectRoot)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cs []Candidate
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// SavePending overwrites cerebrum-pending.json with the supplied set.
func SavePending(projectRoot string, cs []Candidate) error {
	p, err := pendingPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, pendingLockTimeout)
	if err != nil {
		return err
	}
	defer release()
	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

// AppendPending adds c to the pending list, deduping by ID. Same ID bumps HitCount.
func AppendPending(projectRoot string, c Candidate) error {
	p, err := pendingPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, pendingLockTimeout)
	if err != nil {
		return err
	}
	defer release()

	var existing []Candidate
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, &existing)
	}
	for i := range existing {
		if existing[i].ID == c.ID {
			existing[i].HitCount += c.HitCount
			data, _ := json.MarshalIndent(existing, "", "  ")
			return state.AtomicWrite(p, data)
		}
	}
	existing = append(existing, c)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

// AppendRejected records that the user rejected candidate id at rejectedAt.
func AppendRejected(projectRoot string, id string, rejectedAt time.Time) error {
	p, err := rejectedPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, pendingLockTimeout)
	if err != nil {
		return err
	}
	defer release()

	var rec []rejectedRecord
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, &rec)
	}
	rec = append(rec, rejectedRecord{ID: id, RejectedAt: rejectedAt.UTC().Format(time.RFC3339)})
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

// LoadRejected returns a set of IDs whose rejection is still within ttlDays.
func LoadRejected(projectRoot string, ttlDays int, now time.Time) (map[string]bool, error) {
	p, err := rejectedPath(projectRoot)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var rec []rejectedRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rec))
	cutoff := now.Add(-time.Duration(ttlDays) * 24 * time.Hour)
	for _, r := range rec {
		ts, err := time.Parse(time.RFC3339, r.RejectedAt)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			out[r.ID] = true
		}
	}
	return out, nil
}

// RemovePending drops candidates whose ID is in remove.
func RemovePending(projectRoot string, remove map[string]bool) error {
	cs, err := LoadPending(projectRoot)
	if err != nil {
		return err
	}
	kept := cs[:0]
	for _, c := range cs {
		if !remove[c.ID] {
			kept = append(kept, c)
		}
	}
	return SavePending(projectRoot, kept)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestCandidateID -v && go test ./tests -run TestSavePendingDedup -v && go test ./tests -run TestRejectedTTLFilter -v`
Expected: PASS all three.

- [ ] **Step 5: Commit**

```bash
git add pkg/cerebrum/pending.go tests/cerebrum_test.go
git commit -m "feat(m9): cerebrum candidate + pending/rejected I/O"
```

---

## Task 6: Cerebrum learner (transcript scan + draft rule)

**Files:**
- Create: `pkg/cerebrum/learner.go`
- Test: `tests/cerebrum_test.go` (extend)
- Test fixtures: `tests/golden/m9/transcripts/*.jsonl`

- [ ] **Step 1: Write the failing test**

Create fixture `tests/golden/m9/transcripts/english.jsonl`:

```jsonl
{"type":"user","message":{"role":"user","content":"please edit pkg/foo/bar.go to fix the bug"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll edit pkg/foo/bar.go now."},{"type":"tool_use","name":"Edit","input":{"file_path":"pkg/foo/bar.go","old_string":"a","new_string":"b"}}]}}
{"type":"user","message":{"role":"user","content":"actually you should not edit that file"}}
```

Create fixture `tests/golden/m9/transcripts/no_match.jsonl`:

```jsonl
{"type":"user","message":{"role":"user","content":"please add a feature"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"sure"}]}}
{"type":"user","message":{"role":"user","content":"thanks, looks good"}}
```

Append to `tests/cerebrum_test.go`:

```go
import "github.com/ranwei/mneme/pkg/state"

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

	// Pre-existing rule with same pattern blocks the candidate.
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestLearn -v`
Expected: FAIL with `cerebrum.Learn undefined`.

- [ ] **Step 3: Implement**

Create `pkg/cerebrum/learner.go`:

```go
package cerebrum

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const (
	defaultRejectTTLDays = 90
	maxUserMsgLen        = 200
	maxPriorAsstLen      = 400
)

type transcriptLine struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

type userMsg struct {
	Content string `json:"content"`
}

type asstMsg struct {
	Content []asstBlock `json:"content"`
}

type asstBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type editInput struct {
	FilePath string `json:"file_path"`
}

// Learn reads a Claude Code transcript JSONL, scans user messages for trigger
// phrases, drafts a candidate rule for each, and persists them to
// cerebrum-pending.json (deduped by ID, pre-empted by existing rules,
// suppressed by recent rejections).
//
// Returns the candidates that were enqueued (already-pending IDs only have
// their HitCount bumped; not returned again).
func Learn(transcriptPath, projectRoot, sessionID string, now time.Time) ([]Candidate, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer f.Close()

	existing, _ := state.ReadCerebrum(projectRoot)
	existingPatterns := make(map[string]bool, len(existing))
	for _, r := range existing {
		existingPatterns[r.Pattern] = true
	}

	rejectTTL := defaultRejectTTLDays
	rejected, _ := LoadRejected(projectRoot, rejectTTL, now)

	var (
		out       []Candidate
		priorTxt  string
		priorFile string
		turn      int
	)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		turn++
		var line transcriptLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}
		switch line.Type {
		case "assistant":
			var m asstMsg
			if err := json.Unmarshal(line.Message, &m); err != nil {
				continue
			}
			priorTxt, priorFile = summarizeAssistant(m)
		case "user":
			var u userMsg
			if err := json.Unmarshal(line.Message, &u); err != nil {
				continue
			}
			hits := ScanText(u.Content)
			for _, h := range hits {
				cand := buildCandidate(h, u.Content, priorTxt, priorFile, turn, sessionID, now)
				if existingPatterns[cand.DraftRule.Pattern] {
					continue
				}
				if rejected[cand.ID] {
					continue
				}
				if err := AppendPending(projectRoot, cand); err != nil {
					return nil, err
				}
				out = append(out, cand)
			}
		}
	}
	return out, sc.Err()
}

func summarizeAssistant(m asstMsg) (text, filePath string) {
	var sb strings.Builder
	for _, b := range m.Content {
		switch b.Type {
		case "text":
			sb.WriteString(b.Text)
			sb.WriteString(" ")
		case "tool_use":
			if b.Name == "Edit" || b.Name == "Write" || b.Name == "MultiEdit" {
				var ei editInput
				if err := json.Unmarshal(b.Input, &ei); err == nil && ei.FilePath != "" && filePath == "" {
					filePath = ei.FilePath
				}
			}
		}
	}
	return strings.TrimSpace(sb.String()), filePath
}

func buildCandidate(h TriggerHit, userMsg, priorTxt, priorFile string, turn int, sessionID string, now time.Time) Candidate {
	pattern := priorFile
	if pattern != "" {
		pattern = regexp.QuoteMeta(pattern)
	} else {
		pattern = firstToken(priorTxt)
	}
	msg := truncate(userMsg, 80)
	cand := Candidate{
		Trigger: Trigger{
			Phrase:    h.Phrase,
			UserMsg:   truncate(userMsg, maxUserMsgLen),
			PriorAsst: truncate(priorTxt, maxPriorAsstLen),
			Turn:      turn,
		},
		DraftRule: state.CerebrumRule{
			Comment: fmt.Sprintf("learned from session %s turn %d", sessionID, turn),
			Pattern: pattern,
			Message: msg,
		},
		Confidence: 1.0,
		QueuedAt:   now.UTC().Format(time.RFC3339),
		HitCount:   1,
	}
	cand.ID = NewCandidateID(h.Phrase, priorTxt)
	return cand
}

func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "."
	}
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' {
			return regexp.QuoteMeta(s[:i])
		}
	}
	return regexp.QuoteMeta(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestLearn -v`
Expected: PASS all three.

- [ ] **Step 5: Commit**

```bash
git add pkg/cerebrum/learner.go tests/cerebrum_test.go tests/golden/m9/transcripts/
git commit -m "feat(m9): cerebrum learner — transcript scan + draft rule"
```

---

## Task 7: `mneme cerebrum review` CLI

**Files:**
- Modify: `cmd/cmd_cerebrum.go`
- Test: `tests/cerebrum_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `tests/cerebrum_test.go`:

```go
import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func TestCerebrumReviewListJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	c := cerebrum.Candidate{
		ID:        "abcdef0123456789",
		Trigger:   cerebrum.Trigger{Phrase: "should not"},
		DraftRule: state.CerebrumRule{Pattern: "foo", Message: "x"},
		QueuedAt:  time.Now().UTC().Format(time.RFC3339),
		HitCount:  1,
	}
	cerebrum.SavePending(root, []cerebrum.Candidate{c})

	cmd := exec.Command(testBinary(t), "cerebrum", "review", "--list", "--json")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	var got []cerebrum.Candidate
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if len(got) != 1 || got[0].ID != "abcdef0123456789" {
		t.Errorf("expected 1 candidate with ID=abc..., got: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestCerebrumReviewListJSON -v`
Expected: FAIL — `unknown cerebrum subcommand: review`.

- [ ] **Step 3: Add `review` subcommand**

In `cmd/cmd_cerebrum.go`, change the dispatch switch:

```go
	switch args[0] {
	case "add":
		cerebrumAdd(args[1:])
	case "list":
		cerebrumList()
	case "remove":
		cerebrumRemove(args[1:])
	case "review":
		cerebrumReview(args[1:])
	default:
```

Add `"encoding/json"` and `"time"` to the existing import block at the top of the file (the file currently imports `bufio`, `flag`, `fmt`, `os`, `regexp`, `strconv`, `strings`, `golang.org/x/term`, `github.com/ranwei/mneme/pkg/state`). Also add `"github.com/ranwei/mneme/pkg/cerebrum"`.

Append the function:

```go
func cerebrumReview(args []string) {
	fs := flag.NewFlagSet("cerebrum review", flag.ContinueOnError)
	listOnly := fs.Bool("list", false, "print pending candidates without prompting")
	asJSON := fs.Bool("json", false, "JSON output (only with --list)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}

	pending, err := cerebrum.LoadPending(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ load pending:", err)
		os.Exit(1)
	}

	if *listOnly {
		if *asJSON {
			data, _ := json.MarshalIndent(pending, "", "  ")
			fmt.Println(string(data))
			return
		}
		if len(pending) == 0 {
			fmt.Println("No pending cerebrum candidates.")
			return
		}
		for i, c := range pending {
			fmt.Printf("%d. [%s] %s — %s\n", i+1, c.ID[:8], c.Trigger.Phrase, c.DraftRule.Message)
		}
		return
	}

	if len(pending) == 0 {
		fmt.Println("No pending cerebrum candidates.")
		return
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "✗ interactive review requires a TTY; use --list for non-interactive output")
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	remove := make(map[string]bool)
	for i, c := range pending {
		fmt.Printf("\n[%d/%d] phrase=%q hits=%d\n", i+1, len(pending), c.Trigger.Phrase, c.HitCount)
		fmt.Printf("    user: %s\n", c.Trigger.UserMsg)
		fmt.Printf("    prior: %s\n", c.Trigger.PriorAsst)
		fmt.Printf("    draft pattern: %s\n", c.DraftRule.Pattern)
		fmt.Printf("    draft message: %s\n", c.DraftRule.Message)
		fmt.Print("    [a]ccept / [r]eject / [s]kip / [q]uit: ")
		scanner.Scan()
		switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
		case "a", "accept":
			if err := state.AppendCerebrumRule(root, c.DraftRule); err != nil {
				fmt.Fprintln(os.Stderr, "✗ append rule:", err)
				continue
			}
			remove[c.ID] = true
		case "r", "reject":
			if err := cerebrum.AppendRejected(root, c.ID, time.Now().UTC()); err != nil {
				fmt.Fprintln(os.Stderr, "✗ record rejection:", err)
				continue
			}
			remove[c.ID] = true
		case "q", "quit":
			break
		}
	}
	if len(remove) > 0 {
		if err := cerebrum.RemovePending(root, remove); err != nil {
			fmt.Fprintln(os.Stderr, "✗ update pending:", err)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./cmd && go test ./tests -run TestCerebrumReviewListJSON -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_cerebrum.go tests/cerebrum_test.go
git commit -m "feat(m9): mneme cerebrum review CLI (interactive + --list --json)"
```

---

## Task 8: Daemon `/cerebrum/learn` endpoint + worker

> Depends on M8. The `daemon.RouteDeps` and `daemon.NewMux` types are defined by M8 Task 10. The worker integrates with M8's task scheduler — but the queue itself is in-memory inside the handler.

**Files:**
- Create: `pkg/daemon/cerebrum_handler.go`
- Modify: `pkg/daemon/routes.go` (M8 file)
- Test: `tests/cerebrum_handler_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/cerebrum_handler_test.go`:

```go
package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/state"
)

func TestCerebrumLearnHandlerEnqueues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(root)
	// Stub origin file so the worker can resolve project_id → root.
	os.WriteFile(filepath.Join(state.GlobalProjectDir(id), "origin"), []byte(root+"\n"), 0644)

	// Copy fixture into tmp so the handler can find it.
	transcript := filepath.Join(tmp, "english.jsonl")
	src, _ := os.ReadFile("golden/m9/transcripts/english.jsonl")
	os.WriteFile(transcript, src, 0644)

	h := daemon.NewCerebrumLearnHandler(daemon.CerebrumLearnDeps{
		LearningEnabled: true,
		RejectTTLDays:   90,
		Now:             func() time.Time { return time.Now().UTC() },
	})

	body, _ := json.Marshal(map[string]string{
		"project_id":      id,
		"transcript_path": transcript,
		"session_id":      "s1",
	})
	req := httptest.NewRequest("POST", "/cerebrum/learn", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202; body=%s", rec.Code, rec.Body.String())
	}

	// Wait briefly for the worker to drain.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cs, _ := cerebrum.LoadPending(root)
		if len(cs) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("no candidates queued within timeout")
}

func TestCerebrumLearnHandlerDisabled(t *testing.T) {
	h := daemon.NewCerebrumLearnHandler(daemon.CerebrumLearnDeps{LearningEnabled: false})
	req := httptest.NewRequest("POST", "/cerebrum/learn", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var out struct {
		Disabled bool `json:"disabled"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.Disabled {
		t.Errorf("expected disabled=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestCerebrumLearnHandler -v`
Expected: FAIL with `daemon.NewCerebrumLearnHandler undefined`.

- [ ] **Step 3: Implement handler + worker**

Create `pkg/daemon/cerebrum_handler.go`:

```go
package daemon

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/state"
)

const cerebrumQueueCapacity = 100

type CerebrumLearnRequest struct {
	ProjectID      string `json:"project_id"`
	TranscriptPath string `json:"transcript_path"`
	SessionID      string `json:"session_id"`
}

type CerebrumLearnDeps struct {
	LearningEnabled bool
	RejectTTLDays   int
	Now             func() time.Time
	// resolveProjectRoot is overridable for tests; production path reads
	// GlobalProjectDir(id)/origin or falls back to GlobalProjectDir(id) itself.
	ResolveProjectRoot func(projectID string) (string, error)
}

type cerebrumLearnHandler struct {
	deps  CerebrumLearnDeps
	queue chan CerebrumLearnRequest
	stop  chan struct{}

	mu     sync.Mutex
	inProg map[string]bool // project_id → true if a learn is running
}

// NewCerebrumLearnHandler returns an http.Handler that enqueues learn requests
// onto an in-memory channel and spawns a worker goroutine to drain them.
func NewCerebrumLearnHandler(deps CerebrumLearnDeps) http.Handler {
	if deps.ResolveProjectRoot == nil {
		deps.ResolveProjectRoot = defaultResolveProjectRoot
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	h := &cerebrumLearnHandler{
		deps:   deps,
		queue:  make(chan CerebrumLearnRequest, cerebrumQueueCapacity),
		stop:   make(chan struct{}),
		inProg: map[string]bool{},
	}
	go h.workerLoop()
	return h
}

func (h *cerebrumLearnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.deps.LearningEnabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"disabled":true}`))
		return
	}
	var req CerebrumLearnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.ProjectID == "" || req.TranscriptPath == "" || req.SessionID == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}
	select {
	case h.queue <- req:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"queued":true}`))
	default:
		http.Error(w, "queue full", http.StatusServiceUnavailable)
	}
}

func (h *cerebrumLearnHandler) workerLoop() {
	for {
		select {
		case <-h.stop:
			return
		case req := <-h.queue:
			h.processOne(req)
		}
	}
}

func (h *cerebrumLearnHandler) processOne(req CerebrumLearnRequest) {
	h.mu.Lock()
	if h.inProg[req.ProjectID] {
		// Drop concurrent requests for the same project; cron.SkipIfStillRunning semantics.
		h.mu.Unlock()
		return
	}
	h.inProg[req.ProjectID] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.inProg, req.ProjectID)
		h.mu.Unlock()
	}()

	projectRoot, err := h.deps.ResolveProjectRoot(req.ProjectID)
	if err != nil {
		return
	}
	defer func() { _ = recover() }()
	cerebrum.Learn(req.TranscriptPath, projectRoot, req.SessionID, h.deps.Now())
}

// defaultResolveProjectRoot reads ~/.mneme/projects/<id>/origin (M7 convention).
// Falls back to GlobalProjectDir(id) so tests and pre-M7 environments still work.
func defaultResolveProjectRoot(projectID string) (string, error) {
	dir := state.GlobalProjectDir(projectID)
	if data, err := os.ReadFile(filepath.Join(dir, "origin")); err == nil {
		root := string(data)
		for len(root) > 0 && (root[len(root)-1] == '\n' || root[len(root)-1] == '\r' || root[len(root)-1] == ' ') {
			root = root[:len(root)-1]
		}
		if root != "" {
			return root, nil
		}
	}
	return dir, nil
}
```

In `pkg/daemon/routes.go` (M8 file), inside `NewMux`, register the handler. M8 Task 10 defined `NewMux(deps RouteDeps)`. Extend `RouteDeps` to include the cerebrum deps:

```go
type RouteDeps struct {
	// ... existing M8 fields ...
	Cerebrum CerebrumLearnDeps
}
```

In `NewMux(deps RouteDeps)`:

```go
	mux.Handle("/cerebrum/learn", NewCerebrumLearnHandler(deps.Cerebrum))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestCerebrumLearnHandler -v`
Expected: PASS both.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon/cerebrum_handler.go pkg/daemon/routes.go tests/cerebrum_handler_test.go
git commit -m "feat(m9): daemon /cerebrum/learn endpoint + worker"
```

---

## Task 9: Stop hook posts to daemon, falls back to in-process

**Files:**
- Modify: `cmd/hook_stop.go`
- Test: `tests/hook_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tests/hook_test.go`:

```go
func TestStopHookFallbackLearnsInProcess(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)
	state.UpsertSession(root, state.Session{SessionID: "s1"})

	transcript := filepath.Join(tmp, "english.jsonl")
	src, _ := os.ReadFile("golden/m9/transcripts/english.jsonl")
	os.WriteFile(transcript, src, 0644)

	ev := hook.Event{
		SessionID:      "s1",
		HookEventName:  "stop",
		Cwd:            root,
		TranscriptPath: transcript,
	}
	body, _ := json.Marshal(ev)

	cmd := exec.Command(testBinary(t), "hook", "stop")
	// HOME points at tmp; daemon socket does not exist, forcing fallback.
	cmd.Env = append(os.Environ(), "HOME="+tmp, "MNEME_DEBUG=0")
	cmd.Stdin = bytes.NewReader(body)
	if err := cmd.Run(); err != nil {
		t.Fatalf("hook run: %v", err)
	}

	cs, err := cerebrum.LoadPending(root)
	if err != nil {
		t.Fatalf("load pending: %v", err)
	}
	if len(cs) == 0 {
		t.Errorf("expected fallback learn to write candidates")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestStopHookFallbackLearnsInProcess -v`
Expected: FAIL — stop hook does not invoke learner.

- [ ] **Step 3: Wire stop hook to daemon + fallback**

Modify `cmd/hook_stop.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/hook"
	"github.com/ranwei/mneme/pkg/state"
)

func runStop(stdin io.Reader) {
	defer recoverAndLog("stop")
	ev := parseOrExit(stdin, "stop")
	if ev.IsRecursiveStop() {
		os.Exit(0)
	}
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.stop")

	if err := state.AggregateTurn(root); err != nil {
		hook.WriteStderr("stop: aggregate turn: " + err.Error())
	}

	if l, err := state.ReadLedger(root); err == nil {
		snap := state.LedgerSnapshot{
			TS:        time.Now().UTC().Format(time.RFC3339),
			SessionID: ev.SessionID,
			Totals:    l.Totals,
		}
		if err := state.AppendLedgerHistory(root, snap); err != nil {
			hook.WriteStderr("stop: ledger history: " + err.Error())
		}
	}

	cfg := config.FromEnv()
	if cfg.CerebrumLearningEnabled && ev.TranscriptPath != "" {
		dispatchCerebrumLearn(root, ev.SessionID, ev.TranscriptPath)
	}

	exitHook("stop", root)
}

func dispatchCerebrumLearn(root, sessionID, transcriptPath string) {
	id, err := state.ReadOrCreateLocalID(root)
	if err != nil {
		return
	}
	if postCerebrumLearn(id, sessionID, transcriptPath) {
		return
	}
	defer func() { _ = recover() }()
	cerebrum.Learn(transcriptPath, root, sessionID, time.Now().UTC())
}

func postCerebrumLearn(projectID, sessionID, transcriptPath string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	socket := filepath.Join(home, ".mneme", "daemon", "socket")
	if _, err := os.Stat(socket); err != nil {
		return false
	}

	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 200 * time.Millisecond}
			return d.DialContext(ctx, "unix", socket)
		},
	}
	client := &http.Client{Transport: tr, Timeout: 200 * time.Millisecond}

	body, _ := json.Marshal(map[string]string{
		"project_id":      projectID,
		"transcript_path": transcriptPath,
		"session_id":      sessionID,
	})
	req, err := http.NewRequest("POST", "http://unix/cerebrum/learn", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests -run TestStopHookFallbackLearnsInProcess -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/hook_stop.go tests/hook_test.go
git commit -m "feat(m9): stop hook posts /cerebrum/learn with in-process fallback"
```

---

## Task 10: Waste report package

**Files:**
- Create: `pkg/waste/report.go`
- Test: `tests/waste_report_test.go`
- Test fixtures: `tests/golden/m9/reports/`

- [ ] **Step 1: Write the failing test**

Create `tests/waste_report_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

func TestGenerateAndRenderReport(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	// Seed two history snapshots: one from this week, one from last.
	now := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	state.AppendLedgerHistory(root, state.LedgerSnapshot{
		TS:        now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		SessionID: "old",
		Totals:    state.LedgerTotals{HookFired: map[string]int{"pre-read": 5}},
	})
	state.AppendLedgerHistory(root, state.LedgerSnapshot{
		TS:        now.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
		SessionID: "new",
		Totals:    state.LedgerTotals{HookFired: map[string]int{"pre-read": 12}},
	})

	out, err := waste.GenerateReport(waste.ReportInput{
		ProjectRoot: root,
		HomeDir:     tmp,
		Now:         now,
		Threshold:   0.15,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out.Date != "2026-04-28" {
		t.Errorf("Date: got %q, want 2026-04-28", out.Date)
	}
	md := waste.RenderMarkdown(out)
	if !strings.Contains(md, "# Mneme Waste Report — 2026-04-28") {
		t.Errorf("missing header: %s", md)
	}
	if !strings.Contains(md, "pre-read") {
		t.Errorf("expected pre-read in deltas section: %s", md)
	}
}

func TestWriteReport(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)

	out := waste.ReportOutput{Date: "2026-04-28"}
	path, err := waste.WriteReport(root, out)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	want := filepath.Join(root, ".mneme", "reports", "waste-2026-04-28.md")
	if path != want {
		t.Errorf("path: got %q, want %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file missing: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestGenerateAndRenderReport -v && go test ./tests -run TestWriteReport -v`
Expected: FAIL — `waste.ReportInput`/`waste.GenerateReport` undefined.

- [ ] **Step 3: Implement**

Create `pkg/waste/report.go`:

```go
package waste

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

type ReportInput struct {
	ProjectRoot string
	HomeDir     string
	Now         time.Time
	Threshold   float64
}

type ReportOutput struct {
	Date               string
	Patterns           []WastePattern
	Deltas             map[string]int
	BreachedThresholds []string
}

func GenerateReport(in ReportInput) (ReportOutput, error) {
	patterns, err := Detect(in.ProjectRoot, in.HomeDir)
	if err != nil {
		return ReportOutput{}, err
	}

	deltas, err := computeWeeklyDeltas(in.ProjectRoot, in.Now)
	if err != nil {
		return ReportOutput{}, err
	}

	var breached []string
	for _, p := range patterns {
		if p.Detected {
			breached = append(breached, p.Name)
		}
	}

	return ReportOutput{
		Date:               in.Now.UTC().Format("2006-01-02"),
		Patterns:           patterns,
		Deltas:             deltas,
		BreachedThresholds: breached,
	}, nil
}

func computeWeeklyDeltas(root string, now time.Time) (map[string]int, error) {
	since := now.Add(-14 * 24 * time.Hour)
	snaps, err := state.ReadLedgerHistory(root, since)
	if err != nil {
		return nil, err
	}
	weekAgo := now.Add(-7 * 24 * time.Hour)

	var thisWeek, prevWeek state.LedgerTotals
	thisWeek.HookFired = map[string]int{}
	prevWeek.HookFired = map[string]int{}

	for _, s := range snaps {
		ts, err := time.Parse(time.RFC3339, s.TS)
		if err != nil {
			continue
		}
		dst := &prevWeek
		if ts.After(weekAgo) {
			dst = &thisWeek
		}
		for k, v := range s.Totals.HookFired {
			if v > dst.HookFired[k] {
				dst.HookFired[k] = v
			}
		}
		if s.Totals.AnatomyHits > dst.AnatomyHits {
			dst.AnatomyHits = s.Totals.AnatomyHits
		}
		if s.Totals.RepeatReads > dst.RepeatReads {
			dst.RepeatReads = s.Totals.RepeatReads
		}
	}

	deltas := map[string]int{}
	for k, v := range thisWeek.HookFired {
		deltas[k] = v - prevWeek.HookFired[k]
	}
	deltas["anatomy_hits"] = thisWeek.AnatomyHits - prevWeek.AnatomyHits
	deltas["repeat_reads"] = thisWeek.RepeatReads - prevWeek.RepeatReads
	return deltas, nil
}

func RenderMarkdown(out ReportOutput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Mneme Waste Report — %s\n\n", out.Date)

	sb.WriteString("## Detected patterns\n\n")
	if len(out.Patterns) == 0 {
		sb.WriteString("_no patterns evaluated_\n\n")
	} else {
		for _, p := range out.Patterns {
			marker := "·"
			if p.Detected {
				marker = "⚠"
			}
			fmt.Fprintf(&sb, "- %s **%s** [%s]", marker, p.Name, p.Severity)
			if p.Details != "" {
				fmt.Fprintf(&sb, " — %s", p.Details)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Week-over-week deltas\n\n")
	if len(out.Deltas) == 0 {
		sb.WriteString("_no history available_\n\n")
	} else {
		keys := make([]string, 0, len(out.Deltas))
		for k := range out.Deltas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "- %s: %+d\n", k, out.Deltas[k])
		}
		sb.WriteString("\n")
	}

	if len(out.BreachedThresholds) > 0 {
		sb.WriteString("## Breached thresholds\n\n")
		for _, n := range out.BreachedThresholds {
			fmt.Fprintf(&sb, "- %s\n", n)
		}
	}
	return sb.String()
}

func WriteReport(projectRoot string, out ReportOutput) (string, error) {
	dir := filepath.Join(projectRoot, ".mneme", "reports")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "waste-"+out.Date+".md")
	return path, state.AtomicWrite(path, []byte(RenderMarkdown(out)))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestGenerateAndRenderReport -v && go test ./tests -run TestWriteReport -v`
Expected: PASS both.

- [ ] **Step 5: Commit**

```bash
git add pkg/waste/report.go tests/waste_report_test.go
git commit -m "feat(m9): waste report package with weekly deltas + markdown"
```

---

## Task 11: `mneme report waste` CLI

**Files:**
- Create: `cmd/cmd_report.go`
- Modify: `cmd/main.go`
- Test: `tests/waste_report_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `tests/waste_report_test.go`. The file's existing import block already has `os`, `path/filepath`, `strings`, `testing`, `time`, plus the `state` and `waste` packages — extend it with `bytes`, `io`, and `os/exec`:

```go
func TestReportWasteDryRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	cmd := exec.Command(testBinary(t), "report", "waste", "--dry-run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v\nstdout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Mneme Waste Report") {
		t.Errorf("expected report header, got: %s", out.String())
	}
	// Dry-run should not have written a file.
	matches, _ := filepath.Glob(filepath.Join(root, ".mneme", "reports", "waste-*.md"))
	if len(matches) > 0 {
		t.Errorf("--dry-run wrote a file: %v", matches)
	}
}

func TestReportWasteWritesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	cmd := exec.Command(testBinary(t), "report", "waste")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	cmd.Stdout = io.Discard
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".mneme", "reports", "waste-*.md"))
	if len(matches) == 0 {
		t.Errorf("expected at least one waste-*.md file")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestReportWaste -v`
Expected: FAIL — `unknown subcommand: report`.

- [ ] **Step 3: Implement CLI**

Create `cmd/cmd_report.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

func dispatchReport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme report <waste>")
		os.Exit(2)
	}
	switch args[0] {
	case "waste":
		runReportWaste(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown report subcommand: %q\n", args[0])
		os.Exit(2)
	}
}

func runReportWaste(args []string) {
	fs := flag.NewFlagSet("report waste", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print report to stdout instead of writing")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}
	home, _ := os.UserHomeDir()
	cfg := config.FromEnv()

	out, err := waste.GenerateReport(waste.ReportInput{
		ProjectRoot: root,
		HomeDir:     home,
		Now:         time.Now().UTC(),
		Threshold:   float64(cfg.WasteThresholdPercent) / 100.0,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ generate report:", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Print(waste.RenderMarkdown(out))
		return
	}
	path, err := waste.WriteReport(root, out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ write report:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ wrote %s\n", path)
}
```

Add to `cmd/main.go` switch:

```go
	case "report":
		dispatchReport(os.Args[2:])
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./cmd && go test ./tests -run TestReportWaste -v`
Expected: PASS both.

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_report.go cmd/main.go tests/waste_report_test.go
git commit -m "feat(m9): mneme report waste CLI"
```

---

## Task 12: Daemon `weekly-waste-report` cron task

> Depends on M8 task registry. The M8 plan defined `daemon.LookupTask(name string) TaskFunc` and a manifest seed list under `pkg/daemon/tasks.go`. This task adds a new entry there.

**Files:**
- Modify: `pkg/daemon/tasks.go`
- Modify: `pkg/daemon/manifest.go`
- Test: `tests/integration/cerebrum_learn_test.go` (extend in Task 18)

- [ ] **Step 1: Write the failing test**

Add to `tests/cerebrum_handler_test.go`:

```go
func TestWeeklyWasteReportTaskWrites(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(root)
	os.WriteFile(filepath.Join(state.GlobalProjectDir(id), "origin"), []byte(root+"\n"), 0644)

	tf, ok := daemon.LookupTask("weekly-waste-report")
	if !ok {
		t.Fatal("weekly-waste-report not registered")
	}
	if err := tf(); err != nil {
		t.Fatalf("task: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".mneme", "reports", "waste-*.md"))
	if len(matches) == 0 {
		t.Errorf("expected report file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestWeeklyWasteReportTaskWrites -v`
Expected: FAIL — task not registered.

- [ ] **Step 3: Add task wrapper + manifest entry**

In `pkg/daemon/tasks.go`, add (preserving existing imports):

```go
import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

func runWeeklyWasteReport() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectsDir := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		originBytes, err := os.ReadFile(filepath.Join(projectsDir, e.Name(), "origin"))
		if err != nil {
			continue
		}
		root := strings.TrimSpace(string(originBytes))
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			continue // origin path no longer exists
		}
		out, err := waste.GenerateReport(waste.ReportInput{
			ProjectRoot: root,
			HomeDir:     home,
			Now:         time.Now().UTC(),
			Threshold:   0.15,
		})
		if err != nil {
			continue
		}
		_, _ = waste.WriteReport(root, out)
	}
	return nil
}
```

Register in the existing task registry (pattern from M8 Task 7):

```go
func init() {
	registerTask("weekly-waste-report", runWeeklyWasteReport)
}
```

In `pkg/daemon/manifest.go` seed list (M8 Task 6 defined `seedManifest`), add:

```go
	{Name: "weekly-waste-report", Schedule: "0 9 * * 1"}, // Mondays 09:00 local
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests -run TestWeeklyWasteReportTaskWrites -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon/tasks.go pkg/daemon/manifest.go tests/cerebrum_handler_test.go
git commit -m "feat(m9): daemon weekly-waste-report cron task"
```

---

## Task 13: Suggestions dismissed file

**Files:**
- Create: `pkg/suggestions/dismissed.go`
- Test: `tests/suggestions_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/suggestions_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/suggestions"
)

func TestAppendAndLoadDismissed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	now := time.Now().UTC()
	if err := suggestions.AppendDismissed(root, "id-recent", now); err != nil {
		t.Fatal(err)
	}
	if err := suggestions.AppendDismissed(root, "id-old", now.Add(-60*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	suppressed, err := suggestions.LoadDismissed(root, 30, now)
	if err != nil {
		t.Fatal(err)
	}
	if !suppressed["id-recent"] {
		t.Errorf("id-recent should be suppressed")
	}
	if suppressed["id-old"] {
		t.Errorf("id-old (60d > 30d ttl) should not be suppressed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestAppendAndLoadDismissed -v`
Expected: FAIL — package not found.

- [ ] **Step 3: Implement**

Create `pkg/suggestions/dismissed.go`:

```go
package suggestions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const dismissedLockTimeout = 5 * time.Second

type dismissedRecord struct {
	ID          string `json:"id"`
	DismissedAt string `json:"dismissed_at"`
}

func dismissedPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "suggestions-dismissed.json"), nil
}

func AppendDismissed(projectRoot, id string, at time.Time) error {
	p, err := dismissedPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, dismissedLockTimeout)
	if err != nil {
		return err
	}
	defer release()

	var rec []dismissedRecord
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, &rec)
	}
	rec = append(rec, dismissedRecord{ID: id, DismissedAt: at.UTC().Format(time.RFC3339)})
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

func LoadDismissed(projectRoot string, ttlDays int, now time.Time) (map[string]bool, error) {
	p, err := dismissedPath(projectRoot)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var rec []dismissedRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rec))
	cutoff := now.Add(-time.Duration(ttlDays) * 24 * time.Hour)
	for _, r := range rec {
		ts, err := time.Parse(time.RFC3339, r.DismissedAt)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			out[r.ID] = true
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests -run TestAppendAndLoadDismissed -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/suggestions/dismissed.go tests/suggestions_test.go
git commit -m "feat(m9): suggestions dismissed-set with TTL filter"
```

---

## Task 14: Suggestion generators

**Files:**
- Create: `pkg/suggestions/generators.go`
- Test: `tests/suggestions_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `tests/suggestions_test.go`:

```go
import (
	"github.com/ranwei/mneme/pkg/suggestions"
	"sort"
)

func TestGenerateStaleAnatomy(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	// Create a real source file with mtime > anatomy mtime + 7d.
	srcPath := filepath.Join(root, "src.go")
	os.WriteFile(srcPath, []byte("package x\n"), 0644)
	os.Chtimes(srcPath, time.Now(), time.Now())

	// Anatomy entry with stale mtime.
	state.WriteAnatomy(root, []state.AnatomyEntry{
		{Path: "src.go", Description: "old", EstTokens: 10, Language: "go"},
	})
	// Backdate anatomy.md mtime by 30 days so it is older than the file by > 7d.
	anatomyPath := filepath.Join(root, ".mneme", "anatomy.md")
	old := time.Now().Add(-30 * 24 * time.Hour)
	os.Chtimes(anatomyPath, old, old)

	got := suggestions.GenStaleAnatomy(root, time.Now().UTC())
	if len(got) == 0 {
		t.Fatalf("expected stale_anatomy suggestion")
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Target < got[j].Target })
	if got[0].Type != "stale_anatomy" || got[0].Target != "src.go" {
		t.Errorf("got %+v", got[0])
	}
}

func TestGenerateStaleRule(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	state.AppendCerebrumRule(root, state.CerebrumRule{Pattern: "neverused", Message: "x"})

	// Backdate cerebrum.md mtime by 100 days.
	old := time.Now().Add(-100 * 24 * time.Hour)
	os.Chtimes(filepath.Join(root, ".mneme", "cerebrum.md"), old, old)

	got := suggestions.GenStaleRule(root, time.Now().UTC())
	if len(got) == 0 {
		t.Fatalf("expected stale_rule suggestion")
	}
	if got[0].Type != "stale_rule" {
		t.Errorf("got type %q", got[0].Type)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestGenerateStale -v`
Expected: FAIL — generators undefined.

- [ ] **Step 3: Implement**

Create `pkg/suggestions/generators.go`:

```go
package suggestions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

func newID(typ, target string) string {
	h := sha256.Sum256([]byte("v1|" + typ + "|" + target))
	return hex.EncodeToString(h[:8])
}

// GenStaleAnatomy emits one suggestion per file whose mtime is more than 7
// days newer than the anatomy.md file.
func GenStaleAnatomy(projectRoot string, now time.Time) []Suggestion {
	anatomyPath := filepath.Join(projectRoot, ".mneme", "anatomy.md")
	anatomyInfo, err := os.Stat(anatomyPath)
	if err != nil {
		return nil
	}
	entries, err := state.ReadAnatomy(projectRoot)
	if err != nil {
		return nil
	}
	var out []Suggestion
	cutoff := anatomyInfo.ModTime().Add(7 * 24 * time.Hour)
	for path := range entries {
		fi, err := os.Stat(filepath.Join(projectRoot, path))
		if err != nil {
			continue
		}
		if fi.ModTime().After(cutoff) {
			out = append(out, Suggestion{
				ID:          newID("stale_anatomy", path),
				Type:        "stale_anatomy",
				Target:      path,
				Title:       "Anatomy entry is stale",
				Detail:      fmt.Sprintf("%s changed after anatomy was generated; run: mneme scan", path),
				GeneratedAt: now.Format(time.RFC3339),
			})
		}
	}
	return out
}

// GenStaleRule emits one suggestion per cerebrum rule when cerebrum.md
// has not been touched in the last 90 days.
func GenStaleRule(projectRoot string, now time.Time) []Suggestion {
	rules, err := state.ReadCerebrum(projectRoot)
	if err != nil || len(rules) == 0 {
		return nil
	}
	info, err := os.Stat(filepath.Join(projectRoot, ".mneme", "cerebrum.md"))
	if err != nil {
		return nil
	}
	if now.Sub(info.ModTime()) < 90*24*time.Hour {
		return nil
	}
	var out []Suggestion
	for _, r := range rules {
		out = append(out, Suggestion{
			ID:          newID("stale_rule", r.Pattern),
			Type:        "stale_rule",
			Target:      r.Pattern,
			Title:       "Cerebrum rule has not matched in 90+ days",
			Detail:      fmt.Sprintf("rule %q may be obsolete; run: mneme cerebrum list", r.Pattern),
			GeneratedAt: now.Format(time.RFC3339),
		})
	}
	return out
}

// GenUnreadFile and GenCoRead are best-effort: they require session-archive
// history beyond the current `_session.json`. M9 does not ship a session
// archive writer; these generators return nil if no archive is found.
// The Refresh entry point silently skips them and emits a one-line note.
func GenUnreadFile(projectRoot string, now time.Time) []Suggestion {
	return nil
}

func GenCoRead(projectRoot string, now time.Time) []Suggestion {
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestGenerateStale -v`
Expected: PASS both.

- [ ] **Step 5: Commit**

```bash
git add pkg/suggestions/generators.go tests/suggestions_test.go
git commit -m "feat(m9): suggestion generators (stale_anatomy + stale_rule)"
```

---

## Task 15: Suggestions engine + persistence

**Files:**
- Create: `pkg/suggestions/engine.go`
- Test: `tests/suggestions_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `tests/suggestions_test.go`:

```go
func TestRefreshOverwritesAndDismissalFiltersAtRead(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	// Seed an anatomy + a stale source file so generator emits at least one item.
	srcPath := filepath.Join(root, "x.go")
	os.WriteFile(srcPath, []byte("package x\n"), 0644)
	state.WriteAnatomy(root, []state.AnatomyEntry{
		{Path: "x.go", Description: "old", EstTokens: 10, Language: "go"},
	})
	old := time.Now().Add(-30 * 24 * time.Hour)
	os.Chtimes(filepath.Join(root, ".mneme", "anatomy.md"), old, old)
	now := time.Now().UTC()

	all, err := suggestions.Refresh(root, now)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("expected ≥1 suggestion")
	}
	id := all[0].ID

	// Dismiss it. List should now exclude it.
	suggestions.AppendDismissed(root, id, now)
	got, err := suggestions.List(root, 30, now)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, s := range got {
		if s.ID == id {
			t.Errorf("dismissed id still present in List output")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestRefreshOverwritesAndDismissalFiltersAtRead -v`
Expected: FAIL — `suggestions.Refresh` undefined.

- [ ] **Step 3: Implement**

Create `pkg/suggestions/engine.go`:

```go
package suggestions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const refreshLockTimeout = 5 * time.Second

type Suggestion struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Target      string `json:"target"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	GeneratedAt string `json:"generated_at"`
}

func suggestionsPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "suggestions.json"), nil
}

// Refresh runs every generator and writes the unfiltered result to
// suggestions.json. Returns the unfiltered set.
func Refresh(projectRoot string, now time.Time) ([]Suggestion, error) {
	var all []Suggestion
	all = append(all, GenStaleAnatomy(projectRoot, now)...)
	all = append(all, GenStaleRule(projectRoot, now)...)
	all = append(all, GenUnreadFile(projectRoot, now)...)
	all = append(all, GenCoRead(projectRoot, now)...)

	p, err := suggestionsPath(projectRoot)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return nil, err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, refreshLockTimeout)
	if err != nil {
		return nil, err
	}
	defer release()

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := state.AtomicWrite(p, data); err != nil {
		return nil, err
	}
	return all, nil
}

// List reads suggestions.json and filters out IDs whose dismissed_at + ttl > now.
func List(projectRoot string, ttlDays int, now time.Time) ([]Suggestion, error) {
	p, err := suggestionsPath(projectRoot)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var all []Suggestion
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	suppressed, err := LoadDismissed(projectRoot, ttlDays, now)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, s := range all {
		if suppressed[s.ID] {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// HumanLine renders one suggestion as a single-line summary.
func HumanLine(s Suggestion) string {
	return fmt.Sprintf("[%s] %s — %s", s.ID[:8], s.Type, s.Title)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests -run TestRefreshOverwritesAndDismissalFiltersAtRead -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/suggestions/engine.go tests/suggestions_test.go
git commit -m "feat(m9): suggestions engine — Refresh + List with TTL filter"
```

---

## Task 16: `mneme suggestions` CLI

**Files:**
- Create: `cmd/cmd_suggestions.go`
- Modify: `cmd/main.go`
- Test: `tests/suggestions_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `tests/suggestions_test.go`. Extend the file's import block with `os/exec` and `strings`:

```go
func TestSuggestionsListJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)

	id, _ := state.ReadOrCreateLocalID(root)
	os.MkdirAll(state.GlobalProjectDir(id), 0755)
	body := `[{"id":"abcdef0123456789","type":"stale_rule","target":"foo","title":"x","detail":"y","generated_at":"2026-04-28T00:00:00Z"}]`
	os.WriteFile(filepath.Join(state.GlobalProjectDir(id), "suggestions.json"), []byte(body), 0644)

	cmd := exec.Command(testBinary(t), "suggestions", "list", "--json")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(string(out), "abcdef0123456789") {
		t.Errorf("missing ID in output: %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestSuggestionsListJSON -v`
Expected: FAIL — `unknown subcommand: suggestions`.

- [ ] **Step 3: Implement**

Create `cmd/cmd_suggestions.go`:

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/suggestions"
)

func dispatchSuggestions(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme suggestions <list|dismiss|refresh>")
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		runSuggestionsList(args[1:])
	case "dismiss":
		runSuggestionsDismiss(args[1:])
	case "refresh":
		runSuggestionsRefresh(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown suggestions subcommand: %q\n", args[0])
		os.Exit(2)
	}
}

func runSuggestionsList(args []string) {
	fs := flag.NewFlagSet("suggestions list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	root := mustProjectRoot()
	cfg := config.FromEnv()

	got, err := suggestions.List(root, cfg.SuggestionsDismissTTLDays, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ list:", err)
		os.Exit(1)
	}
	if *asJSON {
		data, _ := json.MarshalIndent(got, "", "  ")
		fmt.Println(string(data))
		return
	}
	if len(got) == 0 {
		fmt.Println("No suggestions.")
		return
	}
	for _, s := range got {
		fmt.Println(suggestions.HumanLine(s))
	}
}

func runSuggestionsDismiss(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme suggestions dismiss <id>")
		os.Exit(2)
	}
	root := mustProjectRoot()
	if err := suggestions.AppendDismissed(root, args[0], time.Now().UTC()); err != nil {
		fmt.Fprintln(os.Stderr, "✗ dismiss:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✓ dismissed", args[0])
}

func runSuggestionsRefresh(args []string) {
	root := mustProjectRoot()
	got, err := suggestions.Refresh(root, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ refresh:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ generated %d suggestion(s)\n", len(got))
}

func mustProjectRoot() string {
	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}
	return root
}
```

Add to `cmd/main.go` switch:

```go
	case "suggestions":
		dispatchSuggestions(os.Args[2:])
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./cmd && go test ./tests -run TestSuggestionsListJSON -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_suggestions.go cmd/main.go tests/suggestions_test.go
git commit -m "feat(m9): mneme suggestions list/dismiss/refresh CLI"
```

---

## Task 17: Daemon `suggestions-refresh` cron task

> Depends on M8 task registry, same as Task 12.

**Files:**
- Modify: `pkg/daemon/tasks.go`
- Modify: `pkg/daemon/manifest.go`
- Test: `tests/cerebrum_handler_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `tests/cerebrum_handler_test.go`:

```go
func TestSuggestionsRefreshTaskWrites(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(root)
	os.WriteFile(filepath.Join(state.GlobalProjectDir(id), "origin"), []byte(root+"\n"), 0644)

	tf, ok := daemon.LookupTask("suggestions-refresh")
	if !ok {
		t.Fatal("suggestions-refresh not registered")
	}
	if err := tf(); err != nil {
		t.Fatalf("task: %v", err)
	}
	p := filepath.Join(state.GlobalProjectDir(id), "suggestions.json")
	if _, err := os.Stat(p); err != nil {
		t.Errorf("suggestions.json missing: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestSuggestionsRefreshTaskWrites -v`
Expected: FAIL — task not registered.

- [ ] **Step 3: Add task wrapper + manifest entry**

In `pkg/daemon/tasks.go`, add:

```go
import (
	// ... existing ...
	"github.com/ranwei/mneme/pkg/suggestions"
)

func runSuggestionsRefresh() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectsDir := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		originBytes, err := os.ReadFile(filepath.Join(projectsDir, e.Name(), "origin"))
		root := strings.TrimSpace(string(originBytes))
		if err != nil || root == "" {
			// Pre-M7 fallback: use the global project dir as the project root.
			root = filepath.Join(projectsDir, e.Name())
		}
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_, _ = suggestions.Refresh(root, time.Now().UTC())
	}
	return nil
}

func init() {
	registerTask("suggestions-refresh", runSuggestionsRefresh)
}
```

In `pkg/daemon/manifest.go` seed list, add:

```go
	{Name: "suggestions-refresh", Schedule: "0 4 * * *"}, // daily 04:00 local
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests -run TestSuggestionsRefreshTaskWrites -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon/tasks.go pkg/daemon/manifest.go tests/cerebrum_handler_test.go
git commit -m "feat(m9): daemon suggestions-refresh cron task"
```

---

## Task 18: Envcheck library

**Files:**
- Create: `pkg/envcheck/envcheck.go`
- Test: `tests/envcheck_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/envcheck_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/envcheck"
)

func TestDetectPackageManagerPnpm(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.PackageManager != "pnpm" {
		t.Errorf("PackageManager: got %q, want pnpm", r.PackageManager)
	}
}

func TestDetectFrameworkNext(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "next.config.js"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.Framework != "next" {
		t.Errorf("Framework: got %q, want next", r.Framework)
	}
	if r.DevServerPort != 3000 {
		t.Errorf("DevServerPort: got %d, want 3000", r.DevServerPort)
	}
}

func TestDetectFrameworkVite(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "vite.config.ts"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.Framework != "vite" {
		t.Errorf("Framework: got %q, want vite", r.Framework)
	}
	if r.DevServerPort != 5173 {
		t.Errorf("DevServerPort: got %d, want 5173", r.DevServerPort)
	}
}

func TestDetectFrameworkAstro(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "astro.config.mjs"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.Framework != "astro" {
		t.Errorf("Framework: got %q, want astro", r.Framework)
	}
}

func TestDetectChromePathFromEnv(t *testing.T) {
	root := t.TempDir()
	chrome := filepath.Join(root, "chrome-stub")
	os.WriteFile(chrome, []byte(""), 0755)
	t.Setenv("CHROME_PATH", chrome)
	r := envcheck.Detect(root)
	if r.ChromePath != chrome {
		t.Errorf("ChromePath: got %q, want %q", r.ChromePath, chrome)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestDetect -v`
Expected: FAIL — package not found.

- [ ] **Step 3: Implement**

Create `pkg/envcheck/envcheck.go`:

```go
package envcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Result struct {
	ChromePath     string
	PackageManager string
	DevServerPort  int
	Framework      string
	DetectedAt     time.Time
}

func Detect(projectRoot string) Result {
	r := Result{DetectedAt: time.Now().UTC()}
	r.ChromePath = findChrome()
	r.PackageManager = findPackageManager(projectRoot)
	r.Framework, r.DevServerPort = findFramework(projectRoot)
	return r
}

func findChrome() string {
	if env := os.Getenv("CHROME_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	candidates := platformChromeCandidates()
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func platformChromeCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "linux":
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		}
	}
	return nil
}

func findPackageManager(root string) string {
	for _, c := range []struct {
		file, mgr string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"package-lock.json", "npm"},
	} {
		if _, err := os.Stat(filepath.Join(root, c.file)); err == nil {
			return c.mgr
		}
	}
	return ""
}

func findFramework(root string) (string, int) {
	cfgs := []struct {
		patterns  []string
		framework string
		port      int
	}{
		{[]string{"next.config.js", "next.config.ts", "next.config.mjs"}, "next", 3000},
		{[]string{"vite.config.js", "vite.config.ts"}, "vite", 5173},
		{[]string{"astro.config.mjs", "astro.config.ts"}, "astro", 4321},
		{[]string{"svelte.config.js"}, "sveltekit", 5173},
	}
	for _, c := range cfgs {
		for _, p := range c.patterns {
			if _, err := os.Stat(filepath.Join(root, p)); err == nil {
				return c.framework, c.port
			}
		}
	}
	return "", 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestDetect -v`
Expected: PASS all five.

- [ ] **Step 5: Commit**

```bash
git add pkg/envcheck/envcheck.go tests/envcheck_test.go
git commit -m "feat(m9): envcheck library — Chrome + package mgr + framework"
```

---

## Task 19: Integration smoke test

**Files:**
- Create: `tests/integration/cerebrum_learn_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/integration/cerebrum_learn_test.go`:

```go
//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/state"
)

func TestEndToEndCerebrumLearn(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)
	id, _ := state.ReadOrCreateLocalID(root)
	os.WriteFile(filepath.Join(state.GlobalProjectDir(id), "origin"), []byte(root+"\n"), 0644)

	transcript := filepath.Join(tmp, "english.jsonl")
	src, _ := os.ReadFile(filepath.Join("..", "golden", "m9", "transcripts", "english.jsonl"))
	os.WriteFile(transcript, src, 0644)

	bin := buildBinary(t)
	dctx, _ := exec.LookPath(bin)
	_ = dctx

	// Start daemon.
	dCmd := exec.Command(bin, "daemon", "start")
	dCmd.Env = append(os.Environ(), "HOME="+tmp)
	if err := dCmd.Start(); err != nil {
		t.Fatalf("daemon start: %v", err)
	}
	defer func() {
		dCmd.Process.Signal(os.Interrupt)
		dCmd.Wait()
	}()

	socket := filepath.Join(tmp, ".mneme", "daemon", "socket")
	waitForSocket(t, socket, 3*time.Second)

	// Hit /cerebrum/learn directly.
	tr := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.Dial("unix", socket)
		},
	}
	client := &http.Client{Transport: tr, Timeout: 2 * time.Second}
	body, _ := json.Marshal(map[string]string{
		"project_id":      id,
		"transcript_path": transcript,
		"session_id":      "s1",
	})
	resp, err := client.Post("http://unix/cerebrum/learn", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", resp.StatusCode)
	}

	// Wait for worker to drain.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		cs, _ := cerebrum.LoadPending(root)
		if len(cs) > 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("no candidates queued in 5s")
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mneme")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func waitForSocket(t *testing.T, p string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(p); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("socket not present after %v: %s", timeout, p)
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test -tags=integration ./tests/integration -run TestEndToEndCerebrumLearn -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/cerebrum_learn_test.go
git commit -m "test(m9): integration smoke for /cerebrum/learn end-to-end"
```

---

## Task 20: Acceptance verification + spec checkbox tick

**Files:**
- Modify: `docs/superpowers/specs/2026-04-28-m9-intelligence-loop-design.md`

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

Also: `go test -tags=integration ./tests/integration`
Expected: PASS.

- [ ] **Step 2: Run lint / vet**

Run: `go vet ./...`
Expected: no findings.

- [ ] **Step 3: Manual smoke (one project)**

In a real mneme-initialized project:

```bash
mneme cerebrum review --list --json    # → []
mneme suggestions list --json          # → null or [...]
mneme report waste --dry-run           # → markdown to stdout
mneme report waste                     # → writes .mneme/reports/waste-*.md
```

If the daemon is running:

```bash
curl --unix-socket ~/.mneme/daemon/socket http://unix/cron/list
# weekly-waste-report and suggestions-refresh should appear in the list
```

- [ ] **Step 4: Tick acceptance checkboxes in spec**

Edit `docs/superpowers/specs/2026-04-28-m9-intelligence-loop-design.md` §11 Acceptance Criteria — change every `- [ ]` to `- [x]` only for items that have been verified above.

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/specs/2026-04-28-m9-intelligence-loop-design.md
git commit -m "docs(m9): tick acceptance criteria after end-to-end verification"
```

---

## Self-review

**Spec coverage map:**

| Spec section | Implementing task(s) |
|---|---|
| §3.1 Trigger flow | 9 (stop hook), 8 (daemon) |
| §3.2 Public types | 5, 6 |
| §3.3 Heuristic v1 | 4 (triggers), 6 (Learn) |
| §3.4 Dedup & pre-emption | 5 (AppendPending), 6 (Learn) |
| §3.5 `mneme cerebrum review` | 7 |
| §3.6 Daemon endpoint | 8 |
| §4.1–§4.3 Waste report | 10, 11, 12 |
| §4.4 Ledger history | 2, 3 |
| §5.1–§5.4 Suggestions | 13, 14, 15, 16 |
| §5.5 unread_file limitation | 14 (returns nil + note) |
| §6 Envcheck | 18 |
| §7 Configuration | 1 |
| §8 Testing strategy | 2, 4–6, 10, 13–15, 18 unit; 8, 11, 12, 16, 17 handler/CLI; 19 integration |
| §10 Open questions | n/a (deferred) |
| §11 Acceptance criteria | 20 |

**Test coverage gap (non-blocking):**
- `unread_file` and `co_read` generators are stubs returning `nil`. The spec acknowledges this in §5.5; tests verify the engine pipeline still works even when generators emit nothing. A future M9.5 task can add session-archive support.
- Daemon-process restart after task registration is not tested in M9; M8's tests already cover scheduler restart.

---

**Roadmap reference:** §3 M9 — Intelligence Loop (`docs/superpowers/specs/2026-04-28-m7-m11-roadmap-design.md`).
**Spec:** `docs/superpowers/specs/2026-04-28-m9-intelligence-loop-design.md`.
