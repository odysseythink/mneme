# M1 — Foundation Design

**Date:** 2026-04-27
**Status:** Draft
**Scope:** Build the harness for the openwolf-port project (Claude Code hooks integrated into the existing `claude-context` Go binary): subcommand dispatcher, minimum state layer, hook protocol parsing, init/uninstall lifecycle, and 5 hook handler stubs. M1 ships no token-saving features — it is the foundation that M2-M6 stand on.

---

## Problem

The architecture spec (`2026-04-27-claude-context-hook-architecture-design.md`) describes 6 milestones (M1-M6). M1 is the entry point: the binary must learn to dispatch subcommands, the install/uninstall lifecycle must work end-to-end, hook subcommands must be callable by Claude Code without ever blocking it, and the existing MCP server behavior must be preserved with zero regression.

M0 (`2026-04-27-m0-hook-protocol-validation-report.md`) validated the hook protocol assumptions empirically. M1 grounds its design on those findings — particularly: use **exit 1** (not exit 0) for informational hooks, the stdin schema includes more fields than originally assumed (e.g., `cwd`, `hook_event_name`, `tool_use_id`), and `Stop` fires per assistant turn (the M3 problem).

---

## Goals

- Subcommand dispatcher in `cmd/main.go`: no-arg → MCP server (zero regression); `hook <event>` / `init` / `stats` route to subcommand handlers
- Minimum `pkg/state` layer: project root + per-machine UUID, file lock, atomic write, ledger CRUD, session-file occupancy
- `pkg/hook/{protocol, feedback}`: parse stdin per M0-validated schema; format stderr with `⚡ claude-context: ...` prefix
- `pkg/installer/{settings, rules, project, uninstall}`: full init / uninstall lifecycle with dry-run preview, y/N confirmation, automatic backups, idempotent re-runs, boundary-marked entries
- 5 hook handler stubs (`pre-read`, `pre-write`, `post-write`, `session-start`, `stop`): parse stdin, locate project, increment ledger counter, exit 0. Default silent. Verbose mode behind `CLAUDE_CONTEXT_DEBUG` env var.
- Tests: unit + integration + a manual smoke checklist
- MCP server zero behavioral regression (golden snapshot of `search_codebase` response)

---

## Non-Goals (M1)

- `pkg/scanner/{tokens, walker}` — moved to M2 (no consumer in M1; YAGNI)
- `scan` command — M2
- The 3 new MCP tools (`describe_codebase` / `get_project_rules` / `find_similar_bugs`) — M4
- Real session lifecycle (turn-vs-session distinction; per-session state files) — M3
- `Stop` hook actual semantics (M1 just bumps a counter; M3 must address the per-turn vs per-session problem)
- Any token-saving feature (anatomy, cerebrum, buglog) — M2/M4
- waste detector / memory consolidation — M5/M6
- First-class Windows support (macOS/Linux only)

---

## Cross-cutting Decisions (locked during M1 brainstorm)

These constrain everything below.

### CD1. Scope trim — defer scanner files to M2

The arch spec §11 listed `pkg/scanner/{tokens, walker}.go` as M1 deliverables. They have no consumer in M1 (their real users are M2's `scan` command and anatomy generation). YAGNI: defer to M2 where their interfaces can be designed alongside their actual consumer. M1 spec covers this trim explicitly; arch spec §11 should be annotated separately.

### CD2. CLI library — stdlib `flag` + manual `os.Args` dispatch

```go
switch os.Args[1] {
case "hook": dispatchHook(os.Args[2:])
case "init": dispatchInit(os.Args[2:])
...
default: runMCPServer()  (only when len(os.Args) < 2)
}
```

Each subcommand uses `flag.NewFlagSet("subcommand", flag.ContinueOnError)` for its own flags. Zero new CLI dependency. Mirrors the Go toolchain itself.

### CD3. Path resolution semantics (4 sub-decisions)

| # | Question | Decision |
|---|---|---|
| CD3a | CLI commands (init/scan/stats) — find project root how? | `git rev-parse --show-toplevel`; fallback to cwd |
| CD3b | Hook subcommands — find project root how? | PreToolUse / PostToolUse: walk up from `tool_input.file_path` (stdin) to find git root or `.claude-context/` marker. SessionStart / Stop: use stdin's `cwd`. |
| CD3c | File outside any project — what to do? | Silent no-op (`ledger.outside_project_skipped++`, exit 0) |
| CD3d | Project moves on disk (path changes → state lost) — how to keep state correlated? | **Per-machine UUID** at `<project>/.claude-context/.local-id` (gitignored). Generated on first `init`. Used as the lookup key for `~/.claude-context/projects/<id>/` instead of path hash. UUID travels with the directory; team members each have their own UUID; multi-machine state stays separate. |

### CD4. Init non-TTY behavior — refuse + helpful error

When `claude-context init` is invoked without a TTY (CI, scripts) AND without `--yes`/`--dry-run`/`--print`:

```
✗ stdin is not a TTY; refusing to apply changes.
  Re-run with --yes to confirm, --dry-run to preview, or --print to see final files.
```

Exit code 1. Detected via `golang.org/x/term` `IsTerminal(int(os.Stdin.Fd()))`. Safety > convenience: silent auto-confirmation under non-TTY would let background processes silently mutate `~/.claude/settings.json`.

### CD5. settings.json merge — append + boundary marker

Merge algorithm (per hook event):
1. Read existing `settings.json` (or `{}` if missing)
2. Find or create the `hooks` key
3. Find or create the event key (e.g., `PreToolUse`)
4. Find or create the matcher object (e.g., `{"matcher": "Read"}`)
5. In that matcher's `hooks` array:
   - Delete all entries with `_managed_by == "claude-context"` (idempotent upgrade)
   - Append our new entry with `_managed_by`, `_version`, `_installed_at` fields
6. Atomic-write back

Pre-existing user entries (without `_managed_by`) are **never touched**. M0 validated that schema validators tolerate `_managed_by` extra field. Multi-hook entries on the same matcher fire in array order (M0 confirmed this works for at least our own entry).

---

## Module Structure

### Files to create

```
cmd/
├── main.go                     ~80 LOC   subcommand dispatcher (top-level switch on os.Args[1])
├── cmd_hook.go                 ~100 LOC  hook subcommand dispatcher + 5 stub handlers
├── cmd_init.go                 ~150 LOC  init subcommand (dry-run / confirm / execute)
├── cmd_uninstall.go            ~50 LOC   --uninstall path
├── cmd_stats.go                ~40 LOC   stats subcommand (M1 minimal: print ledger counts)
├── cmd_version.go              ~10 LOC   version subcommand
└── usage.go                    ~30 LOC   top-level + subcommand usage strings

pkg/state/
├── paths.go                    ~80 LOC   project root resolution + .local-id UUID + global path generator
├── lock.go                     ~30 LOC   gofrs/flock wrapper with timeout
├── atomic.go                   ~25 LOC   atomic write (tmp + fsync + rename)
├── ledger.go                   ~80 LOC   token-ledger.json CRUD + RMW (M1 stubs only write hook-fired counters)
└── session.go                  ~50 LOC   _session.json CRUD (M1: create-or-touch only)

pkg/hook/
├── protocol.go                 ~80 LOC   stdin JSON parsing + Event type
└── feedback.go                 ~30 LOC   stderr formatter ("⚡ claude-context: ...") + exit code helpers

pkg/installer/
├── settings.go                 ~180 LOC  ~/.claude/settings.json merge + uninstall reverse
├── rules.go                    ~120 LOC  claude-context-rules.md write + CLAUDE.md @import injection/removal
├── project.go                  ~70 LOC   <project>/.claude-context/ scaffolding + .gitignore + .local-id
├── uninstall.go                ~40 LOC   uninstall orchestrator (calls reverse of above three)
└── templates/
    └── rules.md                ~50 LOC   embedded rules template
```

### Files to modify

```
cmd/mcp/main.go                 → renamed to cmd/mcp_server.go OR have its body extracted to a function
                                  callable from cmd/main.go's runMCPServer().
                                  Existing `claude mcp add ... -- claude-context` registrations
                                  must continue to work without modification.
go.mod                          → +1 dep github.com/gofrs/flock
                                  (golang.org/x/term is in standard ecosystem, marginal cost)
```

### Test files

```
tests/
├── state_test.go               unit: paths / lock / atomic / ledger / session
├── hook_test.go                unit: protocol stdin parsing (uses M0 fixtures) + feedback format
├── installer_test.go           unit: settings.json merge + boundary detection + uninstall reverse
├── integration/
│   ├── init_lifecycle_test.go  e2e: temp HOME → init --yes → verify → uninstall → diff empty
│   ├── hook_chain_test.go      e2e: chained hook fires using M0 fixtures
│   └── mcp_regression_test.go  regression: search_codebase response byte-equal to golden
└── golden/
    └── mcp_search_response.json  captured during M1, frozen thereafter
```

Total: ~17 new files + ~1 modified + 6 test files + 1 golden + 1 template = ~1500 LOC of Go.

---

## Key Data Shapes

### `_session.json` — M1 minimum fields

```json
{
  "_version": 1,
  "session_id": "<uuid from stdin SessionStart>",
  "project_id": "<uuid from .local-id>",
  "started_at": "<RFC3339>",
  "claude_code_model": "<from SessionStart payload's model field>",
  "stop_count": 0
}
```

The M1 SessionStart stub creates this file (or upserts it by `session_id`). No other M1 stub modifies it. M3 will use this for real session tracking.

### `token-ledger.json` — M1 schema

```json
{
  "_version": 1,
  "project_id": "<uuid>",
  "totals": {
    "hook_fired": {
      "pre-read": 0,
      "pre-write": 0,
      "post-write": 0,
      "session-start": 0,
      "stop": 0
    },
    "hook_errors": 0,
    "stdin_parse_failures": 0,
    "outside_project_skipped": 0,
    "write_skipped": 0
  },
  "first_recorded": "<RFC3339>",
  "last_updated": "<RFC3339>"
}
```

Each hook fire bumps the corresponding counter. M2-M6 will add fields to `totals` (anatomy_hits, writes, savings_estimated_tokens, etc.) — those additions are forward-compatible.

### Hook stdin Event type — `pkg/hook/protocol.go`

```go
type Event struct {
    SessionID            string          `json:"session_id"`
    TranscriptPath       string          `json:"transcript_path"`
    Cwd                  string          `json:"cwd"`
    HookEventName        string          `json:"hook_event_name"`

    // PreToolUse / PostToolUse / Stop
    PermissionMode       string          `json:"permission_mode,omitempty"`

    // PreToolUse / PostToolUse
    ToolName             string          `json:"tool_name,omitempty"`
    ToolInput            json.RawMessage `json:"tool_input,omitempty"`
    ToolUseID            string          `json:"tool_use_id,omitempty"`

    // PostToolUse only
    ToolResponse         json.RawMessage `json:"tool_response,omitempty"`
    DurationMs           *int            `json:"duration_ms,omitempty"`

    // SessionStart only
    Source               string          `json:"source,omitempty"`
    Model                string          `json:"model,omitempty"`

    // Stop only
    StopHookActive       *bool           `json:"stop_hook_active,omitempty"`
    LastAssistantMessage string          `json:"last_assistant_message,omitempty"`
}

// Helpers used by M1 stubs:
func ParseEvent(stdin io.Reader) (*Event, error)
func (e *Event) FilePathFromToolInput() (string, bool) // parses tool_input.file_path JSON
func (e *Event) IsRecursiveStop() bool                  // stop_hook_active == true
```

Using `json.RawMessage` for `ToolInput` and `ToolResponse` defers the per-tool schema parsing to M3-M4 when we actually need it. M1 stubs only need `tool_input.file_path` extraction (helper `FilePathFromToolInput`).

---

## CLI Dispatcher

### `cmd/main.go` top-level

```go
func main() {
    if len(os.Args) < 2 {
        runMCPServer()  // unchanged behavior; existing `claude mcp add` calls still work
        return
    }

    switch os.Args[1] {
    case "hook":     dispatchHook(os.Args[2:])
    case "init":     dispatchInit(os.Args[2:])
    case "stats":    dispatchStats(os.Args[2:])
    case "version":  printVersion()
    case "-h", "--help", "help":
        printTopUsage()
    default:
        fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n", os.Args[1])
        printTopUsage()
        os.Exit(2)
    }
}
```

**Hard constraints:**

- Anything other than the listed subcommands or no-args → exit 2 + usage (do **not** fall through to MCP server). Including `claude-context --foo` (starts with `-`) which would be misuse.
- The only path to MCP server is no-args. This preserves zero regression for `claude mcp add` registrations which spawn the binary with no args.

### `dispatchHook` — `cmd/cmd_hook.go`

```go
func dispatchHook(args []string) {
    if len(args) < 1 { os.Exit(2) }
    switch args[0] {
    case "pre-read":      runPreRead(os.Stdin)
    case "pre-write":     runPreWrite(os.Stdin)
    case "post-write":    runPostWrite(os.Stdin)
    case "session-start": runSessionStart(os.Stdin)
    case "stop":          runStop(os.Stdin)
    default:
        os.Exit(0)  // unknown event: silent (forward-compat with future Claude Code events)
    }
}
```

Forward-compat note: future Claude Code might add `SessionEnd` or `PreCompact` events. Old binaries will silently ignore them rather than fail.

---

## Init Lifecycle

### `claude-context init` flow

```
1. Parse flags
   --yes              skip y/N confirmation
   --dry-run          print plan only, do not write
   --print            print final file contents (zero-trust mode)
   --project          target = <project>/.claude/settings.json (project-level)
   --local            target = <project>/.claude/settings.local.json (project-local, gitignored)
   --no-scan          M1 placeholder; does nothing (scan is M2)
   --uninstall        delegate to uninstall flow (see below)

2. Resolve targets
   - Project root: git rev-parse --show-toplevel ?? cwd  (per CD3a)
   - settings.json target:
       default:    ~/.claude/settings.json
       --project:  <project>/.claude/settings.json
       --local:    <project>/.claude/settings.local.json
   - CLAUDE.md target: ~/.claude/CLAUDE.md  (fixed)
   - rules.md target:  ~/.claude/claude-context-rules.md  (fixed)
   - Project local dir: <project>/.claude-context/

3. Detect existing install (idempotency check)
   - Read settings.json, look for entries with _managed_by:"claude-context"
   - Read CLAUDE.md, look for <!-- claude-context-managed BEGIN --> marker
   - Existing → upgrade mode (delete old entries, write new — clean replacement)
   - Not existing → fresh install

4. TTY + flag mode resolution (per CD4)
   - No TTY AND no --yes AND no --dry-run AND no --print → print error, exit 1
   - --print → print each target's final content to stdout, exit 0
   - --dry-run OR (TTY AND no --yes) → continue to "show plan"
   - --yes OR user replied y to prompt → continue to "execute"

5. Show plan (always, unless --print which prints final contents directly)
   ╭─ Plan ────────────────────────────────────────────────────╮
   │ Will modify ~/.claude/settings.json:                       │
   │   + add 5 hook entries (PreToolUse:Read, ...)              │
   │ Will create ~/.claude/claude-context-rules.md (~50 lines)  │
   │ Will append 3 lines to ~/.claude/CLAUDE.md:                │
   │   <!-- claude-context-managed BEGIN -->                    │
   │   @~/.claude/claude-context-rules.md                       │
   │   <!-- claude-context-managed END -->                      │
   │ Will create <project>/.claude-context/                     │
   │   ├── .gitignore                                            │
   │   └── .local-id  (UUID for state correlation)              │
   │ Backups: <each-target-path>.bak.<timestamp>                │
   ╰────────────────────────────────────────────────────────────╯

   --dry-run → exit 0 here
   No --yes → "Apply these changes? [y/N]: " ; read one line of stdin
              y/Y/yes → execute; anything else → exit 0 without writing

6. Execute (sequential; failure stops; previous steps NOT auto-rolled-back — backups exist)
   step 1: backup each soon-to-modify target → <path>.bak.<RFC3339>
              - settings.json (if exists)
              - CLAUDE.md (if exists)
              - rules.md (if exists; upgrade mode only)
   step 2: settings.json: load → merge hooks (per CD5) → atomic write
   step 3: rules.md: write (overwrite from embedded template)
   step 4: CLAUDE.md: if no boundary marker present → append 3-line block
   step 5: project: mkdir <project>/.claude-context/
   step 6: project: write .gitignore (content: "_session.json\n*.bak.*\n")
   step 7: project: if .local-id missing → generate UUIDv4 + write

7. Print summary
   ✓ Registered 5 hooks in ~/.claude/settings.json
   ✓ Wrote rules to ~/.claude/claude-context-rules.md
   ✓ Added @import to ~/.claude/CLAUDE.md
   ✓ Initialized <project>/.claude-context/ (project ID: <uuid>)

   Backups stored at *.bak.<timestamp>

   To verify: start a new Claude Code session and run `claude-context stats` after.
   To uninstall: claude-context init --uninstall
```

### `claude-context init --uninstall` flow

```
1. Parse same flags (--yes / --dry-run / --print / --project / --local)
2. Detect installed targets (by boundary markers)
   - None found → exit 0 with "no claude-context installation found"
3. Show plan (same UI as init)
4. Execute
   step 1: backup each target
   step 2: settings.json: filter out entries where _managed_by == "claude-context"; atomic write
       - If matcher's hooks array becomes empty → delete the matcher object
       - If event's matcher array becomes empty → delete the event key
       - If hooks top-level becomes empty → delete the hooks key (preserve other settings.json content)
   step 3: CLAUDE.md: locate <!-- claude-context-managed BEGIN/END --> block; remove (3 lines)
   step 4: delete rules.md file
   step 5: leave <project>/.claude-context/ directory alone (user data); print cleanup hint
5. Print reverse summary
   ✓ Removed 5 hook entries from settings.json
   ✓ Removed @import from CLAUDE.md
   ✓ Deleted ~/.claude/claude-context-rules.md
   ! Project state at <project>/.claude-context/ kept intact.
     To remove: rm -rf <project>/.claude-context/
```

---

## Boundary Markers

### settings.json — `_managed_by` field on each entry

```json
{
  "type": "command",
  "command": "claude-context hook pre-read",
  "_managed_by": "claude-context",
  "_version": 1,
  "_installed_at": "2026-04-27T16:30:00Z"
}
```

- `_managed_by`: identifier for filter-on-uninstall
- `_version`: schema version (allows migration logic in future)
- `_installed_at`: nice-to-have debug field; uninstall ignores it

M0 R4 validated that Claude Code 2.1.119 tolerates these extra fields.

### CLAUDE.md — HTML comment boundary

```markdown
<!-- claude-context-managed BEGIN -->
@~/.claude/claude-context-rules.md
<!-- claude-context-managed END -->
```

Fixed 3-line block. BEGIN/END comments are detection + removal anchors. The `@import` uses tilde form (`@~/...`) per M0 R5 — most portable across user homes.

---

## Hook Stub Behavior

### Common skeleton (all 5 stubs)

```go
func runPreRead(stdin io.Reader) {
    defer recoverAndLog("pre-read")  // any panic → log + exit 0; never crash visibly

    event, err := hook.ParseEvent(stdin)
    if err != nil {
        ledger.IncrementSafe("stdin_parse_failures")
        os.Exit(0)  // silent — hook never blocks Claude
    }

    // Project root resolution (per CD3b)
    projectRoot, ok := resolveProjectFromEvent(&event)
    if !ok {
        ledger.IncrementSafe("outside_project_skipped")
        os.Exit(0)
    }

    // M1 stub work: bump counter, that's it
    ledger.IncrementSafe("hook_fired.pre-read")

    // Default: silent (exit 0, no stderr)
    // Diagnostic visibility via $CLAUDE_CONTEXT_DEBUG
    if debugEnabled() {
        feedback.WriteStderr("M1 stub: pre-read fired (project=" + projectID + ")")
        os.Exit(1)  // exit 1 makes Claude actually see the stderr per M0 R2
    }

    os.Exit(0)
}
```

### Per-stub differences

| stub | Project resolution | Extra logic | Default exit code |
|---|---|---|---|
| `pre-read` | from `tool_input.file_path` | none | 0 |
| `pre-write` | from `tool_input.file_path` | none | 0 |
| `post-write` | from `tool_input.file_path` | none | 0 |
| `session-start` | from `event.cwd` | upsert `_session.json` (occupancy: session_id, project_id, started_at, model) | 0 |
| `stop` | from `event.cwd` | bump counter; **completely ignore** `stop_hook_active` and `last_assistant_message` (M3 territory) | 0 |

### Exit code rationale (per M0 R2)

- M1 stubs have no message for Claude → exit 0 + no stderr is the silent + correct combo.
- M2+ real hooks WILL have messages → those will use exit 1 + `⚡ claude-context: ...` stderr (per M0 R2 finding that exit 1 is the informational channel; exit 0 stderr is silent to Claude).
- Stubs do NOT use exit 1 by default to avoid spamming Claude transcripts during M1 deployment.

### Debug visibility — three levels

| Env | Behavior |
|---|---|
| (default) | Silent: ledger only. User sees nothing in their Claude Code session. |
| `CLAUDE_CONTEXT_DEBUG=1` | Stderr feedback via exit 1: each stub fires `⚡ claude-context: M1 stub: <event> fired (project=<uuid>)` so the user can verify in Claude transcripts that hooks actually triggered. |
| `CLAUDE_CONTEXT_DEBUG=2` | Above + extra detail to stdout (only user terminal sees, not Claude). Used during local debugging. |

M1 acceptance + manual smoke test both run with `CLAUDE_CONTEXT_DEBUG=1`. Production users keep it unset.

---

## Concurrency & Locking (M1 minimum viable)

Reuses arch spec §7 design: single `runtime.lock` per project at `~/.claude-context/projects/<project_id>/runtime.lock`.

M1 only writes to:
- `_session.json` (SessionStart stub)
- `token-ledger.json` (every stub)

Both use:
1. Acquire flock-X on `runtime.lock` (timeout per arch spec: 50ms reads, 50ms preread writes, 100ms others)
2. Read current state (or empty if missing)
3. Modify in memory
4. Atomic write (temp + fsync + rename)
5. Release lock

If lock timeout exceeded → log to `~/.claude-context/projects/<project_id>/hook-errors.log` (best-effort append, no lock — tolerate races on this log file since rotation is per-line and lossy is acceptable) and exit 0. The skip is **not** recorded in `ledger.write_skipped` (we can't take the lock to write it); accept that under high contention the counter under-reports. M3 with proper session-keyed state files will largely eliminate the contention scenario.

`pkg/state/atomic.go`:
```go
func atomicWrite(path string, data []byte) error {
    tmp := path + ".tmp." + randSuffix()
    if err := os.WriteFile(tmp, data, 0644); err != nil { return err }
    f, _ := os.Open(tmp); f.Sync(); f.Close()
    return os.Rename(tmp, path)
}
```

Lock backend: `github.com/gofrs/flock` (small dep; cross-platform `fcntl` wrapper). Stdlib `syscall.Flock` is platform-specific and would need our own abstraction — `gofrs/flock` is the right call.

Read tolerance: JSON parse failure → don't panic; return empty state struct. Covers the rare race where another process is mid-rename.

---

## Testing Strategy

### Unit tests

```
tests/state_test.go
├── TestProjectRootGitRepo          (cwd in git subdir → resolves to git root)
├── TestProjectRootNonGit           (cwd not in git → uses cwd)
├── TestLocalIDGeneration           (.local-id missing → generates UUIDv4)
├── TestLocalIDStable               (.local-id present → reads same UUID twice)
├── TestAtomicWriteCrashSafe        (simulated panic mid-write → file is old or new, never half)
├── TestLedgerIncrementRMW          (concurrent 100 ++ → final == 100, no lost increments)
└── TestSessionJSONUpsert           (SessionStart fires multiple times → same session_id no duplicate)

tests/hook_test.go
├── TestParseEvent_PreRead          (uses fixtures/hook-payloads/pre-read.json)
├── TestParseEvent_PreWrite         (same for pre-write)
├── TestParseEvent_PostWrite        (same)
├── TestParseEvent_SessionStart     (same)
├── TestParseEvent_Stop             (same)
├── TestParseEvent_BadJSON          (truncated/non-JSON → ParseEvent returns error, no panic)
└── TestFeedbackFormat              (formatStderr("foo") → "⚡ claude-context: foo")

tests/installer_test.go
├── TestMergeEmptySettings          (empty file → adds 5 entries)
├── TestMergeExistingHooks          (user has my-bash-logger on Bash → untouched, ours added)
├── TestMergeUpgrade                (we have an old entry → delete old + add new = idempotent)
├── TestUninstallByMarker           (entries with _managed_by → cleaned; others kept)
├── TestUninstallEmptyMatcher       (after delete, matcher hooks empty → matcher object removed)
├── TestRulesMDInjection            (CLAUDE.md missing → created with boundary block)
└── TestRulesMDIdempotent           (re-init → boundary block exists once, not duplicated)
```

### Integration tests

```
tests/integration/
├── init_lifecycle_test.go
│   ├── TestInitFreshThenUninstall  (temp HOME → init --yes → 5-file state correct → uninstall --yes → settings.json + CLAUDE.md diff is empty vs pre-init)
│   └── TestInitNonTTYRefusal       (no TTY, no --yes → error + exit 1)
│
├── hook_chain_test.go
│   ├── TestSessionStartCreatesSession  (pipe SessionStart fixture stdin → ledger.session-start == 1, _session.json created)
│   ├── TestPreReadIncrementsCounter    (pipe pre-read fixture → ledger.pre-read == 1)
│   ├── TestPreWriteIncrementsCounter   (same for pre-write)
│   ├── TestPostWriteIncrementsCounter  (same for post-write)
│   ├── TestStopFireMultipleTimes       (per M0 finding: pipe stop 3x → ledger.stop == 3)
│   └── TestBadJSONStdinSafe            (pipe garbage → ledger.stdin_parse_failures == 1, exit 0)
│
└── mcp_regression_test.go
    └── TestSearchCodebaseUnchanged  (spawn cmd/main.go no-args as MCP server → send search_codebase JSON-RPC request → assert response byte-equal to tests/golden/mcp_search_response.json)
```

### Golden snapshot

`tests/golden/mcp_search_response.json` is captured during M1 development by running the MCP server against a known fixture index and saving the response. After that point, **any** behavioral change to `search_codebase` response (field order, formatting, additional fields) will fail the regression test. This is intentional — M1's job includes "MCP zero regression"; loose assertions defeat the point. If M2+ deliberately changes search response shape, that PR also re-captures the golden.

### Manual smoke checklist (`docs/m1-smoke-checklist.md`)

Not in CI. Validates real Claude Code integration:

```
1. claude-context init --yes
2. CLAUDE_CONTEXT_DEBUG=1 claude  (in some throwaway git repo)
3. Issue a few prompts: "read README.md" / "create hello.txt" / "edit hello.txt"
4. Each command should produce in Claude's transcript:
   "Failed with non-blocking status code: ⚡ claude-context: M1 stub: pre-read fired (project=<uuid>)"
   (one line per fired hook event)
5. /exit
6. cd <repo> && claude-context stats
   - Should show non-zero counts for pre-read, pre-write/post-write, session-start, stop
7. claude-context init --uninstall --yes
8. git status ~/.claude/  (or diff against .bak files)
   - Should be identical to pre-init state
```

---

## Performance Budget

```
BenchmarkParseEvent      < 5ms     (json.Unmarshal of ~1KB payload)
BenchmarkLedgerIncrement < 20ms    (RMW + flock + atomic write of ~1KB ledger)
BenchmarkHookStubE2E     < 80ms p95 (full path: parse → resolve project → bump ledger → exit)
```

If `BenchmarkHookStubE2E` exceeds 80ms p95, M1 is **not** done — must profile and optimize before ship. M0 confirmed Claude Code does not enforce a hook timeout under 3000ms, so 80ms is a self-imposed ceiling rooted in arch spec §5's preread budget.

---

## Risks & TODOs for M2-M3

| # | Risk / TODO | Mitigation |
|---|---|---|
| RM1 | Init failure mid-flow does not auto-rollback | `.bak` files exist for every modified target; user runs `init --yes` again (upgrade mode is idempotent) or restores manually. Documented in init's failure messages. |
| RM2 | User's settings.json schema validator (some 3rd-party tool) might reject `_managed_by` extra field on hook entries despite Claude Code accepting it (M0 R4) | Backup-first + atomic-write tolerates corruption. If reported by users → revisit boundary scheme in a future M2 or M3 sub-spec; could fall back to a parallel `~/.claude/settings.json.claude-context-managed` file listing managed entry IDs. |
| RM3 | `gofrs/flock` cross-platform behavior differences | Unit tests cover macOS + Linux on CI; Windows is best-effort, no CI guarantee. |
| RM4 | `_session.json` race when two concurrent Claude Code sessions touch the same project | M1 stub is harmless (occupancy only). M3 will switch to `_session_<id>.json` when implementing real session tracking. M1 spec flags this explicitly. |
| RM5 | Stop hook's per-turn fires inflate ledger counts beyond user expectation | `claude-context stats` output adds a one-line note: "stop counts include per-turn fires (per M0 finding); session-end semantics deferred to M3". |
| RM6 | `_installed_at` ISO timestamp string in settings.json could trip strict validators | Risk low (JSON allows extra fields by default); if reported, M2 changes to unix timestamp number or removes the field. |

### Explicit TODOs left for M2-M3 (called out in M1 spec for traceability)

- **M2**: implement `pkg/scanner/{tokens, walker}.go` and the `scan` command
- **M2**: upgrade hook stubs to real functionality (pre-read calls anatomy lookup, repeat-read suppression)
- **M3**: solve `Stop` turn-vs-session distinction; possibly introduce `_session_<id>.json` keyed by session
- **M3**: handle `stop_hook_active` recursion guard in real stop logic
- **M4**: add 3 new MCP tools (`describe_codebase`, `get_project_rules`, `find_similar_bugs`)

---

## Acceptance Criteria

M1 is **done** when ALL of these hold:

1. ✅ `claude-context` (no args) → MCP server starts; `tests/integration/mcp_regression_test.go` passes (golden snapshot match)
2. ✅ `claude-context init --yes` runs end-to-end against a temp HOME; produces the 5 expected file changes
3. ✅ `claude-context init --uninstall --yes` reverses cleanly: settings.json + CLAUDE.md diffs against pre-init state are empty
4. ✅ All 5 hook stubs callable via `claude-context hook <event>`, parse the corresponding M0 fixture stdin, never panic, increment ledger correctly
5. ✅ `claude-context stats` prints meaningful ledger output (M1 minimal version: counter table)
6. ✅ Three benchmarks (§Performance Budget) pass within budget
7. ✅ Manual smoke checklist (`docs/m1-smoke-checklist.md`) executed against a real Claude Code session and produces the expected `⚡ claude-context: M1 stub: ...` feedback in Claude transcripts (with `CLAUDE_CONTEXT_DEBUG=1`)
8. ✅ All M1 unit + integration tests pass on macOS and Linux CI

---

## Out of Scope for M1

(Already covered in §Non-Goals; restated here for the reviewer's convenience.)

- Token-saving features (anatomy / cerebrum / buglog) — M2 / M4
- `scan` command and `pkg/scanner/{tokens, walker}` — M2
- 3 new MCP tools — M4
- Real session lifecycle (turn-vs-session distinction; per-session state file naming) — M3
- `Stop` hook's `stop_hook_active` recursion handling — M3
- Cross-machine state sync — out of project scope (use git for sharable artifacts)
- waste detector, memory consolidation — M5 / M6
- Windows tier-1 support
- Auto-update mechanism, telemetry

---

## Next Step After This Spec

`superpowers:writing-plans` produces an executable, task-by-task plan derived from this spec. Plan goes to `docs/superpowers/plans/2026-04-27-m1-foundation.md`. Then `subagent-driven-development` executes the plan task-by-task with two-stage review per task.
