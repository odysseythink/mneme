# M8 — Daemon + Cron Scheduler — Design Spec

## Overview

M8 introduces a long-running daemon process that owns three things mneme has lacked until now:

1. **Scheduled background work** — anatomy rescans, memory consolidation, backup pruning run on a cron without the user running CLI commands.
2. **A shared HTTP control plane** — `mneme status`, `mneme scan`, `mneme update`, `mneme restore` (M7) prefer to talk to the daemon over a Unix socket; they fall back to running locally when the daemon is down.
3. **A foundation for M10/M11** — the same HTTP server later hosts the Dashboard's REST + WebSocket and Design QC's capture endpoints.

This spec finalizes the design decisions deferred from the M7–M11 roadmap (`docs/superpowers/specs/2026-04-28-m7-m11-roadmap-design.md`, §2.1–§2.3, §2.7, §2.9) and adds the M8-specific decisions made during brainstorming.

---

## 1. Goals & Non-Goals

### Goals

- A persistent daemon process that survives terminal close and (optionally) restarts at user login.
- A cron scheduler that runs three built-in tasks (`anatomy-rescan`, `consolidate-memory`, `prune-backups`) with retry → dead-letter behavior and crash isolation per task.
- An HTTP/1.1 control plane on a Unix socket (always) and TCP loopback (off by default in M8, on in M10), reusing the same route table for both transports.
- Lock coordination so daemon-driven and CLI-driven operations can never corrupt the same project state, regardless of which path executed.
- Clean shutdown on SIGTERM with bounded drain; manifest reload on SIGHUP; log rotation on SIGUSR1.
- `mneme daemon install` writes a launchd plist (macOS) or systemd user unit (Linux) for opt-in auto-start at login.

### Non-goals

- **No dashboard endpoints** — `/api/dashboard/*` and `/ws` are M10. M8 ships only the operational endpoints listed in §5.
- **No subprocess task execution** — every task is an in-process Go function (see §4). Subprocess isolation is rejected as YAGNI.
- **No multi-machine mode** — single user, single host, one daemon per `$HOME`.
- **No Windows-first support** — daemon targets macOS + Linux. Windows works on best effort (no SIGUSR1; no install command).
- **No arbitrary user-defined cron tasks** — the manifest references only built-in registered tasks (see §4).

---

## 2. Architecture

### 2.1 Package layout

```
pkg/daemon/
├── daemon.go        process entrypoint (Run); orchestrates server + scheduler + log + heartbeat
├── server.go        HTTP listener (Unix socket + optional TCP loopback), middleware chain
├── routes.go        endpoint dispatch + handler implementations
├── scheduler.go     manifest-driven cron loop; uses robfig/cron/v3
├── tasks.go         registry: name → TaskFunc; built-in task implementations
├── state.go         load/save cron-state.json with atomic write
├── log.go           daily-rotated daemon log; SIGUSR1 trigger
├── heartbeat.go     30-min mtime touch on heartbeat.json
└── install.go       launchd plist / systemd user unit writer

pkg/daemonclient/
└── client.go        typed HTTP client; TryDial() returns ErrUnavailable when socket is missing/refused

cmd/cmd_daemon.go    start | stop | restart | status | logs | install | uninstall
```

`pkg/daemonclient` lives in its own package (not `pkg/daemon`) so CLI subcommands can depend on it without pulling in scheduler/server code.

### 2.2 Process topology

```
mneme daemon start            # long-running daemon
  ├── HTTP listener (Unix socket; optional TCP)
  ├── scheduler goroutine (1s tick)
  ├── heartbeat goroutine (30m tick)
  └── signal handler (TERM | HUP | USR1)

mneme <subcommand>            # CLI client
  → pkg/daemonclient.TryDial()
     ├── success → send request, render response
     └── ErrUnavailable → fall back to local code path (M7 behavior)
```

The daemon does **not** fork itself. `mneme daemon start` runs in the foreground by default; `--detach` does a single fork + setsid, prints the new PID, and exits 0 in the parent.

### 2.3 State directory layout

```
~/.mneme/daemon/
├── pid                       gofrs/flock; refused if held by a live process
├── socket                    Unix domain socket (mode 0600)
├── token                     32 bytes hex, regenerated on every start (mode 0600)
├── cron-manifest.json        task definitions (auto-seeded; user-editable)
├── cron-state.json           per-task last_run / failures / dead-letter
├── heartbeat.json            { ts, pid, version }
└── logs/
    └── daemon-YYYYMMDD.log   rotated daily, kept 14 days
```

All files are mode 0600; the directory is mode 0700.

---

## 3. Lifecycle

### 3.1 Startup sequence (`mneme daemon start`)

1. Resolve `~/.mneme/daemon/`; create with `0700` if missing.
2. Acquire `pid` via `gofrs/flock.New(pidPath).TryLock()`.
   - On failure: read PID from file. If the process is alive (`os.FindProcess` + `Signal(syscall.Signal(0))`), exit 1 with `daemon already running (pid N)`.
   - If the PID is stale (process gone), remove pid + socket files and retry once.
3. Generate token (32 bytes hex), write `~/.mneme/daemon/token` with mode 0600.
4. Open Unix listener at `~/.mneme/daemon/socket` with mode 0600. Remove any pre-existing socket file first (we already proved the holder is dead in step 2).
5. If `daemon.dashboard_port_enabled=true`, also bind TCP `127.0.0.1:<dashboard_port>`. Default is `false` in M8.
6. Start scheduler goroutine: load manifest + state, register cron entries, run.
7. Start heartbeat goroutine: write `heartbeat.json` immediately, then every 30 minutes.
8. Install signal handler for `SIGTERM | SIGINT | SIGHUP | SIGUSR1`.
9. Block on signal channel.

If any step after (2) fails, release the flock and remove the pid file before exiting non-zero.

### 3.2 Signals

| Signal           | Behavior                                                                         |
|------------------|----------------------------------------------------------------------------------|
| `SIGTERM`/`SIGINT` | graceful shutdown: stop accepting new HTTP, drain in-flight requests up to `daemon.shutdown_timeout_seconds` (default 10s), cancel scheduler ctx, close listeners, release flock, exit 0 |
| `SIGHUP`         | re-read `cron-manifest.json` only; rebuild scheduler entries; existing in-flight tasks continue. `config.yaml` is NOT reloaded — use `daemon restart` for that. |
| `SIGUSR1`        | rotate log: close current daily file, reopen with today's date. No-op on Windows. |

### 3.3 Crash recovery

If the daemon dies hard (`kill -9`, OOM), the next `daemon start` detects:

- **Stale PID:** PID file exists but process is gone → delete pid + socket, proceed.
- **Stale socket:** socket file exists, dial fails with `ECONNREFUSED` → delete socket, proceed.

A live daemon's PID lock prevents accidental double-start. The socket file alone is not used for liveness because Unix sockets don't auto-clean on process death.

### 3.4 Install / uninstall

`mneme daemon install`:

- **macOS:** writes `~/Library/LaunchAgents/com.mneme.daemon.plist` with `KeepAlive=true`, `RunAtLoad=true`, `ProgramArguments=[<absPath to mneme>, daemon, start]`, `StandardOutPath` and `StandardErrorPath` pointing to `~/.mneme/daemon/logs/launchd.log`. Then `launchctl bootstrap gui/<uid> <path>` and `launchctl enable gui/<uid>/com.mneme.daemon`.
- **Linux:** writes `~/.config/systemd/user/mneme-daemon.service` with `[Service] Type=simple ExecStart=<absPath> daemon start Restart=on-failure`. Then `systemctl --user daemon-reload && systemctl --user enable --now mneme-daemon.service`.
- Prints summary: where the unit was written, the command used to load it, how to uninstall.

`mneme daemon uninstall` reverses both steps. If the unit file was hand-edited (sha256 of file != sha256 mneme would write today), abort with `unit appears modified; remove manually`.

---

## 4. Scheduler & Task Registry

### 4.1 Manifest

Path: `~/.mneme/daemon/cron-manifest.json`. Auto-seeded on first `daemon start` if missing; never overwritten thereafter. SIGHUP re-reads it.

```json
{
  "version": 1,
  "tasks": [
    { "name": "anatomy-rescan",     "schedule": "@every 6h", "enabled": true },
    { "name": "consolidate-memory", "schedule": "0 3 * * *", "enabled": true },
    { "name": "prune-backups",      "schedule": "30 3 * * 0", "enabled": true }
  ]
}
```

- `schedule` accepts the cron-5 form (`m h dom mon dow`) or `@every <duration>` parsed by `github.com/robfig/cron/v3`.
- `name` must match a key in the in-binary task registry (§4.3). Unknown names log a warning and are skipped — they do not abort startup.
- `enabled=false` removes the task from the active scheduler without removing the manifest entry.
- All `cron-5` schedules are evaluated in `time.Local`. Users in non-default timezones can edit the manifest schedule expression accordingly.

### 4.2 State

Path: `~/.mneme/daemon/cron-state.json`. Atomic-written via `pkg/state.AtomicWrite` after every state change.

```json
{
  "version": 1,
  "tasks": {
    "anatomy-rescan": {
      "last_run":              "2026-04-28T12:34:56Z",
      "last_success":          "2026-04-28T12:34:56Z",
      "last_error":            "",
      "consecutive_failures":  0,
      "dead_lettered_at":      ""
    }
  }
}
```

`""` distinguishes "never set" from a zero `time.Time`.

### 4.3 Task registry

```go
type TaskFunc func(ctx context.Context, log Logger) error

var registry = map[string]TaskFunc{
    "anatomy-rescan":     runAnatomyRescan,
    "consolidate-memory": runConsolidateMemory,
    "prune-backups":      runPruneBackups,
}
```

Each task's behavior:

- **`runAnatomyRescan`** — enumerate `~/.mneme/projects/*/origin`, call `scanner.ScanProjectIncremental(root)` for each existing root. Skip projects whose `origin` file points to a missing directory (logged at debug). Returns nil on success even if some projects skipped; only IO errors that prevent any progress return non-nil.
- **`runConsolidateMemory`** — same project enumeration; for each project call `consolidator.ConsolidateIfNeeded(projectDir, 50)` (50 = existing M6 threshold).
- **`runPruneBackups`** — enumerate `~/.mneme/backups/<id>/`, apply retention rule (14 days OR 10 most recent, whichever is larger; rule lives in `pkg/installer/backup.go` from M7), delete pruned snapshots.

Each task acquires the per-project flock (§6) before touching that project's files, so two ticks running long enough to overlap cannot corrupt state.

### 4.4 Scheduler loop

`scheduler.Run(ctx)`:

- Constructs a `cron.Cron` with `cron.WithSeconds()=false`, `cron.WithLocation(time.Local)`, and a custom `cron.Logger` that writes to the daemon log.
- For each enabled manifest task, registers a wrapper closure that:
  1. Looks up `registry[task.Name]`.
  2. Wraps execution in `defer recover()` — panic = caught, logged with stack, treated as task error.
  3. Calls the registered `TaskFunc(ctx, log)`.
  4. On error: increment `consecutive_failures`, set `last_run` + `last_error`, schedule retry per §4.5.
  5. On success: reset `consecutive_failures` to 0; set `last_run` + `last_success`.
  6. Atomic-write `cron-state.json`.
- Starts the cron scheduler.
- On `ctx.Done()`, calls `cron.Stop()` which waits for in-flight tasks to finish (bounded by daemon's shutdown timeout).

### 4.5 Retry & dead-letter

On task error, the scheduler enqueues retries at offsets **30s, 2min, 10min** from the failure time using `time.AfterFunc`. After three retry failures (so four total attempts including the original tick), the task is **dead-lettered**:

- `dead_lettered_at` is set to the failure time.
- The task does not auto-run on its cron schedule until `POST /cron/retry { "name": "..." }` clears `dead_lettered_at` and resets `consecutive_failures`.
- The daemon log records `task=<name> status=dead_lettered failures=4`.

Retries skip if the daemon is shutting down (ctx cancelled). On next start, a dead-lettered task remains dead-lettered — state is durable.

### 4.6 Concurrency

- Each task wrapper is registered with `cron.WithChain(cron.SkipIfStillRunning(cronLogger))` so overlapping ticks of the **same** task are skipped (not queued, not stacked) — preferable to falling behind on a stuck task.
- Two **different** tasks (e.g., `anatomy-rescan` and `consolidate-memory`) targeting the same project may run concurrently — the per-project flock (§6) is the single serialization point.
- Tasks must respect `ctx.Done()` and return promptly when the daemon is shutting down. Long-running internal loops (e.g., scanning many projects) check `ctx.Err()` between projects.

---

## 5. HTTP Server

### 5.1 Listeners

- **Unix socket** — `~/.mneme/daemon/socket`, mode 0600. Always on.
- **TCP loopback** — `127.0.0.1:<daemon.dashboard_port>`. Off by default in M8 (`daemon.dashboard_port_enabled=false`); M10 flips the default to true.

Both listeners share one `http.Server` instance via two parallel `srv.Serve(listener)` goroutines. Identical route table, identical middleware chain.

### 5.2 Endpoint table (M8 baseline)

| Method | Path           | Body                              | Purpose                                                                         |
|--------|----------------|-----------------------------------|---------------------------------------------------------------------------------|
| GET    | `/health`      | —                                 | `{ pid, uptime_s, version, started_at }`                                        |
| GET    | `/status`      | —                                 | aggregate status: per-project anatomy mtime, embedding reachability, vector DB row counts, last hook fire times per type |
| POST   | `/scan`        | `{ project_id?, force?, check? }` | runs scan; returns `{ project_id, files_scanned, drift_count?, took_ms }`       |
| POST   | `/update`      | `{ project_id?, dry_run? }`       | template sync (M7 logic); returns per-project summary                            |
| POST   | `/restore`     | `{ project_id, timestamp }`       | restore from backup; returns `{ project_id, restored_files }`                   |
| GET    | `/cron/list`   | —                                 | merged manifest + state                                                          |
| POST   | `/cron/run`    | `{ name }`                        | immediate one-shot run, off-schedule, still locked                              |
| POST   | `/cron/retry`  | `{ name }`                        | clears `dead_lettered_at`; reschedules normally                                  |

`/api/dashboard/*` and `/ws` are explicitly **deferred to M10**.

### 5.3 Auth

- **Unix socket**: filesystem permission (0600) is the auth boundary. The middleware accepts these requests unconditionally.
- **TCP loopback**: requires `Authorization: Bearer <token>` matching `~/.mneme/daemon/token`. Constant-time compare via `subtle.ConstantTimeCompare`. Token is 32 bytes hex, regenerated on every `daemon start` (so a daemon restart invalidates old Dashboard sessions — this is acceptable; M10 cookie flow will redirect to re-auth).

The middleware distinguishes by the `*http.Request.Context()` value injected by the per-listener `http.Server.ConnContext` callback.

### 5.4 Middleware chain

In order, outermost first:

1. **`recoverMW`** — defers a recover; on panic, log stack, return 500 with `{"error":"internal"}`.
2. **`authMW`** — Unix socket bypasses; TCP requires bearer token. 401 on missing/invalid.
3. **`bodyLimitMW`** — wraps `r.Body` with `http.MaxBytesReader(w, r.Body, 1<<20)`; handlers reading past 1 MiB get 413.
4. **`logMW`** — after handler returns, append `RFC3339 INFO http method=GET path=/health status=200 dur_ms=2` to the daemon log.

Handlers return JSON with `Content-Type: application/json`. Errors use `{"error":"<message>","code":"<short_code>"}` shape.

### 5.5 Lock coordination semantics

Every state-mutating handler (`/scan`, `/update`, `/restore`, `/cron/run`) wraps its work in:

```go
unlock, err := state.AcquireLock(filepath.Join(projectDir, ".lock"), 30*time.Second)
if err != nil { return ErrBusy }
defer unlock()
```

`pkg/state.AcquireLock` already exists (used by M6 consolidator). The CLI fallback path (M7's `cmd_status`/`cmd_scan`/`cmd_update`/`cmd_restore`) calls the **same** helper — so a `mneme scan` running locally and the daemon's `/scan` handler arbitrate via the same flock file, regardless of which path the user took. This is what makes "daemon optional" actually safe.

`/cron/retry` does not need the lock (it only mutates `cron-state.json`); it serializes via a small in-memory `sync.Mutex` on the scheduler.

---

## 6. Lock Coordination (cross-cutting)

The single shared primitive is per-project `gofrs/flock` on `~/.mneme/projects/<id>/.lock`.

| Caller                           | Acquires lock?       |
|----------------------------------|----------------------|
| Daemon scheduler task            | yes (per-project)    |
| Daemon HTTP handler (mutating)   | yes                  |
| CLI subcommand fallback (mutating) | yes (same code path) |
| Read-only operations             | no                   |

Timeout is 30 seconds. On timeout the caller returns `ErrBusy` (HTTP 409 from the daemon, exit 1 from the CLI fallback) with message `another mneme operation is in progress`.

This rule is the technical reason the M7-defined fallback is safe: there is no CLI-vs-daemon mode; there is just "did the request go through the socket or not", with identical lock semantics either way.

---

## 7. Logging & Heartbeat

### 7.1 Daemon log

Path: `~/.mneme/daemon/logs/daemon-YYYYMMDD.log`. Mode 0600.

Format (one event per line):

```
2026-04-28T12:34:56Z INFO http method=GET path=/health status=200 dur_ms=2
2026-04-28T13:00:00Z INFO sched task=anatomy-rescan status=start
2026-04-28T13:00:42Z INFO sched task=anatomy-rescan status=ok dur_s=42 projects=3
2026-04-28T19:00:00Z WARN sched task=consolidate-memory status=fail attempt=1 next_retry=30s err="lock timeout"
2026-04-28T19:00:30Z INFO sched task=consolidate-memory status=ok attempt=2
```

Levels: `DEBUG INFO WARN ERROR`. Default `INFO`. `LOG_LEVEL=debug` env var overrides.

Rotation: at midnight `time.Local`, close current file, open the next-day file. SIGUSR1 forces an immediate rotation regardless of clock. After rotation, files older than `daemon.log_retention_days` (default 14) are deleted.

### 7.2 Heartbeat

`~/.mneme/daemon/heartbeat.json`:

```json
{ "ts": "2026-04-28T13:00:00Z", "pid": 12345, "version": "0.2.0-m8" }
```

Rewritten atomically every 30 minutes and once at startup. M9's suggestions engine reads `mtime(heartbeat.json)`; if older than 90 minutes, daemon is presumed dead and a "daemon offline" suggestion is emitted.

---

## 8. Configuration

New keys in `~/.mneme/config.yaml` (read by `pkg/config`):

```yaml
daemon:
  cron_enabled:           true     # turn off scheduler entirely; HTTP server still serves /health, /status
  dashboard_port:         18801    # TCP loopback port
  dashboard_port_enabled: false    # off in M8; M10 flips default
  shutdown_timeout_seconds: 10     # graceful drain deadline on SIGTERM
  log_retention_days:     14
```

Env var override pattern (matches existing `EMBEDDING_API_KEY` etc):

| Env var                     | Overrides                          |
|-----------------------------|------------------------------------|
| `DAEMON_CRON_ENABLED`       | `daemon.cron_enabled`              |
| `DAEMON_DASHBOARD_PORT`     | `daemon.dashboard_port`            |
| `DAEMON_DASHBOARD_PORT_ENABLED` | `daemon.dashboard_port_enabled` |
| `DAEMON_SHUTDOWN_TIMEOUT_SECONDS` | `daemon.shutdown_timeout_seconds` |
| `DAEMON_LOG_RETENTION_DAYS` | `daemon.log_retention_days`        |

---

## 9. CLI Surface

`mneme daemon <subcommand>`:

| Subcommand | Flags                              | Behavior                                                                                |
|------------|------------------------------------|-----------------------------------------------------------------------------------------|
| `start`    | `--detach`                         | foreground by default; `--detach` does single fork+setsid, prints PID, parent exits 0   |
| `stop`     | `--timeout <s>` (default 15)       | reads pid, sends SIGTERM, polls socket close; exit 2 with hint if still alive at timeout|
| `restart`  | `--timeout <s>`                    | stop + start; preserves `--detach` semantics                                            |
| `status`   | `--json`                           | hits `/health`; renders human or JSON                                                    |
| `logs`     | `-f` (follow), `--date YYYY-MM-DD` | tails today's log; follow mode polls every 500ms                                         |
| `install`  | —                                  | writes launchd plist (macOS) or systemd user unit (Linux), loads it                     |
| `uninstall`| —                                  | unloads + deletes unit; refuses if unit file appears hand-edited                        |

All subcommands except `start` short-circuit if `~/.mneme/daemon/pid` does not exist (no daemon configured).

---

## 10. Testing Strategy

Per M7–M11 roadmap §2.9 row M8, with M8-specific elaboration:

| Layer            | Approach                                                                                           |
|------------------|----------------------------------------------------------------------------------------------------|
| Scheduler        | `pkg/daemon/scheduler_test.go` — retry-delay function injected (default `time.AfterFunc`; tests pass a synchronous variant) so retry timing is deterministic; tasks injected as failing/panicking funcs; assert retry sequence triggers, dead-letter after 4 attempts, `cron-state.json` writes after each transition. Cron-trigger timing itself is not unit-tested (we trust robfig/cron); a single integration test verifies one tick fires within 1 minute of an `@every 30s` schedule. |
| Server (Unix)    | `pkg/daemon/server_test.go` — `httptest.NewUnstartedServer`, override listener with `net.Listen("unix", ...)`; assert auth bypass on socket; assert middleware order (recover before auth before bodyLimit before log); assert 413 on >1 MiB body |
| Server (TCP)     | same file — `httptest.NewServer`; assert 401 on missing token, 401 on wrong token, 200 on correct token via `Authorization: Bearer` |
| Lock contention  | `pkg/daemon/server_test.go` — two goroutines hitting `/scan` concurrently; assert second returns 409 within timeout window |
| Install (golden) | `pkg/daemon/install_test.go` — `t.TempDir()` for HOME stub; render plist/unit; compare against golden file in `pkg/daemon/testdata/` |
| Heartbeat        | `pkg/daemon/heartbeat_test.go` — fake clock; assert mtime advances on tick; assert atomic write semantics |
| Integration      | `tests/daemon_integration_test.go` (`-tags=integration`) — `go build ./cmd && exec.Command(bin, "daemon", "start", "--detach")`; wait for socket; hit `/health`; send SIGTERM; assert pid file removed and exit 0 |

Cron task functions (`runAnatomyRescan` etc) are tested via the existing `pkg/scanner` and `pkg/consolidator` test suites — the daemon wrappers add only ctx cancellation and panic recovery, which are tested directly with synthetic injected functions.

CI runs the full unit + integration suite on macOS-latest and ubuntu-latest. `install_test.go` skips on the wrong platform via `runtime.GOOS`.

---

## 11. Out of Scope (Deferred)

The following are intentionally not part of M8; each is owned by a later milestone:

- `/api/dashboard/*` REST endpoints — M10
- `/ws` WebSocket fanout — M10
- Embedded `web/dist` static asset server — M10
- One-time browser-token cookie flow for Dashboard — M10
- M9 cron tasks (`weekly-waste-report`, `cerebrum-learn`, `suggestions-refresh`) — M9 (registered into the same manifest format defined here)
- `/designqc/capture` endpoint — M11
- Multi-user / multi-machine daemon mode — explicitly out of scope (see §1)
- Arbitrary user-defined cron tasks via shell command — explicitly out of scope (see §1)
- Windows-native install (would require Service Control Manager wiring) — best effort; CLI subcommands work, `install` errors with a clear message

---

## 12. Acceptance Criteria

M8 is "done" when:

- [ ] `mneme daemon start` runs persistently, owns the pid lock, serves `/health` over the Unix socket within 1 second of start.
- [ ] `mneme daemon stop` cleanly shuts down within `daemon.shutdown_timeout_seconds`; pid file removed; socket file removed; subsequent `daemon start` succeeds.
- [ ] `mneme daemon install` on macOS produces a launchd plist that survives logout/login and re-launches the daemon. (Manual smoke; not part of CI.)
- [ ] `mneme daemon install` on Linux produces a systemd user unit that survives logout/login. (Manual smoke; not part of CI.)
- [ ] All three built-in cron tasks (`anatomy-rescan`, `consolidate-memory`, `prune-backups`) execute on schedule and update `cron-state.json` atomically.
- [ ] A failing task retries at 30s/2min/10min and dead-letters on the fourth attempt; `POST /cron/retry` clears the dead-letter and reschedules.
- [ ] A panicking task is contained: daemon stays up, error logged with stack, treated as task failure.
- [ ] `/scan`, `/update`, `/restore`, `/cron/run` acquire the per-project flock; concurrent calls return 409 within the 30s timeout.
- [ ] CLI fallback (`mneme scan` with daemon down) acquires the same flock, so a scheduled task and a CLI invocation cannot collide.
- [ ] M7 subcommands (`status`, `scan`, `update`, `restore`) prefer the daemon socket and fall back transparently when it is down. (M7 already has the fallback path; M8 adds the socket-preferring client.)
- [ ] Daemon log rotates at midnight; SIGUSR1 forces immediate rotate; files older than 14 days deleted.
- [ ] Heartbeat mtime advances every 30 minutes.
- [ ] All unit + integration tests pass on macOS-latest and ubuntu-latest GitHub Actions matrix.
- [ ] M0–M6 features (MCP server, hooks, scan, consolidator, cerebrum, buglog, memory) keep working unchanged when the daemon is **not** running. Backward compatibility is mandatory.

---

## 13. Open Questions (non-blocking)

These can be resolved during implementation without blocking M8 acceptance:

- **HTTP timeouts** — server-level read/write timeouts. Default proposal: 30s read, 5min write (long enough for a multi-project scan response).
- **`/status` aggregation cost** — building it on every call may be expensive for users with many projects. Default: build live; revisit with a 10s in-memory cache if profiling shows hot path.
- **`daemon logs --date` for very old dates** — what if the file was already pruned? Default: error with `log file <date> not found (retention: 14 days)`.
- **launchd / systemd unit content evolution** — when `install` content changes between mneme versions, should `daemon start` notice the user has an outdated unit? Default: no warning in v1; add a hash check in a later milestone if it becomes a real source of confusion.
