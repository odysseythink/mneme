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
		Available bool             `json:"available"`
		Report    *designqc.Report `json:"report"`
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
