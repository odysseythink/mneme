# M10c — Remaining Dashboard Panels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the seven remaining dashboard panels (Cerebrum / Memory / Anatomy / BugLog / Suggestions / Token / DesignQC), the sidebar project picker, four mutation endpoints, and an inline SVG sparkline component — completing M10's openwolf parity and closing the M9 auto-learning loop in the browser.

**Architecture:** Backend adds one file per panel domain in `pkg/dashboard/` (each ~80–150 LOC) wiring 7 read endpoints + 4 mutation endpoints. Reads use `?project=<id>` and `state.ReadOrigin(id)` to resolve the project root. Mutations call existing helpers (`state.AppendCerebrumRule`, `cerebrum.RemovePending`, `cerebrum.AppendRejected`, `suggestions.AppendDismissed`, `state.WriteBuglog`) and publish a new event type so other tabs SSE-refetch. Frontend adds an `ActiveProjectProvider` (URL `?project=<id>`-synced React context), one new `<select>` in the sidebar, three small shared components (`Sparkline`, `ConfirmButton`, `ProjectPicker`), and seven panels.

**Tech Stack:** Go 1.25 (`net/http`, `encoding/json`, `path/filepath`), React 19, TypeScript 5.7, `react-router-dom@^7` (existing M10b), Tailwind v4. **No new third-party deps.**

**Spec:** `docs/superpowers/specs/2026-04-28-m10c-remaining-panels-design.md`

**Depends on M10b** being implemented (this plan assumes `pkg/events.Bus`, `useFetch`, `useSSE`, `AppShell`, `api/client.ts`, `events.jsonl`, `/api/overview`, `/api/projects`, `/api/activity`, `/api/cron`, `/events`, `/events/publish` are all in place).

---

## File structure

### Backend — created

| Path | Responsibility |
|---|---|
| `pkg/dashboard/cerebrum.go` | `CerebrumHandler` (GET), `CerebrumApproveHandler`, `CerebrumRejectHandler` |
| `pkg/dashboard/memory.go` | `MemoryHandler` (parses `~/.claude/mneme-memory.md`) |
| `pkg/dashboard/anatomy.go` | `AnatomyHandler` (groups `state.ReadAnatomy` by directory) |
| `pkg/dashboard/buglog.go` | `BugLogHandler`, `BugLogDeleteHandler` |
| `pkg/dashboard/suggestions.go` | `SuggestionsHandler`, `SuggestionsDismissHandler` |
| `pkg/dashboard/token.go` | `TokenHandler` (reads ledger + ledger-history) |
| `pkg/dashboard/designqc.go` | `DesignQCHandler` (returns `{available:false}`) |
| `tests/dashboard_cerebrum_test.go` | tests for the three cerebrum endpoints |
| `tests/dashboard_memory_test.go` | tests for memory parsing |
| `tests/dashboard_anatomy_test.go` | tests for anatomy grouping |
| `tests/dashboard_buglog_test.go` | tests for buglog read + delete |
| `tests/dashboard_suggestions_test.go` | tests for suggestions read + dismiss |
| `tests/dashboard_token_test.go` | tests for token totals + history |
| `tests/dashboard_designqc_test.go` | one-shot stub test |

### Backend — modified

| Path | Change |
|---|---|
| `pkg/daemon/routes.go` | Register 7 read + 4 mutation endpoints |
| `tests/integration/dashboard_panels_test.go` (M10b) | Add hits for `/api/cerebrum`, `/api/anatomy` |

### Frontend — created

| Path | Responsibility |
|---|---|
| `web/src/api/cerebrum.ts` | `getCerebrum(projectId)`, `approveCandidate`, `rejectCandidate` |
| `web/src/api/memory.ts` | `getMemory()` |
| `web/src/api/anatomy.ts` | `getAnatomy(projectId)` |
| `web/src/api/buglog.ts` | `getBugLog(projectId)`, `deleteEntry(projectId, entryId)` |
| `web/src/api/suggestions.ts` | `getSuggestions(projectId)`, `dismissSuggestion(projectId, suggestionId)` |
| `web/src/api/token.ts` | `getToken(projectId)` |
| `web/src/api/designqc.ts` | `getDesignQC(projectId)` |
| `web/src/hooks/useActiveProject.ts` | Context + URL `?project=` sync |
| `web/src/components/ProjectPicker.tsx` | Sidebar `<select>` |
| `web/src/components/Sparkline.tsx` | Inline SVG sparkline (~50 LOC) |
| `web/src/components/ConfirmButton.tsx` | Two-click inline confirm |
| `web/src/panels/Cerebrum.tsx` | Active rules + pending review + Approve/Reject |
| `web/src/panels/Memory.tsx` | Cross-project timeline |
| `web/src/panels/Anatomy.tsx` | Directory tree + filename search |
| `web/src/panels/BugLog.tsx` | Table + per-row Delete |
| `web/src/panels/Suggestions.tsx` | Card list + Dismiss |
| `web/src/panels/Token.tsx` | Totals + Sparkline + hook breakdown |
| `web/src/panels/DesignQC.tsx` | M11 empty state |
| `web/src/__tests__/ProjectPicker.test.tsx` | Picker URL sync |

### Frontend — modified

| Path | Change |
|---|---|
| `web/src/api/types.ts` | Add response shapes for the 7 new endpoints |
| `web/src/components/AppShell.tsx` | Add `<ProjectPicker/>` + 7 new `NavLink`s |
| `web/src/App.tsx` | Wrap router in `<ActiveProjectProvider/>`; add 7 routes |
| `web/src/__tests__/App.test.tsx` | Assert sidebar contains all 10 panel links |

---

## Conventions used in this plan

**Project resolution helper.** Every per-project handler starts with the same 3 lines. Define it once near the top of `pkg/dashboard/cerebrum.go` (the first file in this plan), then reuse from sibling files in the same package.

```go
// resolveProject reads the ?project= query, looks up its root, writes errors
// directly to w on failure. Returns ("", "") if the request was already responded to.
func resolveProject(w http.ResponseWriter, r *http.Request) (projectID, projectRoot string) {
	projectID = r.URL.Query().Get("project")
	if projectID == "" {
		http.Error(w, `{"error":"missing_project"}`, http.StatusBadRequest)
		return "", ""
	}
	root, err := state.ReadOrigin(projectID)
	if err != nil {
		http.Error(w, `{"error":"project_not_found"}`, http.StatusNotFound)
		return "", ""
	}
	return projectID, root
}
```

**Mutation body decode** is similar — extract once in `cerebrum.go`, reuse:

```go
type mutateBody struct {
	ProjectID string `json:"project_id"`
	ID        string `json:"-"`
}

func decodeMutate(w http.ResponseWriter, r *http.Request, idField string) (mutateBody, string, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return mutateBody{}, "", false
	}
	raw := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, `{"error":"bad_body"}`, http.StatusBadRequest)
		return mutateBody{}, "", false
	}
	body := mutateBody{ProjectID: raw["project_id"]}
	id := raw[idField]
	if body.ProjectID == "" || id == "" {
		http.Error(w, `{"error":"missing_field"}`, http.StatusBadRequest)
		return mutateBody{}, "", false
	}
	root, err := state.ReadOrigin(body.ProjectID)
	if err != nil {
		http.Error(w, `{"error":"project_not_found"}`, http.StatusNotFound)
		return mutateBody{}, "", false
	}
	body.ID = id
	return body, root, true
}
```

**Test stub logger** — every Go test in this plan uses the existing `stubLogger{}` in `tests/`.

**Tests run from repo root.** Build tag `integration` is required for `tests/integration/...`.

**Commit message format:** `feat(m10c):` / `test(m10c):` / `fix(m10c):`. Co-author trailer in actual interactive commits; omitted in plan examples for brevity.

---

## Task 1: pkg/dashboard/cerebrum.go — read + approve + reject

**Files:**
- Create: `pkg/dashboard/cerebrum.go`
- Create: `tests/dashboard_cerebrum_test.go`
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing tests**

Create `tests/dashboard_cerebrum_test.go`:

```go
package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func seedProject(t *testing.T, home, projectID string) string {
	t.Helper()
	root := filepath.Join(home, "work", projectID)
	if err := os.MkdirAll(filepath.Join(root, ".mneme"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(state.GlobalProjectDir(projectID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mneme", ".local-id"), []byte(projectID), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteOrigin(projectID, root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAPI_Cerebrum_GET(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	if err := state.AppendCerebrumRule(root, state.CerebrumRule{
		Comment: "always commit", Pattern: "TODO", Message: "remove TODO",
	}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/cerebrum?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Rules   []map[string]string `json:"rules"`
		Pending []map[string]any    `json:"pending"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Rules) != 1 || body.Rules[0]["pattern"] != "TODO" {
		t.Errorf("rules: got %+v", body.Rules)
	}
	if body.Pending == nil {
		t.Errorf("pending: got nil, want []")
	}
}

func TestAPI_Cerebrum_GET_MissingProject(t *testing.T) {
	bus, _ := events.NewBus(t.TempDir(), &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Bus: bus})

	req := httptest.NewRequest("GET", "/api/cerebrum", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing project: got %d, want 400", rec.Code)
	}
}

func TestAPI_Cerebrum_Approve(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	cand := cerebrum.Candidate{
		ID:        "cand-1",
		Trigger:   cerebrum.Trigger{Phrase: "we should always X", Turn: 1},
		DraftRule: state.CerebrumRule{Comment: "always X", Pattern: "X", Message: "do X"},
		QueuedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := cerebrum.SavePending(root, []cerebrum.Candidate{cand}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	body, _ := json.Marshal(map[string]string{"project_id": "p1", "candidate_id": "cand-1"})
	req := httptest.NewRequest("POST", "/cerebrum/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("approve: status %d body=%s", rec.Code, rec.Body.String())
	}

	rules, _ := state.ReadCerebrum(root)
	if len(rules) != 1 || rules[0].Pattern != "X" {
		t.Errorf("rule was not appended: %+v", rules)
	}
	pending, _ := cerebrum.LoadPending(root)
	if len(pending) != 0 {
		t.Errorf("candidate was not removed from pending: %d remain", len(pending))
	}
}

func TestAPI_Cerebrum_Reject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	if err := cerebrum.SavePending(root, []cerebrum.Candidate{{ID: "cand-2"}}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	body, _ := json.Marshal(map[string]string{"project_id": "p1", "candidate_id": "cand-2"})
	req := httptest.NewRequest("POST", "/cerebrum/reject", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("reject: status %d body=%s", rec.Code, rec.Body.String())
	}
	pending, _ := cerebrum.LoadPending(root)
	if len(pending) != 0 {
		t.Errorf("candidate was not removed: %d remain", len(pending))
	}
}
```

`cerebrum.SavePending(root, []Candidate) error` already exists in `pkg/cerebrum/pending.go:78` — the test uses it directly, no helper needs adding.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./tests -run TestAPI_Cerebrum -v
```

Expected: FAIL — handler doesn't exist (404).

- [ ] **Step 3: Create `pkg/dashboard/cerebrum.go`**

```go
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

// resolveProject reads ?project= from r, looks up the project root,
// writes an error response on failure. Returns ("", "") on failure.
func resolveProject(w http.ResponseWriter, r *http.Request) (string, string) {
	projectID := r.URL.Query().Get("project")
	if projectID == "" {
		http.Error(w, `{"error":"missing_project"}`, http.StatusBadRequest)
		return "", ""
	}
	root, err := state.ReadOrigin(projectID)
	if err != nil {
		http.Error(w, `{"error":"project_not_found"}`, http.StatusNotFound)
		return "", ""
	}
	return projectID, root
}

// decodeMutate parses a POST body of {project_id, <idField>: ...}.
// On failure, writes the error and returns (_, _, false).
func decodeMutate(w http.ResponseWriter, r *http.Request, idField string) (string, string, string, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return "", "", "", false
	}
	raw := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, `{"error":"bad_body"}`, http.StatusBadRequest)
		return "", "", "", false
	}
	pid := raw["project_id"]
	id := raw[idField]
	if pid == "" || id == "" {
		http.Error(w, `{"error":"missing_field"}`, http.StatusBadRequest)
		return "", "", "", false
	}
	root, err := state.ReadOrigin(pid)
	if err != nil {
		http.Error(w, `{"error":"project_not_found"}`, http.StatusNotFound)
		return "", "", "", false
	}
	return pid, root, id, true
}

func publishEvent(bus *events.Bus, projectID, eventType string, payload map[string]any) {
	if bus == nil {
		return
	}
	data, _ := json.Marshal(payload)
	bus.Publish(events.Event{
		TS:        time.Now().UnixMilli(),
		Type:      eventType,
		ProjectID: projectID,
		Data:      data,
	})
}

// CerebrumHandler returns active rules + pending candidates for the project.
func CerebrumHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		rules, err := state.ReadCerebrum(root)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		pending, err := cerebrum.LoadPending(root)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		if rules == nil {
			rules = []state.CerebrumRule{}
		}
		if pending == nil {
			pending = []cerebrum.Candidate{}
		}
		writeJSON(w, 200, map[string]any{
			"rules":   rules,
			"pending": pending,
		})
	}
}

// CerebrumApproveHandler appends the candidate's draft rule to .mneme/cerebrum.md
// and removes the candidate from pending.
func CerebrumApproveHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, candID, ok := decodeMutate(w, r, "candidate_id")
		if !ok {
			return
		}
		pending, err := cerebrum.LoadPending(root)
		if err != nil {
			http.Error(w, `{"error":"load_pending"}`, http.StatusInternalServerError)
			return
		}
		var found *cerebrum.Candidate
		for i := range pending {
			if pending[i].ID == candID {
				found = &pending[i]
				break
			}
		}
		if found == nil {
			http.Error(w, `{"error":"candidate_not_found"}`, http.StatusNotFound)
			return
		}
		if err := state.AppendCerebrumRule(root, found.DraftRule); err != nil {
			http.Error(w, `{"error":"append_rule"}`, http.StatusInternalServerError)
			return
		}
		if err := cerebrum.RemovePending(root, map[string]bool{candID: true}); err != nil {
			http.Error(w, `{"error":"remove_pending"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "cerebrum.approved", map[string]any{
			"candidate_id": candID, "rule_pattern": found.DraftRule.Pattern,
		})
		writeJSON(w, 200, map[string]any{"status": "approved"})
	}
}

// CerebrumRejectHandler appends to rejected list and removes from pending.
func CerebrumRejectHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, candID, ok := decodeMutate(w, r, "candidate_id")
		if !ok {
			return
		}
		if err := cerebrum.AppendRejected(root, candID, time.Now().UTC()); err != nil {
			http.Error(w, `{"error":"append_rejected"}`, http.StatusInternalServerError)
			return
		}
		if err := cerebrum.RemovePending(root, map[string]bool{candID: true}); err != nil {
			http.Error(w, `{"error":"remove_pending"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "cerebrum.rejected", map[string]any{"candidate_id": candID})
		writeJSON(w, 200, map[string]any{"status": "rejected"})
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

In `NewMux`, after the existing M10b registrations:

```go
mux.Handle("/api/cerebrum", dashboard.CerebrumHandler())
mux.Handle("/cerebrum/approve", dashboard.CerebrumApproveHandler(deps.Bus))
mux.Handle("/cerebrum/reject", dashboard.CerebrumRejectHandler(deps.Bus))
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_Cerebrum -v
```

Expected: 4 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/cerebrum.go pkg/daemon/routes.go tests/dashboard_cerebrum_test.go
git commit -m "feat(m10c): /api/cerebrum + approve/reject endpoints"
```

---

## Task 2: pkg/dashboard/memory.go — cross-project timeline

**Files:**
- Create: `pkg/dashboard/memory.go`
- Create: `tests/dashboard_memory_test.go`
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing test**

Create `tests/dashboard_memory_test.go`:

```go
package tests

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
)

func TestAPI_Memory_ParsesRows(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "## 2026-04-28T10:00:00Z\nturns: 5\nworked on auth refactor\n\n" +
		"## 2026-04-28T11:00:00Z\nturns: 3\nfixed login bug\n"
	if err := os.WriteFile(filepath.Join(dir, "mneme-memory.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/memory", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Rows []map[string]any `json:"rows"`
		Raw  string           `json:"raw"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Rows) != 2 {
		t.Errorf("rows: got %d, want 2", len(body.Rows))
	}
	if body.Raw == "" {
		t.Errorf("raw should always be returned")
	}
}

func TestAPI_Memory_EmptyFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/memory", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Rows []map[string]any `json:"rows"`
		Raw  string           `json:"raw"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Rows == nil {
		t.Errorf("rows should be [] not nil")
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestAPI_Memory -v
```

Expected: FAIL — 404.

- [ ] **Step 3: Create `pkg/dashboard/memory.go`**

```go
package dashboard

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MemoryRow is the parsed-row shape returned by /api/memory.
type MemoryRow struct {
	StartedAt string `json:"started_at"`
	TurnCount int    `json:"turn_count"`
	Summary   string `json:"summary"`
}

// MemoryHandler serves the cross-project memory file.
func MemoryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		home, err := os.UserHomeDir()
		if err != nil {
			http.Error(w, `{"error":"home"}`, http.StatusInternalServerError)
			return
		}
		path := filepath.Join(home, ".claude", "mneme-memory.md")
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			http.Error(w, `{"error":"read"}`, http.StatusInternalServerError)
			return
		}
		raw := string(data)
		rows := parseMemoryRows(raw)
		writeJSON(w, 200, map[string]any{"rows": rows, "raw": raw})
	}
}

// parseMemoryRows splits on "## " section headers and extracts the timestamp
// (first line after the header), an optional "turns: N" line, and the rest as
// summary. Best-effort: malformed sections are skipped silently.
func parseMemoryRows(raw string) []MemoryRow {
	out := []MemoryRow{}
	if raw == "" {
		return out
	}
	sections := strings.Split(raw, "\n## ")
	if len(sections) > 0 && strings.HasPrefix(sections[0], "## ") {
		sections[0] = strings.TrimPrefix(sections[0], "## ")
	} else if len(sections) > 0 {
		// First section before any ## header — skip.
		sections = sections[1:]
	}
	for _, s := range sections {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		lines := strings.SplitN(s, "\n", 2)
		ts := strings.TrimSpace(lines[0])
		if ts == "" {
			continue
		}
		row := MemoryRow{StartedAt: ts}
		body := ""
		if len(lines) > 1 {
			body = lines[1]
		}
		if strings.HasPrefix(body, "turns: ") {
			rest := strings.SplitN(body, "\n", 2)
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(rest[0], "turns: "))); err == nil {
				row.TurnCount = n
			}
			if len(rest) > 1 {
				body = rest[1]
			} else {
				body = ""
			}
		}
		row.Summary = strings.TrimSpace(body)
		out = append(out, row)
	}
	return out
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/memory", dashboard.MemoryHandler())
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_Memory -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/memory.go pkg/daemon/routes.go tests/dashboard_memory_test.go
git commit -m "feat(m10c): /api/memory cross-project timeline"
```

---

## Task 3: pkg/dashboard/anatomy.go — directory-grouped tree

**Files:**
- Create: `pkg/dashboard/anatomy.go`
- Create: `tests/dashboard_anatomy_test.go`
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing test**

Create `tests/dashboard_anatomy_test.go`:

```go
package tests

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func TestAPI_Anatomy_GroupsByDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	if err := state.WriteAnatomy(root, []state.AnatomyEntry{
		{Path: "pkg/a/x.go", Description: "x", EstTokens: 100, Language: "go"},
		{Path: "pkg/a/y.go", Description: "y", EstTokens: 50, Language: "go"},
		{Path: "pkg/b/z.go", Description: "z", EstTokens: 75, Language: "go"},
	}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/anatomy?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Directories []struct {
			Path  string                   `json:"path"`
			Files []map[string]interface{} `json:"files"`
		} `json:"directories"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Directories) != 2 {
		t.Errorf("directories: got %d, want 2", len(body.Directories))
	}
	for _, d := range body.Directories {
		if d.Path == "pkg/a" && len(d.Files) != 2 {
			t.Errorf("pkg/a: got %d files, want 2", len(d.Files))
		}
		if d.Path == "pkg/b" && len(d.Files) != 1 {
			t.Errorf("pkg/b: got %d files, want 1", len(d.Files))
		}
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestAPI_Anatomy -v
```

Expected: FAIL — 404.

- [ ] **Step 3: Create `pkg/dashboard/anatomy.go`**

```go
package dashboard

import (
	"net/http"
	"path/filepath"
	"sort"

	"github.com/ranwei/mneme/pkg/state"
)

type AnatomyFile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	EstTokens   int    `json:"est_tokens"`
	Language    string `json:"language"`
}

type AnatomyDir struct {
	Path  string        `json:"path"`
	Files []AnatomyFile `json:"files"`
}

func AnatomyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		entries, err := state.ReadAnatomy(root)
		if err != nil {
			http.Error(w, `{"error":"read_anatomy"}`, http.StatusInternalServerError)
			return
		}
		// Group by directory.
		groups := map[string][]AnatomyFile{}
		for path, e := range entries {
			dir := filepath.Dir(path)
			groups[dir] = append(groups[dir], AnatomyFile{
				Name: filepath.Base(path), Description: e.Description,
				EstTokens: e.EstTokens, Language: e.Language,
			})
		}
		dirs := make([]string, 0, len(groups))
		for d := range groups {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		out := make([]AnatomyDir, 0, len(dirs))
		for _, d := range dirs {
			files := groups[d]
			sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
			out = append(out, AnatomyDir{Path: d, Files: files})
		}
		genTime, _ := state.ReadAnatomyGeneratedTime(root)
		writeJSON(w, 200, map[string]any{
			"directories":  out,
			"generated_at": genTime.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/anatomy", dashboard.AnatomyHandler())
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_Anatomy -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/anatomy.go pkg/daemon/routes.go tests/dashboard_anatomy_test.go
git commit -m "feat(m10c): /api/anatomy directory-grouped tree"
```

---

## Task 4: pkg/dashboard/buglog.go — read + delete

**Files:**
- Create: `pkg/dashboard/buglog.go`
- Create: `tests/dashboard_buglog_test.go`
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing tests**

Create `tests/dashboard_buglog_test.go`:

```go
package tests

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func TestAPI_BugLog_GET(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	if err := state.AppendBuglogEntry(root, state.BuglogEntry{
		ID: "bug-1", File: "main.go", Description: "missed nil check",
	}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/buglog?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Entries []map[string]any `json:"entries"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Entries) != 1 || body.Entries[0]["id"] != "bug-1" {
		t.Errorf("entries: %+v", body.Entries)
	}
}

func TestAPI_BugLog_Delete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	state.AppendBuglogEntry(root, state.BuglogEntry{ID: "bug-1", Description: "a"})
	state.AppendBuglogEntry(root, state.BuglogEntry{ID: "bug-2", Description: "b"})

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	body, _ := json.Marshal(map[string]string{"project_id": "p1", "entry_id": "bug-1"})
	req := httptest.NewRequest("POST", "/buglog/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("delete: status %d body=%s", rec.Code, rec.Body.String())
	}
	entries, _ := state.ReadBuglog(root)
	if len(entries) != 1 || entries[0].ID != "bug-2" {
		t.Errorf("expected only bug-2 left, got %+v", entries)
	}
}

func TestAPI_BugLog_Delete_Unknown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	body, _ := json.Marshal(map[string]string{"project_id": "p1", "entry_id": "nope"})
	req := httptest.NewRequest("POST", "/buglog/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("delete unknown: got %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./tests -run TestAPI_BugLog -v
```

Expected: FAIL — handler doesn't exist.

- [ ] **Step 3: Create `pkg/dashboard/buglog.go`**

```go
package dashboard

import (
	"net/http"

	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func BugLogHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		entries, err := state.ReadBuglog(root)
		if err != nil {
			http.Error(w, `{"error":"read_buglog"}`, http.StatusInternalServerError)
			return
		}
		if entries == nil {
			entries = []state.BuglogEntry{}
		}
		writeJSON(w, 200, map[string]any{"entries": entries})
	}
}

func BugLogDeleteHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, entryID, ok := decodeMutate(w, r, "entry_id")
		if !ok {
			return
		}
		entries, err := state.ReadBuglog(root)
		if err != nil {
			http.Error(w, `{"error":"read_buglog"}`, http.StatusInternalServerError)
			return
		}
		var kept []state.BuglogEntry
		found := false
		for _, e := range entries {
			if e.ID == entryID {
				found = true
				continue
			}
			kept = append(kept, e)
		}
		if !found {
			http.Error(w, `{"error":"entry_not_found"}`, http.StatusNotFound)
			return
		}
		if err := state.WriteBuglog(root, kept); err != nil {
			http.Error(w, `{"error":"write_buglog"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "buglog.deleted", map[string]any{"entry_id": entryID})
		writeJSON(w, 200, map[string]any{"status": "deleted"})
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/buglog", dashboard.BugLogHandler())
mux.Handle("/buglog/delete", dashboard.BugLogDeleteHandler(deps.Bus))
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_BugLog -v
```

Expected: 3 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/buglog.go pkg/daemon/routes.go tests/dashboard_buglog_test.go
git commit -m "feat(m10c): /api/buglog + delete endpoint"
```

---

## Task 5: pkg/dashboard/suggestions.go — read + dismiss

**Files:**
- Create: `pkg/dashboard/suggestions.go`
- Create: `tests/dashboard_suggestions_test.go`
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing tests**

Create `tests/dashboard_suggestions_test.go`:

```go
package tests

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/suggestions"
)

func writeSuggestionsFile(t *testing.T, projectID string, entries []suggestions.Suggestion) {
	t.Helper()
	path := filepath.Join(state.GlobalProjectDir(projectID), "suggestions.json")
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAPI_Suggestions_GET(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	writeSuggestionsFile(t, "p1", []suggestions.Suggestion{
		{ID: "s1", Type: "memory_bloat", Title: "Memory is large", Detail: "...", GeneratedAt: time.Now().UTC().Format(time.RFC3339)},
	})

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/suggestions?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Suggestions []map[string]any `json:"suggestions"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Suggestions) != 1 {
		t.Errorf("suggestions: got %d, want 1", len(body.Suggestions))
	}
}

func TestAPI_Suggestions_Dismiss(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	writeSuggestionsFile(t, "p1", []suggestions.Suggestion{
		{ID: "s1", Type: "memory_bloat", Title: "x", Detail: "y", GeneratedAt: time.Now().UTC().Format(time.RFC3339)},
	})

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	body, _ := json.Marshal(map[string]string{"project_id": "p1", "suggestion_id": "s1"})
	req := httptest.NewRequest("POST", "/suggestions/dismiss", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("dismiss: status %d body=%s", rec.Code, rec.Body.String())
	}
	dismissed, err := suggestions.LoadDismissed(root, 30, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !dismissed["s1"] {
		t.Errorf("dismissed map missing s1: %+v", dismissed)
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./tests -run TestAPI_Suggestions -v
```

Expected: FAIL — 404.

- [ ] **Step 3: Create `pkg/dashboard/suggestions.go`**

```go
package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/suggestions"
)

func SuggestionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projectID, root := resolveProject(w, r)
		if root == "" {
			return
		}
		// Read raw suggestions file (engine.List filters by dismissed; we
		// reuse that below).
		path := filepath.Join(state.GlobalProjectDir(projectID), "suggestions.json")
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			http.Error(w, `{"error":"read"}`, http.StatusInternalServerError)
			return
		}
		var all []suggestions.Suggestion
		if len(data) > 0 {
			_ = json.Unmarshal(data, &all)
		}
		dismissed, _ := suggestions.LoadDismissed(root, 30, time.Now().UTC())
		out := make([]suggestions.Suggestion, 0, len(all))
		for _, s := range all {
			if !dismissed[s.ID] {
				out = append(out, s)
			}
		}
		writeJSON(w, 200, map[string]any{"suggestions": out})
	}
}

func SuggestionsDismissHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, sid, ok := decodeMutate(w, r, "suggestion_id")
		if !ok {
			return
		}
		if err := suggestions.AppendDismissed(root, sid, time.Now().UTC()); err != nil {
			http.Error(w, `{"error":"append_dismissed"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "suggestion.dismissed", map[string]any{"suggestion_id": sid})
		writeJSON(w, 200, map[string]any{"status": "dismissed"})
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/suggestions", dashboard.SuggestionsHandler())
mux.Handle("/suggestions/dismiss", dashboard.SuggestionsDismissHandler(deps.Bus))
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_Suggestions -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/suggestions.go pkg/daemon/routes.go tests/dashboard_suggestions_test.go
git commit -m "feat(m10c): /api/suggestions + dismiss endpoint"
```

---

## Task 6: pkg/dashboard/token.go — totals + history

**Files:**
- Create: `pkg/dashboard/token.go`
- Create: `tests/dashboard_token_test.go`
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing test**

Create `tests/dashboard_token_test.go`:

```go
package tests

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func TestAPI_Token_TotalsAndHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	state.IncrementSafe(root, "hook_fired.pre-write")
	state.IncrementSafe(root, "scan_count")

	if err := state.AppendLedgerHistory(root, state.LedgerSnapshot{
		TS: time.Now().UTC().Format(time.RFC3339), SessionID: "s1",
	}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/token?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Totals  map[string]any   `json:"totals"`
		History []map[string]any `json:"history"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Totals == nil {
		t.Errorf("totals missing")
	}
	if len(body.History) != 1 {
		t.Errorf("history: got %d, want 1", len(body.History))
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestAPI_Token -v
```

Expected: FAIL — 404.

- [ ] **Step 3: Create `pkg/dashboard/token.go`**

```go
package dashboard

import (
	"net/http"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const tokenHistoryCap = 12

func TokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		ledger, err := state.ReadLedger(root)
		if err != nil {
			http.Error(w, `{"error":"read_ledger"}`, http.StatusInternalServerError)
			return
		}
		// since=0 returns everything; we cap to last tokenHistoryCap below.
		hist, _ := state.ReadLedgerHistory(root, time.Time{})
		if len(hist) > tokenHistoryCap {
			hist = hist[len(hist)-tokenHistoryCap:]
		}
		out := map[string]any{
			"totals":         ledger.Totals,
			"first_recorded": ledger.FirstRecorded,
			"last_updated":   ledger.LastUpdated,
			"history":        hist,
		}
		writeJSON(w, 200, out)
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/token", dashboard.TokenHandler())
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_Token -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/token.go pkg/daemon/routes.go tests/dashboard_token_test.go
git commit -m "feat(m10c): /api/token totals + history (last 12 snapshots)"
```

---

## Task 7: pkg/dashboard/designqc.go — M11 stub

**Files:**
- Create: `pkg/dashboard/designqc.go`
- Create: `tests/dashboard_designqc_test.go`
- Modify: `pkg/daemon/routes.go`

- [ ] **Step 1: Write the failing test**

Create `tests/dashboard_designqc_test.go`:

```go
package tests

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
)

func TestAPI_DesignQC_Stub(t *testing.T) {
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
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Available bool             `json:"available"`
		Reason    string           `json:"reason"`
		Captures  []map[string]any `json:"captures"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Available {
		t.Errorf("available should be false until M11")
	}
	if body.Captures == nil {
		t.Errorf("captures should be [] not nil")
	}
}
```

- [ ] **Step 2: Run test**

```bash
go test ./tests -run TestAPI_DesignQC -v
```

Expected: FAIL — 404.

- [ ] **Step 3: Create `pkg/dashboard/designqc.go`**

```go
package dashboard

import (
	"net/http"
)

func DesignQCHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		// Validate project=<id> exists; reuse the helper for parity with
		// other panels even though the stub does not read project state.
		if _, root := resolveProject(w, r); root == "" {
			return
		}
		writeJSON(w, 200, map[string]any{
			"available": false,
			"reason":    "M11 not yet implemented",
			"captures":  []any{},
		})
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/designqc", dashboard.DesignQCHandler())
```

- [ ] **Step 5: Run tests**

```bash
go test ./tests -run TestAPI_DesignQC -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/designqc.go pkg/daemon/routes.go tests/dashboard_designqc_test.go
git commit -m "feat(m10c): /api/designqc stub (available:false until M11)"
```

---

## Task 8: Frontend — `useActiveProject` + `<ProjectPicker/>`

**Files:**
- Create: `web/src/hooks/useActiveProject.ts`
- Create: `web/src/components/ProjectPicker.tsx`
- Create: `web/src/__tests__/ProjectPicker.test.tsx`

- [ ] **Step 1: Create `web/src/hooks/useActiveProject.ts`**

```tsx
import { createContext, useContext, useEffect, useMemo, type ReactNode } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useFetch } from './useFetch'
import { getProjects } from '../api/projects'
import type { ProjectSummary } from '../api/types'

type ActiveProjectValue = {
  active: string | null
  projects: ProjectSummary[]
  setActive: (id: string) => void
  loading: boolean
}

const Ctx = createContext<ActiveProjectValue | null>(null)

export function ActiveProjectProvider({ children }: { children: ReactNode }): JSX.Element {
  const { data, loading } = useFetch(getProjects)
  const [params, setParams] = useSearchParams()
  const queryProject = params.get('project')

  // If URL has no ?project=, default to first registered project.
  useEffect(() => {
    if (loading || queryProject) return
    const first = data?.projects[0]?.id
    if (first) {
      const next = new URLSearchParams(params)
      next.set('project', first)
      setParams(next, { replace: true })
    }
  }, [loading, queryProject, data, params, setParams])

  const value = useMemo<ActiveProjectValue>(() => ({
    active: queryProject,
    projects: data?.projects ?? [],
    loading,
    setActive: (id: string) => {
      const next = new URLSearchParams(params)
      next.set('project', id)
      setParams(next)
    },
  }), [queryProject, data, loading, params, setParams])

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useActiveProject(): ActiveProjectValue {
  const v = useContext(Ctx)
  if (!v) throw new Error('useActiveProject must be used inside ActiveProjectProvider')
  return v
}
```

- [ ] **Step 2: Create `web/src/components/ProjectPicker.tsx`**

```tsx
import { useActiveProject } from '../hooks/useActiveProject'

export function ProjectPicker(): JSX.Element {
  const { active, projects, setActive, loading } = useActiveProject()
  if (loading) return <div className="text-xs text-gray-500">loading…</div>
  if (projects.length === 0) {
    return <div className="text-xs text-gray-500">no projects registered</div>
  }
  return (
    <select
      value={active ?? ''}
      onChange={(e) => setActive(e.target.value)}
      className="w-full rounded border px-2 py-1 bg-white"
    >
      {projects.map(p => (
        <option key={p.id} value={p.id}>{p.origin || p.id}</option>
      ))}
    </select>
  )
}
```

- [ ] **Step 3: Create `web/src/__tests__/ProjectPicker.test.tsx`**

```tsx
import { describe, expect, test, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route, useSearchParams } from 'react-router-dom'
import { ActiveProjectProvider } from '../hooks/useActiveProject'
import { ProjectPicker } from '../components/ProjectPicker'

vi.stubGlobal('fetch', vi.fn(async (url: string) => {
  if (url.includes('/api/projects')) {
    return new Response(JSON.stringify({
      projects: [
        { id: 'p1', origin: '/work/a', anatomy_files: 0, cerebrum_pending: 0, memory_bytes: 0, last_activity_ts: 0 },
        { id: 'p2', origin: '/work/b', anatomy_files: 0, cerebrum_pending: 0, memory_bytes: 0, last_activity_ts: 0 },
      ],
    }), { headers: { 'Content-Type': 'application/json' } })
  }
  return new Response('{}')
}))

function ShowQuery(): JSX.Element {
  const [params] = useSearchParams()
  return <div data-testid="q">{params.get('project') ?? 'none'}</div>
}

describe('ProjectPicker', () => {
  test('writes project to URL on change', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <ActiveProjectProvider>
          <Routes>
            <Route path="/" element={<><ProjectPicker /><ShowQuery /></>} />
          </Routes>
        </ActiveProjectProvider>
      </MemoryRouter>
    )
    // Wait for first project to populate the URL.
    await screen.findByText('p1')
    const select = await screen.findByRole('combobox')
    await userEvent.selectOptions(select, 'p2')
    expect(screen.getByTestId('q').textContent).toBe('p2')
  })
})
```

- [ ] **Step 4: Install `@testing-library/user-event`**

```bash
cd web && pnpm add -D @testing-library/user-event@^14 && cd ..
```

- [ ] **Step 5: Run the test**

```bash
cd web && pnpm test -- ProjectPicker && cd ..
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/hooks/useActiveProject.ts web/src/components/ProjectPicker.tsx web/src/__tests__/ProjectPicker.test.tsx web/package.json web/pnpm-lock.yaml
git commit -m "feat(m10c): useActiveProject context + ProjectPicker (URL-synced)"
```

---

## Task 9: Frontend — Sparkline + ConfirmButton components

**Files:**
- Create: `web/src/components/Sparkline.tsx`
- Create: `web/src/components/ConfirmButton.tsx`

- [ ] **Step 1: Create `web/src/components/Sparkline.tsx`**

```tsx
type SparklineProps = {
  data: number[]
  width?: number
  height?: number
  color?: string
}

export function Sparkline({ data, width = 120, height = 30, color = '#2563eb' }: SparklineProps): JSX.Element {
  if (data.length < 2) return <span style={{ display: 'inline-block', width, height }} />
  const max = Math.max(...data)
  const min = Math.min(...data)
  const range = max - min || 1
  const stepX = width / (data.length - 1)
  const points = data
    .map((v, i) => `${i * stepX},${height - ((v - min) / range) * height}`)
    .join(' ')
  return (
    <svg width={width} height={height} className="overflow-visible">
      <polyline points={points} fill="none" stroke={color} strokeWidth={1.5} />
    </svg>
  )
}
```

- [ ] **Step 2: Create `web/src/components/ConfirmButton.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react'

type ConfirmButtonProps = {
  onConfirm: () => Promise<void> | void
  label: string
  confirmLabel?: string
  className?: string
}

export function ConfirmButton({ onConfirm, label, confirmLabel = 'Confirm?', className = '' }: ConfirmButtonProps): JSX.Element {
  const [armed, setArmed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const timer = useRef<number | null>(null)

  useEffect(() => () => { if (timer.current) window.clearTimeout(timer.current) }, [])

  const arm = () => {
    setArmed(true)
    if (timer.current) window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setArmed(false), 5000)
  }

  const fire = async () => {
    setBusy(true); setErr(null)
    try { await onConfirm(); setArmed(false) }
    catch (e) { setErr((e as Error).message) }
    finally { setBusy(false) }
  }

  return (
    <span className="inline-flex items-center gap-2">
      <button
        type="button"
        disabled={busy}
        onClick={armed ? fire : arm}
        className={'px-2 py-1 rounded border text-sm disabled:opacity-50 ' + (armed ? 'bg-red-100 border-red-400' : 'hover:bg-gray-100') + ' ' + className}
      >
        {armed ? confirmLabel : label}
      </button>
      {err && <span className="text-xs text-red-700">{err}</span>}
    </span>
  )
}
```

- [ ] **Step 3: Type-check**

```bash
cd web && pnpm tsc --noEmit && cd ..
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Sparkline.tsx web/src/components/ConfirmButton.tsx
git commit -m "feat(m10c): Sparkline + ConfirmButton shared components"
```

---

## Task 10: Frontend — api/types + 7 api wrappers

**Files:**
- Modify: `web/src/api/types.ts`
- Create: `web/src/api/cerebrum.ts`
- Create: `web/src/api/memory.ts`
- Create: `web/src/api/anatomy.ts`
- Create: `web/src/api/buglog.ts`
- Create: `web/src/api/suggestions.ts`
- Create: `web/src/api/token.ts`
- Create: `web/src/api/designqc.ts`

- [ ] **Step 1: Append to `web/src/api/types.ts`**

```ts
export type CerebrumRule = {
  comment: string
  pattern: string
  message: string
}

export type CerebrumTrigger = {
  phrase: string
  user_msg: string
  prior_asst: string
  turn: number
}

export type CerebrumCandidate = {
  id: string
  trigger: CerebrumTrigger
  draft_rule: CerebrumRule
  confidence: number
  queued_at: string
  hit_count: number
}

export type CerebrumResponse = {
  rules: CerebrumRule[]
  pending: CerebrumCandidate[]
}

export type MemoryRow = {
  started_at: string
  turn_count: number
  summary: string
}

export type MemoryResponse = {
  rows: MemoryRow[]
  raw: string
}

export type AnatomyFile = {
  name: string
  description: string
  est_tokens: number
  language: string
}

export type AnatomyDir = {
  path: string
  files: AnatomyFile[]
}

export type AnatomyResponse = {
  directories: AnatomyDir[]
  generated_at: string
}

export type BugLogEntry = {
  id: string
  created_at: string
  source: string
  file: string
  description: string
  bad_code: string
}

export type BugLogResponse = {
  entries: BugLogEntry[]
}

export type SuggestionEntry = {
  id: string
  type: string
  target: string
  title: string
  detail: string
  generated_at: string
}

export type SuggestionsResponse = {
  suggestions: SuggestionEntry[]
}

export type LedgerTotals = {
  hook_fired: Record<string, number>
  hook_errors: number
  stdin_parse_failures: number
  outside_project_skipped: number
  write_skipped: number
  anatomy_hits: number
  repeat_reads: number
  scan_count: number
  edit_patterns?: Record<string, number>
  memory_rows_written?: number
}

export type LedgerSnapshot = {
  ts: string
  session_id: string
  totals: LedgerTotals
}

export type TokenResponse = {
  totals: LedgerTotals
  first_recorded: string
  last_updated: string
  history: LedgerSnapshot[]
}

export type DesignQCResponse = {
  available: boolean
  reason: string
  captures: unknown[]
}
```

- [ ] **Step 2: Create the 7 api wrapper files**

`web/src/api/cerebrum.ts`:
```ts
import { request } from './client'
import type { CerebrumResponse } from './types'

export const getCerebrum = (projectId: string) =>
  request<CerebrumResponse>('GET', `/api/cerebrum?project=${encodeURIComponent(projectId)}`)
export const approveCandidate = (projectId: string, candidateId: string) =>
  request<{ status: string }>('POST', '/cerebrum/approve', { project_id: projectId, candidate_id: candidateId })
export const rejectCandidate = (projectId: string, candidateId: string) =>
  request<{ status: string }>('POST', '/cerebrum/reject', { project_id: projectId, candidate_id: candidateId })
```

`web/src/api/memory.ts`:
```ts
import { request } from './client'
import type { MemoryResponse } from './types'

export const getMemory = () => request<MemoryResponse>('GET', '/api/memory')
```

`web/src/api/anatomy.ts`:
```ts
import { request } from './client'
import type { AnatomyResponse } from './types'

export const getAnatomy = (projectId: string) =>
  request<AnatomyResponse>('GET', `/api/anatomy?project=${encodeURIComponent(projectId)}`)
```

`web/src/api/buglog.ts`:
```ts
import { request } from './client'
import type { BugLogResponse } from './types'

export const getBugLog = (projectId: string) =>
  request<BugLogResponse>('GET', `/api/buglog?project=${encodeURIComponent(projectId)}`)
export const deleteEntry = (projectId: string, entryId: string) =>
  request<{ status: string }>('POST', '/buglog/delete', { project_id: projectId, entry_id: entryId })
```

`web/src/api/suggestions.ts`:
```ts
import { request } from './client'
import type { SuggestionsResponse } from './types'

export const getSuggestions = (projectId: string) =>
  request<SuggestionsResponse>('GET', `/api/suggestions?project=${encodeURIComponent(projectId)}`)
export const dismissSuggestion = (projectId: string, suggestionId: string) =>
  request<{ status: string }>('POST', '/suggestions/dismiss', { project_id: projectId, suggestion_id: suggestionId })
```

`web/src/api/token.ts`:
```ts
import { request } from './client'
import type { TokenResponse } from './types'

export const getToken = (projectId: string) =>
  request<TokenResponse>('GET', `/api/token?project=${encodeURIComponent(projectId)}`)
```

`web/src/api/designqc.ts`:
```ts
import { request } from './client'
import type { DesignQCResponse } from './types'

export const getDesignQC = (projectId: string) =>
  request<DesignQCResponse>('GET', `/api/designqc?project=${encodeURIComponent(projectId)}`)
```

- [ ] **Step 3: Type-check**

```bash
cd web && pnpm tsc --noEmit && cd ..
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/api/
git commit -m "feat(m10c): frontend api types + 7 wrappers"
```

---

## Task 11: AppShell extension + App.tsx routes

**Files:**
- Modify: `web/src/components/AppShell.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/__tests__/App.test.tsx`

- [ ] **Step 1: Modify `web/src/components/AppShell.tsx`**

Add `<ProjectPicker/>` + 7 new `NavLink`s. Replace the contents:

```tsx
import { Link, NavLink, Outlet } from 'react-router-dom'
import { useSSEConnected } from '../hooks/useSSE'
import { useActiveProject } from '../hooks/useActiveProject'
import { ProjectPicker } from './ProjectPicker'

export function AppShell(): JSX.Element {
  const connected = useSSEConnected()
  const { active } = useActiveProject()
  const q = active ? `?project=${encodeURIComponent(active)}` : ''

  return (
    <div className="flex min-h-screen">
      <aside className="w-60 border-r bg-gray-50 p-4">
        <Link to="/" className="block text-lg font-semibold mb-4">mneme</Link>
        <div className="mb-4"><ProjectPicker /></div>
        <nav className="flex flex-col gap-1">
          <NavLink to="/" end className={navClass}>Overview</NavLink>
          <NavLink to="/activity" className={navClass}>Activity</NavLink>
          <NavLink to="/cron" className={navClass}>Cron</NavLink>
          <hr className="my-2" />
          <NavLink to={'/cerebrum' + q} className={navClass}>Cerebrum</NavLink>
          <NavLink to="/memory" className={navClass}>Memory</NavLink>
          <NavLink to={'/anatomy' + q} className={navClass}>Anatomy</NavLink>
          <NavLink to={'/buglog' + q} className={navClass}>BugLog</NavLink>
          <NavLink to={'/suggestions' + q} className={navClass}>Suggestions</NavLink>
          <NavLink to={'/token' + q} className={navClass}>Token</NavLink>
          <NavLink to={'/designqc' + q} className={navClass}>DesignQC</NavLink>
        </nav>
        <div className="mt-8 text-xs text-gray-500">
          stream: <span className={connected ? 'text-green-600' : 'text-red-600'}>{connected ? 'live' : 'offline'}</span>
        </div>
      </aside>
      <main className="flex-1 p-6 overflow-auto"><Outlet /></main>
    </div>
  )
}

function navClass({ isActive }: { isActive: boolean }) {
  return 'px-2 py-1 rounded ' + (isActive ? 'bg-gray-200 font-medium' : 'hover:bg-gray-100')
}
```

- [ ] **Step 2: Modify `web/src/App.tsx`**

```tsx
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { Overview } from './panels/Overview'
import { Activity } from './panels/Activity'
import { Cron } from './panels/Cron'
import { Cerebrum } from './panels/Cerebrum'
import { Memory } from './panels/Memory'
import { Anatomy } from './panels/Anatomy'
import { BugLog } from './panels/BugLog'
import { Suggestions } from './panels/Suggestions'
import { Token } from './panels/Token'
import { DesignQC } from './panels/DesignQC'
import { Bootstrap } from './Bootstrap'
import { SSEProvider } from './hooks/useSSE'
import { ActiveProjectProvider } from './hooks/useActiveProject'

const router = createBrowserRouter([
  {
    path: '/',
    element: <ActiveProjectProvider><AppShell /></ActiveProjectProvider>,
    children: [
      { index: true, element: <Overview /> },
      { path: 'activity', element: <Activity /> },
      { path: 'cron', element: <Cron /> },
      { path: 'cerebrum', element: <Cerebrum /> },
      { path: 'memory', element: <Memory /> },
      { path: 'anatomy', element: <Anatomy /> },
      { path: 'buglog', element: <BugLog /> },
      { path: 'suggestions', element: <Suggestions /> },
      { path: 'token', element: <Token /> },
      { path: 'designqc', element: <DesignQC /> },
    ],
  },
])

export function App(): JSX.Element {
  return (
    <>
      <Bootstrap />
      <SSEProvider>
        <RouterProvider router={router} />
      </SSEProvider>
    </>
  )
}
```

> **Note:** `ActiveProjectProvider` is wrapped around `<AppShell/>` (not the whole router) because it depends on `useSearchParams` which only works inside a router context. Tests must remember to mount providers in this order.

- [ ] **Step 3: Extend the App smoke test**

Replace `web/src/__tests__/App.test.tsx`:

```tsx
import { describe, expect, test, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { App } from '../App'

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.includes('/api/overview')) {
      return new Response(JSON.stringify({
        daemon: { pid: 1, uptime_s: 1, version: 't', started_at: 0 },
        totals: { projects: 0, anatomy_files: 0, cerebrum_pending: 0, open_suggestions: 0 },
        projects: [],
      }), { headers: { 'Content-Type': 'application/json' } })
    }
    if (url.includes('/api/projects')) {
      return new Response(JSON.stringify({
        projects: [{ id: 'p1', origin: '/work/a', anatomy_files: 0, cerebrum_pending: 0, memory_bytes: 0, last_activity_ts: 0 }],
      }), { headers: { 'Content-Type': 'application/json' } })
    }
    return new Response('{}', { headers: { 'Content-Type': 'application/json' } })
  }))
})

describe('App', () => {
  test('sidebar shows all 10 panel links', async () => {
    render(<App />)
    await waitFor(() => {
      for (const label of ['Overview', 'Activity', 'Cron', 'Cerebrum', 'Memory', 'Anatomy', 'BugLog', 'Suggestions', 'Token', 'DesignQC']) {
        expect(screen.getByRole('link', { name: label })).toBeInTheDocument()
      }
    })
  })
})
```

> **Note:** at this point, `App.tsx` references panel components that don't exist yet. Subsequent tasks (12–18) create them. Type-check / build will fail until all are in place; that's expected. Tests for individual components are not run here.

- [ ] **Step 4: Stub the missing panels (so the build doesn't break before Task 12)**

Create temporary stubs so type-check passes after this task:

```bash
for p in Cerebrum Memory Anatomy BugLog Suggestions Token DesignQC; do
  cat > "web/src/panels/$p.tsx" <<EOF
export function $p(): JSX.Element {
  return <div className="text-gray-500">$p panel — implementation in progress</div>
}
EOF
done
```

- [ ] **Step 5: Type-check**

```bash
cd web && pnpm tsc --noEmit && cd ..
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/AppShell.tsx web/src/App.tsx web/src/__tests__/App.test.tsx web/src/panels/
git commit -m "feat(m10c): wire 7 routes + panel stubs + project picker in sidebar"
```

---

## Task 12: Cerebrum panel (with approve/reject)

**Files:**
- Modify: `web/src/panels/Cerebrum.tsx`

- [ ] **Step 1: Replace the stub**

```tsx
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getCerebrum, approveCandidate, rejectCandidate } from '../api/cerebrum'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'

export function Cerebrum(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getCerebrum(active) : () => Promise.resolve({ rules: [], pending: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['cerebrum.candidate', 'cerebrum.approved', 'cerebrum.rejected'], () => refetch())

  if (!active) return <NoProject />
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">cerebrum</h1>

      <section>
        <h2 className="text-lg font-medium mb-2">active rules ({data.rules.length})</h2>
        {data.rules.length === 0 ? (
          <div className="text-sm text-gray-500">no rules yet</div>
        ) : (
          <ul className="rounded border bg-white">
            {data.rules.map((r, i) => (
              <li key={i} className="border-b last:border-b-0 px-3 py-2 text-sm">
                <div className="text-gray-500 text-xs mb-1">{r.comment}</div>
                <div><span className="font-mono bg-gray-100 px-1 rounded">{r.pattern}</span> → {r.message}</div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <h2 className="text-lg font-medium mb-2">pending review ({data.pending.length})</h2>
        {data.pending.length === 0 ? (
          <div className="text-sm text-gray-500">no candidates pending</div>
        ) : (
          <div className="flex flex-col gap-3">
            {data.pending.map((c) => (
              <div key={c.id} className="rounded border bg-white p-3">
                <div className="text-xs text-gray-500 mb-1">trigger</div>
                <div className="italic mb-2">"{c.trigger.phrase}"</div>
                {c.trigger.prior_asst && (
                  <>
                    <div className="text-xs text-gray-500 mb-1">context</div>
                    <div className="text-sm text-gray-700 mb-2 line-clamp-3">{c.trigger.prior_asst}</div>
                  </>
                )}
                <div className="text-xs text-gray-500 mb-1">draft rule</div>
                <div className="text-sm mb-3">
                  <span className="font-mono bg-gray-100 px-1 rounded">{c.draft_rule.pattern}</span> → {c.draft_rule.message}
                </div>
                <div className="flex gap-2">
                  <ConfirmButton
                    label="Approve"
                    onConfirm={async () => { await approveCandidate(active, c.id); refetch() }}
                  />
                  <ConfirmButton
                    label="Reject"
                    onConfirm={async () => { await rejectCandidate(active, c.id); refetch() }}
                  />
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function NoProject(): JSX.Element {
  return <div className="text-gray-500">No project selected. Pick one in the sidebar.</div>
}
```

- [ ] **Step 2: Type-check + commit**

```bash
cd web && pnpm tsc --noEmit && cd ..
git add web/src/panels/Cerebrum.tsx
git commit -m "feat(m10c): Cerebrum panel with Approve/Reject"
```

---

## Task 13: Memory panel

**Files:**
- Modify: `web/src/panels/Memory.tsx`

- [ ] **Step 1: Replace the stub**

```tsx
import { useFetch } from '../hooks/useFetch'
import { getMemory } from '../api/memory'

export function Memory(): JSX.Element {
  const { data, error } = useFetch(getMemory, { intervalMs: 60_000 })

  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">memory</h1>
      <div className="text-xs text-gray-600">Memory is cross-project. The sidebar picker is ignored on this page.</div>

      {data.rows.length === 0 ? (
        data.raw ? (
          <pre className="rounded border bg-white p-3 text-sm whitespace-pre-wrap">{data.raw}</pre>
        ) : (
          <div className="text-gray-500">no memory rows yet</div>
        )
      ) : (
        <ul className="flex flex-col gap-2">
          {data.rows.map((row, i) => (
            <li key={i} className="rounded border bg-white p-3 flex gap-3">
              <div className="text-xs text-gray-500 w-40 shrink-0">{row.started_at}</div>
              {row.turn_count > 0 && (
                <span className="text-xs bg-gray-100 rounded px-2 py-0.5 self-start shrink-0">{row.turn_count} turns</span>
              )}
              <div className="text-sm whitespace-pre-wrap flex-1">{row.summary}</div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd web && pnpm tsc --noEmit && cd ..
git add web/src/panels/Memory.tsx
git commit -m "feat(m10c): Memory panel (cross-project timeline)"
```

---

## Task 14: Anatomy panel

**Files:**
- Modify: `web/src/panels/Anatomy.tsx`

- [ ] **Step 1: Replace the stub**

```tsx
import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getAnatomy } from '../api/anatomy'
import { useActiveProject } from '../hooks/useActiveProject'

export function Anatomy(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getAnatomy(active) : () => Promise.resolve({ directories: [], generated_at: '' })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 60_000 })
  useSSE(['scan.complete'], () => refetch())
  const [filter, setFilter] = useState('')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  const needle = filter.trim().toLowerCase()
  const dirs = needle === '' ? data.directories : data.directories
    .map(d => ({ ...d, files: d.files.filter(f => f.name.toLowerCase().includes(needle)) }))
    .filter(d => d.files.length > 0)

  const toggle = (path: string) => {
    setCollapsed(prev => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path); else next.add(path)
      return next
    })
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">anatomy</h1>
      <div className="text-xs text-gray-500">generated {data.generated_at || '(never)'}</div>
      <input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder="filter by filename…"
        className="w-full rounded border px-3 py-2 text-sm"
      />
      {dirs.length === 0 ? (
        <div className="text-gray-500">no files {needle && 'match'}</div>
      ) : (
        <div className="flex flex-col gap-3">
          {dirs.map(d => {
            const isCollapsed = collapsed.has(d.path)
            return (
              <div key={d.path} className="rounded border bg-white">
                <button
                  className="w-full text-left px-3 py-2 font-medium border-b bg-gray-50 hover:bg-gray-100"
                  onClick={() => toggle(d.path)}
                >
                  {isCollapsed ? '▶' : '▼'} {d.path}/ ({d.files.length})
                </button>
                {!isCollapsed && (
                  <ul>
                    {d.files.map(f => (
                      <li key={f.name} className="px-3 py-2 border-b last:border-b-0 text-sm flex gap-3">
                        <span className="font-mono">{f.name}</span>
                        <span className="text-xs bg-gray-100 rounded px-1 py-0.5 self-start shrink-0">{f.est_tokens}t</span>
                        <span className="text-gray-700 flex-1">{f.description}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd web && pnpm tsc --noEmit && cd ..
git add web/src/panels/Anatomy.tsx
git commit -m "feat(m10c): Anatomy panel (collapsible tree + filename filter)"
```

---

## Task 15: BugLog panel (with delete)

**Files:**
- Modify: `web/src/panels/BugLog.tsx`

- [ ] **Step 1: Replace the stub**

```tsx
import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getBugLog, deleteEntry } from '../api/buglog'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'

export function BugLog(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getBugLog(active) : () => Promise.resolve({ entries: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['buglog.deleted'], () => refetch())
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  const toggle = (id: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">buglog</h1>
      {data.entries.length === 0 ? (
        <div className="text-gray-500">no bug entries</div>
      ) : (
        <table className="w-full bg-white rounded border">
          <thead className="text-left text-sm text-gray-600 border-b">
            <tr>
              <th className="px-3 py-2 w-40">created</th>
              <th className="px-3 py-2 w-20">source</th>
              <th className="px-3 py-2">file</th>
              <th className="px-3 py-2">description</th>
              <th className="px-3 py-2 w-32">actions</th>
            </tr>
          </thead>
          <tbody>
            {data.entries.map(e => (
              <>
                <tr key={e.id} className="border-b text-sm">
                  <td className="px-3 py-2 cursor-pointer" onClick={() => toggle(e.id)}>{e.created_at}</td>
                  <td className="px-3 py-2 cursor-pointer" onClick={() => toggle(e.id)}>{e.source}</td>
                  <td className="px-3 py-2 font-mono text-xs cursor-pointer" onClick={() => toggle(e.id)}>{e.file}</td>
                  <td className="px-3 py-2 cursor-pointer" onClick={() => toggle(e.id)}>{e.description}</td>
                  <td className="px-3 py-2">
                    <ConfirmButton
                      label="Delete"
                      onConfirm={async () => { await deleteEntry(active, e.id); refetch() }}
                    />
                  </td>
                </tr>
                {expanded.has(e.id) && e.bad_code && (
                  <tr key={e.id + '-code'} className="border-b">
                    <td colSpan={5} className="px-3 py-2 bg-gray-50">
                      <pre className="text-xs overflow-auto whitespace-pre">{e.bad_code}</pre>
                    </td>
                  </tr>
                )}
              </>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd web && pnpm tsc --noEmit && cd ..
git add web/src/panels/BugLog.tsx
git commit -m "feat(m10c): BugLog panel with row expand + Delete"
```

---

## Task 16: Suggestions panel (with dismiss)

**Files:**
- Modify: `web/src/panels/Suggestions.tsx`

- [ ] **Step 1: Replace the stub**

```tsx
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getSuggestions, dismissSuggestion } from '../api/suggestions'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'

export function Suggestions(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getSuggestions(active) : () => Promise.resolve({ suggestions: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['suggestion.new', 'suggestion.dismissed'], () => refetch())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">suggestions</h1>
      {data.suggestions.length === 0 ? (
        <div className="text-gray-500">no open suggestions</div>
      ) : (
        <div className="flex flex-col gap-3">
          {data.suggestions.map(s => (
            <div key={s.id} className="rounded border bg-white p-3">
              <div className="flex gap-2 items-center mb-1">
                <span className="text-xs bg-gray-100 rounded px-2 py-0.5">{s.type}</span>
                <span className="text-xs text-gray-500">{s.generated_at}</span>
              </div>
              <div className="font-medium">{s.title}</div>
              <div className="text-sm text-gray-700 mt-1 mb-3 whitespace-pre-wrap">{s.detail}</div>
              <ConfirmButton
                label="Dismiss"
                onConfirm={async () => { await dismissSuggestion(active, s.id); refetch() }}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd web && pnpm tsc --noEmit && cd ..
git add web/src/panels/Suggestions.tsx
git commit -m "feat(m10c): Suggestions panel with Dismiss"
```

---

## Task 17: Token panel (with sparkline)

**Files:**
- Modify: `web/src/panels/Token.tsx`

- [ ] **Step 1: Replace the stub**

```tsx
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getToken } from '../api/token'
import { useActiveProject } from '../hooks/useActiveProject'
import { Sparkline } from '../components/Sparkline'

export function Token(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getToken(active) : () => Promise.resolve(null as never)
  const { data, error, refetch } = useFetch(fn, { intervalMs: 10_000 })
  useSSE(['hook.fired', 'cron.tick'], () => refetch())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  const totals = data.totals
  const hookFired = totals.hook_fired ?? {}
  const totalFires = Object.values(hookFired).reduce((a, b) => a + b, 0)
  const trend = data.history.map(h => Object.values(h.totals.hook_fired ?? {}).reduce((a, b) => a + b, 0))

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">token</h1>

      <section className="rounded border bg-white p-3">
        <div className="text-sm text-gray-600 mb-2">totals</div>
        <div className="grid grid-cols-4 gap-3">
          <Stat label="hook fires" value={totalFires} />
          <Stat label="anatomy hits" value={totals.anatomy_hits} />
          <Stat label="repeat reads" value={totals.repeat_reads} />
          <Stat label="scan count" value={totals.scan_count} />
        </div>
      </section>

      <section className="rounded border bg-white p-3">
        <div className="text-sm text-gray-600 mb-2">trend (last {trend.length} snapshots)</div>
        {trend.length < 2 ? (
          <div className="text-gray-500 text-sm">Not enough history yet.</div>
        ) : (
          <Sparkline data={trend} width={240} height={40} />
        )}
      </section>

      <section className="rounded border bg-white p-3">
        <div className="text-sm text-gray-600 mb-2">hook fires by type</div>
        <ul className="text-sm">
          {Object.entries(hookFired).map(([k, v]) => (
            <li key={k} className="flex justify-between border-b last:border-b-0 py-1">
              <span className="font-mono text-xs">{k}</span>
              <span>{v}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }): JSX.Element {
  return (
    <div>
      <div className="text-xs text-gray-600">{label}</div>
      <div className="text-2xl font-semibold mt-1">{value}</div>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd web && pnpm tsc --noEmit && cd ..
git add web/src/panels/Token.tsx
git commit -m "feat(m10c): Token panel with totals + sparkline + breakdown"
```

---

## Task 18: DesignQC panel (M11 stub)

**Files:**
- Modify: `web/src/panels/DesignQC.tsx`

- [ ] **Step 1: Replace the stub**

```tsx
import { useFetch } from '../hooks/useFetch'
import { getDesignQC } from '../api/designqc'
import { useActiveProject } from '../hooks/useActiveProject'

export function DesignQC(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getDesignQC(active) : () => Promise.resolve({ available: false, reason: '', captures: [] })
  const { data } = useFetch(fn)

  if (!active) return <div className="text-gray-500">No project selected.</div>

  if (!data || !data.available) {
    return (
      <div>
        <h1 className="text-2xl font-semibold mb-4">design qc</h1>
        <div className="rounded border bg-white p-4 max-w-2xl">
          <p className="font-medium mb-2">Design QC is part of M11.</p>
          <p className="text-sm text-gray-700">
            Once <code className="bg-gray-100 px-1 rounded">mneme designqc</code> runs against this project,
            captures and the latest report will appear here.
          </p>
        </div>
      </div>
    )
  }

  // M11 future: render captures grid + report. Reachable only when /api/designqc returns available:true.
  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">design qc</h1>
      <div className="text-sm text-gray-500">{data.captures.length} captures (UI lands in M11)</div>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd web && pnpm tsc --noEmit && cd ..
git add web/src/panels/DesignQC.tsx
git commit -m "feat(m10c): DesignQC stub panel with M11 empty state"
```

---

## Task 19: Integration test + final verification

**Files:**
- Modify: `tests/integration/dashboard_panels_test.go`

- [ ] **Step 1: Extend the integration smoke**

Locate the existing slice in `dashboard_panels_test.go` (M10b) that hits `/api/overview`, `/api/projects`, etc. Add `/api/cerebrum?project=<seeded>` and `/api/anatomy?project=<seeded>`. Since the test uses a fresh `home` with no seeded project, both endpoints will respond 400 ("missing_project") or 404 — that's fine for a smoke check that verifies the route is registered. Use `?project=fake` and accept 404 as the success criterion.

Replace the for-loop:

```go
type endpointCheck struct {
	path      string
	wantCodes []int
}
checks := []endpointCheck{
	{"/api/overview", []int{200}},
	{"/api/projects", []int{200}},
	{"/api/activity?limit=5", []int{200}},
	{"/api/cron", []int{200}},
	{"/api/cerebrum?project=fake", []int{404}},
	{"/api/anatomy?project=fake", []int{404}},
	{"/api/memory", []int{200}},
	{"/api/designqc?project=fake", []int{404}},
}
for _, c := range checks {
	resp, err := client.Get("http://unix" + c.path)
	if err != nil {
		t.Errorf("GET %s: %v", c.path, err)
		continue
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	ok := false
	for _, code := range c.wantCodes {
		if resp.StatusCode == code {
			ok = true
			break
		}
	}
	if !ok {
		t.Errorf("GET %s: status=%d body=%s", c.path, resp.StatusCode, body)
	}
}
```

- [ ] **Step 2: Run all Go tests**

```bash
go test ./... -count=1
go test -tags=integration ./tests/integration/... -count=1
```

Expected: all pass.

- [ ] **Step 3: Run all frontend tests + build**

```bash
cd web && pnpm test && pnpm tsc --noEmit && pnpm build && cd ..
make web-build
```

Expected: tests pass, type-check passes, vite build succeeds, `pkg/dashboard/dist/` updated.

- [ ] **Step 4: Manual acceptance**

```bash
go build -o ./bin/mneme ./cmd
./bin/mneme daemon stop || true
./bin/mneme daemon start
sleep 1
./bin/mneme dashboard
```

Verify in browser:
- Sidebar shows the project picker + 10 nav links.
- Switching the picker updates `?project=<id>` in the URL; refresh keeps the selection.
- `/cerebrum` lists rules + pending; if a candidate is present, Approve appends to `.mneme/cerebrum.md` and the row disappears within ~500 ms.
- `/memory` shows timeline rows from `~/.claude/mneme-memory.md`.
- `/anatomy` shows the directory tree; filename filter works; toggle expand/collapse works.
- `/buglog` lists entries (seed one via `mneme buglog add` if needed); Delete confirms then removes.
- `/suggestions` shows open suggestions; Dismiss removes a row.
- `/token` shows totals + sparkline (or "Not enough history yet.") + breakdown.
- `/designqc` shows the M11 empty state.

```bash
./bin/mneme daemon stop
```

- [ ] **Step 5: Commit any updated `pkg/dashboard/dist/`**

```bash
git status
# If anything changed (regenerated dist), commit it.
git add pkg/dashboard/dist/ tests/integration/dashboard_panels_test.go
git commit -m "test(m10c): integration smoke + regenerated dashboard dist"
```

- [ ] **Step 6: Push branch + open PR**

```bash
git push -u origin feat/m10c
gh pr create --title "feat(m10c): seven remaining dashboard panels + project picker" --body "$(cat <<'EOF'
## Summary
- 7 new panels: Cerebrum (with approve/reject), Memory, Anatomy, BugLog (with delete), Suggestions (with dismiss), Token (with sparkline), DesignQC (M11 stub)
- Sidebar project picker synced to ?project=<id>
- 4 new mutation endpoints: /cerebrum/approve, /cerebrum/reject, /buglog/delete, /suggestions/dismiss
- New event types: cerebrum.approved, cerebrum.rejected, buglog.deleted, suggestion.dismissed
- Custom inline SVG <Sparkline/> + reusable <ConfirmButton/>
- No new third-party deps

## Test plan
- [ ] go test ./... + go test -tags=integration ./tests/integration/...
- [ ] pnpm test, pnpm tsc --noEmit, pnpm build
- [ ] Manual: project picker drives URL; all 7 panels render; mutations land in state files; SSE refreshes other tabs
EOF
)"
```

---

## Self-review against spec

**Spec coverage:**
- §3.1 7 read endpoints → Tasks 1-7
- §3.2 4 mutation endpoints → Tasks 1, 4, 5
- §3.3 4 new event types (cerebrum.approved/rejected, suggestion.dismissed, buglog.deleted) → published in Tasks 1, 4, 5
- §2.2 ActiveProjectProvider + ?project= URL sync → Task 8
- §2.4 Frontend file layout (api / hooks / components / panels) → Tasks 8-18
- §4.1 Routes (10 total in App.tsx) → Task 11
- §4.2 Panel sketches (active rules + pending review, timeline, tree, table, cards, totals+sparkline, empty state) → Tasks 12-18
- §4.3 Sparkline → Task 9
- §4.4 ConfirmButton → Task 9
- §6 Tests → Tasks 1-7 (Go) + Task 8 (ProjectPicker) + Task 11 (App smoke updated) + Task 19 (integration extended)
- §7 Acceptance criteria → Task 19 step 4

**Type/method consistency:**
- `state.ReadOrigin(projectID) (string, error)` — used by `resolveProject` and `decodeMutate`
- `state.ReadCerebrum(root) ([]CerebrumRule, error)` — Task 1
- `cerebrum.LoadPending(root) ([]Candidate, error)`, `cerebrum.RemovePending(root, map[string]bool) error`, `cerebrum.AppendRejected(root, id, time) error`, `cerebrum.SavePending(root, []Candidate) error` — Tasks 1
- `state.AppendCerebrumRule(root, CerebrumRule) error` — Task 1
- `state.ReadAnatomy(root) (map[string]AnatomyEntry, error)` — Task 3
- `state.ReadAnatomyGeneratedTime(root) (time.Time, error)` — Task 3
- `state.ReadBuglog`, `state.WriteBuglog`, `state.AppendBuglogEntry`, `state.BuglogEntry` — Task 4
- `suggestions.AppendDismissed`, `suggestions.LoadDismissed`, `suggestions.Suggestion` (fields ID/Type/Target/Title/Detail/GeneratedAt) — Task 5
- `state.ReadLedger`, `state.ReadLedgerHistory`, `state.LedgerSnapshot`, `state.LedgerTotals` — Task 6
- Frontend types in `web/src/api/types.ts` mirror Go field names exactly (Task 10)

**Placeholder scan:** no TBD / TODO / "implement later" / "similar to" — every step has full code or a complete shell command. Stubs in Task 11 step 4 are minimal one-line components; their full implementations land in Tasks 12-18.

---
