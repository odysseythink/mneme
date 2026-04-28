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

	// Verify the server is reachable
	t.Logf("Test server at %s", srv.URL)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("server check failed: %v", err)
	}
	resp.Body.Close()
	t.Logf("server check passed")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	report, err := designqc.Run(ctx, designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		BaseURL:       srv.URL,
		Framework:     "vite",
		RouteOverride: []string{"/"},
		Quality:       80,
		MaxWidth:      1440,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Version != 1 || len(report.Captures) < 1 {
		t.Fatalf("report mismatch: expected at least 1 capture, got %d captures", len(report.Captures))
	}
	// Note: chromedp has issues with httptest + timeout contexts for multiple routes
	// This test verifies successful capture of at least one route works end-to-end
	t.Logf("successfully captured %d route(s)", len(report.Captures))

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
	got, err := designqc.ReadReport(filepath.Join(home, ".mneme", "designqc", "p1"))
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}
	if !strings.HasPrefix(got.BaseURL, "http://") {
		t.Errorf("BaseURL: %q", got.BaseURL)
	}
}
