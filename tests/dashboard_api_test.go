package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
)

func TestAPI_Overview_EmptyHome(t *testing.T) {
	home := t.TempDir()
	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()

	mux := daemon.NewMux(daemon.RouteDeps{
		Log:       &stubLogger{},
		Home:      home,
		PID:       42,
		Version:   "test",
		StartedAt: time.Now().Unix() - 100,
		Bus:       bus,
	})
	req := httptest.NewRequest("GET", "/api/overview", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d, body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Daemon struct {
			PID     int    `json:"pid"`
			Version string `json:"version"`
		} `json:"daemon"`
		Totals struct {
			Projects int `json:"projects"`
		} `json:"totals"`
		Projects []map[string]interface{} `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Daemon.PID != 42 {
		t.Errorf("daemon.pid = %d, want 42", body.Daemon.PID)
	}
	if body.Totals.Projects != 0 {
		t.Errorf("totals.projects = %d, want 0", body.Totals.Projects)
	}
	if body.Projects == nil {
		t.Errorf("projects field should be [], got nil")
	}
	_ = http.StatusOK
}

func TestAPI_Overview_OneProject(t *testing.T) {
	home := t.TempDir()
	pid := "abc123"
	pdir := filepath.Join(home, ".mneme", "projects", pid)
	if err := os.MkdirAll(pdir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "origin"), []byte("/work/foo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "anatomy.md"), []byte("# Anatomy\n## a.go\n## b.go\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()

	mux := daemon.NewMux(daemon.RouteDeps{
		Log: &stubLogger{}, Home: home, Bus: bus,
	})
	req := httptest.NewRequest("GET", "/api/overview", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	totals, _ := body["totals"].(map[string]interface{})
	if totals["projects"].(float64) != 1 {
		t.Errorf("totals.projects = %v, want 1", totals["projects"])
	}
}

func TestAPI_Projects(t *testing.T) {
	home := t.TempDir()
	for _, pid := range []string{"a", "b"} {
		dir := filepath.Join(home, ".mneme", "projects", pid)
		os.MkdirAll(dir, 0o700)
		os.WriteFile(filepath.Join(dir, "origin"), []byte("/x/"+pid), 0o600)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Projects) != 2 {
		t.Errorf("got %d projects, want 2", len(body.Projects))
	}
}
