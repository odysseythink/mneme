# M4b: Buglog — Design Spec

## Overview

M4b adds a per-project buglog that records previously fixed bugs and warns Claude before it re-introduces them. It extends the M4a prewrite hook with a second matching pass and extends the M3 postwrite hook with auto-upsert.

Three deliverables:
- `pkg/state/buglog.go` — `BuglogEntry` type and state API
- `pkg/match/tokens.go` — tokenizer and token-overlap matcher
- `cmd/cmd_buglog.go` — `buglog {add,list,clear}` CLI

Plus modifications to `cmd/hook_postwrite.go` (auto-upsert) and `cmd/hook_prewrite.go` (buglog match pass).

M4a (cerebrum) and M4c (MCP tools) are separate milestones.

---

## 1. Architecture & Data Flow

```
PostToolUse (Write|Edit|MultiEdit) fires
  → cmd/hook_postwrite.go: classifyEdit (M3)
  → if category == "bugfix" AND old_string tokens >= 5:
      create BuglogEntry{source:"auto", bad_code: old_string, file: path,
                         description: "possible re-introduction of bugfix in <basename>"}
      state.AppendBuglogEntry(root, entry)

PreToolUse (Write|Edit|MultiEdit) fires
  → cmd/hook_prewrite.go: runPreWrite(stdin)
  → pass 1: cerebrum rules (M4a)
  → pass 2: buglog entries
      state.ReadBuglog(root) → []BuglogEntry
      extractAddedLines(ev) → map[lineNo]string
      Tokenize(joined added lines) → newTokens
      for each entry: if TokenOverlap(newTokens, Tokenize(entry.BadCode)) >= 3 → match
  → combine cerebrum + buglog matches → single warning block
  → if any matches: exit 1 with warning; else exit 0

buglog add [--description D] [--code C] [--file F]
  → cmd/cmd_buglog.go
  → prompts interactively for any missing flags
  → validates description and code are non-empty
  → state.AppendBuglogEntry(root, BuglogEntry{source:"manual", ...})

buglog list
  → state.ReadBuglog(root) → formatted output

buglog clear [--yes]
  → state.ReadBuglog → confirm → state.WriteBuglog(root, nil)
```

### File map

**New files:**

| File | Responsibility |
|---|---|
| `pkg/state/buglog.go` | `BuglogEntry` type, `ReadBuglog`, `AppendBuglogEntry`, `WriteBuglog` |
| `pkg/match/tokens.go` | `Tokenize`, stop-word list, `TokenOverlap` |
| `cmd/cmd_buglog.go` | `dispatchBuglog` + subcommands |

**Modified files:**

| File | Change |
|---|---|
| `cmd/hook_postwrite.go` | Add auto-upsert after classify |
| `cmd/hook_prewrite.go` | Add buglog match pass, merge with cerebrum matches |
| `cmd/main.go` | Add `buglog` dispatch case |
| `cmd/usage.go` | Add `buglog` help, version → `0.1.0-m4b` |
| `tests/integration/hook_chain_test.go` | 3 new integration tests |

---

## 2. Data Model

### `pkg/state/buglog.go`

```go
type BuglogEntry struct {
    ID          string `json:"id"`           // UUID
    CreatedAt   string `json:"created_at"`   // RFC3339
    Source      string `json:"source"`       // "auto" | "manual"
    File        string `json:"file"`         // relative path to project root; empty if unknown
    Description string `json:"description"` // shown to Claude on match
    BadCode     string `json:"bad_code"`     // the buggy code snippet
}

// internal envelope — not exported
type buglogFile struct {
    Version int           `json:"version"`
    Entries []BuglogEntry `json:"entries"`
}

// ReadBuglog reads buglog.json; returns nil, nil if file does not exist.
// Unlocked read — stale is acceptable in hook context.
func ReadBuglog(projectRoot string) ([]BuglogEntry, error)

// AppendBuglogEntry appends one entry to buglog.json.
// Creates the file with version:1 if absent.
// Uses AtomicWrite (flock-X, 50ms timeout).
func AppendBuglogEntry(projectRoot string, e BuglogEntry) error

// WriteBuglog overwrites buglog.json with the given slice.
// Used by buglog clear. Passing nil entries writes an empty entries array.
// Uses AtomicWrite (flock-X, 50ms timeout).
func WriteBuglog(projectRoot string, entries []BuglogEntry) error
```

**Storage path:** `<project>/.mneme/buglog.json`

**File format:**
```json
{
  "version": 1,
  "entries": [
    {
      "id": "a1b2c3d4-...",
      "created_at": "2026-04-28T14:32:00Z",
      "source": "auto",
      "file": "pkg/state/session.go",
      "description": "possible re-introduction of bugfix in session.go",
      "bad_code": "var x = someFunc()\nsession.StopCount++"
    }
  ]
}
```

### `pkg/match/tokens.go`

```go
// Tokenize splits s on non-alphanumeric boundaries, lowercases all tokens,
// and removes stop words and single-character tokens.
func Tokenize(s string) []string

// TokenOverlap returns the count of tokens in a that also appear in b.
// Both slices should already be tokenized via Tokenize.
func TokenOverlap(a, b []string) int

// Stop words stripped by Tokenize:
// single-character tokens, plus:
// "err", "nil", "if", "else", "return", "func", "var", "type",
// "the", "and", "for", "true", "false", "range", "len", "make"
```

**Threshold:** hardcoded at 3. Not configurable in M4b.

**Auto-upsert guard:** postwrite only appends if `old_string` has ≥ 5 tokens after `Tokenize` — prevents noise entries from trivial one-word edits.

---

## 3. Matching Algorithm & Prewrite Integration

The prewrite hook runs two passes and combines results:

```
pass 1 (cerebrum, M4a):
  for each CerebrumRule:
    re.MatchString(addedLine) → cerebrum match

pass 2 (buglog, M4b):
  newTokens := Tokenize(strings.Join(values(addedLines), "\n"))
  for each BuglogEntry:
    badTokens := Tokenize(entry.BadCode)
    if TokenOverlap(newTokens, badTokens) >= 3:
      firstLine := first addedLine whose own TokenOverlap(Tokenize(line), badTokens) >= 1
      record match{entry.Description, entry.BadCode, firstLine}
```

**Combined warning block format:**

```
⚡ mneme: ⚠️ N rule(s)/match(es):
  • [cerebrum] prefer := for short variable declarations (line 3)
  • [buglog] possible re-introduction of bugfix in session.go (line 7)
    was: var x = someFunc()
```

Each buglog match line includes `was: <bad_code first line>` to give Claude the pattern. If `bad_code` spans multiple lines, only the first line is shown inline; the full snippet is not printed.

---

## 4. `buglog` CLI

```
mneme buglog add [--description D] [--code C] [--file F]
mneme buglog list
mneme buglog clear [--yes]
```

### `buglog add`

Flags take priority; missing flags prompt interactively:

```
$ mneme buglog add
Description: possible nil deref when session is uninitialized
Bad code snippet (paste, then Ctrl-D):
session.StopCount++
^D
✓ Entry added (4 entries total in .mneme/buglog.json)
```

If `--description` and `--code` are both provided, no prompts. `--file` is optional. Exits 1 with `"description and code are required"` if either is empty after prompting.

### `buglog list`

```
Buglog entries (2):
  1. [auto] possible re-introduction of bugfix in session.go  (2026-04-28T14:32Z)
     file: pkg/state/session.go
     was: var x = someFunc()
  2. [manual] possible nil deref when session is uninitialized  (2026-04-28T15:01Z)
     was: session.StopCount++
```

Exits 0 with `"No buglog entries. Run: mneme buglog add"` if file is empty or absent.

### `buglog clear`

```
$ mneme buglog clear
Remove all 2 buglog entries? [y/N]: y
✓ Cleared.
```

- Non-TTY stdin without `--yes`: exits 1 with `"confirmation required: re-run with --yes to skip"`.
- Empty buglog: exits 0 with `"No buglog entries to clear"`.

---

## 5. Error Handling & Safety

| Error | Behavior |
|---|---|
| `buglog.json` missing | `ReadBuglog` returns nil, nil → no hints (same as empty) |
| `buglog.json` corrupt | Return nil, nil — log parse error to `hook-errors.log`, never block hook |
| Auto-upsert lock timeout | Log to `hook-errors.log`, continue — never blocks postwrite |
| `buglog add` empty description or code | Exits 1 with `"description and code are required"` |
| `buglog clear` on empty file | Exits 0 with `"No buglog entries to clear"` |
| Hook panic | `recoverAndLog` catches it, exit 0 |

Warn-only invariant: the prewrite hook **never blocks** a write — buglog matches exit 1 (informational), same as cerebrum matches.

---

## 6. Testing Strategy

**Unit tests** in `tests/buglog_test.go`:
- `TestReadBuglogEmpty` — missing file → nil, nil
- `TestAppendAndReadEntries` — append 2 entries, read back, verify all fields
- `TestTokenizeStripsStopWords` — verify stop words stripped, lowercased, single-chars removed
- `TestTokenOverlapThreshold` — overlap=3 matches, overlap=2 does not
- `TestAutoUpsertGuard` — old_string with <5 tokens after tokenize → entry not appended

**Integration tests** in `tests/integration/hook_chain_test.go`:
- `TestPreWriteBuglogMatchWarns` — init project, append buglog entry via `AppendBuglogEntry`, fire prewrite with ≥3 overlapping tokens → assert exit 1, stderr contains `[buglog]` prefix + description + `was:` line
- `TestPreWriteBuglogNoMatchExitsZero` — non-overlapping content → assert exit 0
- `TestPostWriteAutoUpsert` — fire postwrite with bugfix-classified Edit payload (old_string ≥5 tokens) → assert `buglog.json` gains one entry with source=`"auto"`

---

## 7. Out of Scope (M4b)

- Deduplication of similar buglog entries (two fixes of the same pattern → two entries)
- `buglog remove <N>` (individual entry removal — use `clear` to reset)
- Configurable match threshold (hardcoded at 3)
- Semantic/embedding-based matching (M5+)
- `find_similar_bugs` MCP tool (M4c)
