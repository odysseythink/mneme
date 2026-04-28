# M7–M11: openwolf Feature-Parity Roadmap — Design Spec

## Overview

This roadmap brings mneme to feature parity with `openwolf-main`'s operations layer (daemon, dashboard, design QC, multi-project lifecycle commands) while preserving mneme's existing differentiators (vector semantic search, MCP server, multi-backend vector DB, function-level AST splitting).

The work decomposes into **five milestones (M7–M11)** delivering the 14 gaps identified by the openwolf comparison:

1. **M7 — CLI Maintenance Suite** — `status` / `scan --check` / `update` / `restore` / `bug search` / `mneme.md` (project identity & per-session Claude instructions)
2. **M8 — Daemon + Cron Scheduler** — long-running daemon, cron-manifest, retries → dead-letter queue, heartbeat, anatomy auto-rescan
3. **M9 — Intelligence Loop** — Cerebrum auto-learning, Token waste weekly reports, Suggestions panel, environment auto-detection
4. **M10 — Web Dashboard** — Vite + React + `embed.FS`, daemon-hosted HTTP+WebSocket, 10 panels
5. **M11 — Design QC + Reframe** — chromedp headless Chrome, route detection, dev-server lifecycle, screenshot capture, 12-framework reframe knowledge base

Each milestone gets its own detailed spec + plan written when implementation starts (M0–M6 pattern). This document is the **roadmap-level spec** that fixes cross-cutting design decisions all five milestones share.

---

## 1. Goals & Non-Goals

### Goals

- **Functional parity** with openwolf's operations layer at the user-experience level: any workflow openwolf supports (daemon, dashboard, design QC, multi-project update, restore, bug search, identity file) has a mneme equivalent with comparable ergonomics.
- **Single Go binary** remains the primary distribution. The Web Dashboard is bundled as `embed.FS` static assets — no runtime Node dependency on the user's machine.
- **Backward compatibility** — every M0–M6 feature (MCP server, hooks, multi-backend vectordb, anatomy, memory, cerebrum, buglog, scan, consolidator) keeps working unchanged. M7+ are additive.

### Non-goals

- **No verbatim copy of openwolf's React component tree.** UI is reimplemented to match mneme's existing data model (per-project `.mneme/` + `~/.mneme/projects/<id>/`, not openwolf's `.wolf/`).
- **No multi-browser testing** (Playwright) — chromedp is sufficient for mneme's design QC scope.
- **No third-party LLM-based "AI description extraction"** in this roadmap. mneme's anatomy descriptions remain heuristic (existing scanner). LLM enrichment can be a later milestone.
- **No Windows-first support.** Daemon target is macOS + Linux. Windows works on best-effort basis (no `os.Signal` USR1 handlers, fall back to TCP-only socket).

---

## 2. Cross-Cutting Design Decisions

These decisions apply across all five milestones. Per-milestone specs reference but do not override them.

### 2.1 Process Topology

```
mneme                          # default = MCP server over stdio (unchanged from M0–M6)
mneme daemon start             # long-running HTTP + cron + scheduler
mneme dashboard                # alias: ensure daemon running, open browser to UI
mneme <subcommand>             # CLI subcommands try daemon socket first, fall back to local
mneme hook <event>             # hook event handlers (unchanged from M0–M6)
```

**Subcommand dispatch rule (M7 onwards):** every CLI subcommand that touches state shared with the daemon (`status`, `scan`, `update`, `restore`, `cerebrum`, `buglog`) follows this protocol:

1. Try to dial `~/.mneme/daemon/socket` (Unix domain) with 200 ms timeout.
2. On success, send subcommand request as JSON; render daemon's response.
3. On dial failure (no daemon, or stale socket), fall back to running the operation locally with a warning printed to stderr if `--verbose`.

This guarantees CLI tools work whether or not the daemon is running, while letting daemon-mediated operations (lock coordination, in-memory caching) take effect when it is.

### 2.2 State Directory Layout

```
~/.mneme/
├── config.yaml                   (existing, M0)
├── projects/
│   └── <project-id>/             (existing, M2–M6)
│       ├── anatomy.md
│       ├── cerebrum.json
│       ├── buglog.json
│       ├── memory.md
│       ├── ledger.json
│       └── identity.md           (NEW, M7 — project identity summary)
├── daemon/                       (NEW, M8)
│   ├── pid                       file lock; daemon refuses to start if held
│   ├── socket                    Unix domain socket (mode 0600)
│   ├── logs/
│   │   └── daemon-YYYYMMDD.log   rotated daily, kept 14 days
│   ├── cron-manifest.json        task definitions
│   ├── cron-state.json           per-task last_run / retry_count / dead-letter status
│   └── heartbeat.json            mtime updated every 30 minutes
├── backups/                      (NEW, M7)
│   └── <project-id>/<timestamp>/ snapshot of project state before destructive op
├── suggestions/                  (NEW, M9)
│   └── <project-id>.json
└── designqc/                     (NEW, M11)
    └── <project-id>/
        ├── captures/             *.jpg (8/24-bit JPEG)
        └── report.json
```

Per-project `.mneme/` (in the project's working directory) remains the authoritative source for that project's state. `~/.mneme/projects/<id>/` is a legacy copy retained for cross-project queries; M7 reconciles them via `scan --check`.

### 2.3 Daemon Communication Protocol

**Transport:** HTTP/1.1 over Unix domain socket (default) and optionally TCP loopback (Dashboard only).

**Why HTTP and not raw JSON-RPC:** Dashboard's WebSocket needs HTTP upgrade handshake. Reusing one HTTP server for both CLI and Dashboard simplifies the listener, route table, and middleware (auth, logging).

**Authentication:** Unix socket relies on filesystem permissions (`0600`, owner-only). TCP listener on `127.0.0.1:18801` (configurable) requires a per-launch token written to `~/.mneme/daemon/token` (`0600`); Dashboard reads token from cookie set by `mneme dashboard` open-browser flow. No external auth.

**Endpoints (M8 baseline; M9–M10 extend):**

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness probe; returns `{ pid, uptime_s, version }` |
| `GET` | `/status` | Aggregate status (replaces `mneme status` local code path when daemon is up) |
| `POST` | `/scan` | Trigger scan; body: `{ project_id, force, check }` |
| `POST` | `/update` | Trigger multi-project template sync |
| `POST` | `/restore` | Restore from backup; body: `{ project_id, timestamp }` |
| `GET` | `/cron/list` | List cron tasks + state |
| `POST` | `/cron/run` | Manually fire a task |
| `POST` | `/cron/retry` | Retry dead-lettered task |
| `GET` | `/api/dashboard/*` | M10 dashboard data fetchers |
| `GET` | `/ws` | M10 WebSocket: live activity / ledger updates |
| `POST` | `/designqc/capture` | M11 trigger capture |

### 2.4 `update` Mechanism (binary + project sync)

**Two phases, run independently:**

**Phase A — multi-project template sync** (`mneme update` default):

1. Enumerate `~/.mneme/projects/<id>/origin` (recorded by `mneme init` as the project root path).
2. For each project: read pinned template version from `.mneme/template-version.txt` (NEW in M7); compare with bundled-binary template version (`pkg/installer/templates/`). Missing file is treated as version `0` (always sync).
3. If bundled is newer: write backup to `~/.mneme/backups/<project-id>/<UTC-ts>/`, then re-emit `rules.md`, `settings.json` hooks block, `mneme.md` from current templates. Existing user-edited content marked by `<!-- mneme:user-section -->` fences is preserved verbatim.
4. Print summary: N updated, N skipped (up-to-date), N skipped (deleted/missing), N failed.

**Phase B — binary self-update** (`mneme update --binary`):

1. Query `https://api.github.com/repos/<owner>/mneme/releases/latest`. Owner is baked at build time via `-ldflags="-X main.releaseRepo=<owner>/mneme"`.
2. Match asset by `runtime.GOOS`/`runtime.GOARCH` (`mneme-darwin-arm64`, `mneme-linux-amd64`, etc).
3. Download to temp file; verify SHA256 against asset's `.sha256` sibling.
4. `os.Rename(temp, os.Executable())` for atomic replace; chmod 0755.
5. Re-exec `mneme update` (Phase A) so newly-bundled templates propagate.

`--binary` errors cleanly (`"no GitHub release configured"`) when the build was not produced by the release pipeline (release version baked in via `-ldflags="-X main.releaseChannel=github"`).

### 2.5 Dashboard Build Pipeline

**Repository layout:**

```
mneme/
├── web/                          (NEW, M10)
│   ├── package.json              pnpm workspace
│   ├── pnpm-lock.yaml
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── src/
│   │   ├── main.tsx
│   │   ├── panels/
│   │   │   ├── Overview.tsx
│   │   │   ├── Activity.tsx
│   │   │   ├── Token.tsx
│   │   │   ├── Cron.tsx
│   │   │   ├── Cerebrum.tsx
│   │   │   ├── Memory.tsx
│   │   │   ├── Anatomy.tsx
│   │   │   ├── BugLog.tsx
│   │   │   ├── Suggestions.tsx
│   │   │   └── DesignQC.tsx
│   │   ├── api/                  REST + WS clients
│   │   └── components/           shared
│   └── dist/                     (gitignored, generated)
├── pkg/dashboard/
│   ├── server.go                 HTTP routes + WebSocket
│   ├── embed.go                  //go:embed dist/* — points to web/dist
│   └── api.go                    REST handlers
└── Makefile                      build orchestrator
```

**Build steps (Makefile):**

```make
build: web-build go-build

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build

go-build:
	go build -o bin/mneme ./cmd
```

**CI:** GitHub Actions caches Go modules + pnpm store separately. Release pipeline (`goreleaser`) runs `pnpm build` once per matrix combination is unnecessary because dist is platform-independent — produce dist once, embed into all platform binaries.

**Stack:** React 18 + Vite + TypeScript + Recharts (charts) + Tailwind (styling). Matches openwolf 1:1 to keep panel implementations transferable as reference.

**Dev mode:** `pnpm -C web dev` runs Vite dev server on `:5173`; daemon's `/api/*` and `/ws` are proxied via Vite's `server.proxy` config to `:18801` (with `ws: true` for the WebSocket route).

### 2.6 Design QC Headless Chrome

**Library:** `github.com/chromedp/chromedp` — pure Go, CDP, no Node driver subprocess, GitHub Actions runners ship Chrome by default.

**Detection:** Before spawning, `chromedp.NewExecAllocator` probes for a Chrome/Chromium binary in standard locations + `$CHROME_PATH`. On failure, `mneme designqc` prints actionable error: `"Chrome/Chromium not found. Install via brew install --cask google-chrome (macOS) or set CHROME_PATH=<path>."` and exits 2 (no panic).

**Capture flow:**

1. Detect framework (`next.config.js`, `vite.config.ts`, `astro.config.mjs`) from project root.
2. Detect dev-server port: read framework conventions (`next: 3000`, `vite: 5173`, `astro: 4321`). If not running and `--start-dev-server` is passed (default off), start it via the project's package manager (`pnpm dev` / `npm run dev`, detected by `pkg/envcheck`) as a background process; ensure cleanup on capture completion or signal.
3. Detect routes: framework-specific (Next.js: scan `pages/` or `app/`; Vite: glob `src/routes/`; Astro: same).
4. For each route, navigate, wait for `networkidle2`, capture full-page JPEG (quality 80, max-width 1440), write to `~/.mneme/designqc/<project-id>/captures/`.
5. Write `report.json` with route → capture-path map + dimensions + capture timestamp.

CLI: `mneme designqc [--route /path] [--quality N] [--max-width N] [--routes auto|all]`.

### 2.7 Backup & Restore

Every destructive operation (template sync, `restore`, future schema migrations) writes a snapshot to `~/.mneme/backups/<project-id>/<UTC-ISO8601-ts>/` containing:

- `rules.md` (pre-mod)
- `settings.json` (pre-mod, hooks block only)
- `mneme.md` (pre-mod)
- `manifest.json` — what the operation was, what files changed, mneme version

Retention: keep 14 days of backups OR 10 most recent (whichever is larger). Pruned by daemon on weekly cron.

`mneme restore` modes:

- `mneme restore` — interactive: list backups for current project, prompt selection.
- `mneme restore --latest` — restore most recent.
- `mneme restore <timestamp>` — restore specific.
- `mneme restore --list` — print table without acting.

### 2.8 Project Identity File (`mneme.md` / `identity.md`)

Two distinct files, both in per-project `.mneme/`:

- **`identity.md`** — auto-generated project identity summary (5–10 lines): primary language, framework, package manager, dev server URL, project intent (extracted from README first paragraph). Written by `scan` and refreshed on `update`. Equivalent to openwolf's `identity.md`.
- **`mneme.md`** — per-session Claude instructions template injected via `pkg/installer/templates/`. Content mirrors openwolf's `OPENWOLF.md` purpose: tells Claude how to use mneme's tools, where state lives, when to call which MCP. Edited by `mneme init` / `mneme update`. User customizations preserved between `<!-- mneme:user-section -->` fences.

`rules.md` (existing) keeps its current role: behavioral rules surfaced to Claude. `mneme.md` is operational documentation; `identity.md` is project-fact reference.

### 2.9 Testing Strategy

Per milestone, in declared order of preference:

| Milestone | Approach |
|---|---|
| M7 | `pkg/cliutil` test helpers; integration tests under `tests/` calling `mneme` binary in subprocess; `--dry-run` and `--yes` flags exercised heavily; backup/restore round-trip golden file tests. |
| M8 | `httptest.Server` for HTTP handlers; daemon-process integration tests with `go test -tags=integration` (spawn binary in `t.TempDir()`, signal SIGTERM, verify clean exit, PID file removed). Cron task tests use injected fake clock. |
| M9 | Snapshot tests with `testdata/golden/` fixtures: feed pre-canned ledger JSON / memory rows / session histories, compare generated reports/rules byte-for-byte. |
| M10 | Backend HTTP/WS via `httptest`. Frontend: vitest unit tests for panel components with mocked API responses. Optional: Playwright e2e against `mneme dashboard` in CI (`tests/e2e/`, gated by `-tags=e2e`). |
| M11 | chromedp tests against fixture HTML served by `httptest.NewServer`. CI uses GitHub Actions' bundled Chrome. Screenshot pixel-diff tolerance: ±5% per channel. |

All milestones require tests pass on macOS + Linux GitHub Actions matrix before merge.

---

## 3. Milestone Breakdown

### M7 — CLI Maintenance Suite

**Deliverables:**

- `cmd/cmd_status.go` — `mneme status` reports daemon health, scanner staleness (anatomy mtime vs filesystem), embedding API reachability, vector DB row count, last hook fire time per type. Output formats: human (default), `--json`.
- `cmd/cmd_scan.go` extended — `mneme scan --check` validates anatomy.md against filesystem (file exists? mtime matches? size matches?). Reports drift without modifying state. Existing `--force` flag unchanged.
- `cmd/cmd_update.go` — multi-project template sync (Phase A from §2.4) and `--binary` (Phase B). Includes `--dry-run`, `--list`, `--project <id>` flags.
- `cmd/cmd_restore.go` — backup browse + restore (§2.7).
- `cmd/cmd_buglog.go` extended — `mneme bug search <term>` does case-insensitive substring match across `bad_code` + `description` fields, ranked by token overlap (reuses `pkg/match`).
- `pkg/installer/templates/identity.md.tmpl` — new template.
- `pkg/installer/identity.go` — generates `identity.md` from project metadata.
- `pkg/installer/backup.go` — backup writer used by `update` and `restore`.

**State / config additions:**

- `~/.mneme/projects/<id>/origin` — file containing absolute project root path (created by `init`, read by `update`).
- `.mneme/template-version.txt` per project — pinned template version baked at last `init`/`update`.

**Out of scope for M7:** daemon, HTTP server, dashboard.

### M8 — Daemon + Cron Scheduler

**Deliverables:**

- `pkg/daemon/daemon.go` — process entrypoint (`daemon start`), PID file lock, signal handlers (`SIGTERM` clean shutdown, `SIGHUP` reload manifest, `SIGUSR1` log rotate).
- `pkg/daemon/server.go` — HTTP listener on Unix socket + TCP loopback; route table per §2.3.
- `pkg/daemon/cron.go` — manifest loader, scheduler tick (1-second resolution), retry policy (3 attempts with exponential backoff: 30s/2min/10min), dead-letter on retry exhaustion.
- `pkg/daemon/log.go` — daily rotation, 14-day retention.
- `pkg/daemon/heartbeat.go` — mtime touch every 30 minutes.
- `cmd/cmd_daemon.go` — `start` / `stop` (sends SIGTERM via PID, waits for socket close) / `restart` / `status` (reads `/health`) / `logs` (tails `daemon-YYYYMMDD.log`).
- Cron tasks pre-registered (manifest seeded by `init`):
  - `anatomy-rescan` — every 6 hours, runs scan in `--check` mode; if drift > 10 files, runs full scan.
  - `consolidate-memory` — daily at 03:00 local, runs existing `memory consolidate` if threshold met.
  - `prune-backups` — weekly Sunday 03:30, applies retention rule.

**State / config additions:**

- `daemon.dashboard_port` (default 18801) in `config.yaml`.
- `daemon.cron_enabled` (default true) in `config.yaml`.

**Out of scope for M8:** intelligence-layer cron tasks (added in M9), dashboard UI (M10).

### M9 — Intelligence Loop

**Deliverables:**

- `pkg/cerebrum/learner.go` — session-end pass: read latest session memory rows, identify "user corrected Claude" patterns (heuristic: rows containing `corrected`, `should not`, `instead`, `避免` etc.), draft new cerebrum rule, queue to `~/.mneme/projects/<id>/cerebrum-pending.json` for human review (no auto-apply). `mneme cerebrum review` interactive accept/reject command.
- `pkg/waste/report.go` — weekly aggregator: reads `ledger.json` plus a new rolling 90-day archive `~/.mneme/projects/<id>/ledger-history.jsonl` (NEW in M9, append-only; backwards compatible — readers tolerate missing file). Computes repeated-read ratio, large-file-read ratio, redundant-edit ratio. If any exceeds threshold from `config.yaml` (`waste_threshold_percent`, default 15), produces report markdown to `.mneme/reports/waste-YYYY-MM-DD.md`.
- `pkg/suggestions/engine.go` — periodic scan of project state, emits suggestions JSON: stale anatomy entries, files never read, files always read together (co-occurrence), rules in cerebrum that haven't matched in 90 days. CLI: `mneme suggestions list/dismiss <id>`.
- `pkg/envcheck/envcheck.go` — environment auto-detection used by M11 design QC and M7 status: locate Chrome/Edge binaries, detect package manager (npm/pnpm/yarn/bun) by lockfile presence, detect dev server port from `package.json` scripts.
- New cron tasks registered in M8's manifest: `weekly-waste-report` (Mondays 09:00), `cerebrum-learn` (session-end via stop hook → daemon HTTP), `suggestions-refresh` (daily 04:00).

**Out of scope for M9:** dashboard panels surfacing this data (M10).

### M10 — Web Dashboard

**Deliverables:**

- `web/` directory per §2.5 layout.
- `pkg/dashboard/embed.go` — `//go:embed dist/*` static asset server.
- `pkg/dashboard/api.go` — REST handlers per §2.3 endpoint table.
- `pkg/dashboard/ws.go` — WebSocket fanout: subscribers receive `{event, payload}` for `hook.fired`, `cron.tick`, `scan.complete`, `suggestion.new`.
- 10 React panels (one per openwolf panel; same data domain, mneme's data shape).
- `Makefile` updates per §2.5.
- `cmd/cmd_dashboard.go` — `mneme dashboard` ensures daemon, opens browser to `http://localhost:18801/` with one-time token.

**Out of scope for M10:** Design QC panel data integration is stubbed (M11 fills in real screenshot data).

### M11 — Design QC + Reframe

**Deliverables:**

- `pkg/designqc/capture.go` — chromedp wrapper, capture single URL → JPEG.
- `pkg/designqc/routes.go` — framework detection + route enumeration (Next.js / Vite / Astro / SvelteKit best-effort).
- `pkg/designqc/devserver.go` — start/stop dev server background process, port detection, healthcheck.
- `cmd/cmd_designqc.go` — `mneme designqc [target]` per §2.6.
- `pkg/installer/templates/reframe-frameworks.md.tmpl` — 12-framework knowledge base. Content: framework name, when-to-use, when-not-to-use, migration cost, link to docs.
- `pkg/dashboard` extended — DesignQC panel reads `report.json`, displays capture grid with route metadata.

**Out of scope for M11:** automated AI design critique (visual analysis pipeline is left to Claude consuming the captures, not built into mneme).

---

## 4. Risk Register & Cut Points

| Risk | Mitigation | Cut point |
|---|---|---|
| M10/M11 are ~50% of total scope; if M7–M9 takes longer than estimated, project may stall before user-visible UI ships | M9-end is a clean checkpoint: mneme already has 80% of openwolf's value (daemon, intelligence, multi-project ops). Re-evaluate whether M10 dashboard is needed vs. CLI + JSON exporter. | Stop after M9; document M10/M11 as deferred. |
| chromedp introduces Chromium runtime dependency | Detection + clear error in §2.6; designqc is opt-in subcommand, not background. | Drop M11 entirely; ship Reframe knowledge base only as static template (no QC). |
| Daemon increases ops complexity (port conflicts, stale sockets, zombie processes) | `daemon start` checks PID file, verifies socket reachability, handles orphan cleanup. M8 testing emphasizes start/stop/restart/crash-recovery. | If daemon proves fragile, fall back to "session-start hook triggered scheduler" (Q2 option C); accept reduced cron frequency. |
| Vite/pnpm in repo introduces Node toolchain to a Go project | Lockfile committed; `make build` orchestrates; Go-only `go build` still works (panics with clear error if `web/dist` absent and `embed` fails). | Drop M10; expose data via `mneme stats --json` and let users build their own UI. |
| GitHub Releases self-update has security implications (binary substitution) | SHA256 verification mandatory; `--binary` requires explicit flag; release artifacts signed (cosign optional, future). | Drop `--binary`; require `go install` or distribution-specific package manager. |
| Cerebrum auto-learning false-positives (drafts bad rules from misread session messages) | All learned rules go to `cerebrum-pending.json` for human review; never auto-apply. | If review burden too high, disable learner (`config.yaml: cerebrum.learning_enabled: false`). |

---

## 5. Open Questions

These do **not** block roadmap acceptance but should be resolved before the relevant milestone:

- **M7:** Should `restore` cover only template files, or also `cerebrum.json` / `buglog.json` (which contain user data)? Default proposal: only template files; user-data restore is a separate command (`mneme buglog restore` etc) added later if needed.
- **M8:** Does the daemon need a multi-machine mode (one daemon serves multiple users)? Default: no — single-user, single-machine.
- **M9:** Cerebrum learner's heuristics are language-specific (Chinese/English keywords). How configurable? Default: hardcoded keyword list in v1, surface as config in v2.
- **M10:** Dashboard authentication beyond per-launch token — needed for shared/team setups? Default: no, single-user only.
- **M11:** Should design QC capture trigger on every PR (CI integration)? Default: no, manual `mneme designqc` only; CI integration is a future milestone.

---

## 6. Implementation Order

Sequential, no parallel milestones:

1. **M7** — small, fast wins, validates `update` plumbing all later milestones reuse.
2. **M8** — daemon foundation; M9–M11 build on top.
3. **M9** — intelligence layer using daemon's cron + state APIs.
4. **M10** — dashboard surfaces M7–M9 data.
5. **M11** — Design QC plugs into daemon + dashboard.

Each milestone gets:

- One spec doc: `docs/superpowers/specs/2026-04-28-m{N}-<topic>-design.md` (this roadmap is the M7–M11 umbrella).
- One implementation plan: `docs/superpowers/plans/2026-04-28-m{N}-<topic>.md`.
- Implementation, code review, merge, then write the next milestone's spec.

---

## 7. Acceptance Criteria for Roadmap (this document)

This roadmap is "done" when:

- [x] All 14 gaps from openwolf comparison appear in milestone deliverables.
- [x] Cross-cutting decisions (process topology, state layout, daemon protocol, update mechanism, dashboard build, chromedp, backup/restore, identity, testing) each have a written decision with rationale.
- [x] Risk register identifies cut points for M10/M11 (largest items).
- [x] Implementation order matches the sequence the user agreed to (M7 → M8 → M9 → M10 → M11).
- [x] Backwards compatibility with M0–M6 is preserved (no breaking changes to MCP server, hooks, vectordb backends).

Per-milestone acceptance criteria are deferred to per-milestone specs.
