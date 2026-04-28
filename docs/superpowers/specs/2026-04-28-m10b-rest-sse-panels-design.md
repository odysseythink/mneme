# M10b — REST + SSE + 3 Panels — Design Spec

## Overview

M10b is the second of three sub-milestones that together deliver M10 (Web Dashboard).

- **M10a (shipped)** — backend route plumbing, `embed.FS` pipeline, `mneme dashboard` CLI, build system, auth handshake, "Backend is up" placeholder page.
- **M10b (this spec)** — `pkg/events` (pub/sub + persistence), `pkg/dashboard/api.go` REST handlers, `pkg/dashboard/sse.go` for live event streaming, plus three panels: Overview, Activity, Cron.
- **M10c (later spec)** — the remaining seven panels (Cerebrum, Memory, Anatomy, BugLog, Suggestions, Token, DesignQC).

After M10b ships, `go build ./cmd/mneme && mneme dashboard` opens a real, useful dashboard with a left sidebar, three working pages, and live event updates.

The roadmap originally said "REST + WebSocket". After brainstorming we picked **Server-Sent Events** instead because the dashboard only needs server→client streaming and SSE costs nothing extra (plain HTTP, native browser API, no new Go deps). See §2.3 for rationale.

## 1. Goals & Non-Goals

### Goals

- **Three working panels.** Overview (cross-project rollup), Activity (live event log), Cron (interactive task control).
- **Live updates within ~500 ms** of an event firing in the daemon.
- **One streaming source of truth.** A single `pkg/events.Bus` publishes to both the SSE endpoint and a persistent JSONL log. M10c panels reuse the same bus.
- **Reusable frontend primitives.** `useFetch`, `useSSE`, `useProjectList`, `AppShell`, `<api/client.ts>` are designed for M10c to drop in seven more panels with no refactor.
- **Single Go binary.** No new runtime deps beyond M10a. One new frontend dep: `react-router-dom@^7`.
- **Backwards compatible.** No M0–M10a schema or behavior changes. Hook handlers and cron scheduler get one new line each (`bus.Publish(…)`).

### Non-goals

- **No M10c panels.** Cerebrum / Memory / Anatomy / BugLog / Suggestions / Token / DesignQC all wait for M10c.
- **No charts or sparklines.** Recharts arrives in M10c if at all.
- **No filters, search, or per-project drill-down.** M10c.
- **No dark theme, mobile layout, or i18n.** M10c / M11.
- **No multi-user auth or team mode.** Roadmap §5 (deferred).
- **No WebSocket.** SSE is sufficient for the use case.

## 2. Architecture

### 2.1 Component diagram

```
┌──────────────────────────────────────────────────┐
│  browser (React, single SPA)                     │
│  ┌─────────────────────────────────────────────┐ │
│  │ AppShell      (layout + nav, react-router)  │ │
│  │ ├ <Overview>  fetch /api/overview           │ │
│  │ ├ <Activity>  fetch /api/activity + SSE     │ │
│  │ └ <Cron>      fetch /api/cron + POSTs       │ │
│  │ shared: api/client.ts, hooks/useSSE,        │ │
│  │         hooks/useProjectList                │ │
│  └─────────────────────────────────────────────┘ │
└────────────────┬─────────────────────────────────┘
                 │ HTTP (cookie auth, M10a)
                 ▼
┌──────────────────────────────────────────────────┐
│  pkg/daemon (extended)                           │
│   routes.go: registers M10b /api/* + /events     │
│   events fanout wired to:                        │
│     - hook handlers (cerebrum, scan, suggestions)│
│     - cron scheduler                             │
└────────────────┬─────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────┐
│  pkg/events  (NEW)                               │
│   Bus: pub/sub with in-memory ring (last 500)    │
│   Writer: appends each event to events.jsonl     │
│   Reader: tails last N from ring, falls back     │
│           to events.jsonl if ring not full       │
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│  pkg/dashboard (extended)                        │
│   api.go      REST: /api/overview, /api/activity,│
│               /api/cron, /api/projects           │
│   sse.go      /events SSE endpoint               │
└──────────────────────────────────────────────────┘
```

### 2.2 Package boundaries

Each unit has one responsibility:

- **`pkg/events`** — pub/sub primitives. Knows nothing about HTTP, daemon, or panel shapes. Public API: `NewBus(home, log)`, `Bus.Publish(Event)`, `Bus.Subscribe() (<-chan Event, unsub)`, `Bus.Tail(limit, since) []Event`, `Bus.Close()`. Internal: ring buffer, JSONL writer with daily rotation, retention sweep.
- **`pkg/dashboard/api.go`** — REST handlers. Read-only data assembly from `~/.mneme/projects/<id>/*` files + manifest + cron-state. No daemon mutation.
- **`pkg/dashboard/sse.go`** — wraps `events.Bus.Subscribe()` as `text/event-stream`. Handles disconnect cleanup, periodic ping, optional type/project_id filter.
- **`pkg/daemon`** — owns the singleton `*events.Bus`, injects it into `RouteDeps`. Cron scheduler + hook handlers publish into it.
- **`web/src/api/`** — thin fetch wrappers, one file per resource. **`web/src/hooks/`** — small reusable React hooks. **`web/src/panels/`** — one folder/file per panel. **`web/src/components/`** — shared UI bits.

**Auth** unchanged from M10a: cookie-or-query-token via `daemon.AuthMW`. SSE endpoint inherits the same middleware. No new headers, no CSRF token (single-user, same-origin only).

### 2.3 Why SSE, not WebSocket

| Concern | SSE | WebSocket |
|---|---|---|
| Direction | server→client only | full-duplex |
| Browser API | `new EventSource(url)` (built-in) | needs lib or thin wrapper |
| Auto-reconnect | built-in | manual |
| Auth | cookie auth works unchanged | same |
| Go deps | none (`net/http` + flusher) | `gorilla/websocket` (~150 KB module) |
| Goroutines per conn | 1 | 2 (read + write loop) |

The dashboard never needs the browser to push to the daemon over the streaming channel — every mutation already has a REST endpoint (`/cron/run`, `/cron/retry`, etc.). SSE is the right shape.

## 3. `pkg/events` — pub/sub package

### 3.1 Types

```go
type Event struct {
    TS        int64           `json:"ts"` // unix ms
    Type      string          `json:"type"`
    ProjectID string          `json:"project_id,omitempty"`
    Data      json.RawMessage `json:"data,omitempty"`
}

type Bus struct {
    // ring: last 500 events, RWMutex
    // subs: map[chan Event]struct{}, buffered chan size 64, RWMutex
    // file: events-YYYYMMDD.jsonl in <home>/.mneme/daemon/, rotated daily
    // log:  Logger for write errors and rotation events
}

func NewBus(home string, log Logger) (*Bus, error)
func (b *Bus) Publish(e Event)                       // ring + jsonl + fan
func (b *Bus) Subscribe() (<-chan Event, func())     // chan + unsubscribe
func (b *Bus) Tail(limit int, since int64) []Event   // ring first, jsonl fallback
func (b *Bus) Close() error
```

### 3.2 Slow subscriber policy

If a subscriber's channel is full when `Publish` runs, the event is dropped for that subscriber (non-blocking send via `select default`). The bus does not block other subs or the publisher. Drop counts surface in a metrics-only field on the bus (visible via `/health` extension; not a hard error).

### 3.3 Persistence + rotation

- File: `~/.mneme/daemon/events-YYYYMMDD.jsonl`, mode 0600.
- One JSON object per line, newline-terminated.
- Append-only. `mu.Lock` around each append (writer is single-threaded; not a hot path at expected ≤10 events/s).
- Rotation: on each append, compare today's date to filename's date. If different, close the old file, open the new one. Same scheme as `pkg/daemon/log.go`.
- Retention: on rotation, list `events-*.jsonl` files; delete those older than 14 days.

### 3.4 `Tail(limit, since)` semantics

- If the ring contains events with `ts >= since`, return up to `limit` newest of them.
- Otherwise, scan the JSONL file backwards (read last 8 KiB, parse, repeat if needed) until enough events are found or all files exhausted.
- `since=0` means "no lower bound." `limit` capped at 500.

### 3.5 Event types published in M10b

| Type | Publisher | Process | Data fields |
|---|---|---|---|
| `hook.fired` | hook runners (`mneme hook <event>` subprocesses) | **out-of-process** | `hook`, `file?`, `tokens?` |
| `cron.tick` | scheduler (start + finish of each run) | in-process | `name`, `status` (started\|ok\|failed), `duration_ms`, `error?` |
| `scan.complete` | anatomy-rescan task | in-process | `files_changed`, `duration_ms` |
| `suggestion.new` | suggestions cron | in-process | `kind`, `count` |
| `cerebrum.candidate` | `/cerebrum/learn` handler | in-process | `count` |

### 3.6 In-process vs out-of-process publishing

**In-process publishers** (cron scheduler, `/cerebrum/learn` handler, suggestions cron) call `bus.Publish(e)` directly — they run inside the daemon goroutine tree.

**Out-of-process publishers** (hook subprocesses spawned by Claude Code) cannot access the bus directly. They post to a new internal endpoint:

```
POST /events/publish        (Unix-socket-only, no TCP)
{ "type": "hook.fired", "project_id": "abc", "data": {"hook": "pre-write", "file": "main.go"} }
```

The handler validates the event and calls `bus.Publish(e)`. Hook runners use `daemonclient.PublishEvent(...)` which `TryDial`s the socket; if the daemon is down, the hook logs a debug line and continues — local state files are still written by the hook itself, but that event is absent from `events.jsonl` and the SSE stream. Documented degraded mode: when daemon is down, the dashboard is also down, so loss is invisible to the user.

`POST /events/publish` is rejected when called over TCP (verified by the existing `daemon.WithTransport` middleware from M8 — `TransportUnix` vs `TransportTCP` is set per-listener in `pkg/daemon/server.go`). This keeps the endpoint a daemon-internal channel even though the dashboard can listen on TCP.

## 4. REST endpoints

All under `/api/*`, all `GET`, all read-only. Mutation endpoints (`/cron/run`, `/cron/retry`, `/scan`, `/update`, `/restore`) keep their existing paths from M8 — the panels call them directly without aliasing.

### 4.1 `GET /api/overview`

```json
{
  "daemon": {"pid": 1234, "uptime_s": 3600, "version": "0.2.0-m10b", "started_at": 1714290000},
  "totals": {
    "projects": 5,
    "anatomy_files": 320,
    "cerebrum_pending": 7,
    "open_suggestions": 2
  },
  "projects": [
    {
      "id": "abc123",
      "origin": "/Users/foo/work/repo",
      "anatomy_files": 42,
      "cerebrum_pending": 3,
      "memory_bytes": 1500,
      "last_activity_ts": 1714290000
    }
  ]
}
```

`last_activity_ts` is the most recent `events.jsonl` event with that `project_id`, or 0 if none.

### 4.2 `GET /api/projects`

```json
{
  "projects": [{ ...same shape as overview.projects[] }]
}
```

Same data as `overview.projects`, exposed standalone for M10c reuse and for the `useProjectList` hook.

### 4.3 `GET /api/activity?limit=N&since=TS&types=t1,t2&project_id=ID`

```json
{
  "events": [
    {"ts": 1714290000123, "type": "hook.fired", "project_id": "abc", "data": {"hook": "pre-read", "file": "main.go"}}
  ],
  "next_cursor": "1714289000000"
}
```

- `limit` defaults to 100, capped at 500.
- `since` is unix ms; events with `ts > since` are returned, newest-first.
- `types` and `project_id` are server-side filters.
- `next_cursor` is the smallest `ts` in the response, suitable for "load older" requests.

### 4.4 `GET /api/cron`

```json
{
  "tasks": [
    {
      "name": "anatomy-rescan",
      "schedule": "*/30 * * * *",
      "enabled": true,
      "state": {
        "last_run": 1714289500,
        "next_run": 1714291300,
        "retry_count": 0,
        "dead_lettered": false,
        "last_error": ""
      }
    }
  ]
}
```

Same data as the existing `GET /cron/list`, with `last_error` exposed (currently in cron-state but not surfaced in M8).

## 5. SSE: `GET /events`

### 5.1 Wire format

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-store
Connection: keep-alive
X-Accel-Buffering: no

data: {"ts":1714290000123,"type":"hook.fired","project_id":"abc","data":{...}}\n
\n
: ping\n
\n
```

- Each event written as a single `data:` line (one JSON object), terminated by `\n\n`.
- A `: ping` comment is sent every 25 s to defeat reverse-proxy idle-close (defensive; we don't use a proxy today, but cheap insurance).
- `flusher.Flush()` is called after every write.

### 5.2 Query filters

| Param | Effect |
|---|---|
| `types=hook.fired,cron.tick` | comma-separated allowlist; default = all |
| `project_id=abc123` | only events with matching project_id; default = all |

Filters apply to live stream only — for historical data, panels use `/api/activity?since=…`.

### 5.3 Lifecycle

- Client opens `EventSource("/events?types=…")`. `AuthMW` checks cookie/query-token as for any other request.
- Handler subscribes to `bus`, ranges over the channel, writes each event as SSE.
- On `r.Context().Done()` (client disconnects, daemon shuts down): unsubscribe and return.
- Disconnect cleanup is symmetric: bus closes the channel from the publisher side, handler exits its `range`.

## 6. Frontend

### 6.1 File layout

```
web/src/
├── main.tsx              (entry, M10a — unchanged)
├── App.tsx               (router root — replaces M10a placeholder)
├── Bootstrap.tsx         (M10a — kept)
├── styles.css
├── api/
│   ├── client.ts         (fetch wrapper: credentials='include', JSON, throws on !ok)
│   ├── overview.ts
│   ├── projects.ts
│   ├── activity.ts
│   └── cron.ts
├── hooks/
│   ├── useFetch.ts       (typed fetcher, manual refetch, optional polling interval)
│   ├── useSSE.ts         (EventSource wrapper with auto-reconnect + types filter)
│   └── useProjectList.ts
├── components/
│   ├── AppShell.tsx
│   ├── HealthCard.tsx    (replaces M10a Health.tsx)
│   ├── ProjectCard.tsx
│   ├── EventRow.tsx
│   └── CronTaskRow.tsx
└── panels/
    ├── Overview.tsx
    ├── Activity.tsx
    └── Cron.tsx
```

### 6.2 Routing

`react-router-dom` v7 with `createBrowserRouter`:

| Path | Element |
|---|---|
| `/` | `<AppShell><Overview/></AppShell>` |
| `/activity` | `<AppShell><Activity/></AppShell>` |
| `/cron` | `<AppShell><Cron/></AppShell>` |
| `*` | redirect to `/` (also handled at server via M10a SPA fallback) |

`AppShell` has a 240 px left sidebar (3 nav links + a tiny "daemon healthy" indicator driven by `useFetch(getOverview, { interval: 10000 })`) and an `<Outlet/>` for the active panel.

### 6.3 Refresh model — SSE-driven, polling fallback

```
useSSE() opens 1 EventSource at app mount, broadcasts events via React context.
Each panel subscribes to event types it cares about, debounces refetch (250 ms).
Polling fallback: each useFetch accepts a fallbackIntervalMs.
If SSE has been disconnected for >10 s, polling kicks in.
```

| Panel | Initial fetch | SSE triggers refetch on | Polling fallback |
|---|---|---|---|
| Overview | `/api/overview` | `scan.complete`, `cerebrum.candidate`, `suggestion.new` | 10 s |
| Activity | `/api/activity?limit=100` | (any event) — prepend in-place, no refetch | 30 s (only if SSE down) |
| Cron | `/api/cron` | `cron.tick` | 5 s |

### 6.4 Dependencies added to `web/package.json`

| Package | Why |
|---|---|
| `react-router-dom@^7` | URL routing |

That is the only new dep. Custom hooks cover SSE and fetching in <200 LOC total. No state library, no fetcher library, no chart library, no toast library.

### 6.5 UI — Tailwind only

No Recharts, no theme switcher, no toasts library. Plain Tailwind v4 (already set up in M10a) plus a tiny inline toast component for cron action results. Light theme only. Function over form for M10b — M10c will polish.

### 6.6 Type safety

A small `web/src/api/types.ts` mirrors the Go response shapes. Hand-written, not generated. The API surface in M10b is small (4 endpoints); wiring up swag/openapi is more cost than value at this size. M10c can revisit.

## 7. Files added or modified

### Added

- `pkg/events/{bus,ring,jsonl,event}.go`
- `pkg/dashboard/{api,sse}.go`
- `tests/events_bus_test.go`
- `tests/events_jsonl_test.go`
- `tests/dashboard_api_test.go`
- `tests/dashboard_sse_test.go`
- `tests/integration/dashboard_panels_test.go` (`//go:build integration`)
- `web/src/App.tsx` (replaces M10a placeholder)
- `web/src/api/{client,overview,projects,activity,cron}.ts`
- `web/src/hooks/{useFetch,useSSE,useProjectList}.ts`
- `web/src/components/{AppShell,HealthCard,ProjectCard,EventRow,CronTaskRow}.tsx`
- `web/src/panels/{Overview,Activity,Cron}.tsx`
- `web/src/api/types.ts`

### Modified

- `pkg/daemon/routes.go` — register `/api/*`, `/events`, and `/events/publish`; extend `RouteDeps` with `Bus *events.Bus`.
- `pkg/daemon/daemon.go` — instantiate `*events.Bus`, inject into `RouteDeps` and `Scheduler`, close on shutdown.
- `pkg/daemon/scheduler.go` — publish `cron.tick` (start) before each run and (`ok`/`failed`) after each run.
- `pkg/daemon/cerebrum_handler.go` — call `bus.Publish` when a candidate is enqueued.
- `pkg/suggestions/engine.go` — return new-suggestion count so the suggestions cron task can publish `suggestion.new`.
- `cmd/hook_prewrite.go`, `cmd/hook_postwrite.go`, `cmd/hook_sessionstart.go`, `cmd/hook_stop.go` — call `daemonclient.PublishEvent("hook.fired", root, data)` after the existing `state.IncrementSafe(...)`.
- `pkg/daemonclient/client.go` — add `PublishEvent(type, projectID, data) error` (TryDial → POST /events/publish; logs and returns nil on `ErrUnavailable`).
- `pkg/dashboard/server.go` — extend `Deps` with `Bus` so `Mount` can wire the SSE handler; existing `Mount` callers (M10a tests) still work because `Bus` is optional (nil disables /events).
- `web/src/Health.tsx` → moved to `web/src/components/HealthCard.tsx` (rename + reshape).
- `web/package.json` — add `react-router-dom@^7`.
- `Makefile` — no changes; `make web-build` already runs `pnpm tsc --noEmit && vite build`.

## 8. Testing

### Go

| File | What |
|---|---|
| `tests/events_bus_test.go` | Publish→Subscribe round-trip; slow-sub drop; `Tail(n, since)` ring-only; filter by type/project_id |
| `tests/events_jsonl_test.go` | Append+rotation across day boundary; 14-day retention sweep; `Tail` falling back to file when ring is short |
| `tests/dashboard_api_test.go` | All four `/api/*` endpoints with `httptest` + `t.TempDir()` HOME and seeded fixture files |
| `tests/dashboard_sse_test.go` | Subscribe via httptest; receive 3 published events; receive `: ping` after simulated tick; client disconnect unsubscribes |
| `tests/integration/dashboard_panels_test.go` | Full daemon up over Unix socket; request all four `/api/*` endpoints + open `/events`; verify auth required without cookie |

### Frontend

- `make web-build` runs `pnpm tsc --noEmit && vite build`. Type errors fail the build.
- One Vitest smoke test: `<App/>` renders the AppShell + Overview without throwing. (Mock fetch returns empty JSON; mock EventSource is a no-op.)
- No deeper UI tests in M10b — visual verification is the manual acceptance pass below; automated visual diff is M11 design QC.

## 9. Acceptance criteria

After M10b ships:

1. `go build ./cmd/mneme && mneme daemon start && mneme dashboard` opens a browser tab with three working pages reachable from the left sidebar.
2. **Overview** shows: HealthCard (pid/uptime/version), totals row (#projects, anatomy files, cerebrum pending, open suggestions), one ProjectCard per project under `~/.mneme/projects/`.
3. **Activity** shows the last ~100 events; firing a hook (e.g. `mneme hook pre-read --file foo.go`) makes a new row appear at the top within ~500 ms.
4. **Cron** shows seeded tasks; clicking **Run** schedules a task and the table reflects new `last_run` within ~5 s; clicking **Retry** on a dead-lettered task clears it.
5. SSE auto-reconnects within 1–2 s of a transient disconnect; polling fallback engages if SSE down >10 s.
6. `~/.mneme/daemon/events-YYYYMMDD.jsonl` exists, rotates daily, files >14 days old are deleted on next rotation.
7. `go test ./...`, `go test -tags=integration ./tests/integration/...`, `pnpm tsc --noEmit`, and `vitest run` all pass.

## 10. Risks

| Risk | Mitigation |
|---|---|
| Browser `EventSource` can't send custom headers | M10a cookie auth covers it; query token also accepted on `/events` via existing `AuthMW`. |
| `events.jsonl` grows unbounded | Daily rotation + 14-day retention sweep. |
| Bus publish blocks if a subscriber stalls | Buffered channels (size 64) + drop-on-full policy; `Publish` is non-blocking by contract. |
| Cron Run button double-clicks fire two runs | `cron.SkipIfStillRunning` chain (M8) de-dupes server-side. UI also disables the button while a request is in flight. |
| react-router v7 has SSR pieces; we only need client routing | Use `createBrowserRouter` + `RouterProvider` directly; no SSR mode. ~12 KB gzipped. |
| pnpm version drift between dev and CI | M10a pinned `pnpm@9.15.0` in `package.json`; unchanged. |
| Slow JSONL append blocks `Publish` under load | At expected ≤10 events/s the lock contention is negligible. If profiling shows otherwise, add a buffered writer goroutine in a future patch. |

## 11. Out of scope (deferred to M10c or later)

- Cerebrum, Memory, Anatomy, BugLog, Suggestions, Token, DesignQC panels — M10c.
- Charts or sparklines — M10c if needed.
- Filters, search, pagination beyond `?limit` cursor — M10c.
- Per-project drill-down routes (`/projects/:id`) — M10c.
- Dark theme, design polish, mobile layout — M10c / M11.
- Multi-user auth, team mode — roadmap §5 (deferred).
- WebSocket transport — explicitly rejected, see §2.3.
