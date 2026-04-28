package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
)

func TestRoutes_Health(t *testing.T) {
	mux := daemon.NewMux(daemon.RouteDeps{
		Log:       &stubLogger{},
		PID:       12345,
		Version:   "0.2.0-m8",
		StartedAt: 1714290000,
	})
	req := httptest.NewRequest("GET", "/health", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["pid"].(float64) != 12345 {
		t.Errorf("pid = %v", body["pid"])
	}
	if body["version"] != "0.2.0-m8" {
		t.Errorf("version = %v", body["version"])
	}
	if _, ok := body["uptime_s"]; !ok {
		t.Errorf("uptime_s missing")
	}
}

func TestRoutes_CronList(t *testing.T) {
	home := t.TempDir()
	if _, err := daemon.LoadOrSeedManifest(home); err != nil {
		t.Fatal(err)
	}

	mux := daemon.NewMux(daemon.RouteDeps{
		Log:  &stubLogger{},
		Home: home,
	})

	req := httptest.NewRequest("GET", "/cron/list", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "anatomy-rescan") {
		t.Errorf("body missing seeded task: %s", rec.Body.String())
	}
}

func TestRoutes_CronRun_UnknownTask(t *testing.T) {
	home := t.TempDir()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home})

	body, _ := json.Marshal(map[string]string{"name": "no-such-task"})
	req := httptest.NewRequest("POST", "/cron/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRoutes_404(t *testing.T) {
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}})
	req := httptest.NewRequest("GET", "/no-such-path", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
