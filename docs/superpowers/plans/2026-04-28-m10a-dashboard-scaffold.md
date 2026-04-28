# M10a — Dashboard Backend & Scaffold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the backend route plumbing, embed pipeline, `mneme dashboard` CLI, build system, and one placeholder page that exercises embed → daemon serve → browser load → token-cookie auth → API round trip per `docs/superpowers/specs/2026-04-28-m10a-dashboard-scaffold-design.md`.

**Architecture:** A `pkg/dashboard` package owns route registration and the embed FS; `cmd/cmd_dashboard.go` orchestrates preflight + browser launch; M8's existing `AuthMW` is extended additively to accept cookie + bootstrap query token; a tiny Vite+React scaffold under `web/` produces a `dist/index.html` that ships a committed placeholder version so `go build` always works.

**Tech Stack:** Go 1.25.7, `embed`, `net/http`, `crypto/subtle`. Frontend: Vite, React 19, TypeScript, Tailwind v4 (`@tailwindcss/vite`), pnpm. Testing: Go `testing` + `httptest`; no Vitest in M10a.

**Hard prerequisite — M8 daemon must ship before M10a.** Tasks 9, 10, 12, 13, 14, 15, 16 touch M8 types (`daemon.AuthMW`, `daemon.RouteDeps`, daemon mux setup, TCP listener, token file, config keys). If M8 has not shipped, these tasks cannot complete. Tasks 1–8 are M8-independent and can be done first.

---

## File Structure

| Path | Type | Purpose |
|---|---|---|
| `web/package.json` | create | Pnpm workspace, scripts, deps |
| `web/pnpm-lock.yaml` | create | Lockfile (committed) |
| `web/tsconfig.json` | create | Strict TS settings |
| `web/vite.config.ts` | create | React + Tailwind v4 plugins; proxy for `/api`, `/ws`, `/health`, `/` |
| `web/src/main.tsx` | create | React entry |
| `web/src/App.tsx` | create | Top-level component (renders Health) |
| `web/src/Bootstrap.tsx` | create | Reads `?token=`, sets cookie, replaceState |
| `web/src/Health.tsx` | create | `fetch('/health')` and renders result |
| `web/src/styles.css` | create | `@import "tailwindcss"` |
| `web/dist.placeholder.html` | create | Restoration source for `make web-clean` |
| `web/dist/index.html` | create | Committed placeholder (overwritten by pnpm build) |
| `web/.gitignore` | create | Ignore `node_modules`, but NOT `dist/` |
| `web/README.md` | create | Dev workflows + dev-token recipe |
| `Makefile` | create | `build`, `web-build`, `web-dev`, `web-clean`, `go-build` |
| `pkg/dashboard/embed.go` | create | `//go:embed dist/*` → `fs.FS`; honors `MNEME_DASHBOARD_DEV_DIR` |
| `pkg/dashboard/server.go` | create | `Mount(mux, deps)`; SPA fallback file server |
| `pkg/dashboard/dev_token.go` | create | `POST /dev-token` handler (gated by `daemon_dev_mode`) |
| `pkg/daemon/middleware.go` | modify (M8 file) | Extend `AuthMW` to accept cookie + bootstrap query token |
| `pkg/daemon/routes.go` | modify (M8 file) | Call `dashboard.Mount` when TCP listener enabled |
| `pkg/config/config.go` | modify | Add `DaemonDevMode` field + env + yaml |
| `cmd/cmd_dashboard.go` | create | Preflight + browser open |
| `cmd/main.go` | modify | Add `dashboard` dispatch case |
| `pkg/dashboard/embed_test.go` | create | FS contains `index.html` |
| `pkg/dashboard/server_test.go` | create | Mount route behavior |
| `pkg/dashboard/dev_token_test.go` | create | `/dev-token` enabled vs disabled |
| `tests/cmd_dashboard_test.go` | create | Preflight exit codes + opener stub |
| `tests/auth_mw_test.go` | extend (M8 file) | Cookie + query-token cases |
| `tests/integration/dashboard_smoke_test.go` | create | `//go:build integration` end-to-end |
| `.gitignore` | modify | Add `web/node_modules/` |

---

## Task 1: Web scaffold — package.json, tsconfig, vite.config

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/vite.config.ts`, `web/.gitignore`

- [ ] **Step 1: Create `web/package.json`**

```json
{
  "name": "mneme-dashboard",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@tailwindcss/vite": "^4.0.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^4.3.4",
    "tailwindcss": "^4.0.0",
    "typescript": "~5.7.0",
    "vite": "^6.0.0"
  },
  "packageManager": "pnpm@9.15.0"
}
```

- [ ] **Step 2: Create `web/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"]
}
```

- [ ] **Step 3: Create `web/vite.config.ts`**

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api':       { target: 'http://localhost:18801', changeOrigin: true },
      '/ws':        { target: 'ws://localhost:18801', ws: true },
      '/health':    { target: 'http://localhost:18801', changeOrigin: true },
      '/dev-token': { target: 'http://localhost:18801', changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
```

- [ ] **Step 4: Create `web/.gitignore`**

```
node_modules/
.vite/
*.log
```

(Note: `dist/` is intentionally NOT ignored — `dist/index.html` is committed.)

- [ ] **Step 5: Commit**

```bash
git add web/package.json web/tsconfig.json web/vite.config.ts web/.gitignore
git commit -m "feat(m10a): web scaffold — package.json, tsconfig, vite config"
```

---

## Task 2: Placeholder `dist/index.html` and restoration source

**Files:**
- Create: `web/dist.placeholder.html`, `web/dist/index.html`

- [ ] **Step 1: Create `web/dist.placeholder.html`**

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>mneme dashboard</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="referrer" content="no-referrer">
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
  <style>
    body { font-family: system-ui, -apple-system, sans-serif; max-width: 40rem;
           margin: 4rem auto; padding: 0 1rem; line-height: 1.5; }
    code { background: #f4f4f4; padding: 0.1em 0.3em; border-radius: 3px; }
    #health { font-family: monospace; }
  </style>
</head>
<body>
  <h1>mneme dashboard</h1>
  <p>Backend is up. The web UI has not been built yet.</p>
  <p>Run <code>make web-build</code> from the repo root, then refresh.</p>
  <p>Daemon health: <span id="health">checking…</span></p>
  <script>
    fetch('/health', { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (j) {
        document.getElementById('health').textContent =
          'pid=' + j.pid + ' uptime=' + j.uptime_s + 's version=' + j.version;
      })
      .catch(function (e) {
        document.getElementById('health').textContent = 'error: ' + e;
      });
  </script>
</body>
</html>
```

- [ ] **Step 2: Copy placeholder to `web/dist/index.html`**

```bash
mkdir -p web/dist
cp web/dist.placeholder.html web/dist/index.html
```

- [ ] **Step 3: Commit**

```bash
git add web/dist.placeholder.html web/dist/index.html
git commit -m "feat(m10a): committed placeholder dist/index.html for go:embed"
```

---

## Task 3: React placeholder source files

**Files:**
- Create: `web/src/main.tsx`, `web/src/App.tsx`, `web/src/Bootstrap.tsx`, `web/src/Health.tsx`, `web/src/styles.css`

- [ ] **Step 1: Create `web/src/styles.css`**

```css
@import "tailwindcss";

body {
  @apply font-sans text-base leading-6 max-w-2xl mx-auto px-4 py-16;
}
```

- [ ] **Step 2: Create `web/src/Bootstrap.tsx`**

```tsx
import { useEffect } from 'react'

// Bootstrap is rendered once on first mount. The inline <script> in
// index.html already extracts the token from the URL before React boots,
// so this component is currently a no-op placeholder for future logic
// (e.g., re-fetching token on cookie expiry). It exists to keep the
// "where does auth live in the app" question answered in source.
export function Bootstrap(): null {
  useEffect(() => {
    // Reserved: future token-refresh logic.
  }, [])
  return null
}
```

- [ ] **Step 3: Create `web/src/Health.tsx`**

```tsx
import { useEffect, useState } from 'react'

interface HealthResponse {
  pid: number
  uptime_s: number
  version: string
}

export function Health(): JSX.Element {
  const [data, setData] = useState<HealthResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch('/health', { credentials: 'same-origin' })
      .then((r) => {
        if (!r.ok) throw new Error('HTTP ' + r.status)
        return r.json() as Promise<HealthResponse>
      })
      .then(setData)
      .catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <p>error: {error}</p>
  if (!data) return <p>checking…</p>
  return (
    <p>
      pid={data.pid} uptime={data.uptime_s}s version={data.version}
    </p>
  )
}
```

- [ ] **Step 4: Create `web/src/App.tsx`**

```tsx
import { Bootstrap } from './Bootstrap'
import { Health } from './Health'

export function App(): JSX.Element {
  return (
    <>
      <Bootstrap />
      <h1 className="text-2xl font-semibold mb-4">mneme dashboard</h1>
      <p className="mb-4">Backend is up. Built UI is loaded.</p>
      <p>Daemon health: <Health /></p>
    </>
  )
}
```

- [ ] **Step 5: Create `web/src/main.tsx`**

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import './styles.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 6: Commit**

```bash
git add web/src/
git commit -m "feat(m10a): React placeholder source — Bootstrap + Health"
```

---

## Task 4: Makefile

**Files:**
- Create: `Makefile`
- Modify: `.gitignore`

- [ ] **Step 1: Create `Makefile`**

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

- [ ] **Step 2: Update `.gitignore`**

Append to `.gitignore`:

```
web/node_modules/
web/.vite/
```

- [ ] **Step 3: Verify `go build` works without pnpm**

Run: `go build -o /tmp/mneme-test ./cmd && rm /tmp/mneme-test`
Expected: PASS (no Node toolchain needed; the embed package will land in Task 5).

- [ ] **Step 4: Commit**

```bash
git add Makefile .gitignore
git commit -m "feat(m10a): Makefile — build, web-build, web-dev, web-clean"
```

---

## Task 5: `pkg/dashboard/embed.go`

**Files:**
- Create: `pkg/dashboard/embed.go`
- Test: `pkg/dashboard/embed_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/dashboard/embed_test.go`:

```go
package dashboard

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFSContainsIndex(t *testing.T) {
	f := FS()
	data, err := fs.ReadFile(f, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(data), "<title>mneme dashboard</title>") {
		t.Errorf("placeholder marker missing in index.html")
	}
}

func TestFSDevDirOverride(t *testing.T) {
	dir := t.TempDir()
	custom := []byte("<html>dev override</html>")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), custom, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MNEME_DASHBOARD_DEV_DIR", dir)

	data, err := fs.ReadFile(FS(), "index.html")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != string(custom) {
		t.Errorf("dev override not applied; got %q", data)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/dashboard -run TestFS -v`
Expected: FAIL — `dashboard.FS` undefined.

- [ ] **Step 3: Implement `pkg/dashboard/embed.go`**

```go
// Package dashboard hosts the M10 web UI as embedded static assets.
//
// At go-build time, //go:embed snapshots the contents of web/dist/*. The
// repo commits a placeholder web/dist/index.html so go build always
// succeeds even on a fresh clone without running pnpm. After
// `make web-build`, web/dist/ contains the real React bundle.
package dashboard

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed dist/*
var distFS embed.FS

// FS returns the filesystem the daemon should serve from.
//
// If MNEME_DASHBOARD_DEV_DIR is set to a non-empty path, that filesystem
// is returned instead of the embed FS — useful with `pnpm build --watch`
// to iterate without re-running `go build`. When the env var is unset,
// the rooted embed FS (rooted at the `dist` subdirectory) is returned.
func FS() fs.FS {
	if dir := os.Getenv("MNEME_DASHBOARD_DEV_DIR"); dir != "" {
		return os.DirFS(dir)
	}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// fs.Sub on a static path can only fail if the path is invalid;
		// embed.FS guarantees `dist` exists at build time so this is unreachable.
		panic(err)
	}
	return sub
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/dashboard -run TestFS -v`
Expected: PASS both tests.

- [ ] **Step 5: Commit**

```bash
git add pkg/dashboard/embed.go pkg/dashboard/embed_test.go
git commit -m "feat(m10a): dashboard embed.FS with MNEME_DASHBOARD_DEV_DIR override"
```

---

## Task 6: `pkg/dashboard/server.go` — Mount + SPA fallback

**Files:**
- Create: `pkg/dashboard/server.go`
- Test: `pkg/dashboard/server_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/dashboard/server_test.go`:

```go
package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMountServesIndexAtRoot(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, Deps{})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<title>mneme dashboard</title>") {
		t.Errorf("body missing dashboard title: %s", body)
	}
}

func TestMountServesAssets(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, Deps{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// /assets/* and /static/* prefixes both forward into the embed FS.
	// The placeholder doesn't contain assets, so we test by adding via DEV_DIR.
	dir := t.TempDir()
	t.Setenv("MNEME_DASHBOARD_DEV_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>x</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('x')"), 0644); err != nil {
		t.Fatal(err)
	}

	// Re-Mount so the new env var is picked up.
	mux2 := http.NewServeMux()
	Mount(mux2, Deps{})
	srv2 := httptest.NewServer(mux2)
	defer srv2.Close()

	resp, err := http.Get(srv2.URL + "/assets/app.js")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestMountSPAFallback(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, Deps{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/some/route/that/does/not/exist")
	if err != nil {
		t.Fatalf("get unknown: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("SPA fallback should return 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<title>mneme dashboard</title>") {
		t.Errorf("SPA fallback should serve index.html: %s", body)
	}
}
```

Add `"os"` and `"path/filepath"` to the import block.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/dashboard -run TestMount -v`
Expected: FAIL — `Mount`/`Deps` undefined.

- [ ] **Step 3: Implement `pkg/dashboard/server.go`**

```go
package dashboard

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Deps is reserved for future composition (logging, metrics, etc.) and is
// intentionally empty in M10a. M10b will populate it with API handler deps.
type Deps struct{}

// Mount registers dashboard routes on mux. AuthMW must already wrap the
// returned handlers — Mount itself does not enforce auth; it relies on
// the daemon's middleware stack. The routes registered are:
//
//   GET  /             SPA root (serves index.html)
//   GET  /assets/*     embedded asset files
//   GET  /static/*     embedded asset files (alias)
//   GET  /<other>      SPA fallback → index.html (for client-side routing)
//
// Excluded paths (mux pattern precedence wins): /api/*, /ws, /health,
// /status, /scan, /update, /restore, /cron/*, /cerebrum/learn,
// /designqc/*, /dev-token. Those are owned by the daemon and other
// dashboard sub-milestones (M10b/M10c/M11).
func Mount(mux *http.ServeMux, _ Deps) {
	mux.Handle("/", spaHandler())
	mux.Handle("/assets/", http.StripPrefix("/assets/", assetHandler()))
	mux.Handle("/static/", http.StripPrefix("/static/", assetHandler()))
}

// spaHandler serves index.html for / and any path that does not exist
// in the embedded FS. Real assets are served via /assets/ or /static/.
func spaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Try to serve a file matching the path (so e.g. /favicon.ico works
		// without /assets/ prefix). If that fails, fall back to index.html
		// for SPA routing.
		f := FS()
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean != "" {
			if file, err := f.Open(clean); err == nil {
				file.Close()
				http.ServeFileFS(w, r, f, clean)
				return
			}
		}
		http.ServeFileFS(w, r, f, "index.html")
	})
}

func assetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := FS()
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(f, clean); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, f, clean)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/dashboard -run TestMount -v`
Expected: PASS all three.

- [ ] **Step 5: Commit**

```bash
git add pkg/dashboard/server.go pkg/dashboard/server_test.go
git commit -m "feat(m10a): dashboard.Mount with SPA fallback file server"
```

---

## Task 7: `pkg/dashboard/dev_token.go` — gated `/dev-token` handler

**Files:**
- Create: `pkg/dashboard/dev_token.go`
- Test: `pkg/dashboard/dev_token_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/dashboard/dev_token_test.go`:

```go
package dashboard

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDevTokenSetsCookieWhenEnabled(t *testing.T) {
	h := DevTokenHandler("the-token", true)
	body := strings.NewReader(`{"token":"the-token"}`)
	req := httptest.NewRequest("POST", "/dev-token", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	cookies := rec.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "mneme_token" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("mneme_token cookie not set")
	}
	if found.Value != "the-token" {
		t.Errorf("cookie value: got %q, want %q", found.Value, "the-token")
	}
}

func TestDevTokenRejectsWrongToken(t *testing.T) {
	h := DevTokenHandler("the-token", true)
	body := strings.NewReader(`{"token":"wrong"}`)
	req := httptest.NewRequest("POST", "/dev-token", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestDevTokenDisabledReturns404(t *testing.T) {
	h := DevTokenHandler("the-token", false)
	body := strings.NewReader(`{"token":"the-token"}`)
	req := httptest.NewRequest("POST", "/dev-token", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("disabled handler should 404; got %d", rec.Code)
	}
}

func TestDevTokenCORSPreflight(t *testing.T) {
	h := DevTokenHandler("the-token", true)
	req := httptest.NewRequest("OPTIONS", "/dev-token", bytes.NewReader(nil))
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status: got %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("missing Access-Control-Allow-Credentials")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/dashboard -run TestDevToken -v`
Expected: FAIL — `DevTokenHandler` undefined.

- [ ] **Step 3: Implement `pkg/dashboard/dev_token.go`**

```go
package dashboard

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// DevTokenHandler returns a handler that, when enabled, sets the
// mneme_token cookie on the request origin so the Vite dev server at
// :5173 can authenticate without manual paste.
//
// When enabled is false, the handler returns 404 (route disabled).
//
// Threat model: this endpoint requires the caller to know the same
// per-launch token used everywhere else. It's NOT a token-issuing
// endpoint — it just packages an existing token as a cookie. The
// caller's bar is identical to AuthMW's: knowledge of the token.
func DevTokenHandler(token string, enabled bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			http.NotFound(w, r)
			return
		}
		// CORS for Vite dev (:5173 → :18801).
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if subtle.ConstantTimeCompare([]byte(body.Token), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "mneme_token",
			Value:    token,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400,
		})
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/dashboard -run TestDevToken -v`
Expected: PASS all four.

- [ ] **Step 5: Commit**

```bash
git add pkg/dashboard/dev_token.go pkg/dashboard/dev_token_test.go
git commit -m "feat(m10a): /dev-token handler with CORS for Vite dev"
```

---

## Task 8: Config — `daemon_dev_mode` key

**Files:**
- Modify: `pkg/config/config.go`
- Test: `tests/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `tests/config_test.go`:

```go
func TestDaemonDevModeDefault(t *testing.T) {
	t.Setenv("DAEMON_DEV_MODE", "")
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	cfg := config.FromEnv()
	if cfg.DaemonDevMode {
		t.Errorf("DaemonDevMode default: got true, want false")
	}
}

func TestDaemonDevModeEnvOverride(t *testing.T) {
	t.Setenv("DAEMON_DEV_MODE", "true")
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	cfg := config.FromEnv()
	if !cfg.DaemonDevMode {
		t.Errorf("DaemonDevMode env override: got false, want true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestDaemonDevMode -v`
Expected: FAIL — `cfg.DaemonDevMode undefined`.

- [ ] **Step 3: Add the field**

In `pkg/config/config.go`, append to `Config` struct:

```go
	DaemonDevMode bool
```

In `fileConfig`:

```go
	DaemonDevMode *bool `yaml:"daemon_dev_mode"`
```

In `FromEnv()` return struct, append:

```go
		DaemonDevMode: resolveBool(os.Getenv("DAEMON_DEV_MODE"), file.DaemonDevMode, false),
```

(`resolveBool` was added in M9 Task 1; reuse it.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestDaemonDevMode -v`
Expected: PASS both.

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go tests/config_test.go
git commit -m "feat(m10a): config — daemon_dev_mode key"
```

---

## Task 9: Extend M8 `AuthMW` for cookie + bootstrap query token

> Depends on M8. Modifies the `pkg/daemon/middleware.go` file from M8 plan Task 9.

**Files:**
- Modify: `pkg/daemon/middleware.go`
- Test: `tests/auth_mw_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tests/auth_mw_test.go`:

```go
func TestAuthMWAcceptsCookie(t *testing.T) {
	h := daemon.AuthMW("the-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/anything", nil)
	req.AddCookie(&http.Cookie{Name: "mneme_token", Value: "the-token"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("cookie auth: got %d, want 200", rec.Code)
	}
}

func TestAuthMWAcceptsQueryTokenOnRoot(t *testing.T) {
	h := daemon.AuthMW("the-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/?token=the-token", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("query token on /: got %d, want 200", rec.Code)
	}
}

func TestAuthMWRejectsQueryTokenOnNonRoot(t *testing.T) {
	h := daemon.AuthMW("the-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/api/x?token=the-token", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("query token on /api/x: got %d, want 401", rec.Code)
	}
}

func TestAuthMWRejectsWrongCookie(t *testing.T) {
	h := daemon.AuthMW("the-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "mneme_token", Value: "wrong"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong cookie: got %d, want 401", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestAuthMW -v`
Expected: FAIL — three new tests fail because cookie + query-token paths don't exist yet (the existing Bearer-only test still passes).

- [ ] **Step 3: Modify `pkg/daemon/middleware.go`**

Replace M8's `AuthMW` body with:

```go
func AuthMW(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Authorization: Bearer <token>
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			provided := strings.TrimPrefix(h, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}
		// 2. Cookie: mneme_token=<token>
		if c, err := r.Cookie("mneme_token"); err == nil {
			if subtle.ConstantTimeCompare([]byte(c.Value), []byte(token)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}
		// 3. Bootstrap-only: ?token=<token> on GET /
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			if q := r.URL.Query().Get("token"); q != "" {
				if subtle.ConstantTimeCompare([]byte(q), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}
```

Verify imports include `"crypto/subtle"`, `"net/http"`, and `"strings"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestAuthMW -v`
Expected: PASS all (existing Bearer test + 4 new).

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon/middleware.go tests/auth_mw_test.go
git commit -m "feat(m10a): extend AuthMW for cookie + bootstrap query token"
```

---

## Task 10: Wire `dashboard.Mount` + `/dev-token` into M8 daemon mux

> Depends on M8. Modifies the `pkg/daemon/routes.go` file from M8 plan Task 10.

**Files:**
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing test**

Append to `tests/cerebrum_handler_test.go` (or a new `tests/daemon_dashboard_routes_test.go`):

```go
func TestDaemonMuxServesDashboardRoot(t *testing.T) {
	mux := daemon.NewMux(daemon.RouteDeps{
		Token:    "the-token",
		DevMode:  false,
		// other M8 fields zeroed
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/?token=the-token")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<title>mneme dashboard</title>") {
		t.Errorf("body missing dashboard title: %s", body)
	}
}

func TestDaemonMuxDevTokenDisabledByDefault(t *testing.T) {
	mux := daemon.NewMux(daemon.RouteDeps{
		Token:   "the-token",
		DevMode: false,
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(
		srv.URL+"/dev-token", "application/json",
		strings.NewReader(`{"token":"the-token"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestDaemonMuxDevTokenEnabled(t *testing.T) {
	mux := daemon.NewMux(daemon.RouteDeps{
		Token:   "the-token",
		DevMode: true,
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(
		srv.URL+"/dev-token", "application/json",
		strings.NewReader(`{"token":"the-token"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestDaemonMuxServes -v && go test ./tests -run TestDaemonMuxDevToken -v`
Expected: FAIL — root path returns 404 because `dashboard.Mount` isn't called yet.

- [ ] **Step 3: Modify `pkg/daemon/routes.go`**

Extend M8's `RouteDeps`:

```go
type RouteDeps struct {
	Token    string
	DevMode  bool
	// existing M8 fields...
	Cerebrum CerebrumLearnDeps  // M9
}
```

Inside `NewMux(deps RouteDeps)`:

```go
import (
	"github.com/ranwei/mneme/pkg/dashboard"
)

// ... existing route registrations ...

// M10a: dashboard static + dev-token
dashboard.Mount(mux, dashboard.Deps{})
mux.Handle("/dev-token", dashboard.DevTokenHandler(deps.Token, deps.DevMode))
```

Place these *before* the catchall AuthMW wrapping so they go through the same `AuthMW` other routes do. The bootstrap route `/?token=` is then handled inside AuthMW per Task 9.

Note: `dashboard.Mount` registers `/`, `/assets/`, `/static/`. These compose with `/api/*`, `/health`, `/cron/*`, etc. — Go's `http.ServeMux` honors longer-prefix matches first.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestDaemonMux -v`
Expected: PASS all three.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon/routes.go tests/cerebrum_handler_test.go
git commit -m "feat(m10a): wire dashboard.Mount + /dev-token into daemon mux"
```

---

## Task 11: Daemon config plumbing — pass `DaemonDevMode` into RouteDeps

> Depends on M8. Modifies the daemon entry point that constructs `RouteDeps`.

**Files:**
- Modify: `pkg/daemon/daemon.go` (M8 file constructing RouteDeps)
- Modify: `cmd/cmd_daemon.go` (M8 file with `--dev` flag)

- [ ] **Step 1: Add `--dev` flag to `mneme daemon start`**

In `cmd/cmd_daemon.go`, locate `runDaemonStart` (M8 Task 14). Add a flag:

```go
	dev := fs.Bool("dev", false, "enable dev-token endpoint and dev-mode features")
```

Pass it through to `daemon.Run`:

```go
	opts := daemon.RunOptions{
		// ... existing M8 opts ...
		DevMode: *dev,
	}
```

- [ ] **Step 2: Add `DevMode` to `daemon.RunOptions`**

In `pkg/daemon/daemon.go`:

```go
type RunOptions struct {
	// ... existing M8 fields ...
	DevMode bool
}
```

In `Run(ctx, opts)`, when constructing `RouteDeps`:

```go
	cfg := config.FromEnv()
	devMode := opts.DevMode || cfg.DaemonDevMode

	rd := RouteDeps{
		Token:   token,
		DevMode: devMode,
		// ... existing M8 fields ...
	}
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/daemon/daemon.go cmd/cmd_daemon.go
git commit -m "feat(m10a): mneme daemon start --dev flag + DaemonDevMode config"
```

---

## Task 12: `cmd/cmd_dashboard.go` — preflight + browser open

**Files:**
- Create: `cmd/cmd_dashboard.go`
- Modify: `cmd/main.go`
- Test: `tests/cmd_dashboard_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/cmd_dashboard_test.go`:

```go
package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/dashboard"
)

func TestDashboardPreflightAllOK(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	mneme := filepath.Join(tmp, ".mneme", "daemon")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, "token"), []byte("the-token\n"), 0600)

	// Stand up an httptest server pretending to be the daemon TCP listener.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Write([]byte(`{"pid":1,"uptime_s":1,"version":"x"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Strip "http://" prefix to get host:port.
	hostPort := strings.TrimPrefix(srv.URL, "http://")
	host, port, _ := splitHostPort(hostPort)

	var openedURL string
	deps := dashboard.CLIDeps{
		UnixSocket:  "", // tests below cover the unix-down path; here we skip it
		TCPHost:     host,
		TCPPort:     port,
		TokenPath:   filepath.Join(mneme, "token"),
		Opener:      func(u string) error { openedURL = u; return nil },
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		SkipUnixCheck: true,
	}
	exit := dashboard.RunCLI(deps, nil)
	if exit != 0 {
		t.Errorf("exit: got %d, want 0", exit)
	}
	if !strings.Contains(openedURL, "?token=the-token") {
		t.Errorf("opener URL missing token: %s", openedURL)
	}
}

func TestDashboardPreflightTCPDown(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	mneme := filepath.Join(tmp, ".mneme", "daemon")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, "token"), []byte("the-token\n"), 0600)

	deps := dashboard.CLIDeps{
		TCPHost:       "127.0.0.1",
		TCPPort:       1, // unused port
		TokenPath:     filepath.Join(mneme, "token"),
		Opener:        func(u string) error { t.Errorf("opener called: %s", u); return nil },
		Stdout:        os.Stdout,
		Stderr:        new(strings.Builder),
		SkipUnixCheck: true,
	}
	exit := dashboard.RunCLI(deps, nil)
	if exit != 1 {
		t.Errorf("exit: got %d, want 1", exit)
	}
}

func splitHostPort(hp string) (host string, port int, err error) {
	parts := strings.Split(hp, ":")
	if len(parts) != 2 {
		return "", 0, errFmt(hp)
	}
	p := 0
	for _, c := range parts[1] {
		p = p*10 + int(c-'0')
	}
	return parts[0], p, nil
}

type fmtErr string

func (e fmtErr) Error() string { return string(e) }
func errFmt(s string) error    { return fmtErr("bad host:port: " + s) }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run TestDashboardPreflight -v`
Expected: FAIL — `dashboard.CLIDeps`/`dashboard.RunCLI` undefined.

- [ ] **Step 3: Implement `cmd/cmd_dashboard.go` as a thin wrapper**

Add to `pkg/dashboard` a separate file `cli.go` (keeps cmd_dashboard.go minimal and the testable code under pkg/):

```go
// pkg/dashboard/cli.go
package dashboard

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type CLIDeps struct {
	UnixSocket    string
	TCPHost       string
	TCPPort       int
	TokenPath     string
	Opener        func(url string) error
	Stdout        io.Writer
	Stderr        io.Writer
	SkipUnixCheck bool
}

// RunCLI executes `mneme dashboard` preflight + browser open. Returns the
// process exit code. Args is parsed for `--no-open`.
func RunCLI(deps CLIDeps, args []string) int {
	noOpen := false
	for _, a := range args {
		if a == "--no-open" {
			noOpen = true
		}
	}

	if !deps.SkipUnixCheck {
		if !pingUnix(deps.UnixSocket) {
			fmt.Fprintln(deps.Stderr,
				"✗ Daemon not running. Run: mneme daemon start")
			return 1
		}
	}

	if !pingTCP(deps.TCPHost, deps.TCPPort) {
		fmt.Fprintf(deps.Stderr,
			"✗ Dashboard TCP listener disabled. Add daemon_dashboard_port_enabled: true "+
				"to ~/.mneme/config.yaml, then: mneme daemon restart\n")
		return 1
	}

	tokenBytes, err := os.ReadFile(deps.TokenPath)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "✗ Token file missing at %s — daemon may have just started.\n",
			deps.TokenPath)
		return 1
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		fmt.Fprintln(deps.Stderr, "✗ Token file is empty")
		return 1
	}

	url := fmt.Sprintf("http://%s:%d/?token=%s", deps.TCPHost, deps.TCPPort, token)

	if noOpen {
		fmt.Fprintln(deps.Stdout, url)
		return 0
	}

	opener := deps.Opener
	if opener == nil {
		opener = systemOpener()
	}
	if err := opener(url); err != nil {
		fmt.Fprintln(deps.Stderr, "(could not open browser:", err, ")")
		fmt.Fprintln(deps.Stdout, url)
	}
	return 0
}

func pingUnix(socket string) bool {
	if socket == "" {
		return false
	}
	if _, err := os.Stat(socket); err != nil {
		return false
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 200 * time.Millisecond}
			return d.DialContext(ctx, "unix", socket)
		},
	}
	client := &http.Client{Transport: tr, Timeout: 200 * time.Millisecond}
	resp, err := client.Get("http://unix/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func pingTCP(host string, port int) bool {
	d := net.Dialer{Timeout: 200 * time.Millisecond}
	conn, err := d.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func systemOpener() func(url string) error {
	return func(url string) error {
		var bin string
		var args []string
		switch runtime.GOOS {
		case "darwin":
			bin = "open"
			args = []string{url}
		case "linux":
			bin = "xdg-open"
			args = []string{url}
		case "windows":
			bin = "cmd"
			args = []string{"/c", "start", url}
		default:
			return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
		}
		// exec.Command imported lazily to avoid pulling os/exec into tests
		// that don't need it.
		_ = filepath.Join // ensure filepath import is used
		return execCommand(bin, args...)
	}
}
```

Add a tiny shim file `pkg/dashboard/exec.go`:

```go
package dashboard

import "os/exec"

func execCommand(bin string, args ...string) error {
	c := exec.Command(bin, args...)
	return c.Start() // fire-and-forget; do not wait for browser
}
```

Now create `cmd/cmd_dashboard.go`:

```go
package main

import (
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/dashboard"
)

func dispatchDashboard(args []string) {
	home, _ := os.UserHomeDir()
	cfg := config.FromEnv()

	deps := dashboard.CLIDeps{
		UnixSocket: filepath.Join(home, ".mneme", "daemon", "socket"),
		TCPHost:    "127.0.0.1",
		TCPPort:    cfg.DaemonDashboardPort, // M8 config field; default 18801
		TokenPath:  filepath.Join(home, ".mneme", "daemon", "token"),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
	os.Exit(dashboard.RunCLI(deps, args))
}
```

In `cmd/main.go` switch:

```go
	case "dashboard":
		dispatchDashboard(os.Args[2:])
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./cmd && go test ./tests -run TestDashboardPreflight -v && go test ./pkg/dashboard -v`
Expected: PASS all.

- [ ] **Step 5: Commit**

```bash
git add pkg/dashboard/cli.go pkg/dashboard/exec.go cmd/cmd_dashboard.go cmd/main.go tests/cmd_dashboard_test.go
git commit -m "feat(m10a): mneme dashboard CLI with preflight + browser open"
```

---

## Task 13: `web/README.md` — dev-mode workflows

**Files:**
- Create: `web/README.md`

- [ ] **Step 1: Create `web/README.md`**

```markdown
# mneme dashboard frontend

Vite + React + TypeScript + Tailwind v4. Built artifacts go in `web/dist/`,
which is committed (the placeholder version) and embedded into the Go
binary at `pkg/dashboard/embed.go`.

## Build

From the repo root:

    make build           # full release: pnpm install + pnpm build + go build
    make web-build       # frontend only
    make go-build        # backend only (uses whatever dist is present)
    make web-clean       # restore dist/index.html to placeholder

## Dev mode A — Vite dev server (recommended)

Run two terminals:

    # Terminal 1 — daemon with dev-token endpoint enabled
    mneme daemon start --dev

    # Terminal 2 — Vite dev server on :5173
    make web-dev

In your browser, visit `http://localhost:5173/`. You'll see "Unauthorized"
until you set the cookie. Open DevTools console and run:

    fetch('/dev-token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: '<paste contents of ~/.mneme/daemon/token>' }),
    }).then(r => r.json()).then(console.log)

The cookie now scopes to `:5173` (because the request went through Vite's
proxy) and HMR works against live daemon data.

## Dev mode B — `MNEME_DASHBOARD_DEV_DIR` filesystem override

If you'd rather build the frontend in watch mode and skip the proxy:

    cd web && pnpm build --watch &
    MNEME_DASHBOARD_DEV_DIR=$PWD/dist mneme daemon start

The daemon serves directly from `web/dist/` — no `go build` needed on
each change. `mneme dashboard` opens the browser to `:18801` as usual.

## Tests

There are no Vitest tests in M10a — the placeholder is too small to need
them. M10b adds a test harness for the panel components.
```

- [ ] **Step 2: Commit**

```bash
git add web/README.md
git commit -m "docs(m10a): web/README.md — dev-mode workflows"
```

---

## Task 14: `web-build` smoke test in CI

**Files:**
- Modify: `.github/workflows/test.yml` (or whatever the existing CI workflow is named)

- [ ] **Step 1: Inspect existing CI**

Run: `ls .github/workflows/`
If a CI workflow exists, modify it. If none exists, create one.

- [ ] **Step 2: Add Node setup step**

Add to the existing `jobs.test.steps` list (or create a workflow if absent):

```yaml
      - name: Setup Node 20
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Setup pnpm
        uses: pnpm/action-setup@v4
        with:
          version: 9.15.0

      - name: Cache pnpm store
        uses: actions/cache@v4
        with:
          path: ~/.local/share/pnpm/store
          key: ${{ runner.os }}-pnpm-${{ hashFiles('web/pnpm-lock.yaml') }}
          restore-keys: |
            ${{ runner.os }}-pnpm-

      - name: Build web
        run: make web-build

      - name: Build go
        run: make go-build

      - name: Test
        run: go test ./...
```

If the workflow file did not exist, the full file should be:

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Setup Node 20
        uses: actions/setup-node@v4
        with:
          node-version: '20'
      - name: Setup pnpm
        uses: pnpm/action-setup@v4
        with:
          version: 9.15.0
      - name: Cache pnpm store
        uses: actions/cache@v4
        with:
          path: ~/.local/share/pnpm/store
          key: ${{ runner.os }}-pnpm-${{ hashFiles('web/pnpm-lock.yaml') }}
          restore-keys: |
            ${{ runner.os }}-pnpm-
      - name: Build web
        run: make web-build
      - name: Build go
        run: make go-build
      - name: Test
        run: go test ./...
```

- [ ] **Step 3: Verify CI config locally**

Run: `make build && go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/
git commit -m "ci(m10a): add Node 20 + pnpm to test matrix"
```

---

## Task 15: Integration smoke — full pipeline end-to-end

**Files:**
- Create: `tests/integration/dashboard_smoke_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/integration/dashboard_smoke_test.go`:

```go
//go:build integration

package integration

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDashboardEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bin := buildBinary(t)

	// Configure daemon: TCP listener enabled.
	cfgDir := filepath.Join(tmp, ".mneme")
	os.MkdirAll(cfgDir, 0755)
	cfg := []byte(
		"daemon_dashboard_port: 18802\n" +
			"daemon_dashboard_port_enabled: true\n",
	)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), cfg, 0644)

	dCmd := exec.Command(bin, "daemon", "start")
	dCmd.Env = append(os.Environ(), "HOME="+tmp)
	if err := dCmd.Start(); err != nil {
		t.Fatalf("daemon start: %v", err)
	}
	defer func() {
		dCmd.Process.Signal(os.Interrupt)
		dCmd.Wait()
	}()

	// Wait for token file.
	tokenPath := filepath.Join(cfgDir, "daemon", "token")
	deadline := time.Now().Add(5 * time.Second)
	var token string
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(tokenPath); err == nil {
			token = strings.TrimSpace(string(data))
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if token == "" {
		t.Fatal("token file did not appear within 5s")
	}

	// Wait for TCP listener.
	tcpURL := "http://127.0.0.1:18802"
	for time.Now().Before(deadline) {
		conn, err := http.Get(tcpURL + "/health?token=" + token)
		if err == nil && conn.StatusCode != 0 {
			conn.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Drive the bootstrap flow with a cookiejar client.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: GET /?token=T → 200, sets nothing (cookie is set client-side).
	resp, err := client.Get(tcpURL + "/?token=" + token)
	if err != nil {
		t.Fatalf("bootstrap GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status: got %d, want 200; body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `mneme_token`) {
		t.Errorf("bootstrap body should contain cookie-setting JS: %s", body)
	}

	// Step 2: simulate browser cookie set.
	parsed, _ := url.Parse(tcpURL)
	jar.SetCookies(parsed, []*http.Cookie{{
		Name:     "mneme_token",
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	}})

	// Step 3: GET /health using cookie only.
	resp, err = client.Get(tcpURL + "/health")
	if err != nil {
		t.Fatalf("/health GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/health with cookie: got %d, want 200", resp.StatusCode)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mneme")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test -tags=integration ./tests/integration -run TestDashboardEndToEnd -v -timeout 30s`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/dashboard_smoke_test.go
git commit -m "test(m10a): integration smoke for dashboard end-to-end"
```

---

## Task 16: Acceptance verification + spec checkbox tick

**Files:**
- Modify: `docs/superpowers/specs/2026-04-28-m10a-dashboard-scaffold-design.md`

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

Run: `go test -tags=integration ./tests/integration`
Expected: PASS.

- [ ] **Step 2: Verify `go build ./cmd` works on a fresh state**

Run: `make web-clean && go build -o /tmp/mneme-test ./cmd && rm /tmp/mneme-test`
Expected: PASS.

- [ ] **Step 3: Verify `make build` succeeds**

Run: `make build`
Expected: PASS — produces `bin/mneme`.

- [ ] **Step 4: Manual smoke (one project, daemon up)**

In a separate terminal:

```bash
echo "daemon_dashboard_port_enabled: true" >> ~/.mneme/config.yaml
mneme daemon restart
mneme dashboard           # opens browser
mneme dashboard --no-open # prints URL
```

Verify the browser tab shows "Backend is up. Built UI is loaded." (or "...not built yet" if you ran `make web-clean` instead of `make web-build`) plus a working `pid=… uptime=… version=…` line.

- [ ] **Step 5: Tick acceptance checkboxes in spec**

Edit `docs/superpowers/specs/2026-04-28-m10a-dashboard-scaffold-design.md` §11 Acceptance Criteria — change every `- [ ]` to `- [x]` only for items that have been verified above.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-04-28-m10a-dashboard-scaffold-design.md
git commit -m "docs(m10a): tick acceptance criteria after end-to-end verification"
```

---

## Self-review

**Spec coverage map:**

| Spec section | Implementing task(s) |
|---|---|
| §2.1 Component diagram | 6 (Mount), 12 (CLI) |
| §2.2 Package layout | 1, 5, 6, 7, 12, 13 |
| §2.3 Daemon-side wiring | 9 (AuthMW), 10 (mux registration) |
| §3.1 Step-by-step auth flow | 9 (AuthMW logic), 12 (CLI URL building), 15 (integration) |
| §3.2 AuthMW changes | 9 |
| §3.3 Bootstrap inline script | 2 (placeholder) — script is identical in placeholder and real builds because Vite preserves head children |
| §3.4 Threat model | 2 (Referrer-Policy meta tag, history.replaceState in script), 9 (cookie + query token) |
| §3.5 WS compatibility | Future M10b (cookie-based) — design preserved |
| §4.1 Makefile | 4 |
| §4.2 Vite dev mode | 1 (vite.config.ts proxy), 7 (/dev-token), 13 (README) |
| §4.3 Dev-dir override | 5 (env var), 13 (README) |
| §4.4 Placeholder index.html | 2 |
| §4.5 CI changes | 14 |
| §5 CLI behavior | 12 |
| §6 Configuration | 8 (DaemonDevMode), 11 (--dev flag wiring) |
| §7 Testing strategy | 5–7 (unit), 9–10 (handler), 12 (CLI), 15 (integration) |
| §8 Backwards-compat invariants | 4 (placeholder enables go build), 9 (AuthMW additive), all tasks (no schema changes) |
| §11 Acceptance criteria | 16 |

**Test coverage gap (non-blocking):**
- The bootstrap inline script is exercised only via the integration test asserting it appears in the response body. The cookie-setting JavaScript itself runs only in a real browser; that's not a Go-test-able surface, but the integration test simulates the cookie set in step 2 to prove the rest of the flow works.
- Vite proxy is documented + smoke-checked manually, not Go-tested.

**Open issues (do not block):**
- Task 12's `cmd_dashboard.go` body is intentionally tiny because the testable code lives in `pkg/dashboard/cli.go`. This pattern (CLI entry point under `cmd/`, runner under `pkg/`) matches the M8 plan's `cmd_daemon.go` ↔ `pkg/daemon/daemon.go` split.

---

**Roadmap reference:** §3 M10 — Web Dashboard (`docs/superpowers/specs/2026-04-28-m7-m11-roadmap-design.md`).
**Spec:** `docs/superpowers/specs/2026-04-28-m10a-dashboard-scaffold-design.md`.
