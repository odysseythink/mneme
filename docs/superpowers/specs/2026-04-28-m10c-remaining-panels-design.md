# M10c — Remaining Dashboard Panels — Design Spec

## Overview

M10c is the third and final sub-milestone of M10 (Web Dashboard). It ships the seven remaining panels that complete openwolf-equivalent parity:

- **M10a (shipped)** — backend route plumbing, embed pipeline, auth handshake, placeholder page.
- **M10b (designed)** — `pkg/events`, REST + SSE, three panels (Overview / Activity / Cron).
- **M10c (this spec)** — seven panels: Cerebrum, Memory, Anatomy, BugLog, Suggestions, Token, DesignQC; sidebar project picker; four new mutation endpoints (cerebrum approve, cerebrum reject, suggestion dismiss, buglog delete); inline SVG sparkline component.

After M10c ships, the dashboard has 10 panels total, project switching, the M9 auto-learning loop is closable from the browser (no CLI needed), and the only deferred feature is real DesignQC data — held until M11 because the underlying capture pipeline doesn't exist yet.

## 1. Goals & Non-Goals

### Goals

- **Seven new panels.** Cerebrum (with approve/reject), Memory (cross-project), Anatomy (directory tree), BugLog (with delete), Suggestions (with dismiss), Token (with sparkline), DesignQC (stub).
- **Project picker.** Sidebar `<select>` synced to `?project=<id>` URL param. One selection drives every per-project panel; deep-links work; refresh-stable.
- **Close the M9 loop in the dashboard.** Cerebrum candidates can be reviewed and approved/rejected from the browser. No CLI required.
- **Reuse M10b primitives.** `useFetch`, `useSSE`, `useProjectList`, `AppShell`, `api/client.ts`, `events.Bus` — all extended, none rewritten.
- **No new third-party deps.** Frontend stays at the M10b dependency set; backend adds nothing.
- **Backwards compatible.** No M0–M10b schema or behavior changes.

### Non-goals

- **No real DesignQC data.** Panel ships as a stub with M11 empty state. M11 plugs in the capture grid and report viewer.
- **No markdown renderer for Memory.** Plain text rows. `react-markdown` is a future drop-in if needed.
- **No bulk operations.** One row at a time for approve / reject / dismiss / delete.
- **No editing.** Mutations are appendOnce / removeOnce only — no edit-rule UI, no edit-bug UI.
- **No optimistic UI.** Every mutation triggers a full panel refetch (debounced). Single-user dashboard makes the cost negligible.
- **No pagination.** Anatomy / BugLog / Activity all return the full set within their existing limits. If a future project hits 10k+ entries, paginate as a follow-up.

## 2. Architecture

### 2.1 Component diagram (extends M10b)

```
sidebar (extended)                      main (active panel)
┌──────────────────────┐  ┌──────────────────────────────────┐
│ mneme                │  │  <Cerebrum>  /cerebrum?project=A │
│ ┌──────────────────┐ │  │  <Memory>    /memory             │
│ │ project: [▼ A  ] │ │  │  <Anatomy>   /anatomy?project=A  │
│ └──────────────────┘ │  │  <BugLog>    /buglog?project=A   │
│ Overview             │  │  <Suggestions> /suggestions?...  │
│ Activity             │  │  <Token>     /token?project=A    │
│ Cron                 │  │  <DesignQC>  /designqc?project=A │
│ ─────────────        │  └──────────────────────────────────┘
│ Cerebrum             │
│ Memory               │
│ Anatomy              │
│ BugLog               │
│ Suggestions          │
│ Token                │
│ DesignQC             │
│ stream: live         │
└──────────────────────┘
```

### 2.2 Project picker as React context

```tsx
type ActiveProject = {
  active: string | null      // project ID
  projects: ProjectSummary[]
  setActive: (id: string) => void
  loading: boolean
}
```

`ActiveProjectProvider` mounts at the top of `App.tsx` (inside `<SSEProvider>`). It calls `useFetch(getProjects)`, reads `?project=` via `useSearchParams`, falls back to the first project when no value is in the URL, and exposes `setActive` (which writes `?project=<id>` via `setSearchParams`).

`<ProjectPicker/>` (sidebar) is a plain `<select>` bound to the context. Switching projects rewrites the URL; deep links land on the chosen project; browser back/forward navigates between selections.

The Memory panel ignores the picker (its data is cross-project). The picker stays visible but the panel's banner notes "Memory is cross-project."

### 2.3 Backend file split

One file per panel domain in `pkg/dashboard/`:

| File | Responsibility |
|---|---|
| `pkg/dashboard/cerebrum.go` | `CerebrumHandler`, `CerebrumApproveHandler`, `CerebrumRejectHandler` |
| `pkg/dashboard/memory.go` | `MemoryHandler` |
| `pkg/dashboard/anatomy.go` | `AnatomyHandler` |
| `pkg/dashboard/buglog.go` | `BugLogHandler`, `BugLogDeleteHandler` |
| `pkg/dashboard/suggestions.go` | `SuggestionsHandler`, `SuggestionsDismissHandler` |
| `pkg/dashboard/token.go` | `TokenHandler` |
| `pkg/dashboard/designqc.go` | `DesignQCHandler` |

Each file is ~80–150 LOC. `pkg/dashboard/api.go` (M10b) keeps `OverviewHandler` / `ProjectsHandler` / `ActivityHandler` / `writeJSON` — it does not grow.

### 2.4 Frontend additions

```
web/src/
├── api/
│   ├── cerebrum.ts, memory.ts, anatomy.ts, buglog.ts,
│   ├── suggestions.ts, token.ts, designqc.ts
├── hooks/
│   ├── useActiveProject.ts        (context + URL sync)
├── components/
│   ├── ProjectPicker.tsx          (sidebar <select>)
│   ├── Sparkline.tsx              (~50 LOC inline SVG)
│   └── ConfirmButton.tsx          (inline two-step confirm)
├── panels/
│   ├── Cerebrum.tsx, Memory.tsx, Anatomy.tsx,
│   ├── BugLog.tsx, Suggestions.tsx, Token.tsx, DesignQC.tsx
```

`AppShell` (M10b) is extended to wrap the existing nav block + new project picker + 7 new `NavLink`s. `App.tsx` adds 7 routes inside the existing `AppShell` parent.

## 3. REST endpoints

All read endpoints under `/api/*`, all `GET`, all use `?project=<id>` query param except `/api/memory` (cross-project). Mutation endpoints at top level (`/cerebrum/*`, `/suggestions/dismiss`, `/buglog/delete`) following the M8 convention where mutations live outside the `/api/*` prefix.

### 3.1 Read endpoints (7)

| Path | Returns |
|---|---|
| `GET /api/cerebrum?project=<id>` | `{rules: [{comment, pattern, message}], pending: [{id, trigger_phrase, context, candidate_rule, created_at}]}` |
| `GET /api/memory` | `{rows: [{started_at, turn_count, summary}], raw: "<markdown>"}` |
| `GET /api/anatomy?project=<id>` | `{directories: [{path, files: [{name, description, est_tokens, language}]}], generated_at}` |
| `GET /api/buglog?project=<id>` | `{entries: [{id, created_at, source, file, description, bad_code}]}` |
| `GET /api/suggestions?project=<id>` | `{suggestions: [{id, kind, title, body, created_at}]}` |
| `GET /api/token?project=<id>` | `{totals: <LedgerTotals>, first_recorded, last_updated, history: [{ts, totals}]}` |
| `GET /api/designqc?project=<id>` | `{available: false, reason: "M11 not yet implemented", captures: []}` |

**Project lookup pattern** for per-project endpoints:

```go
projectID := r.URL.Query().Get("project")
if projectID == "" { writeError(w, 400, "missing_project", "project query param required"); return }
projectRoot, err := state.ReadOrigin(projectID)
if err != nil { writeError(w, 404, "not_found", "project not found"); return }
// ... read state files at projectRoot/.mneme/...
```

`/api/anatomy` reuses `state.ReadAnatomy(projectRoot)` (returns `map[string]AnatomyEntry` keyed by path) and groups in-memory by `filepath.Dir(path)`. No new parser needed.

`/api/memory` parses rows from `~/.claude/mneme-memory.md` by splitting on `## ` headers and extracting `started_at` / `turn_count` / `summary`. Returns both `rows` (parsed) and `raw` markdown — frontend prefers `rows`, falls back to `<pre>{raw}</pre>` if parsing yields no rows.

`/api/token` reads ledger via `state.ReadLedger(projectRoot)` and history via `state.ReadLedgerHistory(projectRoot)` (M9). History is capped to the last 12 snapshots in the response (Token panel's sparkline width).

### 3.2 Mutation endpoints (4)

| Path | Body | Effect |
|---|---|---|
| `POST /cerebrum/approve` | `{project_id, candidate_id}` | Resolves root via `state.ReadOrigin`. Reads pending via `cerebrum.LoadPending`. Writes rule via `state.AppendCerebrumRule`. Removes from pending via `cerebrum.RemovePending`. Publishes `cerebrum.approved`. |
| `POST /cerebrum/reject` | `{project_id, candidate_id}` | Resolves root. Appends to rejected via `cerebrum.AppendRejected` (TTL 30d). Removes from pending. Publishes `cerebrum.rejected`. |
| `POST /suggestions/dismiss` | `{project_id, suggestion_id}` | Resolves root. Calls `suggestions.AppendDismissed(root, id, time.Now())`. Publishes `suggestion.dismissed`. |
| `POST /buglog/delete` | `{project_id, entry_id}` | Resolves root. Reads entries, filters out matching id, writes back via `state.WriteBuglog`. Publishes `buglog.deleted`. |

All mutations:

- Return `{status: "approved|rejected|dismissed|deleted"}` on success.
- Return 404 if `project_id` is unknown, 400 if the candidate/entry/suggestion id is unknown, 500 on file write errors.
- Use existing `daemon.AuthMW` (cookie or query token) and `daemon.BodyLimitMW`.

### 3.3 New event types published to the bus

| Type | When | Data |
|---|---|---|
| `cerebrum.approved` | candidate approved via dashboard | `{candidate_id, rule_pattern}` |
| `cerebrum.rejected` | candidate rejected via dashboard | `{candidate_id}` |
| `suggestion.dismissed` | suggestion dismissed via dashboard | `{suggestion_id}` |
| `buglog.deleted` | buglog entry deleted via dashboard | `{entry_id}` |

These events feed into the Activity panel automatically (it subscribes to all types). They also let other open browser tabs SSE-refetch their data when one tab mutates.

## 4. Frontend

### 4.1 Routes (added to `App.tsx`)

| Path | Element |
|---|---|
| `/cerebrum` | `<AppShell><Cerebrum/></AppShell>` |
| `/memory` | `<AppShell><Memory/></AppShell>` |
| `/anatomy` | `<AppShell><Anatomy/></AppShell>` |
| `/buglog` | `<AppShell><BugLog/></AppShell>` |
| `/suggestions` | `<AppShell><Suggestions/></AppShell>` |
| `/token` | `<AppShell><Token/></AppShell>` |
| `/designqc` | `<AppShell><DesignQC/></AppShell>` |

All routes preserve `?project=<id>` from the URL. Sidebar `NavLink`s append the active project ID at click time (memoised from `useActiveProject`).

### 4.2 Panel sketches

**Cerebrum:** two sections.

- *Active rules* — list rendered from `data.rules`. Each entry: comment (gray), pattern (mono), arrow, message.
- *Pending review* — card list rendered from `data.pending`. Each card: trigger phrase quote, context excerpt (gray), proposed rule (mono), `[Approve]` `[Reject]` (`ConfirmButton`).
- SSE: refetch on `cerebrum.candidate`, `cerebrum.approved`, `cerebrum.rejected`. Polling fallback: 30 s.

**Memory:** vertical timeline of rows. Each row: timestamp (left), turn-count badge (small pill), summary (right). Project picker grayed-out banner: "Memory is cross-project." Polling fallback: 60 s.

**Anatomy:** collapsible directory tree. Top has a search box that filters file names live (client-side). Each file row: name (mono), token-estimate badge, description text. SSE: refetch on `scan.complete`. Polling fallback: 60 s.

**BugLog:** table with five columns (created_at / source / file / description / actions). Row click expands to show `bad_code` (preformatted block). `[Delete]` button per row uses `ConfirmButton` (two-click confirm). SSE: refetch on `buglog.deleted`. Polling fallback: 30 s.

**Suggestions:** card list. Each card: kind badge, title, body, `[Dismiss]` (`ConfirmButton`). SSE: refetch on `suggestion.new`, `suggestion.dismissed`. Polling fallback: 30 s.

**Token:** three sections.

- *Totals* — card with hook fires (per type as small bars), anatomy hits, repeat reads, scan count.
- *Trend* — `<Sparkline data={history.map(h => sumOfHookFires(h.totals))} />` plus a small "last N snapshots" label.
- *Hook breakdown* — text list: `pre-write: 12,345`, `post-write: 9,876`, etc.
- SSE: refetch on `hook.fired` (debounced 1 s) and `cron.tick`. Polling fallback: 10 s.

**DesignQC:** empty-state card with two paragraphs and a docs link:

> Design QC is part of M11. Once `mneme designqc` runs, captures and the latest report appear here for the active project.

When `/api/designqc` returns `{available: true, ...}`, the panel renders the captures grid. M11 ships that path; M10c only ships the empty state.

### 4.3 Inline SVG `<Sparkline/>` component

```tsx
export function Sparkline({data, width=120, height=30, color='#2563eb'}: {
  data: number[]; width?: number; height?: number; color?: string
}): JSX.Element {
  if (data.length < 2) return <span style={{display:'inline-block', width, height}} />
  const max = Math.max(...data), min = Math.min(...data)
  const range = max - min || 1
  const stepX = width / (data.length - 1)
  const points = data
    .map((v, i) => `${i*stepX},${height - ((v - min) / range) * height}`)
    .join(' ')
  return (
    <svg width={width} height={height} className="overflow-visible">
      <polyline points={points} fill="none" stroke={color} strokeWidth={1.5} />
    </svg>
  )
}
```

That covers M10c's chart needs. M11 may upgrade if it adds chart-heavy panels.

### 4.4 `<ConfirmButton/>` component

```tsx
export function ConfirmButton({onConfirm, label, confirmLabel='Confirm?'}: {
  onConfirm: () => Promise<void> | void
  label: string
  confirmLabel?: string
}): JSX.Element {
  const [armed, setArmed] = useState(false)
  const [busy, setBusy] = useState(false)
  // First click arms; second click executes; auto-disarms after 5 s.
}
```

Used by Cerebrum (approve / reject), BugLog (delete), Suggestions (dismiss). One component, three call sites.

### 4.5 No new frontend dependencies

All work uses the M10b stack: `react`, `react-dom`, `react-router-dom`, `tailwindcss`. The sparkline is custom SVG. Memory rows are plain text. Confirm UI is a single component, not a modal library.

## 5. Files added or modified

### Added

- `pkg/dashboard/{cerebrum,memory,anatomy,buglog,suggestions,token,designqc}.go`
- `tests/dashboard_{cerebrum,memory,anatomy,buglog,suggestions,token,designqc}_test.go`
- `web/src/api/{cerebrum,memory,anatomy,buglog,suggestions,token,designqc}.ts`
- `web/src/hooks/useActiveProject.ts`
- `web/src/components/{ProjectPicker,Sparkline,ConfirmButton}.tsx`
- `web/src/panels/{Cerebrum,Memory,Anatomy,BugLog,Suggestions,Token,DesignQC}.tsx`
- `web/src/__tests__/ProjectPicker.test.tsx`

### Modified

- `pkg/daemon/routes.go` — register 7 read + 4 mutation endpoints.
- `pkg/dashboard/api.go` — no logic changes; only adjacent file imports if shared types move.
- `web/src/App.tsx` — wrap router in `ActiveProjectProvider`; add 7 routes.
- `web/src/components/AppShell.tsx` — add `<ProjectPicker/>` and 7 new `NavLink`s.
- `web/src/api/types.ts` — add response shapes for the 7 new endpoints.
- `web/src/__tests__/App.test.tsx` — extend smoke test to assert sidebar contains all 10 links.
- `tests/integration/dashboard_panels_test.go` — add `/api/cerebrum`, `/api/anatomy` smoke checks.

## 6. Testing

### Go

| File | What |
|---|---|
| `tests/dashboard_cerebrum_test.go` | GET returns rules + pending; approve appends rule + removes from pending; reject appends to rejected list + removes from pending; missing project → 404 |
| `tests/dashboard_memory_test.go` | GET parses rows from a fixture `~/.claude/mneme-memory.md`; raw field present; empty file → `{rows: [], raw: ""}` |
| `tests/dashboard_anatomy_test.go` | GET groups entries by directory; empty anatomy → `{directories: []}`; missing project → 404 |
| `tests/dashboard_buglog_test.go` | GET returns entries; delete by id removes from `buglog.json`; delete unknown id → 404 |
| `tests/dashboard_suggestions_test.go` | GET filters dismissed; dismiss writes `dismissed.json` record; dismissed suggestions don't appear on next GET |
| `tests/dashboard_token_test.go` | GET returns totals + history (history seeded via `state.AppendLedgerHistory`) |
| `tests/dashboard_designqc_test.go` | GET returns `{available: false, reason: "...", captures: []}` |

### Frontend

- `web/src/__tests__/App.test.tsx` (extended): mock `/api/projects` to return two projects; assert sidebar contains "Cerebrum", "Memory", "Anatomy", "BugLog", "Suggestions", "Token", "DesignQC" links.
- `web/src/__tests__/ProjectPicker.test.tsx` (new): change selection → URL contains `?project=<id>`.

### Integration

- `tests/integration/dashboard_panels_test.go` (extended in M10c): add hits for `/api/cerebrum?project=<seeded>` and `/api/anatomy?project=<seeded>`. Smoke-checks 200 + JSON shape.

## 7. Acceptance criteria

After M10c ships:

1. Sidebar shows a project picker `<select>`. Switching updates `?project=<id>` in the URL; deep links land on the selected project.
2. **Cerebrum** lists active rules + pending candidates. Clicking Approve appends the rule to `.mneme/cerebrum.md` and the candidate disappears within ~500 ms (SSE event drives refetch). Reject also disappears the candidate.
3. **Memory** shows a timeline of recent rows. Project picker is grayed out on this panel.
4. **Anatomy** shows directories with their files; toggling a directory expands/collapses; search box filters live.
5. **BugLog** lists entries; Delete removes an entry from `buglog.json` after inline confirm.
6. **Suggestions** lists open suggestions; Dismiss writes a 30-day TTL record and the row disappears.
7. **Token** shows totals + sparkline of the last 12 history snapshots + hook-fire breakdown.
8. **DesignQC** shows the M11-pending empty state.
9. Mutations in one browser tab propagate to other open tabs within ~500 ms via SSE.
10. `go test ./...`, `go test -tags=integration ./tests/integration/...`, `pnpm tsc --noEmit`, `pnpm test`, `pnpm build` all pass.

## 8. Risks

| Risk | Mitigation |
|---|---|
| Two tabs approve same candidate at once | `state.AppendCerebrumRule` and `cerebrum.RemovePending` already lock on `.mneme/cerebrum.lock`. Loser tab gets 404 from the second-attempt remove; UI shows error inline; SSE reconciles. |
| Project picker is empty (no `~/.mneme/projects/<id>/origin`) | Per-project panels show: "No registered projects yet — run `mneme init` in a project directory." |
| `mneme-memory.md` malformed after user edits | Parser is best-effort. Failed rows are skipped, raw markdown is always returned. Frontend falls back to `<pre>{raw}</pre>` if `rows.length === 0`. |
| Big anatomy files (10k+ files) blow up the response | At expected scale (≤2 MB) we send everything. Profiling can drive pagination later if needed. |
| Sparkline gives wrong picture with sparse data | Component renders empty `<span/>` if `data.length < 2`. Token panel falls back to "Not enough history yet." |
| TTL records (`dismissed.json`, `rejected.json`) accumulate | Existing M9 cleanup: `LoadDismissed(ttlDays, now)` filters at read time. No new cleanup needed. |
| `react-router` route count grows | All routes nested under one `AppShell` parent in `App.tsx`. Adding a route is one entry. |
| DesignQC panel breaks once M11 lands real data | The `available: false` envelope is forward-compatible. M11 flips to `available: true` and adds `captures` + `report` keys; frontend must check `available` before rendering anything. The empty-state code path stays as the default. |

## 9. Out of scope (deferred)

- Real DesignQC integration — M11.
- Search / filter in Suggestions / BugLog — M11+ if needed.
- Bulk operations — never; one row at a time.
- Editing existing data — never; only approve / reject / dismiss / delete.
- Markdown rendering in Memory rows — plain text suffices.
- Pagination — none of the panels hit a scale problem at typical project size.
- Optimistic UI — full panel refetch is fine for single-user.
- Multi-user auth, team mode — roadmap §5 (deferred).
