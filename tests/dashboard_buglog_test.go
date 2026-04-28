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
