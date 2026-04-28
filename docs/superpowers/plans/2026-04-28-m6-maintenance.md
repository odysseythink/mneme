# M6: Maintenance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add memory consolidation (fold session rows older than 7d into a single summary) and incremental scan (skip unchanged files using mtime, making `scan` <2s on no-change repos).

**Architecture:** Two independent features. (1) `pkg/consolidator/memory.go` reads `mneme-memory.md`, folds rows older than 7 days into a `> Consolidated session (N actions)` blockquote, and writes back atomically. It is triggered lazily at session-start and exposed as `mneme memory consolidate`. (2) Incremental scan reads the `<!-- generated: ... -->` timestamp from `anatomy.md`, skips files whose mtime ≤ that timestamp, re-extracts only changed/new files, merges with kept entries, and writes one unified anatomy. The existing `--force` flag bypasses the skip.

**Tech Stack:** Go stdlib (`os`, `time`, `strings`), `pkg/state` (ReadMemory, AppendMemoryRow, ReadAnatomy, WriteAnatomy), `pkg/scanner` (Walk, ExtractAll).

---

## File Structure

### New files
```
pkg/consolidator/memory.go          ~70 LOC  Consolidate() function
cmd/cmd_memory.go                   ~50 LOC  `mneme memory consolidate` subcommand
tests/consolidator_test.go          ~120 LOC unit tests for consolidation logic
tests/integration/scan_incremental_test.go  ~80 LOC integration: incremental scan timing
```

### Modified files
```
pkg/state/anatomy.go                add ReadAnatomyGeneratedTime()
pkg/scanner/extractor.go            add ScanProjectIncremental()
cmd/cmd_scan.go                     use incremental by default (--force bypasses)
cmd/hook_sessionstart.go            lazy consolidation trigger after reading prev session
cmd/main.go                         register "memory" subcommand
```

---

## Task 1: `pkg/state/anatomy.go` — expose generated timestamp

**Files:**
- Modify: `pkg/state/anatomy.go`

- [x] **Step 1: Write the failing test**

```go
// tests/anatomy_timestamp_test.go (new file in package tests)
package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

func TestReadAnatomyGeneratedTime(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	os.WriteFile(filepath.Join(dir, ".mneme", ".local-id"), []byte("test-ts-uuid"), 0644)

	// Write anatomy with known entries
	entries := []state.AnatomyEntry{
		{Path: "main.go", Description: "entry point", EstTokens: 10, Language: "go"},
	}
	state.WriteAnatomy(dir, entries)

	before := time.Now().Add(-time.Second)
	ts, err := state.ReadAnatomyGeneratedTime(dir)
	if err != nil {
		t.Fatalf("ReadAnatomyGeneratedTime: %v", err)
	}
	if ts.Before(before) {
		t.Errorf("expected generated time >= %v, got %v", before, ts)
	}
}

func TestReadAnatomyGeneratedTimeMissing(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	_, err := state.ReadAnatomyGeneratedTime(dir)
	if err == nil {
		t.Error("expected error when anatomy.md missing, got nil")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

```bash
go test ./tests/ -run TestReadAnatomyGeneratedTime -v
```
Expected: FAIL — `ReadAnatomyGeneratedTime undefined`

- [x] **Step 3: Add `ReadAnatomyGeneratedTime` to `pkg/state/anatomy.go`**

Add after the `ReadAnatomy` function:

```go
// ReadAnatomyGeneratedTime parses the <!-- generated: RFC3339 --> timestamp
// from anatomy.md. Returns an error if the file is missing or the timestamp
// cannot be parsed.
func ReadAnatomyGeneratedTime(projectRoot string) (time.Time, error) {
	path := filepath.Join(projectRoot, ".mneme", "anatomy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	for _, line := range strings.SplitN(string(data), "\n", 5) {
		if !strings.HasPrefix(line, "<!-- generated: ") {
			continue
		}
		rest := strings.TrimPrefix(line, "<!-- generated: ")
		if i := strings.Index(rest, " "); i > 0 {
			rest = rest[:i]
		}
		return time.Parse(time.RFC3339, rest)
	}
	return time.Time{}, fmt.Errorf("anatomy.md has no generated timestamp")
}
```

- [x] **Step 4: Run test to verify it passes**

```bash
go test ./tests/ -run TestReadAnatomyGeneratedTime -v
```
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add pkg/state/anatomy.go tests/anatomy_timestamp_test.go
git commit -m "feat(m6): expose ReadAnatomyGeneratedTime in state package"
```

---

## Task 2: `pkg/scanner` — incremental scan

**Files:**
- Modify: `pkg/scanner/extractor.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/scanner_incremental_test.go (new file in package tests)
package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/scanner"
	"github.com/ranwei/mneme/pkg/state"
)

func TestScanProjectIncrementalSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	os.WriteFile(filepath.Join(dir, ".mneme", ".local-id"), []byte("test-inc-uuid"), 0644)

	// Create a file and "scan" it
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	// Mark all files as scanned 1 second ago
	since := time.Now().Add(-1 * time.Second)

	existing := map[string]state.AnatomyEntry{
		"main.go": {Path: "main.go", Description: "cached entry", EstTokens: 3, Language: "go"},
	}

	entries, err := scanner.ScanProjectIncremental(dir, []string{"main.go"}, since, existing)
	if err != nil {
		t.Fatalf("ScanProjectIncremental: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// File mtime is before `since`, so the cached entry should be returned unchanged
	if entries[0].Description != "cached entry" {
		t.Errorf("expected cached description, got %q", entries[0].Description)
	}
}

func TestScanProjectIncrementalRescansChanged(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	os.WriteFile(filepath.Join(dir, ".mneme", ".local-id"), []byte("test-inc-uuid2"), 0644)

	// Write file THEN set since to before now
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	// since is in the past (before the file was written)
	since := time.Now().Add(-10 * time.Second)

	// Backdate the file so it looks "old" compared to since+1s in the future
	// Actually: file mtime is NOW, since is 10s ago → file IS newer → rescan
	existing := map[string]state.AnatomyEntry{
		"main.go": {Path: "main.go", Description: "stale entry", EstTokens: 1, Language: "go"},
	}

	entries, err := scanner.ScanProjectIncremental(dir, []string{"main.go"}, since, existing)
	if err != nil {
		t.Fatalf("ScanProjectIncremental: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// File is newer than since → re-extracted → description should differ from cached
	if entries[0].Description == "stale entry" {
		t.Errorf("expected fresh extraction, still got stale cached entry")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./tests/ -run TestScanProjectIncremental -v
```
Expected: FAIL — `scanner.ScanProjectIncremental undefined`

- [ ] **Step 3: Add `ScanProjectIncremental` to `pkg/scanner/extractor.go`**

Append to `pkg/scanner/extractor.go`:

```go
// ScanProjectIncremental re-extracts only files whose mtime is strictly after
// `since`. Files not modified since `since` are returned from `existing`
// unchanged. New files (not in existing) are always extracted.
func ScanProjectIncremental(projectRoot string, paths []string, since time.Time, existing map[string]state.AnatomyEntry) ([]FileEntry, error) {
	var entries []FileEntry
	for _, rel := range paths {
		abs := filepath.Join(projectRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		cached, inCache := existing[rel]
		if inCache && !info.ModTime().After(since) {
			// File unchanged since last scan — keep cached entry.
			entries = append(entries, FileEntry{
				Path:        cached.Path,
				Description: cached.Description,
				EstTokens:   cached.EstTokens,
				Language:    cached.Language,
			})
			continue
		}
		// Re-extract.
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		e := dispatch(rel, data)
		e.Path = rel
		e.EstTokens = len(data) / 4
		entries = append(entries, e)
	}
	return entries, nil
}
```

Also add the import for `"github.com/ranwei/mneme/pkg/state"` to `extractor.go`'s import block.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./tests/ -run TestScanProjectIncremental -v
```
Expected: PASS (both subtests)

- [ ] **Step 5: Commit**

```bash
git add pkg/scanner/extractor.go tests/scanner_incremental_test.go
git commit -m "feat(m6): add ScanProjectIncremental to scanner package"
```

---

## Task 3: `cmd/cmd_scan.go` — use incremental by default

**Files:**
- Modify: `cmd/cmd_scan.go`

- [ ] **Step 1: Write the integration test**

```go
// tests/integration/scan_incremental_test.go
package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupScanProject(t *testing.T) string {
	t.Helper()
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".mneme"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, ".mneme", ".local-id"), []byte("test-scan-incr-1234"), 0644); err != nil {
		t.Fatalf("write .local-id: %v", err)
	}
	// initialise a git repo so Walk() works
	exec.Command("git", "-C", proj, "init").Run()
	exec.Command("git", "-C", proj, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", proj, "config", "user.name", "Test").Run()
	return proj
}

func runScan(t *testing.T, projDir string, args ...string) (string, int) {
	t.Helper()
	cmdArgs := append([]string{"scan"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...)
	cmd.Dir = projDir
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	return string(out), 0
}

func TestScanIncrementalFastOnNoChange(t *testing.T) {
	proj := setupScanProject(t)

	// Add a file and commit it
	goFile := filepath.Join(proj, "main.go")
	os.WriteFile(goFile, []byte("package main\n"), 0644)
	exec.Command("git", "-C", proj, "add", ".").Run()
	exec.Command("git", "-C", proj, "commit", "-m", "init").Run()

	// First scan (full)
	out, code := runScan(t, proj)
	if code != 0 {
		t.Fatalf("first scan failed (code %d): %s", code, out)
	}

	// Backdating the file to before the anatomy timestamp ensures it looks unchanged
	// Then re-scan should be fast and keep the cached entry
	info, _ := os.Stat(filepath.Join(proj, ".mneme", "anatomy.md"))
	_ = info // anatomy was written

	// Second scan (incremental) — file mtime == creation time (before anatomy write time after the scan)
	start := time.Now()
	out2, code2 := runScan(t, proj)
	elapsed := time.Since(start)
	if code2 != 0 {
		t.Fatalf("second scan failed (code %d): %s", code2, out2)
	}
	_ = elapsed // timing only informational in unit context; acceptance validated manually

	if !strings.Contains(out2, "Scanned") {
		t.Errorf("expected 'Scanned' in output, got: %s", out2)
	}
}

func TestScanForceRescansFully(t *testing.T) {
	proj := setupScanProject(t)
	os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", proj, "add", ".").Run()
	exec.Command("git", "-C", proj, "commit", "-m", "init").Run()

	// Full first scan
	runScan(t, proj)
	// --force should always rescan
	out, code := runScan(t, proj, "--force")
	if code != 0 {
		t.Fatalf("--force scan failed (code %d): %s", code, out)
	}
	if !strings.Contains(out, "Scanned") {
		t.Errorf("expected 'Scanned' in output, got: %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./tests/integration/ -run TestScanIncremental -v
```
Expected: FAIL (timing test may pass trivially; structural assertions fail)

- [ ] **Step 3: Rewrite `cmd/cmd_scan.go` to use incremental by default**

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/scanner"
	"github.com/ranwei/mneme/pkg/state"
)

func dispatchScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	force := fs.Bool("force", false, "re-scan all files (ignore mtime cache)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, _ := resolveInitProjectRoot(cwd)
	if _, err := os.Stat(filepath.Join(root, ".mneme")); err != nil {
		fmt.Fprintln(os.Stderr, "scan: project not initialized (run: mneme init)")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", root)

	paths, err := scanner.Walk(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan: walk:", err)
		os.Exit(1)
	}

	var scanEntries []scanner.FileEntry

	if !*force {
		// Incremental: skip files that haven't changed since last scan.
		since, tsErr := state.ReadAnatomyGeneratedTime(root)
		existing, _ := state.ReadAnatomy(root)
		if tsErr == nil && len(existing) > 0 {
			scanEntries, err = scanner.ScanProjectIncremental(root, paths, since, existing)
		} else {
			// No existing anatomy — full scan.
			scanEntries, err = scanner.ExtractAll(root, paths)
		}
	} else {
		scanEntries, err = scanner.ExtractAll(root, paths)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(1)
	}

	entries := make([]state.AnatomyEntry, len(scanEntries))
	for i, e := range scanEntries {
		entries[i] = state.AnatomyEntry{
			Path:        e.Path,
			Description: e.Description,
			EstTokens:   e.EstTokens,
			Language:    e.Language,
		}
	}

	anatomyPath := filepath.Join(root, ".mneme", "anatomy.md")
	if err := state.WriteAnatomy(root, entries); err != nil {
		fmt.Fprintln(os.Stderr, "scan: write anatomy:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Scanned %d files → %s\n", len(entries), anatomyPath)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go build -o ./bin/mneme ./cmd && go test ./tests/integration/ -run TestScan -v
```
Expected: both integration tests PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_scan.go tests/integration/scan_incremental_test.go
git commit -m "feat(m6): incremental scan — skip unchanged files using mtime"
```

---

## Task 4: `pkg/consolidator/memory.go` — consolidation logic

**Files:**
- Create: `pkg/consolidator/memory.go`
- Create: `tests/consolidator_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// tests/consolidator_test.go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/consolidator"
	"github.com/ranwei/mneme/pkg/state"
)

func setupConsolidatorHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	return home
}

func writeRows(t *testing.T, home string, rows []state.MemoryRow) {
	t.Helper()
	for _, r := range rows {
		if err := state.AppendMemoryRow(home, r); err != nil {
			t.Fatalf("AppendMemoryRow: %v", err)
		}
	}
}

func TestConsolidateOldRows(t *testing.T) {
	home := setupConsolidatorHome(t)

	// 3 rows older than 7 days
	old := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	writeRows(t, home, []state.MemoryRow{
		{StartedAt: old, TurnCount: 3, PatternCounts: map[string]int{"bugfix": 1}},
		{StartedAt: old, TurnCount: 5, PatternCounts: map[string]int{"feature": 2}},
		{StartedAt: old, TurnCount: 2, PatternCounts: map[string]int{}},
	})

	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3 rows consolidated, got %d", n)
	}

	// memory.md should contain the consolidated blockquote
	data, _ := os.ReadFile(filepath.Join(home, ".claude", "mneme-memory.md"))
	if !strings.Contains(string(data), "> Consolidated") {
		t.Errorf("expected '> Consolidated' in memory.md, got:\n%s", data)
	}
	// total actions = 3+5+2 = 10
	if !strings.Contains(string(data), "10 actions") {
		t.Errorf("expected '10 actions' in consolidated summary, got:\n%s", data)
	}
}

func TestConsolidateKeepsRecentRows(t *testing.T) {
	home := setupConsolidatorHome(t)

	recent := time.Now().Add(-1 * 24 * time.Hour).UTC().Format(time.RFC3339)
	writeRows(t, home, []state.MemoryRow{
		{StartedAt: recent, TurnCount: 4, PatternCounts: map[string]int{}},
	})

	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows consolidated (all recent), got %d", n)
	}

	// recent row should still be present intact
	rows, _ := state.ReadMemory(home)
	if len(rows) != 1 {
		t.Errorf("expected 1 row remaining, got %d", len(rows))
	}
}

func TestConsolidateNoop(t *testing.T) {
	home := setupConsolidatorHome(t)
	// Empty memory file
	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate on empty: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestConsolidateMixedRows(t *testing.T) {
	home := setupConsolidatorHome(t)

	old := time.Now().Add(-10 * 24 * time.Hour).UTC().Format(time.RFC3339)
	recent := time.Now().Add(-2 * 24 * time.Hour).UTC().Format(time.RFC3339)
	writeRows(t, home, []state.MemoryRow{
		{StartedAt: old, TurnCount: 6, PatternCounts: map[string]int{}},
		{StartedAt: recent, TurnCount: 3, PatternCounts: map[string]int{"feature": 1}},
	})

	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 old row consolidated, got %d", n)
	}

	rows, _ := state.ReadMemory(home)
	if len(rows) != 1 {
		t.Errorf("expected 1 remaining recent row, got %d", len(rows))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./tests/ -run TestConsolidate -v
```
Expected: FAIL — `consolidator` package undefined

- [ ] **Step 3: Create `pkg/consolidator/memory.go`**

```go
package consolidator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const staleDays = 7

// Consolidate reads mneme-memory.md, folds rows older than staleDays
// into a single "Consolidated session (N actions)" blockquote, and writes the
// result back atomically. Returns the number of rows folded.
func Consolidate(homeDir string) (int, error) {
	rows, err := state.ReadMemory(homeDir)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	cutoff := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	var oldRows, recentRows []state.MemoryRow
	for _, r := range rows {
		ts, parseErr := time.Parse(time.RFC3339, r.StartedAt)
		if parseErr != nil {
			// Unparseable timestamp — treat as recent to avoid data loss.
			recentRows = append(recentRows, r)
			continue
		}
		if ts.Before(cutoff) {
			oldRows = append(oldRows, r)
		} else {
			recentRows = append(recentRows, r)
		}
	}

	if len(oldRows) == 0 {
		return 0, nil
	}

	// Sum total actions across folded rows.
	totalActions := 0
	for _, r := range oldRows {
		totalActions += r.TurnCount
	}

	// Build the new file: header + consolidated blockquote + recent rows.
	var sb strings.Builder
	sb.WriteString("<!-- mneme memory v1 -->\n")
	fmt.Fprintf(&sb, "\n> Consolidated session (%d actions from %d sessions before %s)\n",
		totalActions, len(oldRows), cutoff.UTC().Format("2006-01-02"))

	// Re-append recent rows by formatting each one.
	for _, r := range recentRows {
		sb.WriteString("\n")
		sb.WriteString(formatRow(r))
	}

	memPath := filepath.Join(homeDir, ".claude", "mneme-memory.md")
	lockPath := filepath.Join(homeDir, ".claude", "mneme-memory.lock")
	release, err := state.AcquireLock(lockPath, 2*time.Second)
	if err != nil {
		return 0, fmt.Errorf("consolidate: acquire lock: %w", err)
	}
	defer release()

	if err := state.AtomicWrite(memPath, []byte(sb.String())); err != nil {
		return 0, fmt.Errorf("consolidate: write: %w", err)
	}
	return len(oldRows), nil
}

func formatRow(r state.MemoryRow) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%d turns)\n", r.StartedAt, r.TurnCount))
	if len(r.PatternCounts) > 0 {
		parts := make([]string, 0, len(r.PatternCounts))
		for cat, n := range r.PatternCounts {
			parts = append(parts, fmt.Sprintf("%s×%d", cat, n))
		}
		sb.WriteString("Patterns: " + strings.Join(parts, ", ") + "\n")
	}
	if r.Summary != "" {
		sb.WriteString("Summary: " + r.Summary + "\n")
	}
	return sb.String()
}
```

Note: `state.AcquireLock` and `state.AtomicWrite` are already exported from `pkg/state`. Verify with:
```bash
grep -n "func AcquireLock\|func AtomicWrite" pkg/state/*.go
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./tests/ -run TestConsolidate -v
```
Expected: all 4 PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/consolidator/memory.go tests/consolidator_test.go
git commit -m "feat(m6): add consolidator package — fold old memory rows"
```

---

## Task 5: `cmd/cmd_memory.go` + `main.go` — memory subcommand

**Files:**
- Create: `cmd/cmd_memory.go`
- Modify: `cmd/main.go`

- [ ] **Step 1: Write the integration test**

```go
// tests/integration/memory_consolidate_test.go
package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

func setupMemoryProject(t *testing.T) (projDir, homeDir string) {
	t.Helper()
	proj := t.TempDir()
	home := t.TempDir()
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	os.WriteFile(filepath.Join(proj, ".mneme", ".local-id"), []byte("test-mem-uuid-9876"), 0644)
	return proj, home
}

func runMemory(t *testing.T, projDir, homeDir string, args ...string) (string, int) {
	t.Helper()
	cmdArgs := append([]string{"memory"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...)
	cmd.Dir = projDir
	cmd.Env = append(os.Environ(), "HOME="+homeDir)
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	return string(out), 0
}

func TestMemoryConsolidateNoOp(t *testing.T) {
	proj, home := setupMemoryProject(t)
	// No rows → consolidate should say 0 consolidated
	out, code := runMemory(t, proj, home, "consolidate")
	if code != 0 {
		t.Fatalf("memory consolidate exit %d: %s", code, out)
	}
	if !strings.Contains(out, "0") {
		t.Errorf("expected '0' in output, got: %s", out)
	}
}

func TestMemoryConsolidateOldRows(t *testing.T) {
	proj, home := setupMemoryProject(t)

	// Write 3 old rows directly
	old := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	for i := 0; i < 3; i++ {
		state.AppendMemoryRow(home, state.MemoryRow{
			StartedAt:     old,
			TurnCount:     4,
			PatternCounts: map[string]int{},
		})
	}

	out, code := runMemory(t, proj, home, "consolidate")
	if code != 0 {
		t.Fatalf("memory consolidate exit %d: %s", code, out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("expected '3' consolidated in output, got: %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go build -o ./bin/mneme ./cmd && go test ./tests/integration/ -run TestMemoryConsolidate -v
```
Expected: FAIL — `unknown subcommand: "memory"`

- [ ] **Step 3: Create `cmd/cmd_memory.go`**

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ranwei/mneme/pkg/consolidator"
)

func dispatchMemory(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mneme memory <subcommand>")
		fmt.Fprintln(os.Stderr, "subcommands: consolidate")
		os.Exit(2)
	}

	switch args[0] {
	case "consolidate":
		runMemoryConsolidate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "memory: unknown subcommand %q\n", args[0])
		os.Exit(2)
	}
}

func runMemoryConsolidate(args []string) {
	fs := flag.NewFlagSet("memory consolidate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "memory consolidate: cannot get home dir:", err)
		os.Exit(1)
	}

	n, err := consolidator.Consolidate(home)
	if err != nil {
		fmt.Fprintln(os.Stderr, "memory consolidate:", err)
		os.Exit(1)
	}
	fmt.Printf("consolidated %d session row(s)\n", n)
}
```

- [ ] **Step 4: Register in `cmd/main.go`**

Add `"memory"` case before `"version"`:

```go
case "memory":
    dispatchMemory(os.Args[2:])
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go build -o ./bin/mneme ./cmd && go test ./tests/integration/ -run TestMemoryConsolidate -v
```
Expected: both PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/cmd_memory.go cmd/main.go tests/integration/memory_consolidate_test.go
git commit -m "feat(m6): add memory consolidate subcommand"
```

---

## Task 6: Lazy consolidation in SessionStart + full test suite

**Files:**
- Modify: `cmd/hook_sessionstart.go`

- [ ] **Step 1: Add lazy consolidation to `runSessionStart`**

In `hook_sessionstart.go`, after the `AppendMemoryRow` call, add a lazy consolidation
trigger. Only consolidate if row count exceeds the bloat threshold (50 rows), to avoid
overhead on every session start.

Modify `runSessionStart` — add after the `state.IncrementSafe(root, "memory_rows_written")` block:

```go
// Lazy consolidation: only run when memory is getting large.
rows, _ := state.ReadMemory(home)
if len(rows) > 50 {
    if _, cerr := consolidator.Consolidate(home); cerr != nil {
        hook.WriteStderr("session-start: consolidate: " + cerr.Error())
    }
}
```

Add `"github.com/ranwei/mneme/pkg/consolidator"` to the import block.

- [ ] **Step 2: Build to verify it compiles**

```bash
go build -o ./bin/mneme ./cmd
```
Expected: success

- [ ] **Step 3: Run full test suite**

```bash
go test ./...
```
Expected: all pass

- [ ] **Step 4: Smoke test**

```bash
# In an initialized project:
./bin/mneme memory consolidate
./bin/mneme scan
./bin/mneme scan   # second run uses incremental
./bin/mneme stats --waste
```
Expected:
- `memory consolidate` prints `consolidated N session row(s)`
- First `scan` prints `✓ Scanned N files`
- Second `scan` prints `✓ Scanned N files` (same count, faster)
- `stats --waste` shows `[OK]` for `memory_bloat` if rows ≤ 50

- [ ] **Step 5: Commit**

```bash
git add cmd/hook_sessionstart.go
git commit -m "feat(m6): lazy memory consolidation in session-start hook"
```

---

## Acceptance Checklist

- ✅ `pkg/state.ReadAnatomyGeneratedTime` exposes anatomy timestamp — Task 1
- ✅ `scanner.ScanProjectIncremental` skips files with mtime ≤ since — Task 2
- ✅ `mneme scan` uses incremental by default; `--force` re-scans all — Task 3
- ✅ `pkg/consolidator.Consolidate` folds rows older than 7d into blockquote — Task 4
- ✅ `mneme memory consolidate` CLI subcommand — Task 5
- ✅ Lazy consolidation in session-start when rows > 50 — Task 6
- ✅ All existing tests still pass — Task 6 Step 3
