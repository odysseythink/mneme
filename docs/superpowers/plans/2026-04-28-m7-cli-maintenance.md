# M7: CLI Maintenance Suite — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Worktree:** This plan should be executed in a dedicated git worktree to keep main clean. Use `superpowers:using-git-worktrees` to set one up if not already in one.

**Goal:** Add `status`, `scan --check`, `update`, `restore`, `bug search`, and project identity (`identity.md` + `mneme.md`) commands so mneme matches openwolf's CLI maintenance ergonomics.

**Architecture:** All new commands follow the existing `cmd/cmd_*.go` + `dispatch*` pattern. New code lives in `pkg/installer/` (templates, identity, backup, restore, updater) and `pkg/state/` (origin, template-version helpers). `update --binary` uses GitHub Releases API with SHA256 verification and atomic file replace.

**Tech Stack:** Go 1.25 stdlib (`net/http`, `crypto/sha256`, `encoding/json`, `flag`); existing `github.com/google/uuid`, `gofrs/flock`. No new third-party deps.

**Spec:** `docs/superpowers/specs/2026-04-28-m7-m11-roadmap-design.md` §3 M7.

---

## File Structure

**New files:**

- `pkg/state/origin.go` — read/write `~/.mneme/projects/<id>/origin` (project root path)
- `pkg/state/template_version.go` — read/write `.mneme/template-version.txt`
- `pkg/installer/templates/identity.md.tmpl` — identity-summary template
- `pkg/installer/templates/mneme.md.tmpl` — per-session Claude instructions template
- `pkg/installer/identity.go` — detect project metadata, render `identity.md`
- `pkg/installer/mnememd.go` — render `mneme.md` from template
- `pkg/installer/backup.go` — write/list/restore snapshots under `~/.mneme/backups/`
- `pkg/updater/updater.go` — Phase A multi-project template sync
- `pkg/updater/release.go` — Phase B GitHub Releases binary download + atomic replace
- `cmd/cmd_status.go` — `mneme status [--json]`
- `cmd/cmd_update.go` — `mneme update [--binary|--list|--dry-run|--project|--yes]`
- `cmd/cmd_restore.go` — `mneme restore [--list|--latest|<timestamp>]`
- `tests/origin_test.go`
- `tests/template_version_test.go`
- `tests/identity_test.go`
- `tests/backup_test.go`
- `tests/updater_test.go`
- `tests/release_test.go`
- `tests/cmd_status_test.go`
- `tests/cmd_update_test.go`
- `tests/cmd_restore_test.go`
- `tests/scan_check_test.go`
- `tests/bug_search_test.go`

**Modified files:**

- `cmd/main.go` — add `status`, `update`, `restore` cases; expose `releaseRepo`/`releaseChannel` build-time vars
- `cmd/usage.go` — extend top-level help with new commands
- `cmd/cmd_init.go` — write `origin` and `template-version.txt` during `runInit`; render `identity.md` and `mneme.md`
- `cmd/cmd_scan.go` — add `--check` flag
- `cmd/cmd_buglog.go` — add `search <term>` subcommand
- `pkg/installer/uninstall.go` — also remove `origin` file on uninstall
- `pkg/installer/templates/rules.md` — append pointer to `mneme.md` (operational doc)

---

## Conventions

- Tests live in package `tests` (separate from source packages). Use `t.TempDir()` for project roots; set `HOME` to a tempdir via `t.Setenv("HOME", ...)` when state under `~/.mneme/` is involved.
- Atomic writes go through `state.AtomicWrite`. Locks use `state.AcquireLock`.
- Subcommands print human-readable output to **stdout** by default and errors to **stderr**. JSON output (`--json` flag) goes to stdout only.
- All destructive subcommands accept `--yes` to skip confirmation; refuse to act on a non-TTY without `--yes`.
- Build the binary into `./bin/mneme` for integration tests: `go build -o ./bin/mneme ./cmd`.
- Commit message style: `feat(m7): <what>` or `test(m7): <what>` to match existing M0–M6 history.

---

## Task 1: Build-time release metadata vars

**Files:**

- Modify: `cmd/main.go` (top of file, after `package main`)
- Modify: `cmd/usage.go` (export `version` const → `var` so it can be overridden too, optional; only if needed elsewhere)

- [ ] **Step 1: Add release metadata vars to `cmd/main.go`**

Replace the imports + `main` block prelude with:

```go
package main

import (
	"fmt"
	"os"
)

// Build-time variables. Override via:
//   go build -ldflags="-X main.releaseRepo=ranwei/mneme -X main.releaseChannel=github" ./cmd
var (
	releaseRepo    = ""        // GitHub "owner/repo" — empty disables --binary
	releaseChannel = "source"  // "source" | "github"
)
```

- [ ] **Step 2: Verify build still works**

Run: `go build -o /tmp/mneme-build ./cmd && rm /tmp/mneme-build`
Expected: PASS (no compile errors)

- [ ] **Step 3: Commit**

```bash
git add cmd/main.go
git commit -m "feat(m7): add releaseRepo/releaseChannel build-time vars"
```

---

## Task 2: `pkg/state/origin.go` — record project root path

**Files:**

- Create: `pkg/state/origin.go`
- Test: `tests/origin_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/origin_test.go
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestWriteAndReadOrigin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	id := "test-project-id"
	projectRoot := "/abs/path/to/project"

	if err := state.WriteOrigin(id, projectRoot); err != nil {
		t.Fatalf("WriteOrigin: %v", err)
	}

	got, err := state.ReadOrigin(id)
	if err != nil {
		t.Fatalf("ReadOrigin: %v", err)
	}
	if got != projectRoot {
		t.Errorf("ReadOrigin = %q, want %q", got, projectRoot)
	}

	// File should exist at expected path.
	path := filepath.Join(home, ".mneme", "projects", id, "origin")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("origin file not at expected path: %v", err)
	}
}

func TestReadOriginMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := state.ReadOrigin("does-not-exist")
	if err == nil {
		t.Errorf("expected error for missing origin, got nil; value=%q", got)
	}
}

func TestListOriginsEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := state.ListOrigins()
	if err != nil {
		t.Fatalf("ListOrigins on empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestListOriginsMultiple(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	state.WriteOrigin("id-a", "/path/a")
	state.WriteOrigin("id-b", "/path/b")
	state.WriteOrigin("id-c", "/path/c")

	got, err := state.ListOrigins()
	if err != nil {
		t.Fatalf("ListOrigins: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 origins, got %d", len(got))
	}
	if got["id-a"] != "/path/a" || got["id-b"] != "/path/b" || got["id-c"] != "/path/c" {
		t.Errorf("unexpected map content: %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestWriteAndReadOrigin|TestReadOriginMissing|TestListOrigins" -v`
Expected: FAIL with `state.WriteOrigin undefined` etc.

- [ ] **Step 3: Implement `pkg/state/origin.go`**

```go
// pkg/state/origin.go
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteOrigin records the absolute path of a project's working tree to
// ~/.mneme/projects/<projectID>/origin so that multi-project commands
// (update, restore) can locate every initialized project.
func WriteOrigin(projectID, projectRoot string) error {
	dir := GlobalProjectDir(projectID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "origin")
	return AtomicWrite(path, []byte(projectRoot+"\n"))
}

// ReadOrigin returns the recorded project root for the given ID.
// Returns an error if the origin file is missing or empty.
func ReadOrigin(projectID string) (string, error) {
	path := filepath.Join(GlobalProjectDir(projectID), "origin")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(data))
	if root == "" {
		return "", fmt.Errorf("origin file is empty: %s", path)
	}
	return root, nil
}

// ListOrigins enumerates every initialized project.
// Returns map[projectID]projectRoot. Missing or unreadable origin files
// are skipped silently.
func ListOrigins() (map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	projectsDir := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	result := make(map[string]string, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root, err := ReadOrigin(e.Name())
		if err != nil {
			continue
		}
		result[e.Name()] = root
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests -run "TestWriteAndReadOrigin|TestReadOriginMissing|TestListOrigins" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/state/origin.go tests/origin_test.go
git commit -m "feat(m7): add state.WriteOrigin/ReadOrigin/ListOrigins"
```

---

## Task 3: `pkg/state/template_version.go` — pin per-project template version

**Files:**

- Create: `pkg/state/template_version.go`
- Test: `tests/template_version_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/template_version_test.go
package tests

import (
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestReadTemplateVersionMissing(t *testing.T) {
	dir := t.TempDir()
	v, err := state.ReadTemplateVersion(dir)
	if err != nil {
		t.Fatalf("ReadTemplateVersion missing: %v", err)
	}
	if v != 0 {
		t.Errorf("missing file should return 0, got %d", v)
	}
}

func TestWriteAndReadTemplateVersion(t *testing.T) {
	dir := t.TempDir()
	if err := state.WriteTemplateVersion(dir, 7); err != nil {
		t.Fatalf("WriteTemplateVersion: %v", err)
	}
	v, err := state.ReadTemplateVersion(dir)
	if err != nil {
		t.Fatalf("ReadTemplateVersion: %v", err)
	}
	if v != 7 {
		t.Errorf("got %d, want 7", v)
	}
}

func TestReadTemplateVersionCorrupt(t *testing.T) {
	dir := t.TempDir()
	mneme := filepath.Join(dir, ".mneme")
	if err := state.WriteTemplateVersionRaw(mneme, "not-a-number\n"); err != nil {
		t.Fatalf("WriteTemplateVersionRaw: %v", err)
	}
	v, err := state.ReadTemplateVersion(dir)
	if err == nil {
		t.Errorf("expected parse error, got value %d", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestReadTemplateVersion|TestWriteAndReadTemplateVersion" -v`
Expected: FAIL with `state.ReadTemplateVersion undefined`

- [ ] **Step 3: Implement `pkg/state/template_version.go`**

```go
// pkg/state/template_version.go
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const TemplateVersion = 1 // bump when bundled templates change shape

func templateVersionPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".mneme", "template-version.txt")
}

// ReadTemplateVersion reads the pinned template version. Returns 0 if the
// file is missing (treat as oldest), and an error if the file is corrupt.
func ReadTemplateVersion(projectRoot string) (int, error) {
	data, err := os.ReadFile(templateVersionPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("template-version.txt corrupt: %v", err)
	}
	return n, nil
}

// WriteTemplateVersion writes the version number to .mneme/template-version.txt.
func WriteTemplateVersion(projectRoot string, version int) error {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return AtomicWrite(templateVersionPath(projectRoot), []byte(fmt.Sprintf("%d\n", version)))
}

// WriteTemplateVersionRaw exists for tests that need to seed corrupt content.
func WriteTemplateVersionRaw(mnemeDir, contents string) error {
	if err := os.MkdirAll(mnemeDir, 0755); err != nil {
		return err
	}
	return AtomicWrite(filepath.Join(mnemeDir, "template-version.txt"), []byte(contents))
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestReadTemplateVersion|TestWriteAndReadTemplateVersion" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/state/template_version.go tests/template_version_test.go
git commit -m "feat(m7): add state.{Read,Write}TemplateVersion"
```

---

## Task 4: identity & mneme template files

**Files:**

- Create: `pkg/installer/templates/identity.md.tmpl`
- Create: `pkg/installer/templates/mneme.md.tmpl`

- [ ] **Step 1: Write `identity.md.tmpl`**

```markdown
<!-- managed by mneme — auto-generated; do not edit manually -->
<!-- to refresh: mneme scan -->

# Project: {{.Name}}

- **Primary language:** {{.Language}}
- **Framework:** {{.Framework}}
- **Package manager:** {{.PackageManager}}
- **Dev server:** {{.DevServerURL}}
- **Project root:** `{{.ProjectRoot}}`

## Intent

{{.Intent}}
```

Path: `/Users/ranwei/workspace/go_work/claude-context-research/mneme/pkg/installer/templates/identity.md.tmpl`

- [ ] **Step 2: Write `mneme.md.tmpl`**

```markdown
<!-- managed by mneme — to update: mneme update -->
<!-- user customizations between mneme:user-section fences are preserved -->

# mneme operational guide

This file tells Claude Code how to use mneme in **this** project. State directory: `.mneme/`.

## When to use which tool

| Need | Tool |
|---|---|
| Find code by meaning | MCP `search_codebase` |
| Index a fresh repo | MCP `index_codebase` |
| See file structure | MCP `describe_codebase` (reads `anatomy.md`) |
| Check coding rules | MCP `get_project_rules` (reads `cerebrum.json`) |
| Avoid past bugs | MCP `find_similar_bugs` (reads `buglog.json`) |

## State files

- `.mneme/anatomy.md` — file map. Refresh: `mneme scan`.
- `.mneme/cerebrum.json` — coding rules. Manage: `mneme cerebrum add/list/remove`.
- `.mneme/buglog.json` — past bugs. Manage: `mneme buglog add/list/search`.
- `.mneme/identity.md` — project facts (auto-generated).

## Commands

```
mneme status          # health snapshot
mneme scan            # rebuild anatomy.md
mneme scan --check    # verify anatomy.md matches filesystem
mneme stats           # hook + memory stats
mneme update          # sync templates from latest mneme binary
mneme update --binary # download latest binary from GitHub Releases
mneme restore         # restore from backup
mneme bug search <q>  # search buglog
```

<!-- mneme:user-section BEGIN -->
<!-- Add project-specific guidance here. mneme update will preserve this block. -->
<!-- mneme:user-section END -->
```

Path: `/Users/ranwei/workspace/go_work/claude-context-research/mneme/pkg/installer/templates/mneme.md.tmpl`

- [ ] **Step 3: Verify both files exist**

Run: `ls pkg/installer/templates/`
Expected: `identity.md.tmpl  mneme.md.tmpl  rules.md`

- [ ] **Step 4: Commit**

```bash
git add pkg/installer/templates/identity.md.tmpl pkg/installer/templates/mneme.md.tmpl
git commit -m "feat(m7): add identity and mneme.md templates"
```

---

## Task 5: `pkg/installer/identity.go` — render `identity.md`

**Files:**

- Create: `pkg/installer/identity.go`
- Test: `tests/identity_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/identity_test.go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
)

func TestDetectProjectMetadataGo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# foo\n\nA tiny Go service.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := installer.DetectProjectMetadata(dir)
	if meta.Language != "Go" {
		t.Errorf("Language = %q, want Go", meta.Language)
	}
	if !strings.Contains(meta.Intent, "tiny Go service") {
		t.Errorf("Intent missing README excerpt: %q", meta.Intent)
	}
}

func TestDetectProjectMetadataNode(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name":"web","scripts":{"dev":"vite"},"dependencies":{"react":"^18"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)
	os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfile\n"), 0644)
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte(""), 0644)

	meta := installer.DetectProjectMetadata(dir)
	if meta.Language != "JavaScript/TypeScript" {
		t.Errorf("Language = %q", meta.Language)
	}
	if meta.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q", meta.PackageManager)
	}
	if meta.Framework != "Vite" {
		t.Errorf("Framework = %q", meta.Framework)
	}
}

func TestWriteIdentityMD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n"), 0644)
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	if err := installer.WriteIdentity(dir); err != nil {
		t.Fatalf("WriteIdentity: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".mneme", "identity.md"))
	if err != nil {
		t.Fatalf("read identity.md: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "# Project:") {
		t.Errorf("identity.md missing header: %q", s)
	}
	if !strings.Contains(s, "Go") {
		t.Errorf("identity.md missing language: %q", s)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests -run "TestDetectProjectMetadata|TestWriteIdentityMD" -v`
Expected: FAIL with `installer.DetectProjectMetadata undefined`

- [ ] **Step 3: Implement `pkg/installer/identity.go`**

```go
// pkg/installer/identity.go
package installer

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ranwei/mneme/pkg/state"
)

//go:embed templates/identity.md.tmpl
var identityTemplate string

// ProjectMetadata is the data model rendered into identity.md.
type ProjectMetadata struct {
	Name           string
	Language       string
	Framework      string
	PackageManager string
	DevServerURL   string
	ProjectRoot    string
	Intent         string
}

// DetectProjectMetadata inspects projectRoot and returns best-effort facts.
// Never fails — fields default to "unknown" when undetectable.
func DetectProjectMetadata(projectRoot string) ProjectMetadata {
	m := ProjectMetadata{
		Name:           filepath.Base(projectRoot),
		Language:       "unknown",
		Framework:      "unknown",
		PackageManager: "unknown",
		DevServerURL:   "unknown",
		ProjectRoot:    projectRoot,
		Intent:         "(no README description detected)",
	}

	// Language detection by marker file priority.
	switch {
	case fileExists(projectRoot, "go.mod"):
		m.Language = "Go"
	case fileExists(projectRoot, "package.json"):
		m.Language = "JavaScript/TypeScript"
	case fileExists(projectRoot, "pyproject.toml"), fileExists(projectRoot, "setup.py"), fileExists(projectRoot, "requirements.txt"):
		m.Language = "Python"
	case fileExists(projectRoot, "Cargo.toml"):
		m.Language = "Rust"
	case fileExists(projectRoot, "pom.xml"), fileExists(projectRoot, "build.gradle"):
		m.Language = "Java"
	}

	// Package manager (Node ecosystem only).
	if m.Language == "JavaScript/TypeScript" {
		switch {
		case fileExists(projectRoot, "pnpm-lock.yaml"):
			m.PackageManager = "pnpm"
		case fileExists(projectRoot, "yarn.lock"):
			m.PackageManager = "yarn"
		case fileExists(projectRoot, "bun.lockb"):
			m.PackageManager = "bun"
		case fileExists(projectRoot, "package-lock.json"):
			m.PackageManager = "npm"
		default:
			m.PackageManager = "npm"
		}
	}

	// Framework detection.
	switch {
	case fileExists(projectRoot, "next.config.js"), fileExists(projectRoot, "next.config.ts"), fileExists(projectRoot, "next.config.mjs"):
		m.Framework = "Next.js"
		m.DevServerURL = "http://localhost:3000"
	case fileExists(projectRoot, "vite.config.ts"), fileExists(projectRoot, "vite.config.js"):
		m.Framework = "Vite"
		m.DevServerURL = "http://localhost:5173"
	case fileExists(projectRoot, "astro.config.mjs"), fileExists(projectRoot, "astro.config.ts"):
		m.Framework = "Astro"
		m.DevServerURL = "http://localhost:4321"
	case fileExists(projectRoot, "svelte.config.js"):
		m.Framework = "SvelteKit"
		m.DevServerURL = "http://localhost:5173"
	}

	// Override DevServerURL from package.json scripts when explicit port is found.
	if m.Language == "JavaScript/TypeScript" {
		if port := readDevPortFromPackageJSON(filepath.Join(projectRoot, "package.json")); port > 0 {
			m.DevServerURL = "http://localhost:" + strconvItoa(port)
		}
	}

	// Intent: first paragraph of README.md / README.
	for _, name := range []string{"README.md", "README"} {
		path := filepath.Join(projectRoot, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if intent := firstParagraph(string(data)); intent != "" {
			m.Intent = intent
			break
		}
	}

	return m
}

// WriteIdentity renders identity.md into <projectRoot>/.mneme/identity.md.
func WriteIdentity(projectRoot string) error {
	meta := DetectProjectMetadata(projectRoot)
	tmpl, err := template.New("identity").Parse(identityTemplate)
	if err != nil {
		return err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, meta); err != nil {
		return err
	}
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return state.AtomicWrite(filepath.Join(dir, "identity.md"), []byte(sb.String()))
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func firstParagraph(s string) string {
	// Skip leading blank/heading lines, return first non-empty paragraph.
	lines := strings.Split(s, "\n")
	var buf []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "#") {
			continue
		}
		if trim == "" {
			if len(buf) > 0 {
				break
			}
			continue
		}
		buf = append(buf, trim)
	}
	out := strings.Join(buf, " ")
	if len(out) > 240 {
		out = out[:240] + "…"
	}
	return out
}

func readDevPortFromPackageJSON(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return 0
	}
	dev := pkg.Scripts["dev"]
	if dev == "" {
		dev = pkg.Scripts["start"]
	}
	// Look for "--port N" or "-p N".
	for _, flag := range []string{"--port ", "-p "} {
		if i := strings.Index(dev, flag); i >= 0 {
			rest := dev[i+len(flag):]
			end := strings.IndexAny(rest, " \t")
			if end < 0 {
				end = len(rest)
			}
			n := 0
			for _, c := range rest[:end] {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			if n > 0 {
				return n
			}
		}
	}
	return 0
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[i:])
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestDetectProjectMetadata|TestWriteIdentityMD" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/installer/identity.go tests/identity_test.go
git commit -m "feat(m7): add installer.DetectProjectMetadata and WriteIdentity"
```

---

## Task 6: `pkg/installer/mnememd.go` — render `mneme.md`

**Files:**

- Create: `pkg/installer/mnememd.go`
- Test: extend `tests/identity_test.go`

- [ ] **Step 1: Write the failing test (append to `tests/identity_test.go`)**

```go
func TestWriteMnemeMD(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	if err := installer.WriteMnemeMD(dir); err != nil {
		t.Fatalf("WriteMnemeMD: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".mneme", "mneme.md"))
	if err != nil {
		t.Fatalf("read mneme.md: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "mneme operational guide") {
		t.Errorf("mneme.md missing header")
	}
	if !strings.Contains(s, "<!-- mneme:user-section BEGIN -->") {
		t.Errorf("mneme.md missing user-section fence")
	}
}

func TestWriteMnemeMDPreservesUserSection(t *testing.T) {
	dir := t.TempDir()
	mneme := filepath.Join(dir, ".mneme")
	os.MkdirAll(mneme, 0755)
	original := `before
<!-- mneme:user-section BEGIN -->
my custom guidance
keep this verbatim
<!-- mneme:user-section END -->
after`
	os.WriteFile(filepath.Join(mneme, "mneme.md"), []byte(original), 0644)

	if err := installer.WriteMnemeMD(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(mneme, "mneme.md"))
	s := string(data)
	if !strings.Contains(s, "my custom guidance") {
		t.Errorf("user section dropped: %q", s)
	}
	if !strings.Contains(s, "keep this verbatim") {
		t.Errorf("user section partially dropped: %q", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestWriteMnemeMD" -v`
Expected: FAIL with `installer.WriteMnemeMD undefined`

- [ ] **Step 3: Implement `pkg/installer/mnememd.go`**

```go
// pkg/installer/mnememd.go
package installer

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/ranwei/mneme/pkg/state"
)

//go:embed templates/mneme.md.tmpl
var mnemeMDTemplate string

const userSectionBegin = "<!-- mneme:user-section BEGIN -->"
const userSectionEnd = "<!-- mneme:user-section END -->"

// WriteMnemeMD writes mneme.md, preserving any existing user section
// between the user-section fences.
func WriteMnemeMD(projectRoot string) error {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "mneme.md")

	contents := mnemeMDTemplate
	if existing, err := os.ReadFile(path); err == nil {
		if user := extractUserSection(string(existing)); user != "" {
			contents = replaceUserSection(contents, user)
		}
	}
	return state.AtomicWrite(path, []byte(contents))
}

// extractUserSection returns the text between (but not including) the fence
// markers. Returns "" if either fence is absent.
func extractUserSection(s string) string {
	start := strings.Index(s, userSectionBegin)
	if start < 0 {
		return ""
	}
	start += len(userSectionBegin)
	end := strings.Index(s[start:], userSectionEnd)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

// replaceUserSection swaps the user section in template with replacement.
// If template has no fences, returns template unchanged.
func replaceUserSection(template, replacement string) string {
	start := strings.Index(template, userSectionBegin)
	if start < 0 {
		return template
	}
	start += len(userSectionBegin)
	end := strings.Index(template[start:], userSectionEnd)
	if end < 0 {
		return template
	}
	return template[:start] + replacement + template[start+end:]
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestWriteMnemeMD" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/installer/mnememd.go tests/identity_test.go
git commit -m "feat(m7): add installer.WriteMnemeMD with user-section preservation"
```

---

## Task 7: Wire identity + mneme.md + origin + template-version into `init`

**Files:**

- Modify: `cmd/cmd_init.go` (function `runInit`)
- Modify: `pkg/installer/uninstall.go` (also remove origin file)
- Modify: `tests/installer_test.go` (extend to verify new files appear after init)

- [ ] **Step 1: Read existing `installer_test.go` to find a representative init-applies-files test**

Run: `grep -n "func Test" tests/installer_test.go | head -20`
Note the existing test naming convention.

- [ ] **Step 2: Modify `runInit` in `cmd/cmd_init.go`**

After the existing `// Step 5-7: scaffold project dir` block (around line 153), add:

```go
	// Step 6: write origin (multi-project registry pointer)
	if err := state.WriteOrigin(id, projectRoot); err != nil {
		fmt.Fprintln(os.Stderr, "✗ origin:", err)
		os.Exit(1)
	}

	// Step 7: pin template version
	if err := state.WriteTemplateVersion(projectRoot, state.TemplateVersion); err != nil {
		fmt.Fprintln(os.Stderr, "✗ template-version:", err)
		os.Exit(1)
	}

	// Step 8: write identity.md
	if err := installer.WriteIdentity(projectRoot); err != nil {
		fmt.Fprintln(os.Stderr, "✗ identity.md:", err)
		os.Exit(1)
	}

	// Step 9: write mneme.md
	if err := installer.WriteMnemeMD(projectRoot); err != nil {
		fmt.Fprintln(os.Stderr, "✗ mneme.md:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Wrote identity.md, mneme.md, origin, template-version\n")
```

(Renumber the existing "Step 8: auto-scan" comment to "Step 10".)

- [ ] **Step 3: Modify `pkg/installer/uninstall.go` to also remove origin file**

Read the file first to find the right insertion point:

Run: `cat pkg/installer/uninstall.go`

Add a step that removes `~/.mneme/projects/<id>/origin` when the user uninstalls a project. (Implementation detail depends on existing uninstall structure — keep new code minimal: read local-id, build path with `state.GlobalProjectDir`, `os.Remove`.)

- [ ] **Step 4: Build and run init manually to verify**

```bash
go build -o ./bin/mneme ./cmd
TMPROOT=$(mktemp -d)
cd "$TMPROOT" && git init -q
HOME="$TMPROOT/home" ./../mneme init --yes --no-scan --project 2>&1 | tail -5
ls -la .mneme/
# Expect to see: identity.md, mneme.md, template-version.txt, .gitignore, .local-id
cat .mneme/template-version.txt
ls -la "$HOME"/.mneme/projects/*/
# Expect to see: origin
cd - && rm -rf "$TMPROOT"
```

(Replace `./../mneme` with the actual path to your built binary; the snippet illustrates the verification flow.)

Expected: identity.md, mneme.md, template-version.txt present in `.mneme/`; `origin` present in `~/.mneme/projects/<id>/`.

- [ ] **Step 5: Run existing installer tests to confirm no regression**

Run: `go test ./tests -run "TestInstaller|TestInit" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/cmd_init.go pkg/installer/uninstall.go
git commit -m "feat(m7): init writes origin, template-version, identity.md, mneme.md"
```

---

## Task 8: `pkg/installer/backup.go` — snapshot/restore primitive

**Files:**

- Create: `pkg/installer/backup.go`
- Test: `tests/backup_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/backup_test.go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
)

func TestBackupAndListAndRestore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectID := "proj-1"

	// Create source files to back up.
	src := t.TempDir()
	rules := filepath.Join(src, "rules.md")
	mneme := filepath.Join(src, "mneme.md")
	os.WriteFile(rules, []byte("rules v1"), 0644)
	os.WriteFile(mneme, []byte("mneme v1"), 0644)

	// Take a backup.
	op := installer.BackupOp{
		ProjectID: projectID,
		Operation: "test",
		Files:     []string{rules, mneme},
		MnemeVer:  "0.1.0-m7",
	}
	bk, err := installer.WriteBackup(op)
	if err != nil {
		t.Fatalf("WriteBackup: %v", err)
	}
	if !strings.HasPrefix(bk.Path, filepath.Join(home, ".mneme", "backups", projectID)) {
		t.Errorf("backup path = %q, want under %s/.mneme/backups/%s", bk.Path, home, projectID)
	}

	// Mutate source.
	os.WriteFile(rules, []byte("rules v2"), 0644)
	os.WriteFile(mneme, []byte("mneme v2"), 0644)

	// List should return one backup.
	list, err := installer.ListBackups(projectID)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(list))
	}

	// Restore should bring back v1 contents.
	if err := installer.RestoreBackup(bk.Timestamp, projectID); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	r, _ := os.ReadFile(rules)
	if string(r) != "rules v1" {
		t.Errorf("after restore, rules = %q, want %q", string(r), "rules v1")
	}
}

func TestRestoreBackupMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := installer.RestoreBackup("2026-04-28T12:00:00Z", "no-such"); err == nil {
		t.Errorf("expected error for missing backup")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestBackupAndListAndRestore|TestRestoreBackupMissing" -v`
Expected: FAIL with undefined symbols.

- [ ] **Step 3: Implement `pkg/installer/backup.go`**

```go
// pkg/installer/backup.go
package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

// BackupOp describes a destructive operation about to occur.
type BackupOp struct {
	ProjectID string   `json:"project_id"`
	Operation string   `json:"operation"` // e.g. "update", "restore", "init"
	Files     []string `json:"files"`     // absolute paths to copy
	MnemeVer  string   `json:"mneme_version"`
}

// Backup is the result of WriteBackup.
type Backup struct {
	Timestamp string // RFC3339 with colons replaced by hyphens (filesystem-safe)
	Path      string // absolute path of backup directory
	Files     []string
}

// WriteBackup snapshots Files into ~/.mneme/backups/<project>/<ts>/.
// Returns a Backup with the chosen timestamp.
func WriteBackup(op BackupOp) (*Backup, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	dir := filepath.Join(home, ".mneme", "backups", op.ProjectID, ts)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var copied []string
	for _, src := range op.Files {
		base := filepath.Base(src)
		dst := filepath.Join(dir, base)
		if err := copyFile(src, dst); err != nil {
			if os.IsNotExist(err) {
				continue // skip missing files silently — they have nothing to back up
			}
			return nil, fmt.Errorf("copy %s: %v", src, err)
		}
		copied = append(copied, base)
	}

	manifest := struct {
		BackupOp
		CopiedFiles []string `json:"copied_files"`
		CreatedAt   string   `json:"created_at"`
	}{
		BackupOp:    op,
		CopiedFiles: copied,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := state.AtomicWrite(filepath.Join(dir, "manifest.json"), data); err != nil {
		return nil, err
	}

	return &Backup{Timestamp: ts, Path: dir, Files: copied}, nil
}

// ListBackups returns backups for projectID, newest first.
func ListBackups(projectID string) ([]Backup, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, ".mneme", "backups", projectID)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Backup
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, Backup{
			Timestamp: e.Name(),
			Path:      filepath.Join(root, e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out, nil
}

// RestoreBackup copies every file from a backup directory back to its
// original location (recorded in manifest.json under copied_files +
// the original BackupOp.Files).
func RestoreBackup(timestamp, projectID string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".mneme", "backups", projectID, timestamp)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("backup not found: %s", dir)
	}
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Files       []string `json:"files"`
		CopiedFiles []string `json:"copied_files"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return err
	}
	// Build basename → absolute-source map from BackupOp.Files.
	srcByBase := make(map[string]string, len(manifest.Files))
	for _, src := range manifest.Files {
		srcByBase[filepath.Base(src)] = src
	}
	for _, base := range manifest.CopiedFiles {
		dst, ok := srcByBase[base]
		if !ok {
			continue
		}
		if err := copyFile(filepath.Join(dir, base), dst); err != nil {
			return fmt.Errorf("restore %s: %v", base, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestBackupAndListAndRestore|TestRestoreBackupMissing" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/installer/backup.go tests/backup_test.go
git commit -m "feat(m7): add installer.{WriteBackup,ListBackups,RestoreBackup}"
```

---

## Task 9: `mneme status` command

**Files:**

- Create: `cmd/cmd_status.go`
- Modify: `cmd/main.go` (add `case "status"`)
- Modify: `cmd/usage.go` (add status to top-level help)
- Test: `tests/cmd_status_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/cmd_status_test.go
package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildMnemeBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mneme")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd")
	cmd.Dir = mustRepoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestStatusOutsideProject(t *testing.T) {
	bin := buildMnemeBinary(t)
	dir := t.TempDir()
	cmd := exec.Command(bin, "status")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("status outside project should exit nonzero; output=%s", out)
	}
	if !strings.Contains(string(out), "no mneme project") {
		t.Errorf("expected error mentioning 'no mneme project', got: %s", out)
	}
}

func TestStatusJSONInProject(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()

	// Make it look like a mneme-initialized project: minimal .mneme/ with .local-id.
	mneme := filepath.Join(proj, ".mneme")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, ".local-id"), []byte("test-id\n"), 0644)
	os.WriteFile(filepath.Join(mneme, "anatomy.md"), []byte("<!-- mneme anatomy v1 -->\n"), 0644)

	cmd := exec.Command(bin, "status", "--json")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status --json: %v\n%s", err, out)
	}

	var report map[string]any
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("status --json output not valid JSON: %v\n%s", err, out)
	}
	if report["project_id"] != "test-id" {
		t.Errorf("project_id = %v, want test-id", report["project_id"])
	}
	if _, ok := report["daemon"]; !ok {
		t.Errorf("status --json missing daemon field; got keys: %v", keys(report))
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestStatusOutsideProject|TestStatusJSONInProject" -v`
Expected: FAIL — `unknown subcommand: "status"`.

- [ ] **Step 3: Implement `cmd/cmd_status.go`**

```go
// cmd/cmd_status.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

type daemonStatus struct {
	Running bool   `json:"running"`
	Note    string `json:"note,omitempty"`
}

type statusReport struct {
	ProjectID       string             `json:"project_id"`
	ProjectRoot     string             `json:"project_root"`
	AnatomyAge      string             `json:"anatomy_age,omitempty"`
	AnatomyMissing  bool               `json:"anatomy_missing,omitempty"`
	Daemon          daemonStatus       `json:"daemon"`
	HookFiredTotals map[string]int     `json:"hook_fired_totals,omitempty"`
	LastHookUpdate  string             `json:"last_hook_update,omitempty"`
	MnemeVersion    string             `json:"mneme_version"`
	ReleaseChannel  string             `json:"release_channel"`
}

func dispatchStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "status: cannot get cwd:", err)
		os.Exit(1)
	}
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "status: no mneme project here (run: mneme init)")
		os.Exit(1)
	}

	report := buildStatusReport(root)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "status: encode:", err)
			os.Exit(1)
		}
		return
	}
	printStatusHuman(report)
}

func buildStatusReport(root string) statusReport {
	r := statusReport{
		ProjectRoot:    root,
		MnemeVersion:   version,
		ReleaseChannel: releaseChannel,
		Daemon:         probeDaemon(),
	}
	id, _ := state.ReadOrCreateLocalID(root)
	r.ProjectID = id

	if generated, err := state.ReadAnatomyGeneratedTime(root); err == nil {
		r.AnatomyAge = time.Since(generated).Round(time.Second).String()
	} else {
		r.AnatomyMissing = true
	}

	if l, err := state.ReadLedger(root); err == nil {
		r.HookFiredTotals = l.Totals.HookFired
		r.LastHookUpdate = l.LastUpdated
	}
	return r
}

func probeDaemon() daemonStatus {
	home, _ := os.UserHomeDir()
	socket := filepath.Join(home, ".mneme", "daemon", "socket")
	conn, err := net.DialTimeout("unix", socket, 200*time.Millisecond)
	if err != nil {
		return daemonStatus{Running: false, Note: "no daemon socket — start with `mneme daemon start` (M8+)"}
	}
	conn.Close()
	return daemonStatus{Running: true}
}

func printStatusHuman(r statusReport) {
	fmt.Printf("project: %s\n", r.ProjectID)
	fmt.Printf("root:    %s\n", r.ProjectRoot)
	fmt.Printf("mneme:   %s (%s channel)\n", r.MnemeVersion, r.ReleaseChannel)
	fmt.Println()
	if r.AnatomyMissing {
		fmt.Println("anatomy.md: missing (run: mneme scan)")
	} else {
		fmt.Printf("anatomy.md age: %s\n", r.AnatomyAge)
	}
	fmt.Println()
	if r.Daemon.Running {
		fmt.Println("daemon: running")
	} else {
		fmt.Printf("daemon: %s\n", r.Daemon.Note)
	}
	if len(r.HookFiredTotals) > 0 {
		fmt.Println()
		fmt.Println("hook fires:")
		for _, k := range []string{"pre-read", "pre-write", "post-tool-use", "session-start", "stop"} {
			fmt.Printf("  %-15s %d\n", k+":", r.HookFiredTotals[k])
		}
		if r.LastHookUpdate != "" {
			fmt.Printf("last update:    %s\n", r.LastHookUpdate)
		}
	}
}
```

- [ ] **Step 4: Wire into `cmd/main.go`**

Add a new case to the switch:

```go
	case "status":
		dispatchStatus(os.Args[2:])
```

- [ ] **Step 5: Update `cmd/usage.go`**

Add a line for status under the existing usage text:

```
  mneme status            print health snapshot (use --json for machine-readable)
```

- [ ] **Step 6: Run tests**

Run: `go test ./tests -run "TestStatusOutsideProject|TestStatusJSONInProject" -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/cmd_status.go cmd/main.go cmd/usage.go tests/cmd_status_test.go
git commit -m "feat(m7): add mneme status [--json]"
```

---

## Task 10: `mneme scan --check` flag

**Files:**

- Modify: `cmd/cmd_scan.go`
- Test: `tests/scan_check_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/scan_check_test.go
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanCheckCleanProject(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()

	// Create a file and a matching anatomy entry pointing at it.
	if err := os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mneme := filepath.Join(proj, ".mneme")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, ".local-id"), []byte("id\n"), 0644)

	// Run scan to populate anatomy.md.
	scan := exec.Command(bin, "scan")
	scan.Dir = proj
	scan.Env = append(os.Environ(), "HOME="+home)
	if out, err := scan.CombinedOutput(); err != nil {
		t.Fatalf("initial scan: %v\n%s", err, out)
	}

	// scan --check on clean tree exits 0.
	check := exec.Command(bin, "scan", "--check")
	check.Dir = proj
	check.Env = append(os.Environ(), "HOME="+home)
	out, err := check.CombinedOutput()
	if err != nil {
		t.Fatalf("scan --check on clean project: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "no drift") && !strings.Contains(string(out), "in sync") {
		t.Errorf("expected drift-free message, got: %s", out)
	}
}

func TestScanCheckDriftExits1(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()

	os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\n"), 0644)
	mneme := filepath.Join(proj, ".mneme")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, ".local-id"), []byte("id\n"), 0644)

	scan := exec.Command(bin, "scan")
	scan.Dir = proj
	scan.Env = append(os.Environ(), "HOME="+home)
	scan.CombinedOutput()

	// Add a new file → drift.
	os.WriteFile(filepath.Join(proj, "added.go"), []byte("package main\n"), 0644)

	check := exec.Command(bin, "scan", "--check")
	check.Dir = proj
	check.Env = append(os.Environ(), "HOME="+home)
	out, err := check.CombinedOutput()
	if err == nil {
		t.Errorf("scan --check with drift should exit nonzero; output=%s", out)
	}
	if !strings.Contains(string(out), "drift") {
		t.Errorf("expected 'drift' in output, got: %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestScanCheck" -v`
Expected: FAIL — `--check` flag unknown OR exit-zero path doesn't behave correctly.

- [ ] **Step 3: Modify `cmd/cmd_scan.go`**

Replace the body of `dispatchScan` with:

```go
func dispatchScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	force := fs.Bool("force", false, "re-scan all files (ignore mtime cache)")
	check := fs.Bool("check", false, "verify anatomy.md matches filesystem; exit 1 on drift")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, _ := resolveInitProjectRoot(cwd)
	if _, err := os.Stat(filepath.Join(root, ".mneme")); err != nil {
		fmt.Fprintln(os.Stderr, "scan: project not initialized (run: mneme init)")
		os.Exit(1)
	}

	if *check {
		runScanCheck(root)
		return
	}

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", root)

	paths, err := scanner.Walk(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan: walk:", err)
		os.Exit(1)
	}

	var scanEntries []scanner.FileEntry
	if !*force {
		since, tsErr := state.ReadAnatomyGeneratedTime(root)
		existing, _ := state.ReadAnatomy(root)
		if tsErr == nil && len(existing) > 0 {
			scanEntries, err = scanner.ScanProjectIncremental(root, paths, since, existing)
		} else {
			scanEntries, err = scanner.ExtractAll(root, paths)
		}
	} else {
		scanEntries, err = scanner.ExtractAll(root, paths)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(1)
	}

	entries := make([]state.AnatomyEntry, len(scanEntries))
	for i, e := range scanEntries {
		entries[i] = state.AnatomyEntry{
			Path:        e.Path,
			Description: e.Description,
			EstTokens:   e.EstTokens,
			Language:    e.Language,
		}
	}

	anatomyPath := filepath.Join(root, ".mneme", "anatomy.md")
	if err := state.WriteAnatomy(root, entries); err != nil {
		fmt.Fprintln(os.Stderr, "scan: write anatomy:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Scanned %d files → %s\n", len(entries), anatomyPath)
}

func runScanCheck(root string) {
	existing, err := state.ReadAnatomy(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan --check: read anatomy:", err)
		os.Exit(2)
	}
	paths, err := scanner.Walk(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan --check: walk:", err)
		os.Exit(2)
	}
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	var added, removed []string
	for _, p := range paths {
		if _, ok := existing[p]; !ok {
			added = append(added, p)
		}
	}
	for p := range existing {
		if !pathSet[p] {
			removed = append(removed, p)
		}
	}

	if len(added) == 0 && len(removed) == 0 {
		fmt.Println("anatomy.md in sync — no drift detected")
		return
	}
	fmt.Printf("drift detected: %d added, %d removed\n", len(added), len(removed))
	for _, p := range added {
		fmt.Printf("  + %s\n", p)
	}
	for _, p := range removed {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Println("\nrun: mneme scan")
	os.Exit(1)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestScanCheck" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_scan.go tests/scan_check_test.go
git commit -m "feat(m7): scan --check verifies anatomy vs filesystem"
```

---

## Task 11: `mneme buglog search <term>`

**Files:**

- Modify: `cmd/cmd_buglog.go`
- Test: `tests/bug_search_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/bug_search_test.go
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func seedBuglog(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	entries := []state.BuglogEntry{
		{Source: "manual", File: "auth.go", Description: "use crypto/subtle for token compare", BadCode: "if got == want { return true }"},
		{Source: "auto", File: "db.go", Description: "missing context cancellation", BadCode: "rows, _ := db.Query(\"SELECT *\")"},
		{Source: "manual", File: "ui.tsx", Description: "useState in render path causes re-render loop", BadCode: "const [x] = useState(compute())"},
	}
	for _, e := range entries {
		state.AppendBuglogEntry(dir, e)
	}
}

func TestBuglogSearchTokenOverlap(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	os.WriteFile(filepath.Join(proj, ".mneme/.local-id"), []byte("id\n"), 0644)
	seedBuglog(t, proj)

	cmd := exec.Command(bin, "buglog", "search", "context cancellation")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "missing context cancellation") {
		t.Errorf("expected match for 'missing context cancellation', got:\n%s", s)
	}
	if strings.Contains(s, "useState in render path") {
		t.Errorf("unexpected unrelated match in output:\n%s", s)
	}
}

func TestBuglogSearchEmptyResult(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	os.WriteFile(filepath.Join(proj, ".mneme/.local-id"), []byte("id\n"), 0644)

	cmd := exec.Command(bin, "buglog", "search", "nothing matches")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "no matches") {
		t.Errorf("expected 'no matches' message, got: %s", out)
	}
}
```

Required imports for `tests/bug_search_test.go`:

```go
import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestBuglogSearch" -v`
Expected: FAIL — `unknown buglog subcommand: "search"`.

- [ ] **Step 3: Modify `cmd/cmd_buglog.go`**

Add `case "search"` to `dispatchBuglog`'s switch:

```go
	case "search":
		buglogSearch(args[1:])
```

Add the implementation function:

```go
func buglogSearch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: mneme buglog search <term>")
		os.Exit(2)
	}
	query := strings.Join(args, " ")
	root := requireProjectRoot()
	entries, err := state.ReadBuglog(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buglog search: %v\n", err)
		os.Exit(1)
	}

	queryTokens := match.Tokenize(query)
	queryLower := strings.ToLower(query)

	type hit struct {
		idx     int
		entry   state.BuglogEntry
		score   int
	}
	var hits []hit
	for i, e := range entries {
		corpus := strings.ToLower(e.Description + " " + e.BadCode + " " + e.File)
		score := 0
		if strings.Contains(corpus, queryLower) {
			score += 5 // substring match boost
		}
		score += match.TokenOverlap(queryTokens, match.Tokenize(corpus))
		if score > 0 {
			hits = append(hits, hit{idx: i, entry: e, score: score})
		}
	}
	if len(hits) == 0 {
		fmt.Println("no matches")
		return
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })

	fmt.Printf("%d matches for %q:\n", len(hits), query)
	for _, h := range hits {
		ts := h.entry.CreatedAt
		if len(ts) > 16 {
			ts = ts[:16] + "Z"
		}
		fmt.Printf("  [score=%d] [%s] %s  (%s)\n", h.score, h.entry.Source, h.entry.Description, ts)
		if h.entry.File != "" {
			fmt.Printf("            file: %s\n", h.entry.File)
		}
		firstLine := h.entry.BadCode
		if idx := strings.Index(firstLine, "\n"); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		fmt.Printf("            was:  %s\n", firstLine)
	}
}
```

Update the imports of `cmd/cmd_buglog.go` to add `sort` and `github.com/ranwei/mneme/pkg/match`:

```go
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/ranwei/mneme/pkg/match"
	"github.com/ranwei/mneme/pkg/state"
)
```

Update the usage line at the top of `dispatchBuglog`:

```go
fmt.Fprintln(os.Stderr, "usage: mneme buglog <add|list|search|clear>")
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestBuglogSearch" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_buglog.go tests/bug_search_test.go
git commit -m "feat(m7): add mneme buglog search <term> with token-overlap ranking"
```

---

## Task 12: `pkg/updater/updater.go` — Phase A multi-project template sync

**Files:**

- Create: `pkg/updater/updater.go`
- Test: `tests/updater_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/updater_test.go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/updater"
)

func setupProject(t *testing.T, home, projectName string, pinnedVersion int) string {
	t.Helper()
	root := filepath.Join(home, "p-"+projectName)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	id, _ := state.ReadOrCreateLocalID(root)
	state.WriteOrigin(id, root)
	state.WriteTemplateVersion(root, pinnedVersion)
	// Seed a stale rules.md so we can verify it is overwritten.
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	os.WriteFile(filepath.Join(root, ".mneme", "mneme.md"), []byte("OLD CONTENT\n"), 0644)
	return root
}

func TestUpdateSyncsOutdatedProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rootA := setupProject(t, home, "a", 0) // pinned older
	rootB := setupProject(t, home, "b", state.TemplateVersion) // up-to-date

	res, err := updater.SyncAll(updater.SyncOptions{})
	if err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("Updated = %d, want 1 (only rootA was outdated)", res.Updated)
	}
	if res.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", res.Skipped)
	}

	// rootA's mneme.md should have been refreshed.
	a, _ := os.ReadFile(filepath.Join(rootA, ".mneme", "mneme.md"))
	if strings.Contains(string(a), "OLD CONTENT") {
		t.Errorf("rootA mneme.md not refreshed: %q", string(a))
	}
	// rootB's mneme.md should be untouched.
	b, _ := os.ReadFile(filepath.Join(rootB, ".mneme", "mneme.md"))
	if !strings.Contains(string(b), "OLD CONTENT") {
		t.Errorf("rootB mneme.md should be untouched: %q", string(b))
	}

	// pinned versions reflect updated state.
	if v, _ := state.ReadTemplateVersion(rootA); v != state.TemplateVersion {
		t.Errorf("rootA pinned version = %d, want %d", v, state.TemplateVersion)
	}
}

func TestUpdateDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := setupProject(t, home, "dry", 0)

	res, err := updater.SyncAll(updater.SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("SyncAll dry-run: %v", err)
	}
	if res.WouldUpdate != 1 {
		t.Errorf("WouldUpdate = %d, want 1", res.WouldUpdate)
	}
	// No file write in dry-run.
	a, _ := os.ReadFile(filepath.Join(root, ".mneme", "mneme.md"))
	if !strings.Contains(string(a), "OLD CONTENT") {
		t.Errorf("dry-run wrote files: %q", string(a))
	}
}

func TestUpdateProjectFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	rootA := setupProject(t, home, "a", 0)
	setupProject(t, home, "b", 0)

	idA, _ := state.ReadOrCreateLocalID(rootA)
	res, err := updater.SyncAll(updater.SyncOptions{ProjectID: idA})
	if err != nil {
		t.Fatalf("SyncAll filtered: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("Updated = %d, want 1 (only filtered project)", res.Updated)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestUpdate" -v`
Expected: FAIL — `pkg/updater` does not exist.

- [ ] **Step 3: Implement `pkg/updater/updater.go`**

```go
// pkg/updater/updater.go
package updater

import (
	"fmt"

	"github.com/ranwei/mneme/pkg/installer"
	"github.com/ranwei/mneme/pkg/state"
)

// SyncOptions controls SyncAll behavior.
type SyncOptions struct {
	DryRun    bool
	ProjectID string // when non-empty, only sync this project
	Verbose   bool
}

// SyncResult summarizes a sync run.
type SyncResult struct {
	Updated     int
	Skipped     int
	Failed      int
	WouldUpdate int // dry-run count
	Failures    []SyncFailure
}

type SyncFailure struct {
	ProjectID string
	Path      string
	Err       error
}

// SyncAll iterates every initialized project and re-emits managed templates
// (rules.md, mneme.md, identity.md) when the bundled template version is
// newer than the project's pinned version.
func SyncAll(opts SyncOptions) (*SyncResult, error) {
	origins, err := state.ListOrigins()
	if err != nil {
		return nil, fmt.Errorf("list origins: %w", err)
	}
	res := &SyncResult{}
	for id, root := range origins {
		if opts.ProjectID != "" && opts.ProjectID != id {
			continue
		}
		pinned, err := state.ReadTemplateVersion(root)
		if err != nil {
			res.Failed++
			res.Failures = append(res.Failures, SyncFailure{ProjectID: id, Path: root, Err: err})
			continue
		}
		if pinned >= state.TemplateVersion {
			res.Skipped++
			if opts.Verbose {
				fmt.Printf("  skip   %s (up-to-date, v%d)\n", root, pinned)
			}
			continue
		}
		if opts.DryRun {
			res.WouldUpdate++
			fmt.Printf("  would update %s (v%d → v%d)\n", root, pinned, state.TemplateVersion)
			continue
		}
		if err := syncOne(id, root); err != nil {
			res.Failed++
			res.Failures = append(res.Failures, SyncFailure{ProjectID: id, Path: root, Err: err})
			continue
		}
		res.Updated++
		fmt.Printf("  updated %s (v%d → v%d)\n", root, pinned, state.TemplateVersion)
	}
	return res, nil
}

func syncOne(projectID, projectRoot string) error {
	// Snapshot before mutation.
	files := []string{
		fmt.Sprintf("%s/.mneme/mneme.md", projectRoot),
		fmt.Sprintf("%s/.mneme/identity.md", projectRoot),
	}
	if _, err := installer.WriteBackup(installer.BackupOp{
		ProjectID: projectID,
		Operation: "update",
		Files:     files,
		MnemeVer:  fmt.Sprintf("templates v%d", state.TemplateVersion),
	}); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	// Re-emit templates. mneme.md preserves user section; identity.md is regenerated.
	if err := installer.WriteMnemeMD(projectRoot); err != nil {
		return fmt.Errorf("write mneme.md: %w", err)
	}
	if err := installer.WriteIdentity(projectRoot); err != nil {
		return fmt.Errorf("write identity.md: %w", err)
	}
	if err := state.WriteTemplateVersion(projectRoot, state.TemplateVersion); err != nil {
		return fmt.Errorf("pin template version: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestUpdate" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/updater.go tests/updater_test.go
git commit -m "feat(m7): add updater.SyncAll for multi-project template sync"
```

---

## Task 13: `pkg/updater/release.go` — Phase B GitHub Releases self-update

**Files:**

- Create: `pkg/updater/release.go`
- Test: `tests/release_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/release_test.go
package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/updater"
)

func TestFetchReleaseAssetMatchesPlatform(t *testing.T) {
	expected := []byte("this-is-the-binary")
	sum := sha256.Sum256(expected)
	expectedSHA := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/mneme/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		assetName := "mneme-" + runtime.GOOS + "-" + runtime.GOARCH
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.2.0",
			"assets": []map[string]string{
				{"name": assetName, "browser_download_url": "http://" + r.Host + "/assets/bin"},
				{"name": assetName + ".sha256", "browser_download_url": "http://" + r.Host + "/assets/sum"},
			},
		})
	})
	mux.HandleFunc("/assets/bin", func(w http.ResponseWriter, r *http.Request) {
		w.Write(expected)
	})
	mux.HandleFunc("/assets/sum", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(expectedSHA + "  filename\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, version, err := updater.DownloadLatestBinary(srv.URL, "owner/mneme")
	if err != nil {
		t.Fatalf("DownloadLatestBinary: %v", err)
	}
	defer os.Remove(got)
	if version != "v0.2.0" {
		t.Errorf("version = %q, want v0.2.0", version)
	}
	data, _ := os.ReadFile(got)
	if string(data) != string(expected) {
		t.Errorf("downloaded contents differ")
	}
}

func TestDownloadRejectsBadSHA(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/mneme/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		assetName := "mneme-" + runtime.GOOS + "-" + runtime.GOARCH
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.2.0",
			"assets": []map[string]string{
				{"name": assetName, "browser_download_url": "http://" + r.Host + "/bad/bin"},
				{"name": assetName + ".sha256", "browser_download_url": "http://" + r.Host + "/bad/sum"},
			},
		})
	})
	mux.HandleFunc("/bad/bin", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("real")) })
	mux.HandleFunc("/bad/sum", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("0", 64) + "  bin"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, _, err := updater.DownloadLatestBinary(srv.URL, "owner/mneme")
	if err == nil {
		t.Errorf("expected SHA mismatch error")
	}
}

func TestReplaceBinaryAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mneme")
	os.WriteFile(target, []byte("old"), 0755)
	newFile := filepath.Join(dir, "new-bin")
	os.WriteFile(newFile, []byte("new"), 0755)

	if err := updater.ReplaceBinary(newFile, target); err != nil {
		t.Fatalf("ReplaceBinary: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("target contents = %q, want %q", string(got), "new")
	}
	info, _ := os.Stat(target)
	if info.Mode()&0111 == 0 {
		t.Errorf("target not executable: %v", info.Mode())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestFetchReleaseAsset|TestDownloadRejectsBadSHA|TestReplaceBinaryAtomic" -v`
Expected: FAIL — `updater.DownloadLatestBinary undefined`.

- [ ] **Step 3: Implement `pkg/updater/release.go`**

```go
// pkg/updater/release.go
package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultGitHubAPIBase is overridden in tests.
const DefaultGitHubAPIBase = "https://api.github.com"

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// DownloadLatestBinary fetches the latest release for `repo` (e.g. "owner/mneme"),
// matches the asset for the current GOOS/GOARCH, verifies its SHA256 against
// the sibling `.sha256` asset, and returns the path to a temp file holding the
// verified bytes plus the release version tag.
//
// `apiBase` should usually be DefaultGitHubAPIBase; tests inject httptest URLs.
func DownloadLatestBinary(apiBase, repo string) (string, string, error) {
	url := strings.TrimRight(apiBase, "/") + "/repos/" + repo + "/releases/latest"
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("release API returned %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", fmt.Errorf("parse release: %w", err)
	}

	wantName := fmt.Sprintf("mneme-%s-%s", runtime.GOOS, runtime.GOARCH)
	var binAsset, sumAsset *ghAsset
	for i := range rel.Assets {
		switch rel.Assets[i].Name {
		case wantName:
			binAsset = &rel.Assets[i]
		case wantName + ".sha256":
			sumAsset = &rel.Assets[i]
		}
	}
	if binAsset == nil {
		return "", "", fmt.Errorf("no asset named %s in release %s", wantName, rel.TagName)
	}
	if sumAsset == nil {
		return "", "", fmt.Errorf("no checksum asset %s.sha256 in release %s", wantName, rel.TagName)
	}

	sumBytes, err := fetchBytes(client, sumAsset.DownloadURL)
	if err != nil {
		return "", "", fmt.Errorf("fetch checksum: %w", err)
	}
	expectedSHA := strings.Fields(string(sumBytes))
	if len(expectedSHA) == 0 {
		return "", "", errors.New("empty checksum file")
	}

	tmp, err := os.CreateTemp("", "mneme-update-*")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmp.Name()
	defer tmp.Close()

	binResp, err := client.Get(binAsset.DownloadURL)
	if err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("fetch binary: %w", err)
	}
	defer binResp.Body.Close()
	if binResp.StatusCode != 200 {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("binary download returned %s", binResp.Status)
	}

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), binResp.Body); err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("write binary: %w", err)
	}
	gotSHA := hex.EncodeToString(hasher.Sum(nil))
	if gotSHA != expectedSHA[0] {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("SHA256 mismatch: got %s, want %s", gotSHA, expectedSHA[0])
	}
	return tmpPath, rel.TagName, nil
}

func fetchBytes(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// ReplaceBinary atomically swaps `target` with the file at `newPath`.
// On Unix this uses os.Rename which is atomic within a filesystem.
func ReplaceBinary(newPath, target string) error {
	// Ensure new file is executable.
	if err := os.Chmod(newPath, 0755); err != nil {
		return err
	}
	// Try cross-device fallback: copy if rename fails with cross-device link.
	if err := os.Rename(newPath, target); err != nil {
		// Fallback: copy then remove. Less atomic but works across mounts.
		in, ferr := os.Open(newPath)
		if ferr != nil {
			return err // return original error
		}
		defer in.Close()
		out, ferr := os.OpenFile(target, os.O_TRUNC|os.O_WRONLY, 0755)
		if ferr != nil {
			out, ferr = os.Create(target)
			if ferr != nil {
				return err
			}
		}
		defer out.Close()
		if _, ferr := io.Copy(out, in); ferr != nil {
			return ferr
		}
		os.Chmod(target, 0755)
		os.Remove(newPath)
		return nil
	}
	return os.Chmod(target, 0755)
}

// TempDirCleanup removes any "mneme-update-*" leftovers in os.TempDir().
func TempDirCleanup() {
	tmp := os.TempDir()
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "mneme-update-") {
			os.Remove(filepath.Join(tmp, e.Name()))
		}
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run "TestFetchReleaseAsset|TestDownloadRejectsBadSHA|TestReplaceBinaryAtomic" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/release.go tests/release_test.go
git commit -m "feat(m7): add updater.DownloadLatestBinary + ReplaceBinary"
```

---

## Task 14: `mneme update` CLI

**Files:**

- Create: `cmd/cmd_update.go`
- Modify: `cmd/main.go` (add `case "update"`)
- Modify: `cmd/usage.go`
- Test: `tests/cmd_update_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/cmd_update_test.go
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestUpdateCLISyncs(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()

	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)
	state.WriteTemplateVersion(proj, 0) // outdated
	os.WriteFile(filepath.Join(proj, ".mneme", "mneme.md"), []byte("OLD"), 0644)

	cmd := exec.Command(bin, "update", "--yes")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("update: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "updated") {
		t.Errorf("expected 'updated' in output: %s", out)
	}
	v, _ := state.ReadTemplateVersion(proj)
	if v != state.TemplateVersion {
		t.Errorf("post-update template version = %d, want %d", v, state.TemplateVersion)
	}
}

func TestUpdateBinaryWithoutReleaseChannel(t *testing.T) {
	bin := buildMnemeBinary(t)
	cmd := exec.Command(bin, "update", "--binary")
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "no GitHub release configured") {
		t.Errorf("expected configuration error, got: %s", out)
	}
}

func TestUpdateDryRunDoesNotMutate(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()

	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)
	state.WriteTemplateVersion(proj, 0)
	os.WriteFile(filepath.Join(proj, ".mneme", "mneme.md"), []byte("OLD"), 0644)

	cmd := exec.Command(bin, "update", "--dry-run")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "would update") {
		t.Errorf("expected 'would update' in dry-run output, got: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(proj, ".mneme", "mneme.md"))
	if string(data) != "OLD" {
		t.Errorf("dry-run mutated mneme.md: %q", string(data))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestUpdateCLI|TestUpdateBinary|TestUpdateDryRunDoesNotMutate" -v`
Expected: FAIL — `unknown subcommand: "update"`.

- [ ] **Step 3: Implement `cmd/cmd_update.go`**

```go
// cmd/cmd_update.go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/updater"
)

func dispatchUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	binaryFlag := fs.Bool("binary", false, "self-update the mneme binary from GitHub Releases")
	listFlag := fs.Bool("list", false, "list known projects without acting")
	dryRun := fs.Bool("dry-run", false, "show what would be updated without writing")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	project := fs.String("project", "", "limit sync to a single project ID")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	_ = yes // reserved for future interactive confirmation

	if *binaryFlag {
		runBinaryUpdate()
		return
	}
	if *listFlag {
		runListProjects()
		return
	}

	res, err := updater.SyncAll(updater.SyncOptions{
		DryRun:    *dryRun,
		ProjectID: *project,
		Verbose:   true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "update:", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Printf("\nDry run: %d would update.\n", res.WouldUpdate)
		return
	}
	fmt.Printf("\nDone: %d updated, %d skipped, %d failed.\n", res.Updated, res.Skipped, res.Failed)
	for _, f := range res.Failures {
		fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", f.Path, f.Err)
	}
	if res.Failed > 0 {
		os.Exit(1)
	}
}

func runBinaryUpdate() {
	if releaseChannel != "github" || releaseRepo == "" {
		fmt.Fprintln(os.Stderr, "no GitHub release configured for this build.")
		fmt.Fprintln(os.Stderr, "Build with: -ldflags=\"-X main.releaseChannel=github -X main.releaseRepo=owner/mneme\"")
		os.Exit(1)
	}
	tmpPath, ver, err := updater.DownloadLatestBinary(updater.DefaultGitHubAPIBase, releaseRepo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "update --binary:", err)
		os.Exit(1)
	}
	target, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "update --binary: locate self:", err)
		os.Exit(1)
	}
	if err := updater.ReplaceBinary(tmpPath, target); err != nil {
		fmt.Fprintln(os.Stderr, "update --binary: replace:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Updated mneme to %s\n", ver)
	fmt.Println("Now running template sync …")
	res, err := updater.SyncAll(updater.SyncOptions{Verbose: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "post-update sync:", err)
		os.Exit(1)
	}
	fmt.Printf("\nDone: %d updated, %d skipped.\n", res.Updated, res.Skipped)
}

func runListProjects() {
	origins, err := state.ListOrigins()
	if err != nil {
		fmt.Fprintln(os.Stderr, "list:", err)
		os.Exit(1)
	}
	if len(origins) == 0 {
		fmt.Println("No initialized projects.")
		return
	}
	fmt.Printf("%-40s  template-version  origin\n", "project-id")
	for id, root := range origins {
		v, _ := state.ReadTemplateVersion(root)
		fmt.Printf("%-40s  %-16d  %s\n", id, v, root)
	}
}
```

- [ ] **Step 4: Wire into `cmd/main.go`**

Add to switch:

```go
	case "update":
		dispatchUpdate(os.Args[2:])
```

- [ ] **Step 5: Update `cmd/usage.go`**

Add lines:

```
  mneme update            sync templates across all projects (--binary self-updates)
  mneme update --list     list initialized projects
```

- [ ] **Step 6: Run tests**

Run: `go test ./tests -run "TestUpdateCLI|TestUpdateBinary|TestUpdateDryRunDoesNotMutate" -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/cmd_update.go cmd/main.go cmd/usage.go tests/cmd_update_test.go
git commit -m "feat(m7): add mneme update [--binary|--list|--dry-run|--project]"
```

---

## Task 15: `mneme restore` command

**Files:**

- Create: `cmd/cmd_restore.go`
- Modify: `cmd/main.go`
- Modify: `cmd/usage.go`
- Test: `tests/cmd_restore_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tests/cmd_restore_test.go
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
	"github.com/ranwei/mneme/pkg/state"
)

func TestRestoreList(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)

	// Restore env so the helper sees the test home.
	t.Setenv("HOME", home)

	mneme := filepath.Join(proj, ".mneme", "mneme.md")
	os.WriteFile(mneme, []byte("v1"), 0644)
	installer.WriteBackup(installer.BackupOp{ProjectID: id, Operation: "test", Files: []string{mneme}, MnemeVer: "0.1"})

	cmd := exec.Command(bin, "restore", "--list")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restore --list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), id) && !strings.Contains(string(out), "T") {
		t.Errorf("restore --list missing project/timestamp; got: %s", out)
	}
}

func TestRestoreLatest(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)
	t.Setenv("HOME", home)

	mneme := filepath.Join(proj, ".mneme", "mneme.md")
	os.WriteFile(mneme, []byte("v1"), 0644)
	installer.WriteBackup(installer.BackupOp{ProjectID: id, Operation: "test", Files: []string{mneme}, MnemeVer: "0.1"})

	// Mutate then restore.
	os.WriteFile(mneme, []byte("v2"), 0644)

	cmd := exec.Command(bin, "restore", "--latest", "--yes")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restore --latest: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(mneme)
	if string(got) != "v1" {
		t.Errorf("post-restore mneme.md = %q, want %q", string(got), "v1")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run "TestRestore" -v`
Expected: FAIL — `unknown subcommand: "restore"`.

- [ ] **Step 3: Implement `cmd/cmd_restore.go`**

```go
// cmd/cmd_restore.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/ranwei/mneme/pkg/installer"
	"github.com/ranwei/mneme/pkg/state"
)

func dispatchRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	listFlag := fs.Bool("list", false, "list backups for current project")
	latest := fs.Bool("latest", false, "restore the most recent backup")
	yes := fs.Bool("yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	root := requireProjectRoot()
	id, err := state.ReadOrCreateLocalID(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "restore: read project id:", err)
		os.Exit(1)
	}

	backups, err := installer.ListBackups(id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "restore: list backups:", err)
		os.Exit(1)
	}
	if len(backups) == 0 {
		fmt.Println("No backups available for this project.")
		return
	}

	if *listFlag {
		fmt.Printf("%-32s  path\n", "timestamp (newest first)")
		for _, b := range backups {
			fmt.Printf("%-32s  %s\n", b.Timestamp, b.Path)
		}
		return
	}

	var chosen string
	switch {
	case *latest:
		chosen = backups[0].Timestamp
	case len(fs.Args()) >= 1:
		chosen = fs.Arg(0)
	default:
		// Interactive selection.
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "restore: no TTY; pass --latest or a timestamp explicitly")
			os.Exit(2)
		}
		fmt.Println("Available backups:")
		for i, b := range backups {
			fmt.Printf("  [%d] %s\n", i+1, b.Timestamp)
		}
		fmt.Print("Select [1]: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		choice := 1
		if n := strings.TrimSpace(line); n != "" {
			fmt.Sscanf(n, "%d", &choice)
		}
		if choice < 1 || choice > len(backups) {
			fmt.Fprintln(os.Stderr, "invalid selection")
			os.Exit(1)
		}
		chosen = backups[choice-1].Timestamp
	}

	if !*yes {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "restore: confirmation required (re-run with --yes)")
			os.Exit(2)
		}
		fmt.Printf("Restore backup %s? Existing files will be overwritten. [y/N]: ", chosen)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	if err := installer.RestoreBackup(chosen, id); err != nil {
		fmt.Fprintln(os.Stderr, "restore:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Restored from %s\n", chosen)
}
```

- [ ] **Step 4: Wire into `cmd/main.go`**

Add to switch:

```go
	case "restore":
		dispatchRestore(os.Args[2:])
```

- [ ] **Step 5: Update `cmd/usage.go`**

Add line:

```
  mneme restore           restore from backup (--list, --latest, or <timestamp>)
```

- [ ] **Step 6: Run tests**

Run: `go test ./tests -run "TestRestore" -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/cmd_restore.go cmd/main.go cmd/usage.go tests/cmd_restore_test.go
git commit -m "feat(m7): add mneme restore [--list|--latest|<timestamp>]"
```

---

## Task 16: End-to-end smoke + full test sweep + spec acceptance

**Files:**

- None (verification only).

- [ ] **Step 1: Build**

Run: `go build -o ./bin/mneme ./cmd`
Expected: PASS, no compile errors.

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 3: Manual smoke test**

```bash
TMP=$(mktemp -d)
HOME="$TMP/home" ./bin/mneme init --yes --no-scan --project 2>&1 | tail -3
cd "$TMP" && git init -q
# (re-run init inside the git tree if necessary)
HOME="$TMP/home" ./bin/mneme status
HOME="$TMP/home" ./bin/mneme status --json | head
HOME="$TMP/home" ./bin/mneme update --list
HOME="$TMP/home" ./bin/mneme update --dry-run
HOME="$TMP/home" ./bin/mneme buglog search nothing
HOME="$TMP/home" ./bin/mneme scan --check
cd - && rm -rf "$TMP"
```

Expected: every command exits cleanly with the expected message.

- [ ] **Step 4: Verify M7 acceptance criteria from spec §3**

For each spec deliverable, confirm a task implemented it:

- [x] `cmd/cmd_status.go` — Task 9
- [x] `cmd/cmd_scan.go --check` — Task 10
- [x] `cmd/cmd_update.go` — Task 14
- [x] `cmd/cmd_restore.go` — Task 15
- [x] `cmd/cmd_buglog.go search` — Task 11
- [x] `pkg/installer/templates/identity.md.tmpl` — Task 4
- [x] `pkg/installer/identity.go` — Task 5
- [x] `pkg/installer/backup.go` — Task 8
- [x] `~/.mneme/projects/<id>/origin` (state addition) — Task 2 (writer), Task 7 (init wires it)
- [x] `.mneme/template-version.txt` (state addition) — Task 3 (writer), Task 7 (init wires it)

- [ ] **Step 5: Final commit if any cleanup needed**

```bash
git status
# If anything uncommitted that should ship with M7, commit it; otherwise skip.
```

- [ ] **Step 6: Update spec acceptance**

Open `docs/superpowers/specs/2026-04-28-m7-m11-roadmap-design.md` and confirm M7 line items in §3 are not stale. No changes required if everything matches.

---

## Self-Review Notes

Coverage check against M7 deliverables in roadmap §3:

- ✅ `cmd_status.go` (Task 9) — daemon health, scanner staleness, embedding API reachability is **partially covered** (anatomy age + ledger present; embedding reachability deferred to M9 envcheck — acceptable per spec, status doesn't mandate it for M7). Vector DB row count is omitted intentionally — adding requires a `Store` interface call from `pkg/types.go`, which would expand M7 scope; the spec wording "vector DB row count" should be re-validated in M9 once daemon centralizes that lookup.
- ✅ `scan --check` (Task 10) — drift detection without modifying state.
- ✅ `update` two-phase (Tasks 12, 13, 14) — Phase A multi-project sync, Phase B GitHub Releases self-update with SHA256 verification, atomic replace, post-update sync.
- ✅ `restore` (Task 15) — interactive, `--list`, `--latest`, explicit timestamp.
- ✅ `bug search` (Task 11) — case-insensitive substring + token-overlap ranking via `pkg/match`.
- ✅ `identity.md` (Tasks 4–5, 7) — template, generator, init wiring.
- ✅ `mneme.md` (Tasks 4, 6, 7) — template, user-section preservation, init wiring.
- ✅ `backup.go` (Task 8) — used by Tasks 12, 15.
- ✅ State additions: `origin` (Task 2), `template-version.txt` (Task 3); init wiring (Task 7).

Type/method consistency: `state.WriteOrigin/ReadOrigin/ListOrigins`, `state.{Read,Write}TemplateVersion`, `installer.WriteBackup/ListBackups/RestoreBackup`, `installer.WriteIdentity/WriteMnemeMD/DetectProjectMetadata`, `updater.SyncAll/SyncOptions/SyncResult/DownloadLatestBinary/ReplaceBinary` — all referenced consistently across tasks.

Out-of-scope (deferred to later milestones, explicitly):

- Vector DB row count in status — needs `Store.Count()` interface change; defer to M8 (daemon centralizes DB lookup) or M9 (status enrichment).
- Daemon-aware CLI fallback — M8 introduces the daemon; M7's CLIs all run locally only. The `status` command's `probeDaemon` already prints a "no daemon — start with mneme daemon start (M8+)" note.
- Suggestions / waste reports / cerebrum learner / envcheck — explicit M9 scope.
