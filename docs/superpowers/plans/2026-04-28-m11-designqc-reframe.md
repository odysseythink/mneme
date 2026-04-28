# M11 — Design QC + Reframe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `mneme designqc` (chromedp headless Chrome capture for Vite projects), flip the M10c DesignQC dashboard panel from empty-state to a live thumbnail grid, and install a 12-framework Reframe knowledge base into every project on `mneme init` / `mneme update`.

**Architecture:** A new `pkg/designqc` package owns the capture pipeline. `pkg/designqc.Run` orchestrates: `DetectChrome` → `DetectFramework` (Vite-only) → `WaitForDevServer` → `EnumerateRoutes` (or `--route` override) → wipe and recreate `~/.mneme/designqc/<id>/captures/` → loop captures with a single shared chromedp browser context → write `report.json`. The dashboard backend's M10c stub at `pkg/dashboard/designqc.go` is replaced with a live handler reading the report plus a sibling endpoint that serves JPEG bytes (with a strict path-traversal regex). The frontend panel becomes a 3-column grid with a click-to-expand overlay. The Reframe template is purely static markdown installed into `.mneme/reframe.md` via the existing `embed.FS` + `state.AtomicWrite` pattern from `pkg/installer/identity.go`.

**Tech Stack:** Go 1.25, `github.com/chromedp/chromedp` (NEW), `image/jpeg`, `net/http`, React 19, TypeScript 5.7. **No new frontend deps.**

**Spec:** `docs/superpowers/specs/2026-04-28-m11-designqc-reframe-design.md`

**Depends on M10c** being implemented (this plan assumes the M10c DesignQC stub exists and the `useActiveProject` / `useFetch` / `getDesignQC` primitives are in place).

---

## File structure

### Backend — created

| Path | Responsibility |
|---|---|
| `pkg/designqc/report.go` | `Report` and `Capture` structs + `WriteReport` / `ReadReport` |
| `pkg/designqc/detect.go` | `Framework` struct + `DetectFramework` (Vite only) |
| `pkg/designqc/routes.go` | `Route` struct + `EnumerateRoutes` (regex over `*router*.{ts,tsx,js,jsx}`) + slug derivation |
| `pkg/designqc/capture.go` | `DetectChrome` + `WaitForDevServer` + `Capture` (chromedp wrapper) |
| `pkg/designqc/runner.go` | `RunOptions` + `Run(ctx, opts)` orchestrator |
| `cmd/cmd_designqc.go` | flag parsing + `dispatchDesignQC` |
| `pkg/installer/reframe.go` | `InstallReframe(projectRoot)` |
| `pkg/installer/templates/reframe.md.tmpl` | 12-framework static knowledge base |
| `tests/designqc_report_test.go` | report R/W round-trip + corrupt + version |
| `tests/designqc_detect_test.go` | Vite config detection across `.ts` / `.js` / `.mjs` |
| `tests/designqc_routes_test.go` | route enumeration + slug derivation + override |
| `tests/designqc_capture_test.go` | chromedp capture against fixture HTML (skip if no Chrome) |
| `tests/designqc_runner_test.go` | runner orchestration with mocked `Capture` |
| `tests/dashboard_designqc_live_test.go` | live handler + path-traversal guard |
| `tests/integration/designqc_e2e_test.go` (`//go:build integration`) | real chromedp + httptest fixture site |

### Backend — modified

| Path | Change |
|---|---|
| `pkg/dashboard/designqc.go` | Replace M10c stub: live handler + bytes handler |
| `pkg/daemon/routes.go` | Register `/api/designqc/captures/` route |
| `cmd/main.go` | Register `designqc` subcommand dispatch |
| `pkg/installer/project.go` | Call `InstallReframe(root)` in init flow |
| `pkg/updater/updater.go` | Call `InstallReframe(root)` in per-project sync flow |
| `go.mod` / `go.sum` | Add `github.com/chromedp/chromedp` |

### Frontend — modified

| Path | Change |
|---|---|
| `web/src/api/types.ts` | Extend `DesignQCResponse` with live shape (`Report`, `Capture`) |
| `web/src/panels/DesignQC.tsx` | Replace M10c empty state with live grid + overlay |
| `web/src/__tests__/App.test.tsx` | Update mock fetch for `/api/designqc` so the smoke test still passes |

---

## Conventions used in this plan

**Project resolution helper.** All `/api/designqc*` handlers reuse the M10c `resolveProject(w, r)` helper from `pkg/dashboard/cerebrum.go`. Anything else needs no new helper.

**Chrome-not-found exit code.** CLI exits `2`. All other recoverable errors exit `1`.

**Test stub logger.** Every Go test that needs a `Logger` uses the existing `stubLogger{}` in `tests/`.

**Test seeding.** Tests reuse the `seedProject(t, home, projectID)` helper introduced in M10c (`tests/dashboard_cerebrum_test.go`). It sets `t.Setenv("HOME", home)`, creates `<home>/work/<id>/.mneme`, writes `.local-id`, and calls `state.WriteOrigin(id, root)`.

**Commit messages:** `feat(m11):` / `test(m11):` / `fix(m11):` / `docs(m11):`. Co-author trailer in interactive runs.

---

## Task 1: pkg/designqc/report.go — Report struct + R/W

**Files:**
- Create: `pkg/designqc/report.go`
- Create: `tests/designqc_report_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/designqc_report_test.go`:

```go
package tests

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/designqc"
)

func TestReport_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &designqc.Report{
		Version:    1,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Framework:  "vite",
		BaseURL:    "http://localhost:5173",
		Captures: []designqc.Capture{
			{Route: "/", File: "route-root.jpg", Width: 1440, Height: 2400, CapturedAtMS: 1714290000000},
		},
	}
	if err := designqc.WriteReport(dir, want); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	got, err := designqc.ReadReport(dir)
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}
	if got.Version != 1 || got.Framework != "vite" || len(got.Captures) != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.Captures[0].Route != "/" || got.Captures[0].File != "route-root.jpg" {
		t.Errorf("capture mismatch: %+v", got.Captures[0])
	}
}

func TestReport_MissingFile(t *testing.T) {
	_, err := designqc.ReadReport(filepath.Join(t.TempDir(), "no-such"))
	if err == nil {
		t.Error("ReadReport on missing dir should error")
	}
}

func TestReport_UnknownVersion(t *testing.T) {
	dir := t.TempDir()
	r := &designqc.Report{Version: 99, Framework: "future"}
	if err := designqc.WriteReport(dir, r); err != nil {
		t.Fatal(err)
	}
	_, err := designqc.ReadReport(dir)
	if err == nil {
		t.Error("ReadReport with unknown version should error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./tests -run TestReport -v
```

Expected: FAIL — `package github.com/ranwei/mneme/pkg/designqc` does not exist.

- [ ] **Step 3: Create `pkg/designqc/report.go`**

```go
package designqc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/state"
)

// ReportVersion is the only supported on-disk schema version in M11.
const ReportVersion = 1

// Report is the on-disk record of a designqc run.
type Report struct {
	Version    int       `json:"version"`
	CapturedAt string    `json:"captured_at"` // RFC3339
	Framework  string    `json:"framework"`
	BaseURL    string    `json:"base_url"`
	Captures   []Capture `json:"captures"`
}

// Capture is a single route's screenshot record.
type Capture struct {
	Route        string `json:"route"`
	File         string `json:"file"` // basename within captures/
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CapturedAtMS int64  `json:"captured_at_ms"`
	Error        string `json:"error,omitempty"`
}

// reportFile is the JSON file inside the run dir.
const reportFile = "report.json"

// WriteReport writes report.json into runDir (creates dir if missing).
func WriteReport(runDir string, r *Report) error {
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return err
	}
	if r.Captures == nil {
		r.Captures = []Capture{}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(filepath.Join(runDir, reportFile), data)
}

// ReadReport loads and validates the report. Errors on missing file
// or unsupported version.
func ReadReport(runDir string) (*Report, error) {
	data, err := os.ReadFile(filepath.Join(runDir, reportFile))
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	if r.Version != ReportVersion {
		return nil, fmt.Errorf("unsupported report version %d (want %d)", r.Version, ReportVersion)
	}
	return &r, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./tests -run TestReport -v
```

Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/designqc/report.go tests/designqc_report_test.go
git commit -m "feat(m11): pkg/designqc — Report struct + R/W with version guard"
```

---

## Task 2: pkg/designqc/detect.go — Vite framework detection

**Files:**
- Create: `pkg/designqc/detect.go`
- Create: `tests/designqc_detect_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/designqc_detect_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/designqc"
)

func TestDetectFramework_DefaultPort(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "vite.config.ts"),
		[]byte("export default { plugins: [] }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fw, err := designqc.DetectFramework(root)
	if err != nil {
		t.Fatalf("DetectFramework: %v", err)
	}
	if fw.Name != "vite" || fw.DefaultPort != 5173 {
		t.Errorf("got %+v, want vite/5173", fw)
	}
}

func TestDetectFramework_ExplicitPort(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "vite.config.js"),
		[]byte("export default {\n  server: { port: 4000 },\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fw, err := designqc.DetectFramework(root)
	if err != nil {
		t.Fatal(err)
	}
	if fw.DefaultPort != 4000 {
		t.Errorf("DefaultPort = %d, want 4000", fw.DefaultPort)
	}
}

func TestDetectFramework_NotFound(t *testing.T) {
	_, err := designqc.DetectFramework(t.TempDir())
	if err == nil {
		t.Error("expected error when no Vite config present")
	}
}

func TestDetectFramework_DynamicPortFallsBack(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "vite.config.mjs"),
		[]byte("export default { server: { port: process.env.PORT } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fw, err := designqc.DetectFramework(root)
	if err != nil {
		t.Fatal(err)
	}
	if fw.DefaultPort != 5173 {
		t.Errorf("non-literal port: got %d, want default 5173", fw.DefaultPort)
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestDetectFramework -v
```

Expected: FAIL — `designqc.DetectFramework` undefined.

- [ ] **Step 3: Create `pkg/designqc/detect.go`**

```go
package designqc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Framework is the detected frontend framework + base config.
type Framework struct {
	Name        string // "vite"
	DefaultPort int    // 5173 unless overridden by literal in config
}

// BaseURL builds the dev-server URL using the framework's port.
func (f *Framework) BaseURL() string {
	return fmt.Sprintf("http://localhost:%d", f.DefaultPort)
}

var viteConfigNames = []string{"vite.config.ts", "vite.config.js", "vite.config.mjs"}
var portRegex = regexp.MustCompile(`port:\s*(\d+)`)

// DetectFramework returns the framework rooted at projectRoot.
// Currently supports Vite only; other frameworks return an error.
func DetectFramework(projectRoot string) (*Framework, error) {
	for _, name := range viteConfigNames {
		path := filepath.Join(projectRoot, name)
		if data, err := os.ReadFile(path); err == nil {
			port := 5173
			if m := portRegex.FindSubmatch(data); m != nil {
				if n, err := strconv.Atoi(string(m[1])); err == nil && n > 0 {
					port = n
				}
			}
			return &Framework{Name: "vite", DefaultPort: port}, nil
		}
	}
	return nil, errors.New("no Vite config found; pass --route /path to skip auto-detection")
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./tests -run TestDetectFramework -v
```

Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/designqc/detect.go tests/designqc_detect_test.go
git commit -m "feat(m11): pkg/designqc — Vite framework detection"
```

---

## Task 3: pkg/designqc/routes.go — route enumeration + slug

**Files:**
- Create: `pkg/designqc/routes.go`
- Create: `tests/designqc_routes_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/designqc_routes_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/designqc"
)

func TestEnumerateRoutes_FromRouter(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatal(err)
	}
	content := `
import { createBrowserRouter } from 'react-router-dom'
const router = createBrowserRouter([
  { path: "/", element: null },
  { path: "/about", element: null },
  { path: "/users/:id", element: null },
])
`
	if err := os.WriteFile(filepath.Join(src, "router.ts"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := designqc.EnumerateRoutes(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("routes: got %d, want 3 (%v)", len(got), got)
	}
	// Sorted alphabetically by Path.
	want := []string{"/", "/about", "/users/:id"}
	for i, r := range got {
		if r.Path != want[i] {
			t.Errorf("[%d] path = %q, want %q", i, r.Path, want[i])
		}
	}
}

func TestEnumerateRoutes_Override(t *testing.T) {
	got, err := designqc.EnumerateRoutes(t.TempDir(), []string{"/x", "/y"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Path != "/x" || got[1].Path != "/y" {
		t.Errorf("override: %+v", got)
	}
}

func TestEnumerateRoutes_FallbackRoot(t *testing.T) {
	// No router file → fallback to "/".
	got, err := designqc.EnumerateRoutes(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/" || got[0].Slug != "root" {
		t.Errorf("fallback: %+v", got)
	}
}

func TestEnumerateRoutes_SlugDerivation(t *testing.T) {
	cases := map[string]string{
		"/":            "root",
		"/about":       "about",
		"/users/:id":   "users-id",
		"/foo/bar":     "foo-bar",
		"/with spaces": "with-spaces",
	}
	for path, wantSlug := range cases {
		got := designqc.SlugForPath(path)
		if got != wantSlug {
			t.Errorf("slug(%q) = %q, want %q", path, got, wantSlug)
		}
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestEnumerateRoutes -v
```

Expected: FAIL — undefined.

- [ ] **Step 3: Create `pkg/designqc/routes.go`**

```go
package designqc

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Route is one path to capture.
type Route struct {
	Path string
	Slug string
}

var (
	routerFileRegex = regexp.MustCompile(`(?i)router.*\.(ts|tsx|js|jsx)$`)
	pathLiteral     = regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`)
	nonAlnum        = regexp.MustCompile(`[^a-z0-9-]+`)
)

// EnumerateRoutes returns the routes to capture. If override is non-empty,
// it is used verbatim. Otherwise routes are extracted from src/*router*.{ts,tsx,js,jsx}.
// Empty result falls back to [{Path:"/", Slug:"root"}].
func EnumerateRoutes(projectRoot string, override []string) ([]Route, error) {
	if len(override) > 0 {
		out := make([]Route, 0, len(override))
		for _, p := range override {
			out = append(out, Route{Path: p, Slug: SlugForPath(p)})
		}
		return out, nil
	}

	srcDir := filepath.Join(projectRoot, "src")
	seen := map[string]struct{}{}
	_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !routerFileRegex.MatchString(d.Name()) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, m := range pathLiteral.FindAllSubmatch(data, -1) {
			seen[string(m[1])] = struct{}{}
		}
		return nil
	})

	if len(seen) == 0 {
		return []Route{{Path: "/", Slug: "root"}}, nil
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]Route, 0, len(paths))
	for _, p := range paths {
		out = append(out, Route{Path: p, Slug: SlugForPath(p)})
	}
	return out, nil
}

// SlugForPath produces a filesystem-safe slug for a route path.
// "/" → "root", "/about" → "about", "/users/:id" → "users-id".
func SlugForPath(p string) string {
	if p == "" || p == "/" {
		return "root"
	}
	s := strings.TrimPrefix(p, "/")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "root"
	}
	return s
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./tests -run TestEnumerateRoutes -v
```

Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/designqc/routes.go tests/designqc_routes_test.go
git commit -m "feat(m11): pkg/designqc — route enumeration + slug derivation"
```

---

## Task 4: pkg/designqc/capture.go — chromedp capture

**Files:**
- Modify: `go.mod` / `go.sum`
- Create: `pkg/designqc/capture.go`
- Create: `tests/designqc_capture_test.go`

- [ ] **Step 1: Add the chromedp dep**

```bash
go get github.com/chromedp/chromedp@latest
go mod tidy
```

Expected: `go.mod` gains `github.com/chromedp/chromedp` and its transitive deps.

- [ ] **Step 2: Write the failing test**

Create `tests/designqc_capture_test.go`:

```go
package tests

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/designqc"
)

const fixtureHTML = `<!doctype html><html><head><title>fix</title></head>
<body style="margin:0">
<div style="width:1440px;height:600px;background:#222;color:#fff;font:48px sans-serif;display:flex;align-items:center;justify-content:center">
hello mneme
</div>
</body></html>`

func TestCapture_HappyPath(t *testing.T) {
	if _, err := designqc.DetectChrome(); err != nil {
		t.Skipf("Chrome not available: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fixtureHTML))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	br := designqc.NewBrowser(ctx)
	defer br.Close()

	jpeg, w, h, err := br.Capture(designqc.CaptureOptions{
		URL: srv.URL, Quality: 80, MaxWidth: 1440, Timeout: 25 * time.Second,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !bytes.HasPrefix(jpeg, []byte{0xFF, 0xD8, 0xFF}) {
		t.Errorf("not a JPEG: first bytes %x", jpeg[:4])
	}
	if w == 0 || h == 0 {
		t.Errorf("dims: %dx%d", w, h)
	}
}

func TestWaitForDevServer_Down(t *testing.T) {
	// Port 1 is reserved + nothing listens. Should fail fast.
	err := designqc.WaitForDevServer("http://127.0.0.1:1", 200*time.Millisecond)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestWaitForDevServer_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer srv.Close()
	if err := designqc.WaitForDevServer(srv.URL, 1*time.Second); err != nil {
		t.Errorf("WaitForDevServer: %v", err)
	}
}
```

- [ ] **Step 3: Run test**

```bash
go test ./tests -run "TestCapture|TestWaitForDevServer" -v
```

Expected: FAIL — undefined.

- [ ] **Step 4: Create `pkg/designqc/capture.go`**

```go
package designqc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"
)

// CaptureOptions controls a single screenshot.
type CaptureOptions struct {
	URL      string
	Quality  int           // 1..100
	MaxWidth int           // pixels
	Timeout  time.Duration // per-route
}

// Browser owns a long-lived chromedp context shared across routes in one run.
type Browser struct {
	allocCancel context.CancelFunc
	ctxCancel   context.CancelFunc
	browserCtx  context.Context
}

// NewBrowser launches Chrome in headless mode and returns a Browser ready
// for many Capture calls. Caller MUST call Close when done.
func NewBrowser(parent context.Context) *Browser {
	allocCtx, allocCancel := chromedp.NewExecAllocator(parent,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	browserCtx, ctxCancel := chromedp.NewContext(allocCtx)
	return &Browser{
		allocCancel: allocCancel,
		ctxCancel:   ctxCancel,
		browserCtx:  browserCtx,
	}
}

// Close tears down the browser and allocator.
func (b *Browser) Close() {
	if b.ctxCancel != nil {
		b.ctxCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
}

// Capture navigates to opts.URL, waits for body, then takes a full-page JPEG.
func (b *Browser) Capture(opts CaptureOptions) ([]byte, int, int, error) {
	if opts.Quality <= 0 || opts.Quality > 100 {
		opts.Quality = 80
	}
	if opts.MaxWidth <= 0 {
		opts.MaxWidth = 1440
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(b.browserCtx, opts.Timeout)
	defer cancel()

	var buf []byte
	var dims [2]int
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(opts.MaxWidth), 900),
		chromedp.Navigate(opts.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.FullScreenshot(&buf, opts.Quality),
		chromedp.Evaluate(
			`[document.documentElement.scrollWidth, document.documentElement.scrollHeight]`,
			&dims,
		),
	)
	if err != nil {
		return nil, 0, 0, err
	}
	return buf, dims[0], dims[1], nil
}

// DetectChrome returns the path to a usable Chrome/Chromium binary, or
// an error with actionable install instructions.
func DetectChrome() (string, error) {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		)
	case "linux":
		// PATH-based candidates; checked via exec.LookPath below.
		candidates = append(candidates, "google-chrome", "chromium", "chromium-browser", "google-chrome-stable")
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", errors.New(
		"Chrome/Chromium not found. Install via:\n" +
			"  macOS:  brew install --cask google-chrome\n" +
			"  Linux:  apt-get install chromium-browser\n" +
			"Or set CHROME_PATH=/path/to/chrome",
	)
}

// WaitForDevServer GETs baseURL with the given timeout. Any HTTP response
// (2xx/3xx/4xx/5xx) counts as "ready". Connection refused / timeout returns error.
func WaitForDevServer(baseURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(baseURL + "/")
	if err != nil {
		return fmt.Errorf("dev server not reachable at %s; run \"pnpm dev\" in another terminal first", baseURL)
	}
	resp.Body.Close()
	return nil
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run "TestCapture|TestWaitForDevServer" -v
```

Expected: 3 PASS (TestCapture skips locally if Chrome is absent).

- [ ] **Step 6: Commit**

```bash
git add pkg/designqc/capture.go tests/designqc_capture_test.go go.mod go.sum
git commit -m "feat(m11): pkg/designqc — chromedp Browser + Capture + DetectChrome + WaitForDevServer"
```

---

## Task 5: pkg/designqc/runner.go — orchestrator

**Files:**
- Create: `pkg/designqc/runner.go`
- Create: `tests/designqc_runner_test.go`

The runner test mocks Capture so it doesn't require Chrome, and exercises wipe-on-rerun + partial-failure semantics.

- [ ] **Step 1: Write the failing test**

Create `tests/designqc_runner_test.go`:

```go
package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/designqc"
	"github.com/ranwei/mneme/pkg/state"
)

// fakeCapturer implements designqc.Capturer for tests without Chrome.
type fakeCapturer struct {
	failOn map[string]bool // route paths that should error
}

func (f *fakeCapturer) Capture(opts designqc.CaptureOptions) ([]byte, int, int, error) {
	if f.failOn != nil {
		// URL ends with the route path; check the last path-component.
		for path := range f.failOn {
			if path != "" && path != "/" && len(opts.URL) >= len(path) && opts.URL[len(opts.URL)-len(path):] == path {
				return nil, 0, 0, errors.New("simulated failure")
			}
		}
	}
	return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, 1440, 800, nil
}
func (f *fakeCapturer) Close() {}

func TestRunner_WipesPreviousCaptures(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	// Pre-create a stale captures directory with junk.
	dir := filepath.Join(home, ".mneme", "designqc", "p1", "captures")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "stale.jpg"), []byte("old"), 0o600)

	report, err := designqc.RunWithCapturer(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		RouteOverride: []string{"/", "/about"},
		BaseURL:       "http://localhost:5173",
		Framework:     "vite",
	}, &fakeCapturer{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Captures) != 2 {
		t.Errorf("got %d captures, want 2", len(report.Captures))
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.jpg")); err == nil {
		t.Error("stale capture not wiped")
	}
}

func TestRunner_PartialFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	report, err := designqc.RunWithCapturer(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		RouteOverride: []string{"/", "/broken"},
		BaseURL:       "http://localhost:5173",
		Framework:     "vite",
	}, &fakeCapturer{failOn: map[string]bool{"/broken": true}})
	if err != nil {
		t.Fatalf("Run (partial): %v", err)
	}
	if len(report.Captures) != 2 {
		t.Fatalf("got %d captures, want 2", len(report.Captures))
	}
	got := map[string]string{}
	for _, c := range report.Captures {
		got[c.Route] = c.Error
	}
	if got["/"] != "" {
		t.Errorf("/ should succeed, got error %q", got["/"])
	}
	if got["/broken"] == "" {
		t.Errorf("/broken should have error")
	}
}

func TestRunner_AllFail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	_, err := designqc.RunWithCapturer(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		RouteOverride: []string{"/x"},
		BaseURL:       "http://localhost:5173",
		Framework:     "vite",
	}, &fakeCapturer{failOn: map[string]bool{"/x": true}})
	if err == nil {
		t.Error("expected error when all routes fail")
	}
	if _, statErr := state.ReadOrigin("p1"); statErr != nil {
		// project still seeded; sanity check.
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestRunner -v
```

Expected: FAIL — `RunWithCapturer` undefined.

- [ ] **Step 3: Create `pkg/designqc/runner.go`**

```go
package designqc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunOptions configures one designqc run.
type RunOptions struct {
	ProjectRoot   string
	ProjectID     string
	HomeDir       string
	RouteOverride []string
	Quality       int
	MaxWidth      int
	Port          int    // 0 = use Framework default
	BaseURL       string // pre-resolved by Run; tests pass it directly
	Framework     string // "vite"
}

// Capturer is the chromedp interface that Run depends on. Production code
// uses *Browser; tests substitute a fake.
type Capturer interface {
	Capture(opts CaptureOptions) ([]byte, int, int, error)
	Close()
}

// Run is the production entry point. It auto-detects framework, probes the
// dev server, enumerates routes, and delegates the actual capture loop to
// RunWithCapturer.
func Run(ctx context.Context, opts RunOptions) (*Report, error) {
	if _, err := DetectChrome(); err != nil {
		return nil, err
	}

	// Resolve framework + base URL unless override + explicit port supply both.
	if opts.Framework == "" {
		fw, err := DetectFramework(opts.ProjectRoot)
		if err != nil {
			return nil, err
		}
		opts.Framework = fw.Name
		if opts.Port == 0 {
			opts.Port = fw.DefaultPort
		}
	}
	if opts.BaseURL == "" {
		port := opts.Port
		if port == 0 {
			port = 5173
		}
		opts.BaseURL = fmt.Sprintf("http://localhost:%d", port)
	}

	if err := WaitForDevServer(opts.BaseURL, 1*time.Second); err != nil {
		return nil, err
	}

	br := NewBrowser(ctx)
	defer br.Close()
	return RunWithCapturer(ctx, opts, br)
}

// RunWithCapturer is the testable core. It assumes framework + BaseURL
// have already been resolved.
func RunWithCapturer(ctx context.Context, opts RunOptions, cap Capturer) (*Report, error) {
	routes, err := EnumerateRoutes(opts.ProjectRoot, opts.RouteOverride)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, errors.New("no routes to capture")
	}

	runDir := filepath.Join(opts.HomeDir, ".mneme", "designqc", opts.ProjectID)
	capturesDir := filepath.Join(runDir, "captures")
	// Wipe + recreate.
	if err := os.RemoveAll(capturesDir); err != nil {
		return nil, fmt.Errorf("wipe captures: %w", err)
	}
	if err := os.MkdirAll(capturesDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir captures: %w", err)
	}

	report := &Report{
		Version:    ReportVersion,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Framework:  opts.Framework,
		BaseURL:    opts.BaseURL,
		Captures:   []Capture{},
	}

	successCount := 0
	for _, r := range routes {
		url := strings.TrimRight(opts.BaseURL, "/") + r.Path
		fileName := fmt.Sprintf("route-%s.jpg", r.Slug)
		jpeg, w, h, err := cap.Capture(CaptureOptions{
			URL: url, Quality: opts.Quality, MaxWidth: opts.MaxWidth,
		})
		entry := Capture{
			Route:        r.Path,
			File:         fileName,
			CapturedAtMS: time.Now().UnixMilli(),
		}
		if err != nil {
			entry.Error = err.Error()
			fmt.Fprintf(os.Stderr, "designqc: warn: route %s: %v\n", r.Path, err)
			report.Captures = append(report.Captures, entry)
			continue
		}
		path := filepath.Join(capturesDir, fileName)
		if werr := os.WriteFile(path, jpeg, 0o600); werr != nil {
			entry.Error = werr.Error()
			report.Captures = append(report.Captures, entry)
			continue
		}
		entry.Width = w
		entry.Height = h
		report.Captures = append(report.Captures, entry)
		successCount++
	}

	if werr := WriteReport(runDir, report); werr != nil {
		return nil, fmt.Errorf("write report: %w", werr)
	}
	if successCount == 0 {
		return report, errors.New("all routes failed to capture")
	}
	return report, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./tests -run TestRunner -v
```

Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/designqc/runner.go tests/designqc_runner_test.go
git commit -m "feat(m11): pkg/designqc — Run orchestrator + testable Capturer interface"
```

---

## Task 6: cmd/cmd_designqc.go + main dispatch

**Files:**
- Create: `cmd/cmd_designqc.go`
- Modify: `cmd/main.go`

- [ ] **Step 1: Create `cmd/cmd_designqc.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ranwei/mneme/pkg/designqc"
	"github.com/ranwei/mneme/pkg/state"
)

type stringSlice []string

func (s *stringSlice) String() string     { return fmt.Sprint(*s) }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

func dispatchDesignQC(args []string) {
	fs := flag.NewFlagSet("designqc", flag.ExitOnError)
	var routes stringSlice
	fs.Var(&routes, "route", "Capture only this route (repeatable). Skips auto-detection.")
	port := fs.Int("port", 0, "Override dev server port (0 = framework default).")
	quality := fs.Int("quality", 80, "JPEG quality 1-100.")
	maxWidth := fs.Int("max-width", 1440, "Browser viewport width.")
	asJSON := fs.Bool("json", false, "Print report.json to stdout instead of human summary.")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		os.Exit(1)
	}
	root, ok := state.FindGitRoot(cwd)
	if !ok {
		root = cwd
	}
	if _, err := os.Stat(root + "/.mneme"); err != nil {
		fmt.Fprintln(os.Stderr, "designqc: no .mneme/ directory; run `mneme init` first")
		os.Exit(1)
	}
	id, err := state.ReadOrCreateLocalID(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		os.Exit(1)
	}

	report, err := designqc.Run(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     id,
		HomeDir:       home,
		RouteOverride: routes,
		Quality:       *quality,
		MaxWidth:      *maxWidth,
		Port:          *port,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		// Distinguish Chrome-not-found (exit 2) from everything else (exit 1).
		if _, derr := designqc.DetectChrome(); derr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}

	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		return
	}
	fmt.Printf("mneme designqc — %s project at %s\n", report.Framework, root)
	fmt.Printf("captures: %d routes\n", len(report.Captures))
	for _, c := range report.Captures {
		if c.Error != "" {
			fmt.Printf("  %-10s → ERROR: %s\n", c.Route, c.Error)
			continue
		}
		fmt.Printf("  %-10s → captures/%s (%d×%d)\n", c.Route, c.File, c.Width, c.Height)
	}
	fmt.Printf("report:    %s/.mneme/designqc/%s/report.json\n", home, id)
}
```

- [ ] **Step 2: Modify `cmd/main.go`**

In the `switch os.Args[1]` block, add a new case alongside the others (alphabetical neighbor: between `dashboard` and `version` is fine):

```go
	case "dashboard":
		dispatchDashboard(os.Args[2:])
	case "designqc":
		dispatchDesignQC(os.Args[2:])
	case "version":
		printVersion()
```

- [ ] **Step 3: Build + sanity check**

```bash
go build -o ./bin/mneme ./cmd
./bin/mneme designqc --help 2>&1 | head -10
```

Expected: build succeeds; `--help` lists the four flags.

- [ ] **Step 4: Commit**

```bash
git add cmd/cmd_designqc.go cmd/main.go
git commit -m "feat(m11): mneme designqc CLI"
```

---

## Task 7: Reframe template content + installer

**Files:**
- Create: `pkg/installer/templates/reframe.md.tmpl`
- Create: `pkg/installer/reframe.go`
- Create: `tests/installer_reframe_test.go`

- [ ] **Step 1: Create the template content**

Create `pkg/installer/templates/reframe.md.tmpl`:

````markdown
<!-- mneme reframe v1 -->
# Reframe — Frontend Framework Knowledge Base

This is a reference for choosing or migrating between frontend frameworks. Claude reads this file to ground recommendations in concrete tradeoffs.

<!-- mneme:user-section start -->
<!-- Add project-specific notes here. Preserved across mneme update. -->
<!-- mneme:user-section end -->

## Next.js

**When to suggest:** Production React apps that need SSR/SSG/ISR, file-based routing, image optimization, and a single full-stack story for API routes. Strong fit for marketing + product surfaces in the same codebase.
**When NOT to suggest:** Pure SPA with no server-side rendering needs (overhead is real). Teams that don't want to be in the React/Vercel ecosystem.
**Migration cost from Vite (raw React):** medium — keep components, replace router + add Next config + adapt data-fetching to RSC.
**Docs:** https://nextjs.org/docs

## Vite (raw React / Solid / Vue)

**When to suggest:** Pure SPA, fast dev server, minimal config, no SSR needs. Great for internal tools, dashboards, embedded apps.
**When NOT to suggest:** Apps that need SEO/server-rendered content; multi-page sites with content workflows.
**Migration cost from create-react-app:** low — drop CRA, adopt Vite config, update imports.
**Docs:** https://vitejs.dev/guide/

## Astro

**When to suggest:** Content-first sites (blogs, docs, marketing) where most pages are static and only some islands need interactivity. MDX + per-component framework choice (React/Solid/Svelte mixable).
**When NOT to suggest:** Highly interactive applications (the islands model fights you). SSR-only dynamic content needing a request lifecycle.
**Migration cost from Next.js:** medium — Astro pages replace Next pages; client components opt in via `client:*` directives.
**Docs:** https://docs.astro.build

## SvelteKit

**When to suggest:** Teams that want minimal runtime, exceptional DX, and full-stack ergonomics in Svelte. Great for product apps where bundle size matters.
**When NOT to suggest:** Teams committed to React/Vue. Ecosystems that need many React-only libraries (still possible via wrappers, but friction).
**Migration cost from Next.js:** high — Svelte's syntax differs from JSX; component code must be rewritten.
**Docs:** https://kit.svelte.dev/docs

## Remix

**When to suggest:** Apps that need progressive enhancement, web-platform-first form handling, nested routing with data, and standard Web APIs (fetch, FormData) over framework abstractions.
**When NOT to suggest:** Static-first sites or pure SPAs (Remix's value is in the request/response loop).
**Migration cost from Next.js:** medium — keep React components, rewrite data loading + routing to Remix's loader/action model.
**Docs:** https://remix.run/docs

## Nuxt

**When to suggest:** Vue's full-stack equivalent of Next.js. Auto-imports, file-based routing, hybrid rendering. Pick when the team is on Vue 3.
**When NOT to suggest:** React/Svelte teams. Apps that need cutting-edge React-only patterns.
**Migration cost from Next.js:** high — components must be rewritten in Vue's SFC syntax.
**Docs:** https://nuxt.com/docs

## Solid Start

**When to suggest:** Apps that want fine-grained reactivity (Solid) with full-stack capabilities. Excellent runtime performance.
**When NOT to suggest:** Teams that need a large ecosystem of components/libraries (Solid's library set is smaller). Beta-tier stability concerns.
**Migration cost from React:** medium-high — JSX-similar syntax but reactivity model differs (signals over hooks).
**Docs:** https://docs.solidjs.com/solid-start

## Qwik / Qwik City

**When to suggest:** Apps where Time-to-Interactive matters above all (e-commerce, ads-heavy pages). Resumability eliminates hydration cost.
**When NOT to suggest:** Apps with light interactivity where the resumability complexity is not justified. Teams unfamiliar with the model.
**Migration cost from React:** high — JSX-like but execution model is fundamentally different.
**Docs:** https://qwik.dev/docs/

## Fresh (Deno)

**When to suggest:** Deno-native shops; edge-first deployments (Deno Deploy); islands architecture without hydration overhead.
**When NOT to suggest:** Node.js shops (operational cost of switching runtimes is high). Apps needing the Node ecosystem.
**Migration cost from Next.js:** high — runtime + framework + module system all change.
**Docs:** https://fresh.deno.dev/docs

## Gatsby

**When to suggest:** Content-heavy SSG sites with many data sources behind a unified GraphQL layer (Contentful, Sanity, markdown).
**When NOT to suggest:** Apps that need SSR or anything dynamic at runtime. Fresh greenfield work — Astro/Next have largely caught up while being simpler.
**Migration cost from Next.js:** medium — pages move; data fetching shifts from RSC/getStaticProps to GraphQL queries.
**Docs:** https://www.gatsbyjs.com/docs/

## Ember

**When to suggest:** Long-lived enterprise apps that benefit from convention over configuration. Strong batteries-included philosophy and stable APIs across major versions.
**When NOT to suggest:** Greenfield SPAs targeting the modern React/Svelte ecosystem. Smaller teams that don't want to learn Ember's strong opinions.
**Migration cost from React:** very high — different component model, different routing, different data layer.
**Docs:** https://guides.emberjs.com/

## Angular

**When to suggest:** Enterprise SPAs with large teams that benefit from TypeScript-first DI, opinionated module structure, RxJS integration, and Google's long-term maintenance.
**When NOT to suggest:** Smaller teams or apps where the framework's surface area is overhead. Greenfield work where bundle size is critical.
**Migration cost from React:** very high — completely different mental model (DI, decorators, modules).
**Docs:** https://angular.dev/overview
````

- [ ] **Step 2: Write the failing test**

Create `tests/installer_reframe_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
)

func TestInstallReframe_CreatesFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mneme"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := installer.InstallReframe(root); err != nil {
		t.Fatalf("InstallReframe: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mneme", "reframe.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, name := range []string{
		"Next.js", "Vite", "Astro", "SvelteKit", "Remix", "Nuxt",
		"Solid Start", "Qwik", "Fresh", "Gatsby", "Ember", "Angular",
	} {
		if !strings.Contains(content, name) {
			t.Errorf("missing framework %q in installed reframe.md", name)
		}
	}
}

func TestInstallReframe_OverwritesExisting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mneme")
	os.MkdirAll(dir, 0o700)
	old := []byte("ancient content")
	os.WriteFile(filepath.Join(dir, "reframe.md"), old, 0o600)

	if err := installer.InstallReframe(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "reframe.md"))
	if string(data) == "ancient content" {
		t.Error("InstallReframe did not overwrite stale file")
	}
}
```

- [ ] **Step 3: Run test**

```bash
go test ./tests -run TestInstallReframe -v
```

Expected: FAIL — `installer.InstallReframe` undefined.

- [ ] **Step 4: Create `pkg/installer/reframe.go`**

```go
package installer

import (
	_ "embed"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/state"
)

//go:embed templates/reframe.md.tmpl
var reframeTemplate string

// InstallReframe writes the bundled Reframe knowledge base to
// <projectRoot>/.mneme/reframe.md, overwriting any existing copy.
// Identical content across projects (no per-project rendering).
func InstallReframe(projectRoot string) error {
	dst := filepath.Join(projectRoot, ".mneme", "reframe.md")
	return state.AtomicWrite(dst, []byte(reframeTemplate))
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestInstallReframe -v
```

Expected: 2 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/installer/reframe.go pkg/installer/templates/reframe.md.tmpl tests/installer_reframe_test.go
git commit -m "feat(m11): Reframe knowledge base — 12-framework template + installer"
```

---

## Task 8: Wire InstallReframe into init + update

**Files:**
- Modify: `pkg/installer/project.go`
- Modify: `pkg/updater/updater.go`

- [ ] **Step 1: Inspect existing call sites**

```bash
grep -n "InstallIdentity\|InstallMnemeMD\|InstallRules" pkg/installer/project.go pkg/updater/updater.go
```

Find the existing block where identity/mneme.md/rules are installed; add `InstallReframe(root)` alongside.

- [ ] **Step 2: Modify `pkg/installer/project.go`**

Right after the existing call to `InstallIdentity(root, ...)` (or whichever sibling installer is closest):

```go
if err := InstallReframe(root); err != nil {
	return fmt.Errorf("install reframe: %w", err)
}
```

- [ ] **Step 3: Modify `pkg/updater/updater.go`**

Find the per-project sync function (named `syncOne` per `pkg/updater/updater.go:69`). After the call that re-emits the identity/mneme.md/rules.md template, add:

```go
if err := installer.InstallReframe(projectRoot); err != nil {
	return fmt.Errorf("update reframe: %w", err)
}
```

(Add `"github.com/ranwei/mneme/pkg/installer"` to imports if not already present.)

- [ ] **Step 4: Run all tests**

```bash
go test ./tests -count=1
```

Expected: PASS — no existing tests assert that init/update do *not* write reframe.md, so adding this is non-breaking.

- [ ] **Step 5: Manual smoke**

```bash
go build -o ./bin/mneme ./cmd
mkdir -p /tmp/m11-smoke && cd /tmp/m11-smoke && ../../path/to/repo/bin/mneme init --yes 2>&1 | tail
ls .mneme/reframe.md
head -3 .mneme/reframe.md
```

Expected: `.mneme/reframe.md` exists; first line is `<!-- mneme reframe v1 -->`.

- [ ] **Step 6: Commit**

```bash
git add pkg/installer/project.go pkg/updater/updater.go
git commit -m "feat(m11): wire InstallReframe into mneme init + update"
```

---

## Task 9: pkg/dashboard/designqc.go — replace M10c stub with live handler

**Files:**
- Modify: `pkg/dashboard/designqc.go`
- Create: `tests/dashboard_designqc_live_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/dashboard_designqc_live_test.go`:

```go
package tests

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/designqc"
	"github.com/ranwei/mneme/pkg/events"
)

func TestAPI_DesignQC_NoReport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/designqc?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Available bool   `json:"available"`
		Reason    string `json:"reason"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Available {
		t.Error("Available should be false when report.json missing")
	}
	if body.Reason == "" {
		t.Error("Reason should be populated when unavailable")
	}
}

func TestAPI_DesignQC_LiveReport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	runDir := filepath.Join(home, ".mneme", "designqc", "p1")
	os.MkdirAll(filepath.Join(runDir, "captures"), 0o700)
	if err := os.WriteFile(filepath.Join(runDir, "captures", "route-root.jpg"),
		[]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := designqc.WriteReport(runDir, &designqc.Report{
		Version:    1,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Framework:  "vite",
		BaseURL:    "http://localhost:5173",
		Captures: []designqc.Capture{
			{Route: "/", File: "route-root.jpg", Width: 1440, Height: 900, CapturedAtMS: 1714290000000},
		},
	}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/designqc?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Available bool                  `json:"available"`
		Report    *designqc.Report      `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Available {
		t.Errorf("Available should be true; reason=%s", rec.Body.String())
	}
	if body.Report == nil || len(body.Report.Captures) != 1 {
		t.Errorf("report mismatch: %+v", body.Report)
	}
}

func TestAPI_DesignQC_UnknownVersion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	runDir := filepath.Join(home, ".mneme", "designqc", "p1")
	os.MkdirAll(runDir, 0o700)
	os.WriteFile(filepath.Join(runDir, "report.json"),
		[]byte(`{"version":99,"framework":"future"}`), 0o600)

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/designqc?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body struct {
		Available bool   `json:"available"`
		Reason    string `json:"reason"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Available {
		t.Error("Available should be false on unknown version")
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestAPI_DesignQC -v
```

Expected: FAIL — current M10c stub always returns `available: false` regardless of file presence.

- [ ] **Step 3: Replace `pkg/dashboard/designqc.go` (M10c stub)**

Replace the entire file content:

```go
package dashboard

import (
	"net/http"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/designqc"
)

// DesignQCHandler reads ~/.mneme/designqc/<project-id>/report.json. When the
// file is missing or the schema version is unknown, returns a structured
// "not available" envelope rather than a 5xx error.
func DesignQCHandler(homeDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projectID, root := resolveProject(w, r)
		if root == "" {
			return
		}
		runDir := filepath.Join(homeDir, ".mneme", "designqc", projectID)
		report, err := designqc.ReadReport(runDir)
		if err != nil {
			// File missing OR unsupported version OR malformed.
			reason := "no captures yet — run mneme designqc"
			if report != nil || (err.Error() != "" && len(err.Error()) > 0) {
				// Use the underlying error if it's about version, fall back to default.
				reason = err.Error()
			}
			writeJSON(w, 200, map[string]any{
				"available": false,
				"reason":    reason,
				"captures":  []any{},
			})
			return
		}
		writeJSON(w, 200, map[string]any{
			"available": true,
			"report":    report,
		})
	}
}
```

- [ ] **Step 4: Update `pkg/daemon/routes.go` registration**

Replace the M10c registration line:

```go
mux.Handle("/api/designqc", dashboard.DesignQCHandler())
```

with:

```go
mux.Handle("/api/designqc", dashboard.DesignQCHandler(deps.Home))
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_DesignQC -v
```

Expected: 3 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/designqc.go pkg/daemon/routes.go tests/dashboard_designqc_live_test.go
git commit -m "feat(m11): /api/designqc reads live report.json (replaces M10c stub)"
```

---

## Task 10: Captures bytes endpoint with path-traversal guard

**Files:**
- Modify: `pkg/dashboard/designqc.go`
- Modify: `pkg/daemon/routes.go`
- Modify: `tests/dashboard_designqc_live_test.go`

- [ ] **Step 1: Add the failing tests**

Append to `tests/dashboard_designqc_live_test.go`:

```go
func TestAPI_DesignQC_Captures_OK(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	dir := filepath.Join(home, ".mneme", "designqc", "p1", "captures")
	os.MkdirAll(dir, 0o700)
	want := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x42, 0x42}
	os.WriteFile(filepath.Join(dir, "route-root.jpg"), want, 0o600)

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/designqc/captures/p1/route-root.jpg", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Content-Type %q", rec.Header().Get("Content-Type"))
	}
	if !bytesEqual(rec.Body.Bytes(), want) {
		t.Errorf("body mismatch")
	}
}

func TestAPI_DesignQC_Captures_PathTraversal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	for _, name := range []string{"../etc/passwd", "..%2Fetc%2Fpasswd", "route-../foo.jpg", "evil.exe"} {
		req := httptest.NewRequest("GET", "/api/designqc/captures/p1/"+name, nil)
		req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != 404 {
			t.Errorf("traversal %q: got %d, want 404", name, rec.Code)
		}
	}
}

func TestAPI_DesignQC_Captures_Missing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/designqc/captures/p1/route-no-such.jpg", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("missing capture: got %d, want 404", rec.Code)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestAPI_DesignQC_Captures -v
```

Expected: FAIL — endpoint not registered.

- [ ] **Step 3: Add the bytes handler in `pkg/dashboard/designqc.go`**

Append to the file:

```go
import "regexp"

// captureFileRegex enforces the safe filename shape; rejects traversal.
var captureFileRegex = regexp.MustCompile(`^route-[a-z0-9-]+\.jpg$`)

// CapturesBytesHandler serves a single capture file's bytes.
// Path: /api/designqc/captures/<project-id>/<file>
func CapturesBytesHandler(homeDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		// Path layout: /api/designqc/captures/<id>/<file>
		const prefix = "/api/designqc/captures/"
		path := r.URL.Path
		if len(path) <= len(prefix) {
			http.NotFound(w, r)
			return
		}
		rest := path[len(prefix):]
		// rest = "<id>/<file>"
		slashIdx := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				slashIdx = i
				break
			}
		}
		if slashIdx < 0 || slashIdx == len(rest)-1 {
			http.NotFound(w, r)
			return
		}
		projectID := rest[:slashIdx]
		fileName := rest[slashIdx+1:]
		if !captureFileRegex.MatchString(fileName) {
			http.NotFound(w, r)
			return
		}
		full := filepath.Join(homeDir, ".mneme", "designqc", projectID, "captures", fileName)
		// Cache-bust on every load: re-runs may overwrite under the same name.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, full)
	}
}
```

- [ ] **Step 4: Register the new route in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/designqc/captures/", dashboard.CapturesBytesHandler(deps.Home))
```

(Note the trailing slash — this is a path-prefix mount.)

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_DesignQC_Captures -v
```

Expected: 3 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/designqc.go pkg/daemon/routes.go tests/dashboard_designqc_live_test.go
git commit -m "feat(m11): /api/designqc/captures/<id>/<file> with path-traversal guard"
```

---

## Task 11: Frontend types + DesignQC panel

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/panels/DesignQC.tsx`
- Modify: `web/src/__tests__/App.test.tsx`

- [ ] **Step 1: Extend `web/src/api/types.ts`**

Replace the existing `DesignQCResponse` (M10c) and add the new shapes:

```ts
export type DesignQCCapture = {
  route: string
  file: string
  width: number
  height: number
  captured_at_ms: number
  error?: string
}

export type DesignQCReport = {
  version: number
  captured_at: string
  framework: string
  base_url: string
  captures: DesignQCCapture[]
}

export type DesignQCResponse = {
  available: boolean
  reason?: string
  report?: DesignQCReport
  captures?: unknown[] // legacy field; kept so M10c-era responses still type-check
}
```

- [ ] **Step 2: Replace `web/src/panels/DesignQC.tsx`**

```tsx
import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { getDesignQC } from '../api/designqc'
import { useActiveProject } from '../hooks/useActiveProject'

export function DesignQC(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active
    ? () => getDesignQC(active)
    : () => Promise.resolve({ available: false, reason: 'select a project' } as const)
  const { data } = useFetch(fn, { intervalMs: 30_000 })
  const [expanded, setExpanded] = useState<string | null>(null)

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  if (!data.available || !data.report) {
    return (
      <div>
        <h1 className="text-2xl font-semibold mb-4">design qc</h1>
        <div className="rounded border bg-white p-4 max-w-2xl">
          <p className="font-medium mb-2">No captures yet.</p>
          <p className="text-sm text-gray-700">
            Run <code className="bg-gray-100 px-1 rounded">mneme designqc</code> in this project to generate captures.
          </p>
          {data.reason && <p className="mt-2 text-xs text-gray-500">reason: {data.reason}</p>}
        </div>
      </div>
    )
  }

  const r = data.report
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
            onClick={() => !c.error && setExpanded(c.file)}
            className={'rounded border bg-white p-2 text-left ' + (c.error ? 'opacity-60' : 'hover:shadow')}
          >
            {c.error ? (
              <div className="w-full h-32 rounded bg-red-50 text-red-700 flex items-center justify-center text-xs px-2">
                {c.error}
              </div>
            ) : (
              <img
                src={`/api/designqc/captures/${active}/${c.file}`}
                className="w-full h-32 object-cover object-top rounded"
                alt={c.route}
              />
            )}
            <div className="mt-2 text-sm font-mono">{c.route}</div>
            {!c.error && (
              <div className="text-xs text-gray-500">{c.width}×{c.height}</div>
            )}
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

- [ ] **Step 3: Update `web/src/__tests__/App.test.tsx` mock fetch**

In the existing `vi.stubGlobal('fetch', ...)` body, ensure `/api/designqc` returns the new shape:

```ts
if (url.includes('/api/designqc')) {
  return new Response(JSON.stringify({ available: false, reason: 'no captures yet' }),
    { headers: { 'Content-Type': 'application/json' } })
}
```

- [ ] **Step 4: Type-check + run frontend tests**

```bash
cd web && pnpm tsc --noEmit && pnpm test && cd ..
```

Expected: all pass.

- [ ] **Step 5: Build frontend + sync into pkg/dashboard/dist**

```bash
make web-build
```

Expected: succeeds; `pkg/dashboard/dist/` regenerated.

- [ ] **Step 6: Commit**

```bash
git add web/src/api/types.ts web/src/panels/DesignQC.tsx web/src/__tests__/App.test.tsx pkg/dashboard/dist/
git commit -m "feat(m11): DesignQC panel — live grid + click-to-expand overlay"
```

---

## Task 12: Integration test — real chromedp + httptest fixture

**Files:**
- Create: `tests/integration/designqc_e2e_test.go`

- [ ] **Step 1: Create the test**

```go
//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/designqc"
	"github.com/ranwei/mneme/pkg/state"
)

const fixtureSPA = `<!doctype html><html><head><title>fix</title></head>
<body style="margin:0;font-family:sans-serif">
<div style="padding:40px">
  <h1>Fixture SPA</h1>
  <p>This page exists so the chromedp capture has something to render.</p>
</div>
</body></html>`

func TestDesignQC_E2E(t *testing.T) {
	if _, err := designqc.DetectChrome(); err != nil {
		t.Skipf("Chrome not available: %v", err)
	}

	home, err := os.MkdirTemp("", "mneme")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	t.Setenv("HOME", home)

	root := filepath.Join(home, "work", "p1")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0o700)
	os.MkdirAll(state.GlobalProjectDir("p1"), 0o700)
	os.WriteFile(filepath.Join(root, ".mneme", ".local-id"), []byte("p1"), 0o600)
	if err := state.WriteOrigin("p1", root); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fixtureSPA))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	report, err := designqc.Run(ctx, designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		BaseURL:       srv.URL,
		Framework:     "vite", // skip framework auto-detect (no vite config in tmp)
		RouteOverride: []string{"/", "/about"},
		Quality:       80,
		MaxWidth:      1440,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Version != 1 || len(report.Captures) != 2 {
		t.Fatalf("report mismatch: %+v", report)
	}

	// Both files exist on disk.
	for _, c := range report.Captures {
		full := filepath.Join(home, ".mneme", "designqc", "p1", "captures", c.File)
		info, err := os.Stat(full)
		if err != nil {
			t.Errorf("capture file %s missing: %v", c.File, err)
			continue
		}
		if info.Size() < 100 {
			t.Errorf("capture %s too small: %d bytes", c.File, info.Size())
		}
	}
	// report.json exists with version 1.
	got, err := designqc.ReadReport(filepath.Join(home, ".mneme", "designqc", "p1"))
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}
	if !strings.HasPrefix(got.BaseURL, "http://") {
		t.Errorf("BaseURL: %q", got.BaseURL)
	}
}
```

- [ ] **Step 2: Run the integration test**

```bash
go test -tags=integration ./tests/integration/... -run TestDesignQC_E2E -v -timeout 90s
```

Expected: PASS locally if Chrome is installed; SKIP otherwise; PASS in CI (GitHub Actions ships Chrome).

- [ ] **Step 3: Commit**

```bash
git add tests/integration/designqc_e2e_test.go
git commit -m "test(m11): integration smoke — chromedp captures fixture httptest server"
```

---

## Task 13: Final verification + acceptance pass

- [ ] **Step 1: Full test suite**

```bash
go test ./... -count=1
go test -tags=integration ./tests/integration/... -count=1 -timeout 120s
cd web && pnpm test && pnpm tsc --noEmit && pnpm build && cd ..
make web-build
```

Expected: all green.

- [ ] **Step 2: Manual acceptance — `mneme designqc` end-to-end**

Pre-req: a real Vite project running `pnpm dev` on `localhost:5173`. The mneme repo's own `web/` directory works:

```bash
go build -o ./bin/mneme ./cmd
cd web && pnpm dev &
DEV_PID=$!
sleep 3
cd ..
./bin/mneme designqc
kill $DEV_PID
ls ~/.mneme/designqc/$(cat .mneme/.local-id)/captures/
cat ~/.mneme/designqc/$(cat .mneme/.local-id)/report.json | head -30
```

Expected: `captures/route-root.jpg` (and any auto-detected routes) exists; report.json has `version: 1`.

- [ ] **Step 3: Manual acceptance — dashboard panel**

```bash
./bin/mneme daemon stop || true
./bin/mneme daemon start
sleep 1
./bin/mneme dashboard
```

In the browser, navigate to `/designqc?project=<id>`:
- Empty state shows the M11 explanatory card if no `report.json` exists.
- After running `mneme designqc`, the panel shows the thumbnail grid within ~30 s (polling).
- Click a thumbnail; full-screen overlay appears; click anywhere to close.

```bash
./bin/mneme daemon stop
```

- [ ] **Step 4: Manual acceptance — Reframe template installed**

```bash
mkdir -p /tmp/m11-init && cd /tmp/m11-init && /path/to/repo/bin/mneme init --yes
ls .mneme/reframe.md && head -5 .mneme/reframe.md
grep -c '^## ' .mneme/reframe.md
```

Expected: `.mneme/reframe.md` exists; first line is `<!-- mneme reframe v1 -->`; grep count is `12`.

- [ ] **Step 5: Push branch + open PR**

```bash
git push -u origin feat/m11
gh pr create --title "feat(m11): designqc capture pipeline + dashboard panel + Reframe" --body "$(cat <<'EOF'
## Summary
- `pkg/designqc` package: detect (Vite) + routes + capture (chromedp) + runner + report
- `mneme designqc [--route /path] [--port N] [--quality N] [--max-width N] [--json]`
- `/api/designqc` flips from M10c stub to live (reads report.json, returns capture list)
- `/api/designqc/captures/<id>/<file>` serves JPEG bytes with strict path-traversal regex
- DesignQC panel becomes a 3-column thumbnail grid + click-to-expand overlay
- Reframe knowledge base — 12-framework static template installed by mneme init/update
- New Go dep: github.com/chromedp/chromedp; no new frontend deps

## Test plan
- [ ] go test ./... + go test -tags=integration ./tests/integration/... pass
- [ ] pnpm tsc --noEmit, pnpm test, pnpm build pass
- [ ] Manual: mneme designqc against a Vite project produces captures + report
- [ ] Manual: dashboard panel renders thumbnails + overlay
- [ ] Manual: mneme init populates .mneme/reframe.md with all 12 frameworks
EOF
)"
```

---

## Self-review against spec

**Spec coverage:**
- §2.1 Component diagram → Tasks 1-6
- §2.2 Package boundaries → file structure ↑
- §2.3 Storage layout → Task 1 (`Report` shape) + Task 5 (wipe + recreate)
- §2.4 Dependencies → Task 4 step 1 (`go get chromedp`)
- §3.1 Vite detection → Task 2
- §3.2 Route enumeration → Task 3
- §3.3 Dev-server probe → Task 4 (`WaitForDevServer`)
- §3.4 Capture (chromedp) → Task 4 (`Browser` + `Capture`)
- §3.5 Chrome detection + actionable error → Task 4 (`DetectChrome`) + Task 6 (CLI exit 2)
- §3.6 Runner orchestration → Task 5
- §3.7 Errors and partial-failure policy → Task 5 (test cases) + Task 6 (exit-code mapping)
- §4 CLI → Task 6
- §5 Dashboard backend → Task 9 (live handler) + Task 10 (bytes endpoint)
- §6 Frontend DesignQC panel → Task 11
- §7 Reframe knowledge base → Task 7 (template) + Task 8 (init/update wiring)
- §9 Tests → Tasks 1-5 (Go unit) + Task 9-10 (dashboard) + Task 12 (integration)
- §10 Acceptance criteria → Task 13

**Type/method consistency:**
- `Report{Version, CapturedAt, Framework, BaseURL, Captures}` — Task 1
- `Capture{Route, File, Width, Height, CapturedAtMS, Error}` — Task 1
- `Framework{Name, DefaultPort}` + `BaseURL()` method — Task 2
- `Route{Path, Slug}` + `SlugForPath` — Task 3
- `CaptureOptions{URL, Quality, MaxWidth, Timeout}` — Task 4
- `Browser` + `Browser.Capture` + `Browser.Close` — Task 4
- `DetectChrome`, `WaitForDevServer` — Task 4
- `RunOptions{ProjectRoot, ProjectID, HomeDir, RouteOverride, Quality, MaxWidth, Port, BaseURL, Framework}` — Task 5
- `Capturer` interface — Task 5
- `Run(ctx, opts)` + `RunWithCapturer(ctx, opts, cap)` — Task 5
- `DesignQCResponse{Available, Reason, Report}` — Task 9
- `CapturesBytesHandler(homeDir)` — Task 10
- Frontend `DesignQCResponse`, `DesignQCReport`, `DesignQCCapture` mirror Go field names exactly — Task 11

**Placeholder scan:** No "TBD" / "TODO" / "implement later" / "similar to". Every step has full code or full commands.

---
