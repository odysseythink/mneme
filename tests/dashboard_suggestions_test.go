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
