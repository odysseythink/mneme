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
