# M9 — Intelligence Loop — Design Spec

## Overview

M9 closes the intelligence loop on top of the M8 daemon by turning per-session signals (transcripts, ledger history, project state) into actionable artifacts the user reviews periodically. It delivers four independent sub-systems behind clear interfaces:

1. **Cerebrum learner** — drafts candidate cerebrum rules from session transcripts; queues them for human accept/reject (never auto-applies).
2. **Weekly waste report** — extends the existing M5 waste detector with week-over-week deltas and writes a markdown report into the project's `.mneme/reports/` directory on a daemon cron.
3. **Suggestions engine** — periodically surfaces stale anatomy entries, unread files, co-read pairs, and stale rules; supports per-suggestion dismissal with TTL.
4. **Environment auto-detection** — library-only helper that locates Chrome, the project's package manager, and dev-server port; consumed by M7 `mneme status` and M11 `mneme designqc`.

Each sub-system lives in its own `pkg/` package, is library-callable, and exposes a CLI surface where applicable. None of them depend on `pkg/daemon` directly: the daemon's task registry calls into them, but local fallbacks work without the daemon (per roadmap §2.1).

## 1. Goals & Non-Goals

### Goals

- **Auto-learn cerebrum rules** with a hardcoded English+Chinese trigger-phrase heuristic over Claude Code transcript JSONL files. All learned rules require human approval.
- **Periodic waste reports** written as markdown into `.mneme/reports/waste-YYYY-MM-DD.md`, generated either by daemon cron (Mondays 09:00 local) or by `mneme report waste`.
- **Suggestion engine** that emits stable-ID suggestions persisted to `~/.mneme/projects/<id>/suggestions.json`, with a 30-day dismissal TTL.
- **Environment auto-detection** as a pure library — no CLI of its own — used by other milestones.
- **Backwards compatible.** No M0–M8 schema breaking changes. New files, additive config keys, additive cron tasks.

### Non-goals

- **No automatic application** of learned cerebrum rules. The learner only drafts candidates.
- **No multi-language NLP / semantic similarity.** Trigger phrases are a hardcoded keyword list in v1.
- **No dashboard panels.** That is M10. M9 ships data files + CLI; M10 reads them.
- **No cross-project aggregation.** Each project gets its own suggestion set and waste report.
- **No new schema migration.** All four sub-systems write new files; nothing existing changes shape.

## 2. Architecture

### 2.1 Component diagram

```
                                 stop hook (per session)
                                   │
                ┌──────────────────┼─────────────────────────┐
                ▼                  ▼                         ▼
  POST /cerebrum/learn   append ledger-history.jsonl   (existing turn aggregator)
        │ (200ms timeout, fall-back: inline)
        ▼
  ┌──────────────────────┐
  │  pkg/cerebrum.Learner │   transcript JSONL → trigger-phrase scan →
  │                       │   draft rule → cerebrum-pending.json
  └──────────────────────┘

           daemon cron (M8 manifest, additions in M9)
   ┌─────────────────────┬──────────────────────┬────────────────────────┐
   │ cerebrum-learn      │ weekly-waste-report  │ suggestions-refresh    │
   │ (idle drain queue)  │ (Mon 09:00 local)    │ (daily 04:00 local)    │
   └─────────────────────┴──────────────────────┴────────────────────────┘
                          │                       │
                          ▼                       ▼
              .mneme/reports/waste-*.md    ~/.mneme/projects/<id>/suggestions.json

   Library only (no cron, no CLI):
   ┌───────────────────────────┐
   │ pkg/envcheck              │  Chrome path + package mgr + dev-server port
   └───────────────────────────┘  consumed by M7 status, M11 designqc
```

### 2.2 Package layout

```
pkg/
├── cerebrum/
│   ├── learner.go            # Learn(transcriptPath, projectRoot) ([]Candidate, error)
│   ├── triggers.go           # hardcoded English+Chinese keyword list
│   └── pending.go            # cerebrum-pending.json reader/writer (lock-protected)
├── waste/
│   ├── detector.go           # existing (M5) — unchanged
│   └── report.go             # new — GenerateReport, RenderMarkdown, WriteReport
├── suggestions/
│   ├── engine.go             # Refresh(projectRoot, now) ([]Suggestion, error)
│   ├── generators.go         # 4 generator funcs
│   └── dismissed.go          # dismissed-set reader/writer
├── envcheck/
│   └── envcheck.go           # Detect(projectRoot) Result
└── state/
    └── ledger_history.go     # AppendLedgerHistory + ReadLedgerHistory + rolling-trim
```

### 2.3 New state files

All under `~/.mneme/projects/<id>/` unless otherwise noted:

| File | Owner | Purpose |
|---|---|---|
| `cerebrum-pending.json` | learner | Queue of `Candidate` records awaiting `mneme cerebrum review` |
| `cerebrum-rejected.json` | `mneme cerebrum review` | `{id, rejected_at}` records; suppress re-learn for 90 days |
| `ledger-history.jsonl` | stop hook | One JSONL line per session-end; rolling 90 days, trimmed when file >1 MiB |
| `suggestions.json` | engine | Current suggestion set (overwrite on each `Refresh`) |
| `suggestions-dismissed.json` | `mneme suggestions dismiss` | `{id, dismissed_at}` records; 30-day TTL |
| `<project-root>/.mneme/reports/waste-YYYY-MM-DD.md` | report | Per-project, written into the project's repo (not global home) |

### 2.4 Cross-component rules

- **Library-first.** Each top-level function (`cerebrum.Learn`, `waste.GenerateReport`, `suggestions.Refresh`, `envcheck.Detect`) takes explicit args (no globals, no daemon dependency) so it's callable from both the daemon's task registry and CLI subcommands.
- **Locking.** Files written by both daemon and CLI use `state.AcquireLock` with `<file>.lock` next to them. Timeout: 1 second for short writes, 5 seconds for the report writer.
- **Time injection.** Every function that depends on "now" accepts a `time.Time` argument so golden-fixture tests can pin dates.

## 3. Cerebrum Learner

### 3.1 Trigger flow

1. Stop hook fires → existing `state.AggregateTurn` runs → existing `state.IncrementSafe` runs.
2. Stop hook tries `POST http://unix/cerebrum/learn` with body `{project_id, transcript_path, session_id}` to `~/.mneme/daemon/socket`. Combined dial+request timeout: 200 ms.
3. **Daemon path:** handler enqueues onto an in-memory channel and returns `202 Accepted`. A daemon worker drains the channel and runs `cerebrum.Learn(...)` per item; same project does not run twice concurrently.
4. **Fallback path:** dial fails → stop hook calls `cerebrum.Learn(...)` inline, wrapped in `defer recover()` so a malformed transcript can never crash the hook.

### 3.2 Public types

```go
type Trigger struct {
    Phrase    string // matched user phrase, e.g. "should not"
    UserMsg   string // user message containing it (truncated to 200 chars)
    PriorAsst string // assistant turn immediately before (truncated to 400 chars)
    Turn      int    // 1-indexed turn within the transcript
}

type Candidate struct {
    ID         string             // sha256("v1|" + Phrase + "|" + PriorAsst)[:16]
    Trigger    Trigger
    DraftRule  state.CerebrumRule // {Comment, Pattern, Message}
    Confidence float64            // 0..1; 1.0 if matched, lower if pre-empted
    QueuedAt   string             // RFC3339
    HitCount   int                // bumped instead of duplicated when same ID re-learned
}

func Learn(transcriptPath string, projectRoot string) ([]Candidate, error)
```

### 3.3 Heuristic v1

- **English triggers:** `corrected`, `should not`, `shouldn't`, `instead`, `wrong`, `don't`, `do not`, `actually`, `revert`.
- **Chinese triggers:** `避免`, `不要`, `不应`, `错误`, `改回`.
- **Match scope:** case-insensitive substring against user-message text only (NOT system prompts, NOT tool results).
- **Draft rule construction (per match):**
  - `Pattern` = first Edit/Write `tool_input.file_path` from the prior assistant turn, escaped to a literal regex via `regexp.QuoteMeta`. If none, the first non-trivial token of the prior assistant message text.
  - `Message` = first 80 chars of the user message.
  - `Comment` = `"learned from session <session_id> turn <N>"`.

### 3.4 Dedup & pre-emption

- **Same-ID dedup:** if `Candidate.ID` already exists in `cerebrum-pending.json`, increment its `HitCount` instead of appending a duplicate.
- **Existing-rule pre-emption:** if `cerebrum.md` already has a rule with the same `Pattern`, drop the candidate.
- **Reject TTL:** if `cerebrum-rejected.json` contains `Candidate.ID` and `rejected_at + cerebrum.reject_ttl_days > now`, drop the candidate.

### 3.5 CLI: `mneme cerebrum review`

Interactive mode (TTY):

```
mneme cerebrum review
  → reads cerebrum-pending.json
  → for each candidate, prints: trigger phrase, prior assistant turn snippet,
    drafted pattern + message, hit count.
  → prompts: [a]ccept / [r]eject / [s]kip / [q]uit
  → accept: state.AppendCerebrumRule(...) + remove from pending
  → reject: remove from pending, append {id, rejected_at} to cerebrum-rejected.json
  → skip: no-op, candidate stays in pending
  → quit: save and exit
```

Non-TTY mode:

- `mneme cerebrum review --list --json` — prints all pending candidates as JSON; no prompt, no mutation. Used by M10 dashboard.

### 3.6 Daemon endpoint

```
POST /cerebrum/learn
  body: { "project_id": "...", "transcript_path": "...", "session_id": "..." }
  → 202 Accepted { "queued": true }                  (normal path)
  → 200 OK       { "disabled": true }                (cerebrum.learning_enabled = false)
  → 400 Bad Request                                  (missing fields)
  → 503 Service Unavailable                          (queue full, > 100 pending)
```

The daemon worker reads project_id, resolves to `state.GlobalProjectDir(id)`, and calls `cerebrum.Learn(transcript_path, projectRoot)` with `cron.SkipIfStillRunning`-style locking (one in-flight learn per project at a time).

## 4. Waste Report

Extends existing `pkg/waste/detector.go` (M5). Detector unchanged.

### 4.1 Inputs

- `state.ReadLedger(projectRoot)` — current absolute totals.
- `~/.mneme/projects/<id>/ledger-history.jsonl` — week-over-week deltas (last 14 days).
- Existing `waste.Detect(projectRoot, homeDir)` — 5 detection patterns.

### 4.2 Public API

```go
type ReportInput struct {
    ProjectRoot string
    HomeDir     string
    Now         time.Time // injected for tests
    Threshold   float64   // from config.waste.threshold_percent
}

type ReportOutput struct {
    Date               string             // YYYY-MM-DD (UTC)
    Patterns           []waste.WastePattern
    Deltas             map[string]int     // last-7d minus prev-7d for selected counters
    BreachedThresholds []string           // pattern names that crossed Threshold
}

func GenerateReport(in ReportInput) (ReportOutput, error)
func RenderMarkdown(out ReportOutput) string
func WriteReport(projectRoot string, out ReportOutput) (path string, err error)
```

### 4.3 Triggers

- **Daemon cron `weekly-waste-report`** (Mondays 09:00 local) — iterates `~/.mneme/projects/<id>/origin`, calls `GenerateReport` + `WriteReport` for each project whose origin path still exists.
- **CLI:** `mneme report waste [--project <id>] [--dry-run]` — same code with daemon-fallback semantics. `--dry-run` prints to stdout; default writes to `<project-root>/.mneme/reports/waste-YYYY-MM-DD.md`.

### 4.4 Ledger history

New helper in `pkg/state/ledger_history.go`:

```go
type LedgerSnapshot struct {
    TS        string             `json:"ts"`         // RFC3339
    SessionID string             `json:"session_id"`
    Totals    state.LedgerTotals `json:"totals"`
}

func AppendLedgerHistory(projectRoot string, snapshot LedgerSnapshot) error
func ReadLedgerHistory(projectRoot string, since time.Time) ([]LedgerSnapshot, error)
```

- **Append point:** stop hook, immediately after `IncrementSafe`. One line per session.
- **Trim:** writer trims lines older than 90 days when the file exceeds 1 MiB. No separate cron task.

## 5. Suggestions

### 5.1 Public API

```go
type Suggestion struct {
    ID          string `json:"id"`           // sha256("v1|" + Type + "|" + Target)[:16]
    Type        string `json:"type"`         // see §5.2
    Target      string `json:"target"`       // path or rule index
    Title       string `json:"title"`
    Detail      string `json:"detail"`
    GeneratedAt string `json:"generated_at"` // RFC3339
}

func Refresh(projectRoot string, now time.Time) ([]Suggestion, error)
```

### 5.2 Generators (v1)

| Type | Target | Trigger |
|---|---|---|
| `stale_anatomy` | file path | anatomy entry mtime + 7d < file mtime |
| `unread_file` | file path | file in anatomy but never appeared in any session's `Reads` over the last 30d (best-effort: uses session archives if available; otherwise generator silently skipped) |
| `co_read` | `<pathA>\|<pathB>` | file pairs that co-appear in ≥3 sessions over 30d (suggests linking them in anatomy) |
| `stale_rule` | rule pattern (literal regex string) | `cerebrum.md` rule with no anatomy/transcript match in last 90d. Target uses the pattern (not the rule index) because indices shift when rules are added/removed. |

### 5.3 Persistence

- `Refresh` overwrites `~/.mneme/projects/<id>/suggestions.json` with the full current set, **unfiltered**.
- Dismissal filtering happens at *read* time (§5.4), not at *Refresh* time. This way, a freshly-dismissed item disappears from `mneme suggestions list` immediately without waiting for the next `Refresh`.

### 5.4 CLI

- `mneme suggestions list [--json]` — read-only; reads `suggestions.json` and filters out IDs in `suggestions-dismissed.json` whose `dismissed_at + suggestions.dismiss_ttl_days > now`.
- `mneme suggestions dismiss <id>` — appends `{id, dismissed_at}` to `suggestions-dismissed.json`.
- `mneme suggestions refresh` — manually invoke `Refresh` (also done daily by daemon cron `suggestions-refresh` at 04:00 local).

### 5.5 Honest limitation

`unread_file` requires read history beyond the current session. M9 reuses session archives only if M5/M6 stored them. If unavailable, the generator is silently skipped and a one-line note appears at the top of `suggestions list`. Adding a session-archive writer is M9 scope creep — defer.

## 6. Envcheck

Library only. No CLI, no cron, no persistence.

### 6.1 Public API

```go
type Result struct {
    ChromePath     string    // first match in standard locations + $CHROME_PATH; "" if none
    PackageManager string    // "npm" | "pnpm" | "yarn" | "bun" | "" (lockfile-driven)
    DevServerPort  int       // 0 if undetected; framework conventions only
    Framework      string    // "next" | "vite" | "astro" | "sveltekit" | ""
    DetectedAt     time.Time
}

func Detect(projectRoot string) Result
```

### 6.2 Detection rules

- **ChromePath:** macOS `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, Linux `/usr/bin/google-chrome`, `/usr/bin/chromium`, then `$CHROME_PATH`. First hit wins.
- **PackageManager:** lockfile presence — `pnpm-lock.yaml` → pnpm, `yarn.lock` → yarn, `bun.lockb` → bun, `package-lock.json` → npm. `""` if none.
- **Framework + port:** `next.config.{js,ts,mjs}` → next/3000, `vite.config.{js,ts}` → vite/5173, `astro.config.{mjs,ts}` → astro/4321, `svelte.config.js` → sveltekit/5173.

### 6.3 Consumers

- M7 `mneme status` (already specced in M7 plan) reads `Detect()` to print "Chrome: detected at /path; package manager: pnpm; framework: next (port 3000)".
- M11 `mneme designqc` will use it for headless Chrome + dev-server lifecycle.

M9 ships envcheck as a pure library; consumers in M9 itself: none. It exists in M9 for M11 to find without further blocking.

## 7. Configuration

| Key (yaml) | Env var | Default | Purpose |
|---|---|---|---|
| `cerebrum.learning_enabled` | `CEREBRUM_LEARNING_ENABLED` | `true` | Cut-point per roadmap §4 |
| `waste.threshold_percent` | `WASTE_THRESHOLD_PERCENT` | `15` | Used by report breach logic |
| `suggestions.dismiss_ttl_days` | `SUGGESTIONS_DISMISS_TTL_DAYS` | `30` | Per-suggestion dismissal window |
| `cerebrum.reject_ttl_days` | `CEREBRUM_REJECT_TTL_DAYS` | `90` | Suppress re-learning rejected candidates |

All four follow the existing `pkg/config.resolve(...)` env > file > default chain. No breaking change — older configs still parse.

## 8. Testing

| Package | Approach |
|---|---|
| `pkg/cerebrum` | Golden-file tests: `testdata/transcripts/*.jsonl` → expected `cerebrum-pending.json` snapshots. Inputs include English-only, Chinese-only, mixed, "no triggers", and pre-empted-by-existing-rule cases. |
| `pkg/waste/report` | Hand-crafted `Ledger` + `ledger-history.jsonl` fixtures → expected markdown report (golden compare with `t.Setenv("TZ", "UTC")` to keep dates stable). Time-injection via `ReportInput.Now`. |
| `pkg/suggestions` | In-memory anatomy + session archives → expected `suggestions.json`. Dismissed-set tests use frozen `now` via `Refresh(root, now)`. |
| `pkg/envcheck` | `t.TempDir()` populated with lockfile + framework-config fixtures; assertion is the `Result` struct. ChromePath assertions skip when `os.Stat` of all candidate paths fails (e.g. CI runner without Chrome installed). |
| `POST /cerebrum/learn` handler | `httptest.NewServer` + the daemon mux from M8; verify queue insertion, 202 response, and concurrency safety (5 parallel posts → 5 entries). |
| Stop-hook integration | `tests/integration/cerebrum_learn_test.go` (`//go:build integration`): start daemon in temp home, fire stop hook, assert `cerebrum-pending.json` written. Also: kill daemon, fire stop hook again, assert in-process fallback ran. |

No new e2e/Playwright tests — that is M10/M11 territory. All tests must pass on macOS + Linux GitHub Actions matrix.

## 9. Out of Scope

- **Multi-language NLP / semantic similarity for cerebrum learner.** Hardcoded keyword list only in v1. Open question §10.
- **Auto-applying learned rules.** Always queued for human review.
- **Dashboard panels for cerebrum/suggestions/waste reports.** That is M10. M9 only exposes JSON files and CLI commands; M10 reads them.
- **Cross-project suggestion aggregation.** Each project gets its own `suggestions.json`; no global "best-of" digest.
- **Schema migration of existing data files.** All new files; nothing existing changes shape.
- **Session-archive writer.** `unread_file` suggestion is best-effort; if archives are not present, generator is silently skipped.

## 10. Open Questions

These do **not** block implementation. They surface for review when v2 of M9 is considered.

- Cerebrum trigger phrases are hardcoded. Should they be a config knob (e.g., `cerebrum.trigger_phrases: [...]`)? Default v1: hardcoded.
- Should `unread_file` suggestion drive a session-archive writer addition? Default v1: no, accept best-effort.
- `weekly-waste-report` runs per project at the same time. If 100 projects exist, that's ~100 reports at 09:00 Monday. Worth staggering? Default v1: no, reports are cheap.
- `Confidence` field in `Candidate` is currently `1.0` for any keyword match. Worth a stronger scoring model (longer match, multi-keyword boost, length penalty)? Default v1: defer.

## 11. Acceptance Criteria

- [ ] `pkg/cerebrum/learner.go` `Learn` is unit-tested with ≥3 transcript fixtures (English, Chinese, mixed).
- [ ] `POST /cerebrum/learn` endpoint registered on M8 daemon mux; stop hook posts there and falls back to in-process when daemon down.
- [ ] `pkg/waste/report.go` writes `<project-root>/.mneme/reports/waste-YYYY-MM-DD.md`; `mneme report waste --dry-run` prints same content to stdout.
- [ ] `pkg/suggestions/engine.go` produces 4 suggestion types; `mneme suggestions list/dismiss/refresh` work; dismissal TTL honored.
- [ ] `pkg/envcheck/envcheck.go` returns populated `Result` for at least one Next.js, Vite, and Astro fixture in tests.
- [ ] Three new daemon cron tasks (`cerebrum-learn` worker, `weekly-waste-report`, `suggestions-refresh`) registered in the M8 manifest's seed list.
- [ ] All existing tests still pass (`go test ./...` green on macOS + Linux).
- [ ] No M0–M8 schema breaking changes.

---

**Roadmap reference:** §3 M9 — Intelligence Loop (`docs/superpowers/specs/2026-04-28-m7-m11-roadmap-design.md`).

**Prerequisite milestones:**

- **M8 (hard):** the daemon's HTTP mux hosts `/cerebrum/learn`, and the cron registry hosts the three new tasks. Without M8, only the CLI surfaces work (and `cerebrum-learn` becomes synchronous in the stop hook).
- **M7 (soft):** the daemon's per-project workers (`/cerebrum/learn`, `weekly-waste-report`) need `~/.mneme/projects/<id>/origin` to resolve project_id → project root. Without M7, the daemon paths still compile but skip projects whose origin file is missing; CLI fallback paths work because the CLI knows its own cwd. M9 ships a stub origin resolver that uses `state.GlobalProjectDir(id)` as the project root if no `origin` file is present (sufficient for tests, degrades gracefully in production until M7 lands).
- **M10, M11:** independent; M9 ships first.
