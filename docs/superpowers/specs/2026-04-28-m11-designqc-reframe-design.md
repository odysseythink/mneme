# M11 — Design QC + Reframe — Design Spec

## Overview

M11 is the final milestone in the M7–M11 roadmap. It ships:

1. A `mneme designqc` CLI that uses headless Chrome (chromedp) to capture full-page JPEGs of a project's routes.
2. The dashboard DesignQC panel — flips the M10c stub from `{available: false}` to a live thumbnail grid with click-to-expand.
3. The Reframe knowledge base — a 12-framework reference template (`reframe.md`) installed into each project's `.mneme/` so Claude can suggest framework choices with grounded tradeoffs.

After M11 ships, the dashboard reaches its full openwolf parity (10 panels, all live), and the M7-M11 roadmap is complete.

## 1. Goals & Non-Goals

### Goals

- **Working `mneme designqc`** that produces `report.json` + JPEG captures for a Vite project's routes within 30 s for a typical 4-route SPA.
- **Dashboard panel goes live.** Replaces the M10c "M11 not yet implemented" empty state with a 3-column thumbnail grid and click-to-expand overlay.
- **Universal escape hatch via `--route`.** Any framework, any project: `mneme designqc --route /path` captures exactly that route, framework detection skipped.
- **Clear errors instead of crashes.** No Chrome → exit 2 with install instructions. No dev server → exit 1 with "run pnpm dev". No routes auto-detected → print one-line note.
- **Reframe template installed by `mneme init` / `mneme update`** with all 12 frameworks.
- **Single new Go dep**: `github.com/chromedp/chromedp`. No new frontend deps.
- **Backwards compatible.** No M0–M10c behavior changes.

### Non-goals

- **Run history.** Each run overwrites the previous report. No `runs/<ts>/` directory in M11.
- **Auto-start dev server.** Decided in brainstorming Q2: user must run `pnpm dev` themselves.
- **Framework auto-detection beyond Vite.** Decided in brainstorming Q3: just Vite. Other frameworks use `--route`.
- **Side-by-side compare in the panel.** Decided in brainstorming Q4: grid + click-to-expand only.
- **AI design critique.** Per roadmap §M11 non-goal: visual analysis is left to Claude consuming the captures.
- **Pixel-diff between runs.** Out of scope; would require run history.
- **Mobile viewport captures.** Single 1440-wide viewport. Multi-viewport is a clean follow-up.
- **Reframe dashboard panel.** Reframe content is for Claude, not the human — no panel.
- **`designqc.complete` SSE event.** Panel uses 30 s polling fallback.

## 2. Architecture

### 2.1 Component diagram

```
mneme designqc [--route /path] [--quality 80] [--max-width 1440]
                         │
                         ▼
┌────────────────────────────────────────────────────────────┐
│  cmd/cmd_designqc.go                                       │
│   - parse flags                                            │
│   - resolve project root + project_id                      │
│   - call pkg/designqc.Run(opts)                            │
└────────────────────────┬───────────────────────────────────┘
                         │
                         ▼
┌────────────────────────────────────────────────────────────┐
│  pkg/designqc/runner.go                                    │
│   1. detect Chrome → fail fast on missing                  │
│   2. detect framework (Vite only)                          │
│   3. probe dev-server port; bail if down                   │
│   4. enumerate routes (or use --route override)            │
│   5. wipe ~/.mneme/designqc/<id>/                          │
│   6. capture each route → write JPEG                       │
│   7. write report.json                                     │
└──────┬──────────┬──────────────┬───────────────────────────┘
       │          │              │
       ▼          ▼              ▼
   detect.go   capture.go    report.go
   (vite)      (chromedp)    (R/W ~/.mneme/designqc/<id>/)

┌────────────────────────────────────────────────────────────┐
│  pkg/dashboard/designqc.go (M10c stub → live)              │
│   - GET /api/designqc → reads report.json                  │
│   - GET /api/designqc/captures/<id>/<file> → JPEG bytes    │
└────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────┐
│  web/src/panels/DesignQC.tsx (M10c empty state → live)     │
│   - thumbnail grid                                         │
│   - click thumbnail → full-screen overlay                  │
└────────────────────────────────────────────────────────────┘
```

**Reframe knowledge base** is independent of the capture pipeline. It is a static template installed into `.mneme/reframe.md` by `mneme init` / `mneme update`. No Go code path reads it; Claude reads it from the file system at agent time. No dashboard panel.

```
pkg/installer/templates/reframe.md.tmpl   ← 12-framework content
pkg/installer/reframe.go                  ← installs into .mneme/reframe.md
```

### 2.2 Package boundaries

| File | One responsibility |
|---|---|
| `pkg/designqc/detect.go` | Vite detection from `vite.config.{ts,js,mjs}`. Returns `{Framework, DefaultPort}` or error. |
| `pkg/designqc/routes.go` | Given project root + `--route` overrides, return `[]Route`. Best-effort regex over `*router*.{ts,tsx,js,jsx}` files; fallback to `[/]`. |
| `pkg/designqc/capture.go` | Given URL + options, return JPEG bytes. Wraps chromedp; handles Chrome-not-found. |
| `pkg/designqc/runner.go` | Orchestrates the full flow. Public `Run(ctx, opts) (*Report, error)`. |
| `pkg/designqc/report.go` | `Report` struct + `WriteReport` / `ReadReport` helpers. |
| `cmd/cmd_designqc.go` | Flag parsing + invocation. ~80 LOC. |
| `pkg/dashboard/designqc.go` | Replaces M10c stub. Reads report.json + serves capture bytes. |
| `pkg/installer/reframe.go` | Installs `reframe.md` template via existing patterns. |

### 2.3 Storage layout

```
~/.mneme/designqc/<project-id>/
├── captures/
│   ├── route-root.jpg
│   ├── route-about.jpg
│   └── route-pricing.jpg
└── report.json
```

Each `mneme designqc` run **overwrites** `report.json` and wipes `captures/`. Single-run-only state in M11.

`report.json` shape:

```json
{
  "version": 1,
  "captured_at": "2026-04-28T10:00:00Z",
  "framework": "vite",
  "base_url": "http://localhost:5173",
  "captures": [
    {
      "route": "/",
      "file": "route-root.jpg",
      "width": 1440,
      "height": 2400,
      "captured_at_ms": 1714290000000
    }
  ]
}
```

### 2.4 Dependencies

| Package | Role |
|---|---|
| `github.com/chromedp/chromedp` | Headless Chrome via CDP (NEW). Pure Go, no Node driver. GitHub Actions ships Chrome by default. |
| Standard library: `image/jpeg`, `os/exec`, `net/http`, `path/filepath` | Port probe + JPEG handling + filesystem layout |

Frontend: **no new deps.** The DesignQC panel uses M10c primitives (`useFetch`, `useActiveProject`, plain `<img>` tags with cache-busting query strings).

## 3. Capture pipeline

### 3.1 Vite detection

```go
type Framework struct {
    Name        string // "vite"
    DefaultPort int    // 5173
    BaseURL     string // built from --port or DefaultPort
}

func DetectFramework(projectRoot string) (*Framework, error)
```

Rules:

1. Check `vite.config.ts`, `vite.config.js`, `vite.config.mjs` in project root. If absent → return `errors.New("no Vite config found; pass --route /path to skip auto-detection")`.
2. Default port 5173.
3. If config has a literal `port: NNNN` line, extract via regex (`port:\s*(\d+)`); else default. Override with `--port` CLI flag.

Future framework support is closed for M11. Adding Next.js / Astro / SvelteKit means implementing a sibling `Detect<Name>()` and registering it in `runner.go` — no architectural change.

### 3.2 Route enumeration

```go
type Route struct {
    Path string // "/", "/about"
    Slug string // "root", "about" — used for filenames
}

func EnumerateRoutes(projectRoot string, override []string) ([]Route, error)
```

- If `override` is non-empty, use exactly those paths. Override is the universal escape hatch.
- Otherwise, scan `src/` for files matching `*router*.{ts,tsx,js,jsx}` and extract any `path: "..."` literals via tolerant regex `path:\s*['"]([^'"]+)['"]`.
- Deduplicate; sort.
- If zero results, return `[{Path: "/", Slug: "root"}]` and print a one-line note to stderr: `"no routes auto-detected; use --route to specify"`.

Slug derivation: strip leading `/`, replace `/` with `-`, drop non-alphanumerics. `/` → `root`, `/about` → `about`, `/users/:id` → `users-id`.

### 3.3 Dev-server probe

```go
func WaitForDevServer(baseURL string, timeout time.Duration) error
```

- Single HTTP GET against `baseURL + "/"` with 1 s timeout.
- Any HTTP response (200/300/4xx/5xx) → server reachable, return nil.
- Connection refused / timeout → return `errors.New("dev server not reachable at <baseURL>; run \"pnpm dev\" in another terminal first")`.
- No retry loop. Fail fast with a clear message.

### 3.4 Capture (chromedp)

```go
type CaptureOptions struct {
    URL       string
    Quality   int           // 1-100, default 80
    MaxWidth  int           // pixels, default 1440
    Timeout   time.Duration // per-route, default 30 s
}

func Capture(parentCtx context.Context, opts CaptureOptions) (jpeg []byte, w int, h int, err error)
```

A single `chromedp.ExecAllocator` + `chromedp.Context` is created **once** by the runner and reused across every route — saves ~1 s of Chrome launch per route. The runner passes that long-lived context as `parentCtx` to `Capture`; `Capture` derives a per-call timeout context but does not allocate a new browser.

```go
ctx, cancel := context.WithTimeout(parentCtx, opts.Timeout)
defer cancel()

var buf []byte
var dims [2]int
err := chromedp.Run(ctx,
    chromedp.Navigate(opts.URL),
    chromedp.WaitReady("body", chromedp.ByQuery),
    chromedp.Sleep(500*time.Millisecond),
    chromedp.FullScreenshot(&buf, opts.Quality),
    chromedp.Evaluate(
        `[document.documentElement.scrollWidth, document.documentElement.scrollHeight]`,
        &dims,
    ),
)
```

`chromedp.FullScreenshot` emits JPEG when `Quality > 0`. We pass through.

### 3.5 Chrome detection + actionable error

```go
func DetectChrome() (string, error)
```

Search order:

1. `$CHROME_PATH` (if set; `os.Stat` validates).
2. macOS: `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, then Chromium variants.
3. Linux: `google-chrome`, `chromium`, `chromium-browser` on `$PATH`.

If no Chrome is found:

```
Chrome/Chromium not found. Install via:
  macOS:  brew install --cask google-chrome
  Linux:  apt-get install chromium-browser
Or set CHROME_PATH=/path/to/chrome
```

Exits with code 2. No state files written.

### 3.6 Runner orchestration

```go
type RunOptions struct {
    ProjectRoot   string
    ProjectID     string
    HomeDir       string
    RouteOverride []string
    Quality       int    // 0 = default 80
    MaxWidth      int    // 0 = default 1440
    Port          int    // 0 = framework default
}

func Run(ctx context.Context, opts RunOptions) (*Report, error)
```

Flow:

1. `DetectChrome()` → fail fast (exit 2 in CLI).
2. `DetectFramework(opts.ProjectRoot)` → `*Framework`. (Skipped if `RouteOverride` non-empty AND `--port` provided — pure manual mode.)
3. `WaitForDevServer(framework.BaseURL, 1*time.Second)` → fail fast (exit 1).
4. `EnumerateRoutes(opts.ProjectRoot, opts.RouteOverride)` → `[]Route`.
5. **Wipe** `~/.mneme/designqc/<id>/captures/`; recreate as a clean directory.
6. Initialize a single `chromedp.ExecAllocator` + `chromedp.Context` for this run.
7. For each route (sequential):
   - `Capture(ctx, ...)` → JPEG bytes, dimensions.
   - On success: write to `~/.mneme/designqc/<id>/captures/route-<slug>.jpg` (mode 0o600); append `Capture` to in-progress report.
   - On failure: log warning to stderr; append a `Capture` entry with `Error: "<msg>"` to the report; continue.
8. Tear down the chromedp context.
9. Write `report.json` via `state.AtomicWrite`.
10. Return `*Report`.

### 3.7 Errors & partial-failure policy

- `DetectChrome` fails → exit 2 with install instructions. **No** state files written.
- `DetectFramework` or `WaitForDevServer` fails → exit 1 with the specific error. **No** state files written.
- Single `Capture` fails → log warning, mark route as `error: "<msg>"` in report, continue.
- After the loop:
  - At least one route succeeded → write `report.json`, exit 0.
  - All routes failed → write `report.json` (with all errors), exit 1.

## 4. CLI

```
mneme designqc [flags]

Flags:
  --route <path>       Capture only this route (repeatable). Skips auto-detection.
  --port <N>           Override dev server port (default: framework default).
  --quality <N>        JPEG quality 1-100 (default 80).
  --max-width <N>      Browser viewport width (default 1440).
  --json               Print final report.json to stdout instead of human summary.

Exit codes:
  0  success (report written, at least one capture succeeded)
  1  recoverable error (dev server down, no routes found, all captures failed)
  2  Chrome/Chromium not installed
```

**Standard output (human mode):**

```
mneme designqc — vite project at /Users/foo/work/site
captures: 4 routes
  /          → captures/route-root.jpg (1440×2400)
  /about     → captures/route-about.jpg (1440×1800)
  /pricing   → captures/route-pricing.jpg (1440×2200)
  /contact   → captures/route-contact.jpg (1440×1900)
report:    ~/.mneme/designqc/<project-id>/report.json
```

`--route` is repeatable. When passed, framework detection still runs for base URL + port (unless `--port` is also passed, fully manual mode).

`--json` prints the report verbatim — easy to pipe.

## 5. Dashboard backend

### 5.1 Replace M10c stub at `pkg/dashboard/designqc.go`

```go
type DesignQCResponse struct {
    Available bool    `json:"available"`
    Reason    string  `json:"reason,omitempty"`
    Report    *Report `json:"report,omitempty"`
}

func DesignQCHandler() http.HandlerFunc {
    // resolveProject (M10c helper) → project_id
    // Read ~/.mneme/designqc/<id>/report.json
    // Missing: {available: false, reason: "no captures yet — run mneme designqc"}
    // Present + version=1: {available: true, report: <parsed>}
    // Present + unknown version: {available: false, reason: "report version unsupported"}
}
```

### 5.2 New endpoint: `GET /api/designqc/captures/<project-id>/<file>`

Serves JPEG bytes from `~/.mneme/designqc/<project-id>/captures/<file>`.

- `<file>` must match `^route-[a-z0-9-]+\.jpg$`. Reject anything else with 404 (path traversal guard).
- Returns 200 + `Content-Type: image/jpeg` + `Cache-Control: no-store`.
- 404 if the file doesn't exist.
- Auth via existing `AuthMW`.

## 6. Frontend DesignQC panel

Replace `web/src/panels/DesignQC.tsx`:

```tsx
import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { getDesignQC } from '../api/designqc'
import { useActiveProject } from '../hooks/useActiveProject'

export function DesignQC(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getDesignQC(active) : () => Promise.resolve(null as never)
  const { data } = useFetch(fn, { intervalMs: 30_000 })
  const [expanded, setExpanded] = useState<string | null>(null)

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  if (!data.available) {
    return (
      <div>
        <h1 className="text-2xl font-semibold mb-4">design qc</h1>
        <div className="rounded border bg-white p-4 max-w-2xl">
          <p className="font-medium mb-2">No captures yet.</p>
          <p className="text-sm text-gray-700">
            Run <code className="bg-gray-100 px-1 rounded">mneme designqc</code> in this project to generate captures.
          </p>
        </div>
      </div>
    )
  }

  const r = data.report!
  return (
    <div>
      <h1 className="text-2xl font-semibold mb-2">design qc</h1>
      <div className="text-sm text-gray-600 mb-4">
        {r.framework} · {r.base_url} · captured {r.captured_at}
      </div>
      <div className="grid grid-cols-3 gap-4">
        {r.captures.map(c => (
          <button
            key={c.route}
            onClick={() => setExpanded(c.file)}
            className="rounded border bg-white p-2 hover:shadow text-left"
          >
            <img
              src={`/api/designqc/captures/${active}/${c.file}`}
              className="w-full h-32 object-cover object-top rounded"
              alt={c.route}
            />
            <div className="mt-2 text-sm font-mono">{c.route}</div>
            <div className="text-xs text-gray-500">{c.width}×{c.height}</div>
          </button>
        ))}
      </div>
      {expanded && (
        <div
          onClick={() => setExpanded(null)}
          className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 cursor-pointer"
        >
          <img
            src={`/api/designqc/captures/${active}/${expanded}`}
            className="max-w-[95vw] max-h-[95vh] object-contain"
            alt={expanded}
          />
        </div>
      )}
    </div>
  )
}
```

Refresh model: 30 s polling fallback. No SSE event from the runner in M11 (clean follow-up if needed).

## 7. Reframe knowledge base

### 7.1 Installer wiring (mirrors `pkg/installer/identity.go` pattern)

```go
// pkg/installer/reframe.go
func InstallReframe(projectRoot string) error {
    src := mustReadTemplate("reframe.md.tmpl") // bundled via embed.FS
    dst := filepath.Join(projectRoot, ".mneme", "reframe.md")
    return state.AtomicWrite(dst, src)
}
```

Called from:

- `pkg/installer/project.go` (during `mneme init`)
- `pkg/updater/updater.go` (during per-project `mneme update`)

User customizations are preserved between `<!-- mneme:user-section -->` fences (same convention as `rules.md`, `identity.md`).

### 7.2 Template content (`pkg/installer/templates/reframe.md.tmpl`)

12 frameworks, each with the same shape:

```markdown
## <name>

**When to suggest:** <one paragraph: project shape, tradeoffs match>
**When NOT to suggest:** <one paragraph: anti-patterns, where it falls short>
**Migration cost from <neighboring framework>:** <low | medium | high — one line>
**Docs:** <official URL>
```

Frameworks:

1. Next.js
2. Vite (raw React / Solid / Vue)
3. Astro
4. SvelteKit
5. Remix
6. Nuxt
7. Solid Start
8. Qwik / Qwik City
9. Fresh (Deno)
10. Gatsby
11. Ember
12. Angular

Total: ~120 lines of structured markdown. Static; no code paths.

## 8. Files added or modified

### Added

- `pkg/designqc/{detect,routes,capture,runner,report}.go`
- `tests/designqc_{detect,routes,capture,report,runner}_test.go`
- `tests/dashboard_designqc_live_test.go`
- `tests/integration/designqc_e2e_test.go` (`//go:build integration`)
- `cmd/cmd_designqc.go`
- `pkg/installer/reframe.go`
- `pkg/installer/templates/reframe.md.tmpl`

### Modified

- `cmd/main.go` — register `designqc` subcommand.
- `pkg/installer/project.go` — call `InstallReframe(root)` in init flow.
- `pkg/updater/updater.go` — call `InstallReframe(root)` in per-project sync.
- `pkg/dashboard/designqc.go` — replace M10c stub with live handler + `/api/designqc/captures/` route.
- `pkg/daemon/routes.go` — register `/api/designqc/captures/` route.
- `web/src/panels/DesignQC.tsx` — replace M10c empty-state with live grid.
- `web/src/api/types.ts` — extend `DesignQCResponse` with the live shape (`Report` + `Capture`).
- `go.mod` / `go.sum` — add `github.com/chromedp/chromedp`.

## 9. Testing

### Go unit

| File | What |
|---|---|
| `tests/designqc_detect_test.go` | Vite config detection across `.ts` / `.js` / `.mjs`; missing config returns clear error; literal port extracted from config; non-literal port falls back to default. |
| `tests/designqc_routes_test.go` | Regex extracts `path:"/about"` from a `*router*.ts` fixture; `--route` override skips enumeration; empty result falls back to `[{Path:"/", Slug:"root"}]`; slug derivation for nested paths (`/users/:id` → `users-id`). |
| `tests/designqc_report_test.go` | `WriteReport` round-trips via `ReadReport` byte-equal; corrupt JSON returns clear error; unknown version flagged. |
| `tests/designqc_runner_test.go` | Wipes existing captures dir on re-run; partial-failure policy (one route fails → others succeed → exit 0); all-fail → exit 1. (Mocks `Capture` to avoid Chrome dep in this unit test.) |

### Go capture (chromedp + httptest)

| File | What |
|---|---|
| `tests/designqc_capture_test.go` | Serves a fixture HTML page via `httptest.NewServer`; calls `Capture`; asserts JPEG magic bytes (`FF D8 FF`); width matches `MaxWidth` ±2 px. Calls `t.Skip` if `DetectChrome()` errors locally; CI uses GitHub Actions' bundled Chrome. |

### Go dashboard

| File | What |
|---|---|
| `tests/dashboard_designqc_live_test.go` | `available: false` when `report.json` absent; after seeding a fake report, `available: true` with the captures slice; `/api/designqc/captures/<id>/route-root.jpg` returns the bytes; path traversal (`../etc/passwd`) → 404; unknown report version → `{available: false, reason: ...}`. |

### Integration (`//go:build integration`)

| File | What |
|---|---|
| `tests/integration/designqc_e2e_test.go` | Starts an `httptest.NewServer` serving a tiny single-page Vite-style HTML; spawns `mneme designqc --route /` against it; asserts `report.json` exists with at least one capture and `version: 1`. |

### Frontend

- Update existing `web/src/__tests__/App.test.tsx` to mock `/api/designqc` returning `available: false` and assert the empty-state copy is rendered when navigating to `/designqc`. (No new test file.)

## 10. Acceptance criteria

After M11 ships:

1. `mneme designqc` in a Vite project with `pnpm dev` running produces `~/.mneme/designqc/<id>/report.json` plus one JPEG per route within 30 s for a 4-route SPA.
2. `mneme designqc --route /pricing` works in *any* project (Vite or not), capturing exactly that one route.
3. Without Chrome installed, `mneme designqc` exits 2 with the actionable install message; **no** state files written.
4. Without dev server running, `mneme designqc` exits 1 with the "run pnpm dev" message; **no** state files written.
5. Dashboard DesignQC panel:
   - Shows the empty-state card before any run.
   - Shows a 3-column thumbnail grid after a run; each thumbnail labeled with route + dimensions.
   - Click expands to a full-screen overlay; click anywhere closes it.
   - Refreshes within 30 s of a re-run (polling fallback).
6. `mneme init` (or `mneme update`) in a fresh project produces `.mneme/reframe.md` containing all 12 framework sections.
7. `go test ./...`, `go test -tags=integration ./tests/integration/...`, `pnpm tsc --noEmit`, `pnpm test`, `pnpm build` all pass.

## 11. Risks

| Risk | Mitigation |
|---|---|
| Chrome / Chromium not installed | `DetectChrome()` runs first; fail fast with actionable install instructions; exit 2 (not panic). |
| Vite config has dynamic port (`port: process.env.PORT`) | Regex extraction skips non-literals; falls back to default 5173. `--port N` flag is the manual override. |
| Route enumeration misses real routes (regex limits) | Output is best-effort. `--route` is the universal escape hatch. Print one-line note when regex finds zero results. |
| Path traversal via `/api/designqc/captures/<file>` | Filename validated against `^route-[a-z0-9-]+\.jpg$` before serving; anything else 404. |
| Single chromedp context leaks state across routes (cookies, localStorage) | Acceptable in M11 (each route is a clean navigation). For session-dependent flows, run `mneme designqc --route /one` per call. |
| Capture exceeds 30 s timeout (slow page) | `--timeout N` is a clean follow-up; per-route timeout is constant in M11. |
| `report.json` schema changes break the panel | Stub returns `{available: false}` either way; live viewer reads `version` field; unknown versions surface as `{available: false, reason: "report version unsupported"}`. |
| Wiping `captures/` on every run loses ad-hoc edits | Documented in `--help`. Users who want to preserve a run can copy the directory aside. |
| chromedp/Chromium ABI changes break tests | Version-pin chromedp in `go.mod`. CI uses GitHub Actions' default Chrome; track upstream bumps via Dependabot. |
