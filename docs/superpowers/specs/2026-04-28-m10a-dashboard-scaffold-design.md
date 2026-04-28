# M10a — Dashboard Backend & Scaffold — Design Spec

## Overview

M10a is the first of three sub-milestones that together deliver M10 (Web Dashboard). Roadmap §4 flags M10/M11 as ~50% of total scope; the three-way decomposition lets each ship working software:

- **M10a (this spec)** — backend route plumbing, embed pipeline, `mneme dashboard` CLI, build system, one tiny "Hello mneme" placeholder page that exercises the full pipeline (embed → daemon serve → browser load → token-cookie auth → API round trip).
- **M10b (later spec)** — `pkg/dashboard/api.go`, `pkg/dashboard/ws.go`, plus the three highest-value panels: Overview, Activity, Cron.
- **M10c (later spec)** — the remaining seven panels (Token, Cerebrum, Memory, Anatomy, BugLog, Suggestions, DesignQC).

M10a is intentionally narrow: prove the distribution and auth pipeline works, leave the actual UI for M10b. After M10a ships, `go build ./cmd && mneme dashboard` opens a browser tab that says "Backend is up — UI not built yet" with live daemon health beneath it.

## 1. Goals & Non-Goals

### Goals

- **Single Go binary.** `go build ./cmd` succeeds on a fresh clone with no Node toolchain. The resulting binary serves a useful (degraded) dashboard.
- **Embed pipeline works.** `//go:embed dist/*` always finds at least one file because the placeholder is committed; `pnpm build` overwrites it with the real React bundle.
- **Token-cookie handshake works** end to end: `mneme dashboard` → URL with `?token=` → browser sets `mneme_token` cookie → subsequent requests authenticated automatically.
- **Both dev modes shipped (Q2-C):** Vite dev server with proxy + `MNEME_DASHBOARD_DEV_DIR` filesystem override.
- **CLI preflight is explicit (Q3-A):** `mneme dashboard` checks Unix socket, TCP listener, and token file; prints actionable errors; does not auto-mutate config.
- **Backwards compatible.** No M0–M9 schema or behavior changes. M8 `AuthMW` extended additively.

### Non-goals

- **No REST `/api/*` handlers.** M10b ships those.
- **No WebSocket fanout.** M10b ships `/ws`.
- **No React panels.** Just a single bootstrap+health placeholder page.
- **No router, no Recharts, no theme system.** All M10b/M10c.
- **No CI changes beyond adding Node setup.** Existing macOS+Linux matrix unchanged.
- **No multi-user auth, no team mode.** Roadmap §5 marks these as deferred.

## 2. Architecture

### 2.1 Component diagram

```
┌─────────────────────────────────────────┐
│  user                                   │
│   1. mneme dashboard                    │
│   2. browser opens http://localhost:18801/?token=<value>
│   3. bootstrap.html sets cookie, redirects to /
│   4. index.html loads, fetches /health │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  cmd/cmd_dashboard.go                   │
│   - dial unix socket /health            │
│   - dial tcp /health                    │
│   - read ~/.mneme/daemon/token          │
│   - exec.Command(opener, url)           │
└────────────┬────────────────────────────┘
             │
             ▼ (browser hits :18801)
┌─────────────────────────────────────────┐
│  pkg/daemon (M8)                        │
│   AuthMW (extended) + dashboard.Mount   │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  pkg/dashboard                          │
│   server.go   route registration        │
│   embed.go    //go:embed dist/* → fs.FS │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│  web/                                   │
│   vite + react + ts + tailwind v4       │
│   src/main.tsx     entry                │
│   src/Bootstrap.tsx  url-token handler  │
│   src/Health.tsx    /health caller      │
│   dist/             gitignored except   │
│                     dist/index.html (placeholder, COMMITTED)
└─────────────────────────────────────────┘
```

### 2.2 Package layout

```
pkg/
└── dashboard/
    ├── embed.go        # //go:embed dist/* → fs.FS
    ├── server.go       # Mount(mux, deps), Deps, file server, dev-dir override
    └── server_test.go  # httptest tests for Mount + bootstrap

cmd/
├── cmd_dashboard.go   # `mneme dashboard` preflight + open browser
└── main.go            # add "dashboard" case

pkg/daemon/
└── middleware.go      # M8 AuthMW extended for cookie + ?token= query

web/
├── package.json
├── pnpm-lock.yaml
├── tsconfig.json
├── vite.config.ts     # proxy /api/*, /ws, /health, / to :18801
├── postcss.config.cjs # (only if Tailwind v4 plugin needs it; v4 prefers vite plugin)
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── Bootstrap.tsx
│   ├── Health.tsx
│   └── styles.css     # @import "tailwindcss"
└── dist/
    ├── index.html     # placeholder, COMMITTED
    └── dist.placeholder.html  # restoration source for `make web-clean`

Makefile               # web-build, web-dev, web-clean, build, go-build
```

### 2.3 Daemon-side wiring

`dashboard.Mount(mux *http.ServeMux, deps Deps)` is called from M8's HTTP server setup when the TCP listener is enabled. It registers:

- `GET /` (and any non-`/api/`, non-`/ws/` path) → file server backed by the embed FS (or `MNEME_DASHBOARD_DEV_DIR` if set). SPA fallback: any 404 from the embed FS returns `index.html` instead so React Router (M10b) works.
- `GET /static/*`, `GET /assets/*` → file server, same backing FS.
- `POST /dev-token` → only registered when `daemon_dev_mode=true`; sets `mneme_token` cookie for any origin so Vite dev (`:5173`) can authenticate.

All routes go through M8's `AuthMW`. The bootstrap is special-cased *inside* `AuthMW`: a `?token=` query param on `GET /` is accepted as authentication for that single request only.

### 2.4 One file = one job

- `embed.go` — only the embed directive + an `FS()` getter and a small fallback that returns the dev-dir filesystem if `MNEME_DASHBOARD_DEV_DIR` is set.
- `server.go` — only routing + middleware glue.
- `cmd_dashboard.go` — only CLI orchestration.

## 3. Auth flow & token bootstrap

### 3.1 Step-by-step

```
1. user runs:    mneme dashboard
2. CLI reads:    ~/.mneme/daemon/token  →  T (32 bytes hex)
3. CLI opens:    http://localhost:18801/?token=T
4. browser GET:  /?token=T               (no cookie yet)
                 → AuthMW accepts query token (only on GET /)
                 → daemon serves bootstrap dist/index.html
5. bootstrap.js: document.cookie = "mneme_token=T; SameSite=Strict; Path=/; Max-Age=86400"
                 history.replaceState(null, '', '/')   // drops ?token= from URL bar
6. browser GET:  /                       (cookie now present)
                 → AuthMW accepts cookie → serves index.html (same dist file)
7. index.html JS: fetch('/health', {credentials: 'same-origin'})
                  → AuthMW accepts cookie → 200 OK with daemon health
```

### 3.2 AuthMW changes (M8 → M10a additive)

```go
func AuthMW(token string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Bearer header (existing M8)
        if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
            if subtle.ConstantTimeCompare(
                []byte(strings.TrimPrefix(h, "Bearer ")), []byte(token)) == 1 {
                next.ServeHTTP(w, r); return
            }
        }
        // 2. Cookie (NEW)
        if c, err := r.Cookie("mneme_token"); err == nil {
            if subtle.ConstantTimeCompare([]byte(c.Value), []byte(token)) == 1 {
                next.ServeHTTP(w, r); return
            }
        }
        // 3. Query token (NEW; only on GET / for bootstrap)
        if r.Method == "GET" && r.URL.Path == "/" && r.URL.Query().Get("token") != "" {
            if subtle.ConstantTimeCompare(
                []byte(r.URL.Query().Get("token")), []byte(token)) == 1 {
                next.ServeHTTP(w, r); return
            }
        }
        http.Error(w, "unauthorized", http.StatusUnauthorized)
    })
}
```

### 3.3 Bootstrap inline script

In `dist/index.html` head (kept by Vite during the React build because we whitelist inline scripts in `vite.config.ts`):

```html
<script>
  (function () {
    var p = new URLSearchParams(location.search);
    var t = p.get('token');
    if (t) {
      document.cookie = 'mneme_token=' + encodeURIComponent(t)
        + '; SameSite=Strict; Path=/; Max-Age=86400';
      history.replaceState(null, '', '/');
    }
  })();
</script>
```

The bootstrap is idempotent: repeated visits with `?token=T` just re-set the same cookie value. Without `?token=` it does nothing.

### 3.4 Threat model — what we cover, what we accept

| Risk | Mitigation |
|---|---|
| Token leaks via Referer header | `Referrer-Policy: no-referrer` on bootstrap response |
| Token persists in browser history | `history.replaceState` strips `?token=` immediately |
| Stale cookie after daemon restart | Daemon writes a fresh token per launch; old cookies fail AuthMW; `Max-Age=86400` is a safety net |
| CSRF | `SameSite=Strict` cookie + localhost-only daemon |
| Cross-tab XSS | Out of scope (single-user localhost) |
| Token in initial-load history entry | Acceptable for localhost-only dashboard |

### 3.5 WebSocket compatibility (M10b carry-over)

Cookies set by the bootstrap *will* be carried on the WebSocket upgrade request because it's a same-origin GET. M10b's `/ws` endpoint uses the same `AuthMW` unchanged.

## 4. Build pipeline & dev modes

### 4.1 Makefile

```makefile
.PHONY: build web-build web-dev web-clean go-build

build: web-build go-build

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build

go-build:
	go build -o bin/mneme ./cmd

web-dev:
	cd web && pnpm install --frozen-lockfile && pnpm dev

web-clean:
	rm -rf web/dist web/node_modules
	mkdir -p web/dist
	cp web/dist.placeholder.html web/dist/index.html
```

- `make build` is the full-release path (used in CI and `goreleaser`).
- `go build ./cmd` continues to work without Node because `web/dist/index.html` is committed.
- `make web-clean` resets to the placeholder so subsequent `go build` keeps producing a runnable binary.

### 4.2 Dev mode A — Vite dev server with proxy

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api':    { target: 'http://localhost:18801', changeOrigin: true },
      '/ws':     { target: 'ws://localhost:18801', ws: true },
      '/health': { target: 'http://localhost:18801', changeOrigin: true },
      '^/$':     { target: 'http://localhost:18801', changeOrigin: true },
    },
  },
})
```

Cookie set by the bootstrap on `:18801` does not apply to `:5173` (different origin). Fix: optional `/dev-token` shortcut on the daemon, gated by `daemon_dev_mode=true` config or `MNEME_DAEMON_DEV_MODE=1` env. When enabled, `mneme daemon start --dev` exposes:

```
POST /dev-token
  body: { "token": "..." }
  → Set-Cookie: mneme_token=<token>; SameSite=Lax; Path=/
```

**Cookie domain note:** the Set-Cookie response from `/dev-token` does not specify `Domain=`, so the cookie inherits the request's origin. When the browser reaches `/dev-token` *via the Vite proxy* (origin `:5173`), the cookie is scoped to `:5173` and works for subsequent proxied fetches. When the browser reaches `/dev-token` directly on `:18801`, the cookie scopes to `:18801` and does not help the dev workflow. Therefore the dev workflow is: run `pnpm dev`, then in DevTools console execute `fetch('/dev-token', {method:'POST', body: JSON.stringify({token:'<from-token-file>'})})` once — the request goes through Vite and the resulting cookie scopes to `:5173`.

The dev shortcut also accepts CORS preflight so the one-shot `fetch` from DevTools works regardless of origin headers. Off by default; documented in `web/README.md`.

Manual fallback (for users who don't want to run with `--dev`): paste

```js
document.cookie = 'mneme_token=<read from ~/.mneme/daemon/token>; Path=/'
```

into Vite's DevTools Console once per dev session.

### 4.3 Dev mode B — `MNEME_DASHBOARD_DEV_DIR` filesystem override

`go:embed` snapshots files at *go build* time. To iterate on the bundle without rebuilding the Go binary, set:

```bash
MNEME_DASHBOARD_DEV_DIR=$PWD/web/dist mneme daemon start
```

When the env var is set, `dashboard.Mount` uses `http.FileServer(http.Dir(...))` instead of the embed FS. Off by default; documented in `web/README.md`.

### 4.4 Placeholder `web/dist/index.html`

Committed to the repo. Doubles as bootstrap + degraded fallback. Contents shown in §3.3 plus the page body:

```html
<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>mneme dashboard</title>
  <meta name="referrer" content="no-referrer">
  <script>
    /* bootstrap (see §3.3) */
  </script>
  <style>
    body { font-family: system-ui; max-width: 40rem; margin: 4rem auto; padding: 0 1rem; }
  </style>
</head>
<body>
  <h1>mneme dashboard</h1>
  <p>Backend is up. The web UI has not been built yet.</p>
  <p>Run <code>make web-build</code> from the repo root, then refresh.</p>
  <p>Daemon health: <span id="health">checking…</span></p>
  <script>
    fetch('/health', { credentials: 'same-origin' })
      .then(r => r.json())
      .then(j => document.getElementById('health').textContent =
        'pid=' + j.pid + ' uptime=' + j.uptime_s + 's version=' + j.version)
      .catch(e => document.getElementById('health').textContent = 'error: ' + e);
  </script>
</body>
</html>
```

When Vite builds, it overwrites `dist/index.html` with the React-bundled version. The bundled version keeps the bootstrap inline script and the `<meta name="referrer">` tag (Vite preserves head children unless explicitly removed); React mounts in a `<div id="root">` after.

A copy of the placeholder is kept at `web/dist.placeholder.html` so `make web-clean` can restore it.

### 4.5 CI changes

- macOS + Linux GitHub Actions matrix (already in use).
- Add `actions/setup-node@v4` with Node 20 to every job that calls `make build`.
- pnpm cache via `actions/cache@v4` keyed on `web/pnpm-lock.yaml` SHA.
- Release pipeline (goreleaser) runs `make web-build` *once* (dist is platform-independent) and embeds the resulting bytes into every platform binary.

## 5. CLI: `mneme dashboard`

### 5.1 Behavior

```
mneme dashboard [--no-open]
```

Flow:

1. Read config (`config.FromEnv()`).
2. Dial Unix socket `~/.mneme/daemon/socket` `GET /health` with 200 ms timeout.
   - Failure → print `Daemon not running. Run: mneme daemon start` to stderr; exit 1.
3. Dial TCP `127.0.0.1:<dashboard_port>` `GET /health` with 200 ms timeout.
   - Failure → print `Dashboard TCP listener disabled. Add daemon_dashboard_port_enabled: true to ~/.mneme/config.yaml, then: mneme daemon restart` to stderr; exit 1.
4. Read `~/.mneme/daemon/token`.
   - Missing → print `Token file missing — daemon may have just started. Try again in 1s.`; exit 1.
5. Build URL: `http://localhost:<port>/?token=<token>`.
6. If `--no-open`: print URL to stdout; exit 0.
7. Otherwise: invoke platform opener (`open` on macOS, `xdg-open` on Linux, `start` on Windows). On opener error: print URL to stdout, exit 0 anyway.

### 5.2 Code structure

```go
type DashboardDeps struct {
    UnixSocket  string
    TCPHost     string
    TCPPort     int
    TokenPath   string
    Opener      func(url string) error // injectable for tests
    Stdout      io.Writer
    Stderr      io.Writer
}

func RunDashboard(deps DashboardDeps, args []string) int
```

`cmd_dashboard.go` wires production defaults and calls `RunDashboard`. Tests provide a mock daemon via `httptest.Server` and stub `Opener` to just record the URL.

## 6. Configuration

| Key (yaml) | Env var | Default | Owner | Purpose |
|---|---|---|---|---|
| `daemon_dashboard_port` | `DAEMON_DASHBOARD_PORT` | `18801` | M8 | TCP loopback port |
| `daemon_dashboard_port_enabled` | `DAEMON_DASHBOARD_PORT_ENABLED` | `false` | M8 | Bind TCP listener |
| `daemon_dev_mode` | `DAEMON_DEV_MODE` | `false` | M10a | Enable `/dev-token` shortcut |
| _(env only)_ | `MNEME_DASHBOARD_DEV_DIR` | `""` | M10a | Serve `web/dist/` from FS instead of embed |

The first two are M8 keys that M10a starts to consume. `daemon_dev_mode` is a new key; `MNEME_DASHBOARD_DEV_DIR` is intentionally env-only because it's a developer override that should not appear in normal config files.

## 7. Testing strategy

| Component | Approach |
|---|---|
| `pkg/dashboard/embed.go` | `embed_test.go` asserts `FS()` contains `index.html` and is non-empty. Catches build regressions where `web/dist/` was emptied. |
| `pkg/dashboard/server.go` Mount | `httptest.Server` + the M8 daemon mux composed with `dashboard.Mount`. Verify: GET `/` with `?token=T` returns 200 + bootstrap HTML; GET `/` without cookie returns 401; GET `/` with valid `mneme_token` cookie returns 200; GET `/static/foo.js` returns 200; GET `/api/x?token=T` returns 401 (query token rejected outside `/`). |
| AuthMW changes | Extend M8's `auth_mw_test.go` with three new cases: cookie-only success, query-token-on-`/` success, query-token-rejected-on-`/api/x`. |
| `cmd_dashboard.go` preflight | `httptest`-driven mock daemon; assert exit codes + stderr message for each preflight failure path. Inject `Opener` stub that records the URL. |
| Browser-opener selection | Per-platform tests using `runtime.GOOS` switch; assert correct `exec.Command` args. |
| `MNEME_DASHBOARD_DEV_DIR` | Test that when env is set, file changes in the dev dir are reflected without rebuilding the binary. |
| End-to-end | `tests/integration/dashboard_smoke_test.go` (`//go:build integration`): start daemon with TCP listener enabled, issue HTTP GET to `/?token=T`, follow Set-Cookie via `cookiejar`, GET `/` again, GET `/health` with cookie — all 200. |
| Vite proxy | Not tested in Go. Documented in `web/README.md` with a `curl` smoke check. |

All tests must pass on macOS + Linux GitHub Actions matrix.

## 8. Backwards-compat invariants

- `go build ./cmd` continues to produce a runnable binary without ever installing Node — placeholder dist guarantees this.
- M0–M9 daemon endpoints (`/health`, `/status`, `/scan`, `/cron/*`, `/cerebrum/learn`) keep their semantics. M10a only *adds* routes (`/`, `/static/*`, `/assets/*`, `/dev-token`).
- M8 `AuthMW` extended additively — Bearer header still works.
- No schema changes to anatomy/memory/ledger/cerebrum/buglog. Dashboard reads existing JSON files only (and not even that in M10a — only `/health`).
- `mneme` MCP server entry point unchanged.

## 9. Out of scope

- `pkg/dashboard/api.go` — REST handlers under `/api/*`. M10b.
- `pkg/dashboard/ws.go` — WebSocket fanout. M10b.
- React Router, panel scaffold, Recharts setup. M10b.
- Light/dark theme, accessibility audit. M10c polish.
- The 10 panels themselves. M10b ships 3, M10c ships 7.
- Multi-user / team auth. Roadmap §5 marks deferred.
- Dashboard for non-localhost (remote-dev) users. Out of scope for M10 entirely.

## 10. Open questions

These do not block implementation:

- **Should the placeholder dist call `/health` only, or also `/status`?** Default v1: `/health` only — minimal proof of round-trip; `/status` aggregates more, useful but unnecessary for M10a's "is the pipeline alive" goal.
- **Is `Max-Age=86400` (24 h) the right cookie lifetime?** Default v1: yes — covers a workday; daemon restarts invalidate older tokens regardless.
- **Should the dashboard CLI prompt to add `daemon_dashboard_port_enabled: true` to config interactively?** Default v1: no — we picked Q3-A (explicit error).
- **Tailwind v3 vs v4?** Default v1: v4 with `@tailwindcss/vite` — current stable, no PostCSS config needed.

## 11. Acceptance criteria

- [ ] `go build ./cmd` succeeds on a fresh clone without running pnpm.
- [ ] `make build` succeeds end-to-end (pnpm install + pnpm build + go build) on macOS + Linux.
- [ ] `mneme dashboard` with daemon up + TCP enabled:
  - opens browser to `http://localhost:18801/?token=...`
  - browser shows page rendering current daemon health (`pid=… uptime=… version=…`)
- [ ] `mneme dashboard` with daemon down: prints actionable error referencing `mneme daemon start`; exit code 1.
- [ ] `mneme dashboard` with daemon up but TCP disabled: prints actionable error referencing config edit + `daemon restart`; exit code 1.
- [ ] AuthMW rejects requests with no token, no cookie, no Bearer header.
- [ ] AuthMW rejects `?token=` query on routes other than `GET /`.
- [ ] `MNEME_DASHBOARD_DEV_DIR=<path>` causes daemon to serve from filesystem instead of embed FS.
- [ ] `mneme daemon start --dev` enables `/dev-token` endpoint; without `--dev` the endpoint returns 404.
- [ ] All M0–M9 tests still pass.
- [ ] `make web-clean && go build ./cmd` produces a working binary that serves the placeholder.

---

**Roadmap reference:** §3 M10 — Web Dashboard (`docs/superpowers/specs/2026-04-28-m7-m11-roadmap-design.md`).

**Prerequisite milestones:**

- **M8 (hard):** the daemon's HTTP mux, AuthMW, TCP listener, token file, and `daemon_dashboard_port*` config keys are M8 deliverables. M10a extends `AuthMW` and registers new routes onto the existing mux.
- **M9 (independent):** no dependency. M9 ships its own data-layer pieces; M10b is the milestone that consumes M9's outputs.

**Successor milestones:**

- **M10b** consumes M10a's `dashboard.Mount` API to register `/api/*` and `/ws` routes; replaces the placeholder `dist/index.html` with a React bundle including 3 panels.
- **M10c** layers in the remaining 7 panels.
- **M11** adds the DesignQC panel data and the chromedp pipeline behind it.
