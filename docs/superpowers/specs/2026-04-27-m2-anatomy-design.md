# M2 — Anatomy Map Design Spec

**Date:** 2026-04-27
**Status:** Approved
**Builds on:** M1 Foundation (`docs/superpowers/specs/2026-04-27-m1-foundation-design.md`)

---

## Goal

Give Claude a per-file description index (anatomy) so it can decide whether to read a file without actually reading it. The pre-read hook becomes informative: it tells Claude what the file contains and warns when a file has already been read this session. Anatomy hits and repeat-read warnings are emitted unconditionally (not debug-only) so Claude always benefits.

**Headline metric:** `ledger.anatomy_hits` counter starts accumulating from the first session after `init`.

---

## Scope

### In scope
- `pkg/scanner/` — walker (git ls-files) + per-language extractors
- `pkg/state/anatomy.go` — `WriteAnatomy` / `ReadAnatomy`
- `pkg/state/session.go` extension — `Reads []string` field + `AppendSessionRead`
- `cmd/cmd_scan.go` — `scan [--force]` subcommand
- `cmd/cmd_init.go` — auto-run scan after scaffold (unless `--no-scan`)
- `cmd/main.go` — add `scan` to dispatcher
- `cmd/cmd_hook.go` — replace `runPreRead` stub with complete implementation
- `pkg/installer/templates/rules.md` — add "consult anatomy first" section
- Tests: unit (scanner, anatomy state, session reads), integration (pre-read with anatomy), golden (anatomy.md format), benchmark (scan latency)

### Out of scope
- Incremental scan (mtime skip) — M6
- `describe_codebase` MCP tool — M4
- Anatomy used by pre-write hook — M3+
- Anatomy for languages beyond Go/Python/JS/TS/Markdown/known-configs (fallback handles others)

---

## Architecture

```
claude-context scan [--force]
  → scanner.Walk(projectRoot)            // git ls-files --cached --others --exclude-standard
  → filter: >1MB, binary (null byte in first 512B)
  → scanner.ExtractAll(root, paths)      // dispatch by extension
  → state.WriteAnatomy(root, entries)   // atomic write .claude-context/anatomy.md
  → state.IncrementSafe("scan_count")

claude-context init --yes
  → [existing M1 steps]
  → dispatchScan(nil)                    // unless --no-scan

claude-context hook pre-read (stdin: PreToolUse Read event)
  → parseOrExit(stdin)
  → resolveProjectFromEvent             // exit 0 if outside project
  → state.IncrementSafe("hook_fired.pre-read")
  → relPath := filepath.Rel(root, absPath)
  → anatomy := state.ReadAnatomy(root)  // no lock, stale OK
  → alreadyRead, _ := state.AppendSessionRead(root, relPath)  // flock-X 50ms
  → if anatomy hit: WriteStderr("relPath — description (~N tok)"), exit 1
  → if alreadyRead: WriteStderr("relPath already read this session (~N tok)"), exit 1
  → exitHook("pre-read", root)          // exit 0 silent (or exit 1 if debug ≥ 1)
```

---

## Package: `pkg/scanner/`

Four files. All extractors are pure functions (bytes in, string out) — no I/O inside extractors.

### `walker.go`

```go
// Walk returns project-relative file paths using git ls-files.
// Filters out files >1MB and binary files (null byte in first 512 bytes).
func Walk(projectRoot string) ([]string, error)
```

Runs: `git ls-files --cached --others --exclude-standard`
Filters: skip if `os.Stat(path).Size() > 1<<20` or first 512 bytes contain `\x00`.

### `extractor.go`

```go
type FileEntry struct {
    Path        string // project-relative
    Description string // ≤100 chars
    EstTokens   int    // len(fileBytes) / 4
    Language    string // "go", "python", "js", "markdown", "config", "unknown"
}

// ExtractAll dispatches each path to the appropriate extractor.
func ExtractAll(projectRoot string, paths []string) []FileEntry

// ScanProject combines Walk + ExtractAll.
func ScanProject(projectRoot string) ([]FileEntry, error)
```

Dispatch by extension:
| Extensions | Extractor | Language tag |
|---|---|---|
| `.go` | `extractGo` | `go` |
| `.py` | `extractPy` | `python` |
| `.js` `.ts` `.jsx` `.tsx` | `extractJS` | `js` |
| `.md` `.markdown` | `extractMD` | `markdown` |
| Known filenames (see below) | `extractKnown` | `config` |
| Everything else | `extractFallback` | `unknown` |

### `ext_go.go`

Scans for the first `func`, `type`, `var`, or `const` declaration. If the preceding line(s) start with `//`, uses the first comment line as description. Otherwise uses the declaration line. Truncates to 100 chars.

### `ext_py.go`

Scans for the first `def` or `class` line. If the next non-empty line is a triple-quoted string, uses its first line as description. Otherwise uses the def/class line. Truncates to 100 chars.

### `ext_js.go`

Handles `.js`, `.ts`, `.jsx`, `.tsx`. Scans for the first `export`, `function`, or `class` line. If preceded by a `/** ... */` JSDoc block, extracts `@description` or the first `*` content line. Truncates to 100 chars.

### `ext_md.go`

Extracts the first `#`-prefixed heading (stripped of `#` and spaces). If none, uses the first non-empty line. Truncates to 100 chars.

### `ext_known.go`

Returns a fixed description for well-known filenames (case-insensitive base name match):

| Filename | Description |
|---|---|
| `go.mod` | Go module definition |
| `go.sum` | Go dependency checksums |
| `package.json` | Node.js package descriptor |
| `package-lock.json` | Node.js lockfile |
| `yarn.lock` | Yarn lockfile |
| `Cargo.toml` | Rust crate manifest |
| `Cargo.lock` | Rust dependency lockfile |
| `Makefile` | Build rules |
| `Dockerfile` | Container image definition |
| `docker-compose.yml` | Multi-container Docker config |
| `.gitignore` | Git ignore rules |
| `tsconfig.json` | TypeScript compiler config |
| `pyproject.toml` | Python project config |
| `requirements.txt` | Python dependencies |
| `CLAUDE.md` | Claude Code project instructions |

### Fallback (`extractor.go`)

First non-empty line that doesn't start with `//`, `#`, `/*`, or `*`. Truncated to 100 chars. If no such line found, returns `"(no description)"`.

### Token estimation

`EstTokens = len(fileBytes) / 4`

Applied after reading file bytes for extraction. Zero dependencies.

---

## Package: `pkg/state/anatomy.go`

`pkg/state` must not import `pkg/scanner` (preserves the M1 invariant: state has no internal imports). Define the anatomy entry type here; `cmd/cmd_scan.go` converts from `scanner.FileEntry`.

```go
// AnatomyEntry is the state package's representation of a scanned file.
type AnatomyEntry struct {
    Path        string
    Description string
    EstTokens   int
    Language    string
}

// WriteAnatomy groups entries by directory, sorts lexicographically,
// renders anatomy.md, and writes atomically. Also increments scan_count.
func WriteAnatomy(projectRoot string, entries []AnatomyEntry) error

// ReadAnatomy parses anatomy.md into a map keyed by project-relative path.
// Returns empty map (not error) if anatomy.md does not exist.
func ReadAnatomy(projectRoot string) (map[string]AnatomyEntry, error)
```

**anatomy.md path:** `<projectRoot>/.claude-context/anatomy.md`

**Format:**
```markdown
<!-- claude-context anatomy v1 -->
<!-- generated: 2026-04-27T10:00:00Z | files: 42 | project: abc-uuid -->

## cmd/

- `main.go` — subcommand dispatcher (go, ~120 tok)
- `cmd_hook.go` — 5 hook handlers + parseOrExit (go, ~430 tok)

## pkg/hook/

- `feedback.go` — WriteStderr, DebugLevel, FormatStderr (go, ~80 tok)
- `protocol.go` — Event type, ParseEvent, FilePathFromToolInput (go, ~220 tok)
```

Entry format: `` - `<filename>` — <description> (<language>, ~<N> tok) ``

Root-level files go in a `## ./` section before subdirectory sections.

**No flock on read** — stale anatomy is acceptable. Write uses `AtomicWrite` (already flock-free via tmp+rename).

---

## `pkg/state/session.go` Extension

### Session struct change

Add `Reads` field:

```go
type Session struct {
    Version         int      `json:"_version"`
    SessionID       string   `json:"session_id"`
    ProjectID       string   `json:"project_id"`
    StartedAt       string   `json:"started_at"`
    ClaudeCodeModel string   `json:"claude_code_model"`
    StopCount       int      `json:"stop_count"`
    Reads           []string `json:"reads,omitempty"`
}
```

Existing `UpsertSession` is unchanged — when called with a new `SessionID` it overwrites the file, clearing `Reads` naturally.

### New function

```go
// AppendSessionRead atomically checks if relPath is in the session's Reads list
// and appends it if not. Returns alreadyRead=true if the path was already present.
// Returns (false, nil) if no session file exists yet (first read of session).
// Uses flock-X with 50ms timeout.
func AppendSessionRead(projectRoot, relPath string) (alreadyRead bool, err error)
```

Lock path: `GlobalProjectDir(id) + "/runtime.lock"` (same lock used by `IncrementSafe`).

---

## `cmd/cmd_scan.go`

```go
func dispatchScan(args []string) {
    fs := flag.NewFlagSet("scan", flag.ContinueOnError)
    force := fs.Bool("force", false, "re-scan even if anatomy.md exists")
    if err := fs.Parse(args); err != nil {
        os.Exit(2)
    }

    cwd, _ := os.Getwd()
    root, _ := resolveInitProjectRoot(cwd) // ok ignored; falls back to cwd
    if _, err := os.Stat(filepath.Join(root, ".claude-context")); err != nil {
        fmt.Fprintln(os.Stderr, "scan: project not initialized (run: claude-context init)")
        os.Exit(1)
    }

    // --force is accepted for forward-compatibility with M6 incremental scan;
    // in M2 we always do a full regeneration regardless.
    _ = force

    anatomyPath := filepath.Join(root, ".claude-context", "anatomy.md")
    fmt.Fprintf(os.Stderr, "Scanning %s...\n", root)
    scanEntries, err := scanner.ScanProject(root)
    if err != nil {
        fmt.Fprintln(os.Stderr, "scan:", err)
        os.Exit(1)
    }
    // Convert scanner.FileEntry → state.AnatomyEntry
    entries := make([]state.AnatomyEntry, len(scanEntries))
    for i, e := range scanEntries {
        entries[i] = state.AnatomyEntry{Path: e.Path, Description: e.Description, EstTokens: e.EstTokens, Language: e.Language}
    }
    if err := state.WriteAnatomy(root, entries); err != nil {
        fmt.Fprintln(os.Stderr, "scan: write anatomy:", err)
        os.Exit(1)
    }
    fmt.Fprintf(os.Stderr, "✓ Scanned %d files → %s\n", len(entries), anatomyPath)
}
```

### `cmd/main.go` change

Add `case "scan": dispatchScan(os.Args[2:])` to the dispatcher switch.

### `cmd/cmd_init.go` change

In `runInit`, after the `ScaffoldProject` step:
```go
if !opts.noScan {
    fmt.Fprintln(os.Stderr, "Running initial scan...")
    dispatchScan(nil)
}
```

---

## `cmd/cmd_hook.go` — Complete `runPreRead`

Replace the M1 stub:

```go
func runPreRead(stdin io.Reader) {
    defer recoverAndLog("pre-read")
    ev := parseOrExit(stdin, "pre-read")
    root, _, resolved := resolveProjectFromEvent(ev)
    if !resolved {
        os.Exit(0)
    }
    state.IncrementSafe(root, "hook_fired.pre-read")

    absPath, ok := ev.FilePathFromToolInput()
    if !ok {
        exitHook("pre-read", root)
        return
    }
    relPath, err := filepath.Rel(root, absPath)
    if err != nil || strings.HasPrefix(relPath, "..") {
        exitHook("pre-read", root) // file outside project tree
        return
    }

    anatomy, _ := state.ReadAnatomy(root)   // returns map[string]state.AnatomyEntry; stale OK
    alreadyRead, _ := state.AppendSessionRead(root, relPath) // ignore lock timeout

    if entry, ok := anatomy[relPath]; ok {
        state.IncrementSafe(root, "anatomy_hits")
        hook.WriteStderr(fmt.Sprintf("%s — %s (~%d tok)", relPath, entry.Description, entry.EstTokens))
        os.Exit(1) // exit 1 makes message visible in Claude transcript
    }
    if alreadyRead {
        state.IncrementSafe(root, "repeat_reads")
        estTok := 0
        if entry, ok := anatomy[relPath]; ok {
            estTok = entry.EstTokens
        }
        msg := fmt.Sprintf("%s already read this session", relPath)
        if estTok > 0 {
            msg += fmt.Sprintf(" (~%d tok)", estTok)
        }
        hook.WriteStderr(msg)
        os.Exit(1)
    }

    exitHook("pre-read", root)
}
```

---

## Ledger Extensions

New counters added to `LedgerTotals`:

```go
type LedgerTotals struct {
    HookFired             map[string]int `json:"hook_fired"`
    HookErrors            int            `json:"hook_errors"`
    StdinParseFailures    int            `json:"stdin_parse_failures"`
    OutsideProjectSkipped int            `json:"outside_project_skipped"`
    WriteSkipped          int            `json:"write_skipped"`
    AnatomyHits           int            `json:"anatomy_hits"`    // M2
    RepeatReads           int            `json:"repeat_reads"`    // M2
    ScanCount             int            `json:"scan_count"`      // M2
}
```

`cmd/cmd_stats.go` updated to print these three counters.

---

## `pkg/installer/templates/rules.md` Update

Add a new section after "## MCP tools available":

```markdown
## Anatomy map

Before reading a file, check if claude-context has already described it:

- If the pre-read hook fires with `⚡ claude-context: <path> — <description> (~N tok)`,
  that description is from the anatomy map. Use it to decide whether to read the full file.
- If the hook says `<path> already read this session`, the file content is already in your
  context window. Do not re-read it unless the content may have changed.

To regenerate the anatomy map after large refactors: `claude-context scan`
```

---

## File Structure

```
pkg/scanner/
  walker.go         ~60 LOC   git ls-files + size/binary filters
  extractor.go      ~80 LOC   FileEntry type, ExtractAll, ScanProject, fallback
  ext_go.go         ~50 LOC   Go extractor
  ext_py.go         ~40 LOC   Python extractor
  ext_js.go         ~50 LOC   JS/TS extractor
  ext_md.go         ~30 LOC   Markdown extractor
  ext_known.go      ~40 LOC   Known-filename table

pkg/state/
  anatomy.go        ~90 LOC   WriteAnatomy, ReadAnatomy
  session.go        +30 LOC   Reads field, AppendSessionRead (extension)
  ledger.go         +10 LOC   AnatomyHits, RepeatReads, ScanCount fields

cmd/
  cmd_scan.go       ~50 LOC   dispatchScan
  cmd_hook.go       ~30 LOC   replace runPreRead stub
  cmd_init.go       ~5 LOC    call dispatchScan after scaffold
  cmd_stats.go      ~10 LOC   print new counters
  main.go           ~3 LOC    add scan case
  usage.go          ~3 LOC    add scan to help text

pkg/installer/templates/rules.md   +15 LOC   anatomy section

tests/
  scanner_test.go                  ~120 LOC  unit tests
  state_test.go                    +60 LOC   anatomy + session reads tests
  golden/anatomy_sample.md                   golden output fixture
  integration/hook_chain_test.go   +40 LOC   TestPreReadAnatomy, TestPreReadRepeat
  bench_test.go                    +15 LOC   BenchmarkScanProject
```

---

## Testing Plan

### Unit tests — `tests/scanner_test.go`
- `TestWalkFilesGitRepo` — Walk returns non-empty list for real repo
- `TestExtractGo` — inline Go source fixture → correct description
- `TestExtractPy` — inline Python fixture → extracts def/docstring
- `TestExtractJS` — inline TS export → extracts description
- `TestExtractMD` — inline markdown → extracts first heading
- `TestExtractKnown` — `go.mod` → "Go module definition"
- `TestExtractFallback` — `.rb` file → first non-comment line
- `TestTokenEstimate` — `len("hello world")/4 == 2`
- `TestScanProjectSmall` — scan a temp dir with 3 known files, verify FileEntry fields

### Unit tests — `tests/state_test.go` (additions)
- `TestWriteReadAnatomy` — round-trip: write 5 entries, ReadAnatomy returns correct map
- `TestReadAnatomyMissing` — returns empty map, no error
- `TestAppendSessionReadFirstTime` — alreadyRead=false, Reads has 1 entry
- `TestAppendSessionReadSecondTime` — alreadyRead=true, Reads still has 1 entry
- `TestAppendSessionReadNewSession` — UpsertSession with new ID clears Reads

### Golden test — `tests/golden/anatomy_sample.md`
Checked in expected anatomy output for a fixture directory with 4 files (one per language). Verified by `TestAnatomyGolden` in **state_test.go** (tests `state.WriteAnatomy` output, not the scanner).

### Integration tests — `tests/integration/hook_chain_test.go` (additions)
- `TestPreReadAnatomy` — init project, scan, fire pre-read hook for a real project file → exit 1, stderr contains file description
- `TestPreReadRepeat` — fire pre-read twice for same path → second call exits 1 with "already read"
- `TestPreReadOutsideProject` — fire pre-read with path outside project root → exit 0, no stderr

### Benchmarks — `tests/bench_test.go` (additions)
- `BenchmarkScanProject` — scan real repo; expected <2s/op on a medium repo (~100 files)
- `BenchmarkReadAnatomy` — parse anatomy.md; expected <1ms/op

---

## Acceptance Criteria

- [ ] `claude-context scan` produces sensible `anatomy.md` for this repo
- [ ] `claude-context init --yes` in a git repo runs scan automatically
- [ ] Pre-read hook emits anatomy description to Claude transcript when file is in anatomy map
- [ ] Pre-read hook emits repeat-read warning when same file read twice in one session
- [ ] `claude-context stats` shows `anatomy_hits`, `repeat_reads`, `scan_count`
- [ ] `go test ./...` passes (all unit + integration tests)
- [ ] `BenchmarkScanProject` < 2s/op on this repo
- [ ] Manual smoke: `CLAUDE_CONTEXT_DEBUG=1 claude` in initialized project → read a file → see `⚡ claude-context:` message in transcript
