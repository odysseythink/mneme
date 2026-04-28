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
	if len(body.Rules) != 1 || body.Rules[0]["Pattern"] != "TODO" {
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
