# M3: Memory + Ledger + Edit Summary — Design Spec

## Overview

M3 closes the session lifecycle loop. After M1 (hook scaffold) and M2 (anatomy map), M3 adds:

- **Turn-level edit classification** via `PostToolUse` hook
- **Session memory rows** written to `~/.claude/claude-context-memory.md` on each `SessionStart`
- **Full lifecycle** for `SessionStart`, `Stop`, and `PostToolUse` hook handlers
- **Stats enhancement** showing edit patterns and recent session history

---

## 1. Architecture & Lifecycle

### SessionStart-as-tombstone (chosen approach)

Claude Code 2.1.119 fires `Stop` after every assistant turn, not at true session exit. M3 solves the turn-vs-session distinction by using `SessionStart` as the tombstone signal for the previous session:

```
SessionStart fires
  → read _session.json (previous session, if any)
  → if previous session exists: call state.AppendMemoryRow(previous session data)
  → create fresh _session.json with new session_id, started_at, model

PostToolUse fires (tool = Write | Edit | MultiEdit | NotebookEdit)
  → classify edit: file path + extension + line delta → one of 13 categories
  → append TurnEdit{file, category, line_delta} to _session.json TurnEdits[]

Stop fires
  → increment stop_count on _session.json
  → aggregate TurnEdits since last Stop into TurnSummary, append TurnSummaries[]
  → clear TurnEdits buffer (already summarized)
```

**Known limitation:** the very last session before a user stops using the tool never gets its tombstone written to `memory.md`. This is acceptable — all prior sessions are persisted, and turn-level counters still accumulate in the ledger.

### File layout

**New files:**

| File | Responsibility |
|---|---|
| `cmd/hook_sessionstart.go` | Tombstone previous session + init new `_session.json` |
| `cmd/hook_stop.go` | Increment stop_count, aggregate TurnEdits → TurnSummary |
| `cmd/hook_postwrite.go` | Classify edit, append TurnEdit to `_session.json` |
| `pkg/state/memory.go` | `AppendMemoryRow`, `ReadMemory`, `MemoryRow` type |

**Modified files:**

| File | Change |
|---|---|
| `pkg/state/session.go` | Add `TurnEdits`, `TurnSummaries`, keep `StopCount` |
| `pkg/state/ledger.go` | Add `MemoryRowsWritten`, `TurnEditsClassified` |
| `cmd/cmd_hook.go` | Add `post-tool-use` dispatch case |
| `cmd/cmd_init.go` | Register `PostToolUse` hook in `settings.json` |
| `cmd/usage.go` | Update version to `0.1.0-m3`, update help text |
| `tests/integration/init_lifecycle_test.go` | Expect `PostToolUse` in hook count (3 hooks → 4) |

---

## 2. Edit Classifier

Lives in `cmd/hook_postwrite.go`. Takes the `PostToolUse` payload and returns one category per file. Rules applied top-to-bottom, first match wins.

### 13 Categories

| Category | Detection rule |
|---|---|
| `new_file` | tool = `Write`, file did not exist before (tool_result indicates creation) |
| `delete_content` | lines removed > 80% of original content |
| `dependency` | filename = `go.mod`, `go.sum`, `package.json`, `requirements.txt`, `Cargo.toml` |
| `config` | extension `.yaml`, `.yml`, `.json`, `.toml`, `.env`, `.ini` |
| `docs` | extension `.md`, `.txt`, `.rst`, `.adoc` |
| `test` | path contains `_test.go`, `test_`, `/tests/`, `.spec.`, `_spec.` |
| `types` | filename contains `types`, `interfaces`, `models`, `schema` |
| `import` | only changed lines match import block patterns (`import`, `require`, `from`) |
| `style` | net line delta = 0 (whitespace/formatting only) |
| `bugfix` | net change < 10 lines, non-test, non-config file |
| `refactor` | lines added ≈ lines removed (within ±20%), non-test file |
| `feature` | lines added > lines removed, non-test, non-config file |
| `unknown` | none of the above matched |

### Input

```json
{
  "tool_name": "Edit",
  "tool_input": {
    "file_path": "pkg/state/session.go",
    "old_string": "...",
    "new_string": "..."
  }
}
```

For `Write`: inspect `content` field. For `MultiEdit`: iterate over each `edits[]` entry. For `NotebookEdit`: treat cell source as content.

### Summary generation (optional C)

After `Stop` aggregates `TurnSummaries`, a simple template map converts category counts to a phrase:

| Pattern | Template |
|---|---|
| `new_file×N` | `"Created N new file(s)."` |
| `bugfix×N` | `"Fixed N bug(s)."` |
| `feature×N` | `"Added N feature(s)."` |
| `refactor×N` | `"Refactored N area(s)."` |
| `test×N` | `"Updated N test(s)."` |
| mixed | concatenate matched templates |
| no match | omit `Summary:` line entirely |

Summary is best-effort: if template output would be garbled or ambiguous, the line is omitted. Memory rows are always valid without it.

---

## 3. Data Model

### Session additions (`pkg/state/session.go`)

```go
type TurnEdit struct {
    File      string `json:"file"`
    Category  string `json:"category"`
    LineDelta int    `json:"line_delta"`
}

type TurnSummary struct {
    TurnIndex int        `json:"turn_index"`
    Edits     []TurnEdit `json:"edits"`
}
```

Added to `Session`:

```go
TurnEdits     []TurnEdit    `json:"turn_edits"`      // buffer, cleared each Stop
TurnSummaries []TurnSummary `json:"turn_summaries"`  // accumulated across all turns
```

(`StopCount int` already exists from M1 — no change needed.)

### Ledger additions (`pkg/state/ledger.go`)

```go
MemoryRowsWritten   int `json:"memory_rows_written"`
TurnEditsClassified int `json:"turn_edits_classified"`
```

### Memory API (`pkg/state/memory.go`)

```go
type MemoryRow struct {
    StartedAt     string
    TurnCount     int                // = previous session's StopCount
    FileSummary   []FileStat         // {File string, Categories []string}
    PatternCounts map[string]int
    Summary       string             // optional; empty string = omit line
}

type FileStat struct {
    File       string
    Categories []string
}

func AppendMemoryRow(homeDir string, row MemoryRow) error
func ReadMemory(homeDir string) ([]MemoryRow, error)
```

`AppendMemoryRow` uses `state.AtomicWrite` (flock-X, 50ms timeout). Memory.md path: `filepath.Join(homeDir, ".claude", "claude-context-memory.md")`.

### memory.md format

```markdown
<!-- claude-context memory v1 -->

## 2026-04-27T14:32 (3 turns)
Files: pkg/state/session.go (refactor×1), cmd/hook_stop.go (new_file×1)
Patterns: refactor×1, new_file×1
Summary: Refactored 1 area, created 1 new file.

## 2026-04-27T16:05 (7 turns)
Files: auth.go (bugfix×2), server.go (feature×1)
Patterns: bugfix×2, feature×1
Summary: Fixed 2 bugs, added 1 feature.
```

Header line `<!-- claude-context memory v1 -->` written once on first append; subsequent calls check for its presence before writing.

---

## 4. Stats Enhancement

`claude-context stats` gains three new sections:

```
=== Hook Totals ===
  pre-read fired:        47
  post-tool-use fired:   23
  session-start fired:    4
  stop fired:            31

=== Edit Patterns ===
  feature:     8
  bugfix:      6
  refactor:    5
  test:        3
  docs:        1

=== Recent Sessions (last 5) ===
  2026-04-27T16:05  7 turns  bugfix×2, feature×1
  2026-04-27T14:32  3 turns  refactor×1, new_file×1
  2026-04-26T09:10  12 turns feature×3, test×2

=== Memory ===
  rows written:           3
  edits classified:      23
  memory.md:             ~/.claude/claude-context-memory.md
```

`ReadMemory` supplies recent sessions. Ledger supplies `MemoryRowsWritten` and `TurnEditsClassified`. `cmd/cmd_stats.go` is extended to call both.

---

## 5. Init Changes

`PostToolUse` is added as the 5th managed hook entry in `settings.json`:

```json
"PostToolUse": [
  {
    "hooks": [
      {
        "type": "command",
        "command": "/path/to/claude-context hook post-tool-use"
      }
    ],
    "_managed_by": "claude-context"
  }
]
```

The existing uninstall logic already removes all entries with `_managed_by: claude-context` — no additional changes needed.

`tests/integration/init_lifecycle_test.go` `TestInitFreshThenUninstall` must be updated to assert `PostToolUse` is present in the installed hooks (currently asserts 3 hooks: `PreToolUse`, `SessionStart`, `Stop`; M3 brings the total to 4).

---

## 6. Error Handling & Safety

All three new hook handlers follow the same M1 invariant: **never crash Claude Code**.

- `PostToolUse` handler: if payload parse fails → `IncrementSafe("stdin_parse_failures")`, exit 0
- `PostToolUse` handler: if project not resolved → exit 0 silently
- `SessionStart` handler: if `AppendMemoryRow` fails → log to stderr via `hook.WriteStderr`, continue with session init (memory write failure must not block session start)
- `Stop` handler: if `UpsertSession` fails → log error, exit 0

---

## 7. Testing Strategy

- **Unit tests** in `tests/state_test.go`: `AppendMemoryRow`, `ReadMemory`, golden format test
- **Unit tests** in `tests/classifier_test.go`: each of 13 categories, table-driven
- **Integration tests** in `tests/integration/hook_chain_test.go`:
  - `TestPostToolUseClassifiesEdit` — fire post-tool-use with Write payload, verify TurnEdit in session
  - `TestSessionStartWritesMemoryRow` — fire session-start twice, verify memory.md has one row after second fire
  - `TestStopAggregatesTurnEdits` — fire post-tool-use ×3, then stop, verify TurnSummaries
- **Updated** `TestInitFreshThenUninstall` — assert `PostToolUse` in installed hooks
- **Benchmark** `BenchmarkPostToolUseE2E` — p99 < 50ms

---

## 8. Out of Scope (M3)

- Embedding or semantic search over memory.md (M5+)
- Injecting memory.md into `CLAUDE.md` automatically (M4)
- Cross-session file heatmap (M5)
- Classifier ML model (never — rule-based is sufficient and deterministic)
