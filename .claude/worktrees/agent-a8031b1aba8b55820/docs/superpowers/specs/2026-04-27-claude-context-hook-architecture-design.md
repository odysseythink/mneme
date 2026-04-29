# Claude-Context Hook Architecture Design

**Date:** 2026-04-27
**Status:** Draft
**Scope:** Embed openwolf-style token-saving mechanisms (Claude Code hooks + project memory) into the existing `claude-context` Go binary, alongside the unchanged MCP server. One binary, multiple subcommands. Six-milestone delivery.

---

## Problem

The current `claude-context` is a stateless MCP server: it answers `index_codebase` / `search_codebase` calls when Claude chooses to invoke them. It has no visibility into Claude's other actions (`Read`, `Edit`, `Write`) and no way to prevent Claude from re-reading the same file ten times in a session, ignoring high-leverage descriptions, or repeating known-bad patterns.

The openwolf project (TypeScript, Claude Code hooks plugin) demonstrates 13 mechanisms that collectively achieve a self-reported 65–80% LLM token reduction, with the headline mechanism (anatomy map + repeated-read suppression) hitting 71% of file reads.

We want those mechanisms inside `claude-context` — but as part of the existing single-binary deployment, not as a second tool the user has to install.

---

## Goals

- Port all 13 openwolf token-saving mechanisms into `claude-context`, organized into 6 deliverable milestones.
- Single binary: subcommand dispatch keeps MCP server mode (current default behavior) untouched while adding `hook`, `scan`, `init`, `stats`, `cerebrum`, `buglog` subcommands.
- One install command (`claude-context init`) registers Claude Code hooks, writes the rules file, and creates the per-project state directory.
- Project-local state files (`anatomy.md`, `cerebrum.md`, `buglog.json`) are git-friendly so teams can share learnings.
- Quantifiable savings via persistent `token-ledger.json`.
- Existing MCP server tools (`index_codebase`, `search_codebase`, `ping_embedding`) have **zero behavioral regression**.

---

## Non-Goals

**Architecture:**
- Inlining `anatomy` descriptions into `search_codebase` responses (deferred experiment, post-M2).
- Background daemon for periodic consolidation — replaced by lazy trigger inside `SessionStart` hook.
- Web UI / dashboard.
- Hook adapters for non–Claude Code clients (Cursor, Cline, etc.); when those gain hook systems with real demand, separate spec.
- Cross-machine cerebrum/buglog sync service — git is the sync mechanism.

**Functionality:**
- LLM-assisted edit-pattern classification (rule-based only, for cost and latency).
- LLM-assisted file description extraction (heuristics only).
- Tree-sitter / true AST parsing — reuse existing splitter heuristics.
- Languages beyond Go / Python / JS / TS / Markdown / known config files in M2; others fall back to "first non-empty non-comment line".
- Embedding-based semantic match for buglog (use stop-word + token overlap; revisit in M4 if quality is poor).
- Auto-PR creation from new cerebrum/buglog entries.
- Streaming MCP responses — anatomy is returned in full.
- User-defined hook events.
- First-class Windows support (macOS / Linux are tier-1; Windows is best-effort).

**Operations:**
- Auto-update mechanism — users use their package manager.
- Telemetry — all data stays local.
- Multi-user isolation on shared machines (rely on OS user separation).

---

## Cross-cutting Decisions

Four decisions resolved during brainstorm; they constrain every milestone.

### D1. Hybrid state storage

| File | Location | Git-share intent |
|---|---|---|
| `anatomy.md` | `<project>/.claude-context/` | yes (team baseline) |
| `cerebrum.md` | `<project>/.claude-context/` | yes (curated rules) |
| `buglog.json` | `<project>/.claude-context/` | optional (auto-learned) |
| `memory.md` | `~/.claude-context/projects/<hash>/` | no (per-user log) |
| `_session.json` | `~/.claude-context/projects/<hash>/` | no (runtime) |
| `token-ledger.json` | `~/.claude-context/projects/<hash>/` | no (per-user stats) |

Project root resolution: `git rev-parse --show-toplevel`, fallback to `cwd`.
`<hash>` = `sha256(absolute project path)[:16]`.

### D2. Init UX — default automatic with explicit consent

`claude-context init` prints a dry-run preview of the 5 changes, prompts `y/N`, then writes:

- `--yes` skip confirmation
- `--dry-run` print only, no writes
- `--print` print final file contents (zero-trust mode)
- `--project` target `<project>/.claude/settings.json` instead of user-level
- `--local` target `<project>/.claude/settings.local.json`
- `--no-scan` skip the optional initial `scan` run
- `--uninstall` reverse all changes (state directory left intact)

Originals are backed up to `<path>.bak.<timestamp>`. Boundary markers make `--uninstall` deterministic.

### D3. CLAUDE.md fragment via independent file + import

Two artifacts:
1. `~/.claude/claude-context-rules.md` — the actual instructions (overwrite-on-upgrade).
2. One-line `@~/.claude/claude-context-rules.md` appended to `~/.claude/CLAUDE.md`, wrapped in HTML comment boundary markers.

Upgrade = overwrite the rules file. Uninstall = delete file + remove the bounded `@import` block.

### D4. MCP / hook integration via shared `pkg/state`

- New shared package `pkg/state` owns all `.claude-context/` reads and writes.
- MCP server's existing tools (`index_codebase` / `search_codebase` / `ping_embedding`) are unchanged.
- MCP server gains 3 new read-only tools backed by `pkg/state`:
  - `describe_codebase` → returns `anatomy.md`
  - `get_project_rules` → returns `cerebrum.md`
  - `find_similar_bugs` → semantic match against `buglog.json`
- Hook subcommands also use `pkg/state` directly (do not call the MCP server).
- File locking via `github.com/gofrs/flock` on a single `runtime.lock` per project.
- All persisted files carry a `_version` field (or `<!-- ... v1 -->` header for Markdown).

---

## Binary Mode Dispatch

`main.go` switches on `os.Args[1]`:

```
claude-context                    → MCP server (default; behavior unchanged)
claude-context hook <event>       → event ∈ {pre-read, pre-write, post-write,
                                            session-start, stop}
claude-context scan [--force]     → generate anatomy.md
claude-context init [flags]       → install hooks + rules
claude-context init --uninstall   → reverse install
claude-context stats [--waste|--json|--errors]   → ledger / waste / errors report
claude-context cerebrum {add|list|remove}        → manage rules
claude-context buglog {list|clear}               → manage bug log
claude-context hook _<internal>                  → internal subcommands prefixed with _,
                                                   not registered with Claude Code; only
                                                   spawned by other hooks (e.g.,
                                                   _consolidate fired by SessionStart)
```

Constraints:
- No-arg invocation **must** remain MCP server mode (existing `claude mcp add` registrations rely on this).
- No subcommand may write a panic stack trace to stderr (in hook mode, stderr is the channel Claude reads).
- Internal subcommands (`_<name>`) bypass init's settings.json registration; they exist only for inter-hook coordination.

---

## Module Structure

```
pkg/
├── types.go            (existing: Indexer/Searcher/Store/EmbeddingProvider/Splitter)
├── mcp/                (existing; +3 new tools registered in M4)
├── context/            (existing)
├── embedding/          (existing)
├── vectordb/           (existing)
├── splitter/           (existing)
├── state/              (NEW — all .claude-context/ reads & writes; flock; atomic writes)
│   ├── paths.go        (project root + global root resolution)
│   ├── anatomy.go
│   ├── cerebrum.go
│   ├── buglog.go
│   ├── memory.go
│   ├── ledger.go
│   ├── session.go
│   ├── lock.go         (gofrs/flock wrapper)
│   └── atomic.go       (temp-file + fsync + rename)
├── hook/               (NEW — one handler per Claude Code event)
│   ├── protocol.go     (stdin JSON parsing)
│   ├── feedback.go     (stderr formatting; "⚡ claude-context: ..." prefix)
│   ├── preread.go
│   ├── prewrite.go
│   ├── postwrite.go
│   ├── sessionstart.go
│   └── stop.go
├── scanner/            (NEW — anatomy generation)
│   ├── walker.go       (file traversal + filters)
│   ├── extractor.go    (per-language dispatcher)
│   ├── ext_go.go
│   ├── ext_py.go
│   ├── ext_js.go
│   ├── ext_md.go
│   ├── ext_known.go    (known config files: package.json, Cargo.toml, go.mod, ...)
│   └── tokens.go       (char-ratio token estimator)
├── installer/          (NEW — init / uninstall logic)
│   ├── settings.go     (~/.claude/settings.json merge)
│   ├── rules.go        (claude-context-rules.md template + @import injection)
│   ├── project.go      (.claude-context/ skeleton + .gitignore)
│   └── uninstall.go
├── waste/              (NEW — M5 diagnostics)
└── consolidator/       (NEW — M6 memory aging)
```

---

## Storage Layout

**Project-local** (`<project>/.claude-context/`):

```
.claude-context/
├── anatomy.md          <!-- claude-context anatomy v1 -->
├── cerebrum.md         <!-- claude-context cerebrum v1 -->
├── buglog.json         {"_version": 1, "entries": [...]}
└── .gitignore          (init-written; ignores nothing here, all files commit-friendly)
```

**Global** (`~/.claude-context/projects/<hash>/`):

```
~/.claude-context/projects/<hash>/
├── memory.md           <!-- claude-context memory v1 -->
├── _session.json       {"_version": 1, "start": "...", "reads": [...], ...}
├── token-ledger.json   {"_version": 1, "totals": {...}, "by_session": [...]}
└── runtime.lock        (gofrs/flock target — this is the global lock)
```

**Existing global** (`~/.claude-context/`): unchanged — `config.yaml`, `chromem/`, plus the new `projects/` parent.

### `buglog.json` schema

```json
{
  "_version": 1,
  "entries": [
    {
      "id": "uuid",
      "first_seen": "RFC3339",
      "last_seen": "RFC3339",
      "occurrence": 3,
      "file": "src/foo.go",
      "category": "null_safety",
      "summary": "added nil check before deref",
      "root_cause": "...",
      "fix_text": "...",
      "tags": ["pointer", "init"]
    }
  ]
}
```

---

## Data Flow

### A. `scan` → anatomy

```
claude-context scan
  → walker: traverse project files; apply filters
            (>1MB | binary detected via first 512B | *.lock | *.min.* |
             .env* | .git | node_modules | dist | build | vendor | .gitignore matches)
  → extractor: dispatch by extension (go/py/js/md/known) → fallback for others
            returns {path, description ≤100 chars, est_tokens, language}
  → aggregate: group by directory, sort lexicographically
  → state.anatomy.Write: flock(W) → atomic write → unlock
  → ledger: scan_count++
```

Incremental mode (mtime skip) deferred to M6.

### B. PreToolUse:Read

```
Claude Code spawns: claude-context hook pre-read
  stdin → {tool_name: "Read", tool_input: {file_path: "/abs/..."}, session_id, ...}

  hook/preread.go:
    1. parse stdin (10ms deadline; on timeout, exit 0 silently)
    2. resolve file → project-relative path; if outside project, exit 0
    3. read anatomy.md (no lock; tolerate stale)
       hit → stderr: "⚡ claude-context: src/foo.go — handles auth (~340 tok)"
    4. read _session.json (flock-S, 50ms)
       already present → stderr: "⚡ claude-context: src/foo.go already read this session (~340 tok)"
    5. flock-X 50ms: append path to session.reads; ledger.read_attempts++;
                    if anatomy_hit then ledger.anatomy_hits++
    6. exit 0

Latency target: <80ms p99
```

### C. PreToolUse:Write/Edit

```
stdin → {tool_name: "Edit", tool_input: {file_path, old_string, new_string}}

  hook/prewrite.go:
    1. read cerebrum.md → extract rule patterns
    2. regex match new_string → on hit, stderr: "⚠️ cerebrum: never use var (line 12)"
    3. read buglog.json → filter (file == this file) OR token overlap with new_string
       Top-3 → stderr: "📋 buglog: 2 past bugs in this file: [summaries]"
    4. exit 0 (warn-only, never block)
```

### D. PostToolUse:Write/Edit

```
stdin → as PreToolUse + tool_response

  hook/postwrite.go:
    1. if tool_response failed → ledger++; exit 0 (do not learn from failures)
    2. classifyEdit(old_string, new_string) → one of 13 fix categories
    3. summary = renderSummary(category, diff_snippet)   // 5–15 words
    4. flock-X 100ms:
         - append summary to memory.md (current session block)
         - upsert buglog.json:
             same file + same category + within 5min → occurrence++
             else → new entry
         - ledger.writes++
    5. exit 0
```

### E. SessionStart

```
hook/sessionstart.go:
    1. write _session.json: {start: now, reads: [], writes: [], anatomy_hits: 0, ...}
    2. lazy-maintenance check:
       if memory.md > 5000 tokens OR last_consolidation > 7d
         → fork: claude-context hook _consolidate &  (background; non-blocking)
    3. exit 0
```

### F. Stop

> ⚠️ **M0 finding**: Claude Code 2.1.119 fires `Stop` after **every assistant turn**, not at session exit. The flow below as originally drafted (write session-row, delete `_session.json`) would corrupt mid-session state. **M3 spec brainstorm must address turn-vs-session distinction** before this flow can be implemented. Likely paths: (a) treat Stop as turn-end and find another signal for session-end; (b) use `stop_hook_active` field as recursion guard; (c) idle-timeout on `_session.json` rather than explicit close. See `2026-04-27-m0-hook-protocol-validation-report.md` "Bonus finding" for full discussion.

```
hook/stop.go (DRAFT — needs M3 turn-vs-session decision before implementation):
    1. read _session.json; compute session totals
    2. flock-X 200ms:
         - append session row to memory.md table   ← would fire per-turn under current Claude Code; bad
         - update token-ledger.json: accumulate counters
         - compute savings = anatomy_hits × avg_anatomy_token_saving + repeated_reads_blocked
    3. delete _session.json   ← would break mid-session under current Claude Code; bad
    4. exit 0
```

---

## Hook Protocol

> **Validated against Claude Code 2.1.119** in the M0 spike (see `2026-04-27-m0-hook-protocol-validation-report.md`). Schema and exit-code semantics below reflect empirical findings, not pre-spike assumptions.

**Input** (Claude Code passes via stdin) — full real schema captured in M0:

**Common fields (every event):**

```json
{
  "session_id": "<uuid>",
  "transcript_path": "<HOME>/.claude/projects/<project-slug>/<session>.jsonl",
  "cwd": "/process/working/directory",
  "hook_event_name": "PreToolUse | PostToolUse | SessionStart | Stop"
}
```

**Per-event additions:**

| Field | PreToolUse | PostToolUse | SessionStart | Stop |
|---|---|---|---|---|
| `permission_mode` | ✓ | ✓ | — | ✓ |
| `tool_name` | ✓ | ✓ | — | — |
| `tool_input` | ✓ | ✓ | — | — |
| `tool_use_id` | ✓ (correlates Pre↔Post) | ✓ | — | — |
| `tool_response` | — | ✓ | — | — |
| `duration_ms` | — | ✓ (int, ms) | — | — |
| `source` | — | — | ✓ ("startup", etc.) | — |
| `model` | — | — | ✓ ("claude-opus-4-7[1m]") | — |
| `stop_hook_active` | — | — | — | ✓ (recursion guard) |
| `last_assistant_message` | — | — | — | ✓ (string) |

**Note:** `tool_name` is **NOT** present in `SessionStart` or `Stop` events — dispatch on `hook_event_name` instead. Canonical sample payloads at `tests/fixtures/hook-payloads/`.

**Output contract** (validated by M0 R2):

| Exit code | What Claude sees | Tool call proceeds? | Use this for |
|---|---|---|---|
| **0** | nothing — stderr is silent to Claude (terminal only) | yes | hooks that have **no** message for Claude (pure state init / silent telemetry) |
| **1** | `Failed with non-blocking status code: <stderr>` | yes | **informational hooks** — anatomy hits, cerebrum warnings, buglog hints (use exit 1, accept the wrapping) |
| **2** | elaborate hook-error block; Claude **perceives blocking** | yes (but Claude reacts as if blocked) | **avoid** — Claude verbally responds as if the action was denied |
| **127** | identical framing to exit 1 | yes | (no use; treat as accidental "command not found") |

🚨 **Critical correction from M0**: Earlier draft assumed `exit 0 + stderr` was the informational channel. **It is not.** Use **exit 1** to feed text to Claude. The "Failed with non-blocking status code:" wrapping is unfortunate cosmetic noise but content is delivered.

**Stderr formatting**: All informational stderr lines carry the `⚡ claude-context: ...` prefix so Claude can identify them inside the wrapped error message. Example end result Claude sees: `Failed with non-blocking status code: ⚡ claude-context: src/foo.go — handles auth (~340 tok)`.

**Follow-up** (post-M0, not blocking M1): investigate whether Claude Code supports JSON stdout from hooks for structured feedback that bypasses the wrapping. A small future spike could verify this with the same probe scripts. If supported, it'd be a cleaner channel.

**Fault tolerance:**

- stdin parse failure (bad JSON / missing fields) → exit 0 silently.
- Top-level `defer recover()` in every hook handler; suppressed panics logged to `hook-errors.log`, never to stderr.
- Soft timeouts (preread 80ms / prewrite 200ms / postwrite 200ms / stop 500ms) → abort the operation, exit 0. A wasted feedback opportunity beats blocking Claude.

---

## Concurrency & Locking

Concurrent writers possible: hook processes (Claude Code may overlap events), background consolidation fork, MCP server's new read tools, manual `scan`, manual `vim cerebrum.md`.

**Strategy — single global lock per project:**

| File | Write | Read |
|---|---|---|
| `anatomy.md` | flock-W on `runtime.lock` + atomic rename | unlocked (tolerate stale) |
| `cerebrum.md` | flock-W (CLI / user edit) | unlocked |
| `buglog.json` | flock-W + read-modify-write + atomic rename | flock-S 50ms |
| `memory.md` | flock-W + append-only | unlocked |
| `_session.json` | flock-W + atomic rename | flock-S 50ms |
| `token-ledger.json` | flock-W + RMW + atomic rename | flock-S |

`runtime.lock` lives in `~/.claude-context/projects/<hash>/`.

**Lock-acquisition timeout policy** (subset of the total hook timeout in §6):
- Reads: 50ms to acquire flock-S → fallback to unlocked read (stale OK; hook must return)
- Writes: per-event lock budgets (preread 50ms, prewrite/postwrite 100ms, stop 200ms) → if exceeded, skip the write and increment `ledger.write_skipped++`

These lock budgets fit inside the larger hook total-time budgets (preread 80ms / prewrite 200ms / postwrite 200ms / stop 500ms from §6) — the rest is for parsing, regex matching, and stderr writes.

**Atomic write:**

```go
func atomicWrite(path string, data []byte) error {
    tmp := path + ".tmp." + randSuffix()
    if err := os.WriteFile(tmp, data, 0644); err != nil { return err }
    f, _ := os.Open(tmp); f.Sync(); f.Close()
    return os.Rename(tmp, path)
}
```

**Read tolerance:** JSON parse failure → don't panic; record `ledger.corrupted_reads++`; return empty state. Covers the rare race where another process is mid-rename.

---

## Init Lifecycle

### `claude-context init`

```
0. Pre-check
   - Verify claude-context binary on PATH
   - Detect existing install via boundary markers → "upgrade mode" replaces our blocks
   - Resolve target settings.json (default ~/.claude/settings.json; --project / --local)
   - Resolve target CLAUDE.md (default ~/.claude/CLAUDE.md; --project-claude-md)

1. Plan (dry-run output, no side effects)
   ├─ [WRITE]  ~/.claude/settings.json   (hook config diff)
   ├─ [WRITE]  ~/.claude/claude-context-rules.md
   ├─ [APPEND] ~/.claude/CLAUDE.md       (one @import line, boundary-marked)
   ├─ [MKDIR]  <project>/.claude-context/
   ├─ [WRITE]  <project>/.claude-context/.gitignore
   └─ [OPTIONAL] claude-context scan

2. Confirmation
   Unless --yes: "Apply these changes? (y/N)"
   --dry-run exits here. --print writes nothing, prints final contents.

3. Execute
   ├─ Backup existing files: <path>.bak.YYYYMMDD-HHMMSS
   ├─ settings.json: load → deep-merge hooks (dedupe by _managed_by) → atomic write
   ├─ claude-context-rules.md: atomic write (template includes <!-- claude-context-rules v1 -->)
   ├─ CLAUDE.md: append boundary-wrapped @import block if absent
   ├─ mkdir + write .gitignore
   └─ scan (unless --no-scan)

4. Post
   Print summary: ✓ hook registered, ✓ rules file, ✓ anatomy generated
   Tell user to restart Claude Code session
```

### `claude-context init --uninstall`

```
1. settings.json: filter out hook entries with _managed_by:"claude-context"
2. CLAUDE.md: locate <!-- claude-context-managed BEGIN/END --> block; remove
3. Delete ~/.claude/claude-context-rules.md
4. Leave <project>/.claude-context/ alone (user data); print cleanup hint
```

### Boundary markers

**settings.json** (JSON has no comments — use a field):

```json
{
  "matcher": "Read",
  "hooks": [{
    "type": "command",
    "command": "claude-context hook pre-read",
    "_managed_by": "claude-context",
    "_version": 1
  }]
}
```

**CLAUDE.md** (HTML comment):

```markdown
<!-- claude-context-managed BEGIN -->
@~/.claude/claude-context-rules.md
<!-- claude-context-managed END -->
```

**Idempotency:** repeated `init` is safe — detected install enters upgrade mode; boundary-marked blocks replaced; user content preserved.

---

## Error Handling Philosophy

Three layers, three policies:

| Layer | Failure mode | Examples |
|---|---|---|
| Hook subcommand | Never block Claude; exit 0; log to `hook-errors.log` (rotated, last 1000 lines) | bad stdin, missing anatomy, lock timeout |
| MCP server tool | Return JSON-RPC `isError: true` content; never crash the process | corrupt buglog, anatomy not generated |
| CLI command (init/scan/stats) | Standard CLI: non-zero exit + stderr; never panic | permission denied, disk full, missing PATH |

| Error | Hook | MCP | CLI |
|---|---|---|---|
| stdin / arg parse failure | exit 0 silent | error content | usage + exit 1 |
| File not found (e.g., anatomy not generated) | exit 0 silent | "not initialized, run scan" | print + exit 1 |
| JSON / MD corrupt | log + empty state + ledger.corrupted++ | error content | offer `--repair` |
| flock timeout | log + skip + ledger.write_skipped++ | error content | extend wait |
| Disk full | log + exit 0 | error content | exit 1 |
| Permission denied | log + exit 0 (means init wasn't run) | error content | exit 1 |
| panic | recover → log full stack → exit 0 | recover → JSON-RPC error | recover → print + exit 1 |

**Hook error log:**
- Path: `~/.claude-context/projects/<hash>/hook-errors.log`
- Format: `<RFC3339> <event> <error>`
- Rotation: truncate keeping last 1MB / 1000 lines when exceeded
- User access: `claude-context stats --errors`

**Rule of thumb:** stderr in hook mode is for Claude. Anything not useful to Claude (stack traces, "DEBUG:" lines, timing notes) goes to stdout (silenced by default; visible with `--debug 2>/dev/null` style).

---

## Testing Strategy

```
                   ┌─────────────────┐
                   │ Smoke (manual)   │  real Claude Code session
                   └─────────────────┘
                ┌──────────────────────┐
                │ Integration (CI)      │  init/uninstall lifecycle, hook chain,
                │                       │  full scan, MCP new tools end-to-end
                └──────────────────────┘
            ┌─────────────────────────────┐
            │ Concurrency (CI)             │  N goroutines vs buglog/ledger
            └─────────────────────────────┘
       ┌──────────────────────────────────────┐
       │ Unit (CI)                             │  state / scanner / hook handlers
       └──────────────────────────────────────┘
```

### File layout

```
tests/
├── state_test.go                  (CRUD + version migration)
├── scanner_test.go                (extractor heuristics, fixture-driven)
├── hook_test.go                   (crafted stdin → assert stderr)
├── installer_test.go              (settings.json merge / boundary detect / uninstall)
├── concurrency_test.go            (200 goroutines vs buglog → no data loss)
├── integration/
│   ├── init_lifecycle_test.go     (temp HOME → init → verify → uninstall → assert clean)
│   ├── scan_anatomy_test.go       (fixture repo → scan → compare anatomy.md to golden)
│   ├── hook_chain_test.go         (session-start → preread X → preread X → stop;
│   │                               assert ledger)
│   └── mcp_new_tools_test.go      (describe_codebase / get_rules / find_similar_bugs)
├── benchmark_test.go              (BenchmarkPreRead/PreWrite/PostWrite, p95 gates)
├── golden/
│   └── mcp_search_response.json   (regression snapshot for existing search_codebase)
└── fixtures/
    ├── repo-small/                (10 files)
    ├── repo-medium/               (500 files)
    ├── repo-large/                (5000 files)
    ├── hook-payloads/             (real Claude Code stdin samples — captured in M1)
    └── code-samples/              (auth.go with godoc, utils.py with docstring, ...)
```

### Key assertions

**Regression:** `tests/golden/mcp_search_response.json` snapshot prevents accidental schema changes to existing MCP tools.

**Performance gates:**
- `BenchmarkPreRead` < 80ms p95 (anatomy 1000 entries + session 50 reads)
- `BenchmarkPreWrite` < 200ms p95 (cerebrum 50 rules + buglog 200 entries)
- `BenchmarkScan(repo-medium)` < 30s (500 files)

**Hook protocol contract:** real Claude Code stdin samples (captured in M1) replayed through handlers; assert correct parse + no panic. Bad-stdin variants (truncated / non-JSON / missing fields) assert `exit 0` + empty stderr.

**Manual smoke (in `docs/smoke-checklist.md`, not CI):** fresh Claude Code session → init → trigger reads on anatomy-known files → verify `⚡ claude-context: ...` in transcript.

---

## Milestones

Independent shipping units. Stop after any one and the user retains the value delivered so far.

```
Dependency graph:
  M1 (Foundation)
    └─ M2 (Anatomy) ←─┬─ M3 (Memory + Ledger)
                      │      └─ M5 (Diagnostics)
                      │      └─ M6 (Maintenance)
                      └─ M4 (Learning + 3 MCP tools)
```

### M1 — Foundation (~3-5 days)

**Deliverables:**
- `cmd/main.go` subcommand dispatcher (no-arg → MCP server unchanged)
- `pkg/state/{paths,session,ledger,lock,atomic}.go` minimum state layer
- `pkg/scanner/tokens.go` token estimator
- `pkg/scanner/walker.go` filter strategy (walk only — no extractor yet)
- `pkg/hook/{protocol,feedback}.go` stdin parsing + stderr conventions
- `pkg/installer/{settings,rules,project,uninstall}.go` init/uninstall
- 5 hook handler stubs (parse stdin, log "fired" to ledger, exit 0)

**Acceptance:**
- `claude-context init` works end-to-end with `y/N` confirmation
- `--uninstall` cleanly reverses (settings.json + CLAUDE.md diff is empty)
- All hook subcommands callable, parse stdin, never panic, log to ledger
- Existing MCP server zero regression (golden snapshot passes)
- Real Claude Code stdin samples captured to `tests/fixtures/hook-payloads/`

**Not in M1:** anatomy extraction, cerebrum, buglog, edit summary — hooks are skeletal.

### M2 — Anatomy Map (~5-7 days)

**Deliverables:**
- `pkg/scanner/extractor.go` + `ext_{go,py,js,md,known}.go`
- `pkg/state/anatomy.go`
- `claude-context scan [--force]`
- `pkg/hook/preread.go` complete (anatomy hit + repeat detection)
- `claude-context-rules.md` template updated to instruct "consult anatomy first"

**Acceptance:**
- `scan` produces sensible `anatomy.md` for repo-small/medium/large
- preread sends anatomy-hit messages to stderr
- preread sends repeat-read warnings
- Integration test: simulated session has `ledger.anatomy_hits > 0`
- Real Claude Code session: `⚡ claude-context: ...` visible in transcript

**Headline-value milestone — quantitative savings measurable starting here.**

### M3 — Memory + Ledger + Edit Summary (~3-5 days)

**Deliverables:**
- `pkg/state/memory.go` complete (M1 had skeleton)
- `pkg/state/ledger.go` accumulators
- `pkg/hook/postwrite.go` edit-pattern classifier (13 categories) → 5-15 word summaries
- `pkg/hook/sessionstart.go` + `stop.go` full lifecycle
- `claude-context stats` command

**Acceptance:**
- After a session, `memory.md` has the session row
- `token-ledger.json` accumulates correctly
- `claude-context stats` prints meaningful savings report

### M4 — Learning Memory + 3 MCP Tools (~7-10 days)

**Deliverables:**
- `pkg/state/cerebrum.go` rule format + regex matching
- `pkg/state/buglog.go` schema + semantic match (stop-word filter + token overlap ≥ 3)
- `pkg/hook/prewrite.go` complete (cerebrum scan + buglog query)
- `pkg/hook/postwrite.go` extended: auto-upsert buglog
- CLI: `cerebrum {add,list,remove}` + `buglog {list,clear}`
- 3 new MCP tools: `describe_codebase` / `get_project_rules` / `find_similar_bugs`

**Acceptance:**
- Cerebrum rules trigger pre-write warnings on real edits
- post-write auto-populates buglog
- `find_similar_bugs` returns relevant entries via MCP
- Cross-client test: Cursor or Claude Desktop calls `describe_codebase` and receives anatomy

**Most complex milestone — semantic match thresholds need real-data tuning.**

### M5 — Diagnostics (~2-3 days)

**Deliverables:**
- `pkg/waste/detector.go` (5 patterns: repeated reads, large reads when anatomy had a description, memory bloat, cerebrum staleness >14d, anatomy miss rate <80%)
- `claude-context stats --waste`
- `claude-context stats --json` for scripting

**Acceptance:** detects all 5 patterns on synthesized fixtures.

### M6 — Maintenance (~2-3 days)

**Deliverables:**
- `pkg/consolidator/memory.go` — fold session rows older than 7d into `> Consolidated session (N actions)`
- Lazy trigger inside SessionStart (no daemon)
- `pkg/scanner/walker.go` incremental mode (mtime skip)

**Acceptance:**
- Simulated 100-session history → `memory.md` size stays bounded (~2K tokens)
- `claude-context scan` on a no-change repo runs in <2s (vs ~30s full)

### Effort total

~22-33 working days, ~4-7 calendar weeks including spec/review checkpoints. Each milestone gets its own `superpowers:executing-plans` review checkpoint.

---

## Risks / Open Questions

R1-R5 validated in the M0 spike (`2026-04-27-m0-hook-protocol-validation-report.md`). R6 and R7 still pending.

| # | Item | Status | Outcome / next |
|---|---|---|---|
| R1 | Hook stdin JSON real schema | ✅ **VALIDATED** (M0) | Schema fully documented in §6; arch spec updated; canonical fixtures at `tests/fixtures/hook-payloads/`. **Bonus finding**: Stop fires per assistant turn — see §5 Flow F note and M0 report. |
| R2 | Exit code semantics | ✅ **VALIDATED** (M0) | exit 0 stderr is silent to Claude; **use exit 1** for informational hooks (wrapped as "non-blocking error"). exit 2 makes Claude perceive blocking — avoid. §6 updated. |
| R3 | Hook latency tolerance | ✅ **VALIDATED objectively** (M0) | Claude Code does NOT enforce a hook timeout under 3000ms. Subjective grades deferred to M2-M4 with real workloads. Arch spec budgets (preread 80ms / prewrite-postwrite 200ms / stop 500ms) **kept** as designed. |
| R4 | `_managed_by` field tolerance | ✅ **VALIDATED** (M0) | Claude Code 2.1.119 accepted `_managed_by` + `_version` fields; hooks fired normally. §8 boundary scheme stands. |
| R5 | `@import` path forms | ✅ **VALIDATED** (M0) | All three forms (`@~/`, `@/abs`, `@./rel`) work. §8 init can use any form. |
| R6 | Hook concurrency model (does Claude Code serialize hook fires?) | ⏳ **PENDING** | Defer to M3 first concurrency conflict; §7 single-lock design is correct under both serial and parallel. |
| R7 | scan memory peak on 10K+ file repos | ⏳ **PENDING** | Defer to M2 (its native domain). |

---

## Out of Scope for This Spec

This spec covers cross-cutting decisions and milestone outlines only. Each milestone (M1-M6) gets its **own detailed spec** before implementation, brainstormed separately. The next document will be the M1 detailed spec.
