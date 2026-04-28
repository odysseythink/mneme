# M5: Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a waste-pattern detector (`pkg/waste`) and `mneme stats --waste` / `--json` flags that diagnose 5 inefficiency patterns from local project state.

**Architecture:** New package `pkg/waste` exposes `Detect(projectRoot, homeDir string) ([]WastePattern, error)` which reads the ledger, anatomy, memory, and cerebrum files and evaluates 5 rules. `cmd/cmd_stats.go` grows `--waste` (text report) and `--json` (machine-readable) flags backed by this function. No embedding API needed — all reads are local state files.

**Tech Stack:** Go stdlib, `pkg/state` (ReadLedger/ReadAnatomy/ReadMemory/ReadCerebrum), `encoding/json`, `flag`.

**Prerequisites:** M1–M4c must be complete (state layer, ledger, anatomy, cerebrum, buglog all present).

---

## File Structure

### New files
```
pkg/waste/detector.go          ~90 LOC  WastePattern struct + Detect() + 5 check fns
tests/waste_test.go            ~220 LOC unit tests for all 5 patterns (14 tests)
tests/integration/stats_flags_test.go  ~90 LOC integration: --waste and --json CLI flags
```

### Modified files
```
cmd/cmd_stats.go               add --waste, --json flags + statsReport JSON struct
```

---

## ✅ Task 1: pkg/waste/detector.go — 5 waste-pattern checks

> **Status: COMPLETE.** `pkg/waste/detector.go` exists and all 14 unit tests in `tests/waste_test.go` pass.

**Files:**
- Create: `pkg/waste/detector.go`
- Create: `tests/waste_test.go`

### The 5 patterns

| Name | Detected when | Severity |
|------|--------------|----------|
| `repeated_reads` | `ledger.RepeatReads > 0` | warn |
| `large_reads_with_anatomy` | `ledger.AnatomyHits > 0` AND any anatomy entry has `EstTokens > 500` | info |
| `memory_bloat` | `len(memoryRows) > 50` | warn |
| `cerebrum_staleness` | `cerebrum.md` exists AND mtime > 14 days ago | warn |
| `anatomy_miss_rate` | `HookFired["pre-read"] >= 10` AND `AnatomyHits / HookFired["pre-read"] < 0.8` | warn |

### `pkg/waste/detector.go` (complete source)

```go
package waste

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const (
	largeEntryTokenThreshold = 500
	memoryBloatThreshold     = 50
	cerebrumStalenessDays    = 14
	anatomyMissRateMinReads  = 10
	anatomyHitRateThreshold  = 0.8
)

// WastePattern describes a single detected (or not) inefficiency.
type WastePattern struct {
	Name     string `json:"name"`
	Detected bool   `json:"detected"`
	Severity string `json:"severity"` // "warn" or "info"
	Details  string `json:"details,omitempty"`
}

// Detect runs all 5 waste checks and returns one WastePattern per check.
// projectRoot is the repo root (containing .mneme/).
// homeDir is the user's home directory (containing .claude/).
func Detect(projectRoot, homeDir string) ([]WastePattern, error) {
	l, err := state.ReadLedger(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	anatomy, _ := state.ReadAnatomy(projectRoot)
	memRows, _ := state.ReadMemory(homeDir)

	return []WastePattern{
		checkRepeatedReads(l),
		checkLargeReadsWithAnatomy(l, anatomy),
		checkMemoryBloat(memRows),
		checkCerebrumStaleness(projectRoot),
		checkAnatomyMissRate(l),
	}, nil
}

func checkRepeatedReads(l *state.Ledger) WastePattern {
	detected := l.Totals.RepeatReads > 0
	details := ""
	if detected {
		details = fmt.Sprintf("%d repeated file read(s) recorded — same files read multiple times in a session; check pre-read hook anatomy coverage", l.Totals.RepeatReads)
	}
	return WastePattern{Name: "repeated_reads", Detected: detected, Severity: "warn", Details: details}
}

func checkLargeReadsWithAnatomy(l *state.Ledger, anatomy map[string]state.AnatomyEntry) WastePattern {
	largeCount := 0
	for _, e := range anatomy {
		if e.EstTokens > largeEntryTokenThreshold {
			largeCount++
		}
	}
	detected := l.Totals.AnatomyHits > 0 && largeCount > 0
	details := ""
	if detected {
		details = fmt.Sprintf("%d large file(s) (>%d tok) have anatomy descriptions; anatomy consulted %d time(s) — ensure Claude trusts anatomy to avoid full reads", largeCount, largeEntryTokenThreshold, l.Totals.AnatomyHits)
	}
	return WastePattern{Name: "large_reads_with_anatomy", Detected: detected, Severity: "info", Details: details}
}

func checkMemoryBloat(rows []state.MemoryRow) WastePattern {
	detected := len(rows) > memoryBloatThreshold
	details := ""
	if detected {
		details = fmt.Sprintf("memory has %d session rows (threshold: %d) — run: mneme memory consolidate (M6)", len(rows), memoryBloatThreshold)
	}
	return WastePattern{Name: "memory_bloat", Detected: detected, Severity: "warn", Details: details}
}

func checkCerebrumStaleness(projectRoot string) WastePattern {
	path := filepath.Join(projectRoot, ".mneme", "cerebrum.md")
	info, err := os.Stat(path)
	if err != nil {
		return WastePattern{Name: "cerebrum_staleness", Detected: false, Severity: "warn"}
	}
	days := int(time.Since(info.ModTime()).Hours() / 24)
	detected := days > cerebrumStalenessDays
	details := ""
	if detected {
		details = fmt.Sprintf("cerebrum.md last updated %d day(s) ago (threshold: %d) — review rules with: mneme cerebrum list", days, cerebrumStalenessDays)
	}
	return WastePattern{Name: "cerebrum_staleness", Detected: detected, Severity: "warn", Details: details}
}

func checkAnatomyMissRate(l *state.Ledger) WastePattern {
	total := l.Totals.HookFired["pre-read"]
	if total < anatomyMissRateMinReads {
		return WastePattern{Name: "anatomy_miss_rate", Detected: false, Severity: "warn"}
	}
	hitRate := float64(l.Totals.AnatomyHits) / float64(total)
	detected := hitRate < anatomyHitRateThreshold
	details := ""
	if detected {
		details = fmt.Sprintf("anatomy hit rate %.0f%% (miss rate %.0f%%, threshold: ≥80%%) — run: mneme scan to add missing files", hitRate*100, (1-hitRate)*100)
	}
	return WastePattern{Name: "anatomy_miss_rate", Detected: detected, Severity: "warn", Details: details}
}
```

### `tests/waste_test.go` (complete source, 14 tests)

```go
package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

func setupWasteProject(t *testing.T) (projectRoot, homeDir string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mneme"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return dir, t.TempDir()
}

func findPattern(patterns []waste.WastePattern, name string) (waste.WastePattern, bool) {
	for _, p := range patterns {
		if p.Name == name {
			return p, true
		}
	}
	return waste.WastePattern{}, false
}

func TestWasteRepeatedReadsDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	for i := 0; i < 3; i++ {
		state.IncrementSafe(dir, "repeat_reads")
	}
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "repeated_reads")
	if !ok {
		t.Fatal("pattern 'repeated_reads' not in result")
	}
	if !p.Detected {
		t.Errorf("expected Detected=true for repeated_reads=3")
	}
	if p.Details == "" {
		t.Errorf("expected non-empty Details when detected")
	}
}

func TestWasteRepeatedReadsNotDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "repeated_reads")
	if !ok {
		t.Fatal("pattern 'repeated_reads' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false when RepeatReads=0")
	}
}

func TestWasteLargeReadsWithAnatomyDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	state.WriteAnatomy(dir, []state.AnatomyEntry{
		{Path: "big.go", Description: "large file", EstTokens: 2000, Language: "go"},
		{Path: "small.go", Description: "small file", EstTokens: 50, Language: "go"},
	})
	state.IncrementSafe(dir, "anatomy_hits")
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "large_reads_with_anatomy")
	if !ok {
		t.Fatal("pattern 'large_reads_with_anatomy' not in result")
	}
	if !p.Detected {
		t.Errorf("expected Detected=true: anatomy has large entry and AnatomyHits>0")
	}
}

func TestWasteLargeReadsWithAnatomyNotDetectedNoHits(t *testing.T) {
	dir, home := setupWasteProject(t)
	state.WriteAnatomy(dir, []state.AnatomyEntry{
		{Path: "big.go", Description: "large file", EstTokens: 2000, Language: "go"},
	})
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "large_reads_with_anatomy")
	if p.Detected {
		t.Errorf("expected Detected=false when AnatomyHits=0")
	}
}

func TestWasteLargeReadsWithAnatomyNotDetectedSmallFiles(t *testing.T) {
	dir, home := setupWasteProject(t)
	state.WriteAnatomy(dir, []state.AnatomyEntry{
		{Path: "small.go", Description: "small file", EstTokens: 100, Language: "go"},
	})
	state.IncrementSafe(dir, "anatomy_hits")
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "large_reads_with_anatomy")
	if p.Detected {
		t.Errorf("expected Detected=false when all files EstTokens<=500")
	}
}

func TestWasteMemoryBloatDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	for i := 0; i < 51; i++ {
		state.AppendMemoryRow(home, state.MemoryRow{
			StartedAt:     "2026-04-01T10:00:00Z",
			TurnCount:     1,
			PatternCounts: map[string]int{},
		})
	}
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "memory_bloat")
	if !p.Detected {
		t.Errorf("expected Detected=true for 51 memory rows")
	}
}

func TestWasteMemoryBloatNotDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	for i := 0; i < 5; i++ {
		state.AppendMemoryRow(home, state.MemoryRow{
			StartedAt:     "2026-04-01T10:00:00Z",
			TurnCount:     1,
			PatternCounts: map[string]int{},
		})
	}
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "memory_bloat")
	if p.Detected {
		t.Errorf("expected Detected=false for 5 memory rows")
	}
}

func TestWasteCerebrumStalenessDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	state.AppendCerebrumRule(dir, state.CerebrumRule{
		Pattern: `fmt\.Println\(`,
		Message: "use structured logger",
	})
	cerPath := filepath.Join(dir, ".mneme", "cerebrum.md")
	old := time.Now().Add(-15 * 24 * time.Hour)
	os.Chtimes(cerPath, old, old)
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "cerebrum_staleness")
	if !p.Detected {
		t.Errorf("expected Detected=true for cerebrum.md 15 days old")
	}
}

func TestWasteCerebrumStalenessNotDetectedRecent(t *testing.T) {
	dir, home := setupWasteProject(t)
	state.AppendCerebrumRule(dir, state.CerebrumRule{
		Pattern: `fmt\.Println\(`,
		Message: "use structured logger",
	})
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "cerebrum_staleness")
	if p.Detected {
		t.Errorf("expected Detected=false for fresh cerebrum.md")
	}
}

func TestWasteCerebrumStalenessNotDetectedMissing(t *testing.T) {
	dir, home := setupWasteProject(t)
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "cerebrum_staleness")
	if p.Detected {
		t.Errorf("expected Detected=false when cerebrum.md is absent")
	}
}

func TestWasteAnatomyMissRateDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	for i := 0; i < 20; i++ {
		state.IncrementSafe(dir, "hook_fired.pre-read")
	}
	for i := 0; i < 10; i++ {
		state.IncrementSafe(dir, "anatomy_hits")
	}
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "anatomy_miss_rate")
	if !p.Detected {
		t.Errorf("expected Detected=true: 10/20=50%% hit rate < 80%%")
	}
}

func TestWasteAnatomyMissRateNotDetectedHighHitRate(t *testing.T) {
	dir, home := setupWasteProject(t)
	for i := 0; i < 20; i++ {
		state.IncrementSafe(dir, "hook_fired.pre-read")
	}
	for i := 0; i < 18; i++ {
		state.IncrementSafe(dir, "anatomy_hits")
	}
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "anatomy_miss_rate")
	if p.Detected {
		t.Errorf("expected Detected=false: 18/20=90%% >= 80%%")
	}
}

func TestWasteAnatomyMissRateNotDetectedFewReads(t *testing.T) {
	dir, home := setupWasteProject(t)
	for i := 0; i < 5; i++ {
		state.IncrementSafe(dir, "hook_fired.pre-read")
	}
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, _ := findPattern(patterns, "anatomy_miss_rate")
	if p.Detected {
		t.Errorf("expected Detected=false: < 10 pre-reads, insufficient data")
	}
}

func TestWasteDetectAlwaysReturns5Patterns(t *testing.T) {
	dir, home := setupWasteProject(t)
	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(patterns) != 5 {
		t.Errorf("expected exactly 5 patterns, got %d", len(patterns))
	}
	names := map[string]bool{
		"repeated_reads": false, "large_reads_with_anatomy": false,
		"memory_bloat": false, "cerebrum_staleness": false, "anatomy_miss_rate": false,
	}
	for _, p := range patterns {
		names[p.Name] = true
	}
	for name, present := range names {
		if !present {
			t.Errorf("expected pattern %q in result", name)
		}
	}
}
```

- [x] **Step 1: Write `tests/waste_test.go`** (14 tests as above)
- [x] **Step 2: Run tests to verify they fail** — `go test ./tests/ -run TestWaste -v` → FAIL (pkg/waste missing)
- [x] **Step 3: Create `pkg/waste/detector.go`** (full source above)
- [x] **Step 4: Run tests to verify they pass** — all 14 PASS
- [x] **Step 5: Commit**

```bash
git add pkg/waste/detector.go tests/waste_test.go
git commit -m "feat(m5): add waste detector with 5 diagnostic patterns"
```

---

## ✅ Task 2: Integration tests for `stats --waste` and `stats --json`

> **Status: COMPLETE (RED).** `tests/integration/stats_flags_test.go` exists with 3 tests that currently fail because `--waste` and `--json` flags are not yet implemented.

**Files:**
- Create: `tests/integration/stats_flags_test.go`
- (Uses `binaryPath` from `TestMain` in `tests/integration/init_lifecycle_test.go`)

### `tests/integration/stats_flags_test.go` (complete source)

```go
package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupStatsProject(t *testing.T) (projectDir, homeDir string) {
	t.Helper()
	proj := t.TempDir()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".mneme"), 0755); err != nil {
		t.Fatalf("mkdir .mneme: %v", err)
	}
	localIDPath := filepath.Join(proj, ".mneme", ".local-id")
	if err := os.WriteFile(localIDPath, []byte("test-uuid-stats-1234"), 0644); err != nil {
		t.Fatalf("write .local-id: %v", err)
	}
	return proj, home
}

func runStats(t *testing.T, projectDir, homeDir string, args ...string) (stdout string, code int) {
	t.Helper()
	cmdArgs := append([]string{"stats"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "HOME="+homeDir)
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	return string(out), 0
}

func TestStatsWasteFlag(t *testing.T) {
	proj, home := setupStatsProject(t)
	out, code := runStats(t, proj, home, "--waste")
	if code != 0 {
		t.Fatalf("stats --waste exited %d: %s", code, out)
	}
	if !strings.Contains(out, "Waste Patterns") {
		t.Errorf("expected 'Waste Patterns' section in output, got:\n%s", out)
	}
	if !strings.Contains(out, "repeated_reads") {
		t.Errorf("expected 'repeated_reads' pattern name, got:\n%s", out)
	}
	if !strings.Contains(out, "anatomy_miss_rate") {
		t.Errorf("expected 'anatomy_miss_rate' pattern name, got:\n%s", out)
	}
}

func TestStatsJSONFlag(t *testing.T) {
	proj, home := setupStatsProject(t)
	out, code := runStats(t, proj, home, "--json")
	if code != 0 {
		t.Fatalf("stats --json exited %d: %s", code, out)
	}
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stats --json output is not valid JSON: %v\noutput: %s", err, out)
	}
	if _, ok := report["project_id"]; !ok {
		t.Errorf("expected 'project_id' field in JSON, got: %s", out)
	}
	if _, ok := report["totals"]; !ok {
		t.Errorf("expected 'totals' field in JSON, got: %s", out)
	}
	if _, ok := report["memory_rows"]; !ok {
		t.Errorf("expected 'memory_rows' field in JSON, got: %s", out)
	}
}

func TestStatsJSONWithWasteFlag(t *testing.T) {
	proj, home := setupStatsProject(t)
	out, code := runStats(t, proj, home, "--json", "--waste")
	if code != 0 {
		t.Fatalf("stats --json --waste exited %d: %s", code, out)
	}
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %s", err, out)
	}
	wasteRaw, ok := report["waste"]
	if !ok {
		t.Fatalf("expected 'waste' field in JSON when --waste is set, got: %s", out)
	}
	wasteSlice, ok := wasteRaw.([]interface{})
	if !ok {
		t.Fatalf("expected 'waste' to be an array, got: %T", wasteRaw)
	}
	if len(wasteSlice) != 5 {
		t.Errorf("expected 5 waste patterns in JSON, got %d", len(wasteSlice))
	}
}
```

- [x] **Step 1: Write `tests/integration/stats_flags_test.go`** (3 tests as above)
- [x] **Step 2: Run tests to verify they fail**

```bash
go test ./tests/integration/ -run TestStats -v
```

Expected: FAIL — `--waste` and `--json` not yet recognized, output is plain text.

---

## ✅ Task 3: `cmd/cmd_stats.go` — add `--waste` and `--json` flags

**Files:**
- Modify: `cmd/cmd_stats.go`

Replace the entire file with this implementation:

- [x] **Step 1: Rewrite `cmd/cmd_stats.go`**

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

type statsReport struct {
	ProjectID     string               `json:"project_id"`
	FirstRecorded string               `json:"first_recorded,omitempty"`
	LastUpdated   string               `json:"last_updated,omitempty"`
	Totals        state.LedgerTotals   `json:"totals"`
	MemoryRows    int                  `json:"memory_rows"`
	Waste         []waste.WastePattern `json:"waste,omitempty"`
}

func dispatchStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	flagWaste := fs.Bool("waste", false, "show waste diagnostic patterns")
	flagJSON := fs.Bool("json", false, "output stats as JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stats: cannot get cwd:", err)
		os.Exit(1)
	}

	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "stats: not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}

	l, err := state.ReadLedger(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stats: read ledger:", err)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	rows, _ := state.ReadMemory(home)

	var wastePatterns []waste.WastePattern
	if *flagWaste {
		wastePatterns, _ = waste.Detect(root, home)
	}

	if *flagJSON {
		report := statsReport{
			ProjectID:     l.ProjectID,
			FirstRecorded: l.FirstRecorded,
			LastUpdated:   l.LastUpdated,
			Totals:        l.Totals,
			MemoryRows:    len(rows),
			Waste:         wastePatterns,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(report) //nolint:errcheck
		return
	}

	// Text output
	fmt.Printf("project: %s\n", l.ProjectID)
	if l.FirstRecorded != "" {
		fmt.Printf("first recorded: %s\n", l.FirstRecorded)
	}
	fmt.Println()

	fmt.Println("=== Hook Totals ===")
	for _, k := range []string{"pre-read", "pre-write", "post-tool-use", "session-start", "stop"} {
		fmt.Printf("  %-24s %d\n", k+":", l.Totals.HookFired[k])
	}
	fmt.Printf("  %-24s %d\n", "hook_errors:", l.Totals.HookErrors)
	fmt.Printf("  %-24s %d\n", "stdin_parse_failures:", l.Totals.StdinParseFailures)
	fmt.Printf("  %-24s %d\n", "outside_project_skipped:", l.Totals.OutsideProjectSkipped)

	if len(l.Totals.EditPatterns) > 0 {
		fmt.Println()
		fmt.Println("=== Edit Patterns ===")
		cats := make([]string, 0, len(l.Totals.EditPatterns))
		for cat := range l.Totals.EditPatterns {
			cats = append(cats, cat)
		}
		sort.Slice(cats, func(i, j int) bool {
			return l.Totals.EditPatterns[cats[i]] > l.Totals.EditPatterns[cats[j]]
		})
		for _, cat := range cats {
			fmt.Printf("  %-20s %d\n", cat+":", l.Totals.EditPatterns[cat])
		}
	}

	if len(rows) > 0 {
		fmt.Println()
		fmt.Println("=== Recent Sessions (last 5) ===")
		start := 0
		if len(rows) > 5 {
			start = len(rows) - 5
		}
		for _, r := range rows[start:] {
			patterns := formatPatternCounts(r.PatternCounts)
			fmt.Printf("  %-32s %2d turns  %s\n", r.StartedAt, r.TurnCount, patterns)
		}
		fmt.Println()
		fmt.Println("=== Memory ===")
		fmt.Printf("  rows written:      %d\n", l.Totals.MemoryRowsWritten)
		fmt.Printf("  memory.md:         %s/.claude/mneme-memory.md\n", home)
	}

	if *flagWaste {
		fmt.Println()
		fmt.Println("=== Waste Patterns ===")
		for _, p := range wastePatterns {
			tag := "[OK]  "
			if p.Detected {
				tag = "[WARN]"
			}
			if p.Details != "" {
				fmt.Printf("  %s %s — %s\n", tag, p.Name, p.Details)
			} else {
				fmt.Printf("  %s %s\n", tag, p.Name)
			}
		}
	}
}

func formatPatternCounts(m map[string]int) string {
	if len(m) == 0 {
		return "(no edits)"
	}
	cats := make([]string, 0, len(m))
	for cat := range m {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool {
		return m[cats[i]] > m[cats[j]]
	})
	parts := make([]string, 0, len(cats))
	for _, cat := range cats {
		parts = append(parts, fmt.Sprintf("%s×%d", cat, m[cat]))
	}
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, ", ")
}
```

- [x] **Step 2: Build to verify it compiles**

```bash
go build -o ./bin/mneme ./cmd
```

Expected: success.

- [x] **Step 3: Run the integration tests**

```bash
go test ./tests/integration/ -run TestStats -v
```

Expected: all 3 PASS.

- [x] **Step 4: Run the full test suite**

```bash
go test ./...
```

Expected: all pass.

- [x] **Step 5: Smoke-test the binary manually**

```bash
./bin/mneme stats --waste
./bin/mneme stats --json
./bin/mneme stats --json --waste | python3 -m json.tool
```

Expected:
- `--waste` shows `=== Waste Patterns ===` with 5 named patterns
- `--json` outputs valid JSON with `project_id`, `totals`, `memory_rows`
- `--json --waste` JSON includes `"waste": [...]` with 5 elements

- [x] **Step 6: Commit**

```bash
git add cmd/cmd_stats.go tests/integration/stats_flags_test.go
git commit -m "feat(m5): add stats --waste and --json flags"
```

---

## Self-Review

**Spec coverage:**
- ✅ `pkg/waste/detector.go` with 5 patterns — Task 1
- ✅ `repeated_reads` — ledger.RepeatReads > 0 — Task 1
- ✅ `large_reads_with_anatomy` — AnatomyHits > 0 AND large anatomy entries — Task 1
- ✅ `memory_bloat` — > 50 memory rows — Task 1
- ✅ `cerebrum_staleness` — mtime > 14 days — Task 1
- ✅ `anatomy_miss_rate` — hit rate < 80% (min 10 reads) — Task 1
- ✅ All 5 patterns detected on synthesized fixtures (14 unit tests) — Task 1
- ✅ `mneme stats --waste` text report — Task 3
- ✅ `mneme stats --json` machine-readable output — Task 3
- ✅ `mneme stats --json --waste` JSON with waste array — Task 3

**Placeholder scan:** No TBD/TODO in any step. All code is complete.

**Type consistency:**
- `waste.WastePattern` defined in `pkg/waste/detector.go` and used in `statsReport.Waste []waste.WastePattern` ✓
- `state.LedgerTotals` used directly in `statsReport.Totals` — has JSON tags in pkg/state/ledger.go ✓
- `waste.Detect(root, home string) ([]WastePattern, error)` signature matches all call sites ✓
- `statsReport` struct field `MemoryRows int json:"memory_rows"` matches test assertion `report["memory_rows"]` ✓
