package tests

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/dashboard"
)

func TestFSContainsIndex(t *testing.T) {
	f := dashboard.FS()
	data, err := fs.ReadFile(f, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(data), "<title>mneme dashboard</title>") {
		t.Errorf("placeholder marker missing in index.html")
	}
}

func TestFSDevDirOverride(t *testing.T) {
	dir := t.TempDir()
	custom := []byte("<html>dev override</html>")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), custom, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MNEME_DASHBOARD_DEV_DIR", dir)

	data, err := fs.ReadFile(dashboard.FS(), "index.html")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != string(custom) {
		t.Errorf("dev override not applied; got %q", data)
	}
}

func TestMountServesIndexAtRoot(t *testing.T) {
	mux := http.NewServeMux()
	dashboard.Mount(mux, dashboard.Deps{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

// Regression: pre-fix the /assets/ route used http.StripPrefix, which dropped
// the "assets/" segment before lookup — but the embedded FS keeps files at
// assets/<name>, so every JS/CSS bundle 404'd and the dashboard rendered blank.
func TestMountServesEmbeddedAsset(t *testing.T) {
	var assetPath string
	_ = fs.WalkDir(dashboard.FS(), "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || assetPath != "" {
			return err
		}
		assetPath = path
		return fs.SkipAll
	})
	if assetPath == "" {
		t.Skip("no built assets present (run `make web-build` to populate dist/assets/)")
	}

	mux := http.NewServeMux()
	dashboard.Mount(mux, dashboard.Deps{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/" + assetPath)
	if err != nil {
		t.Fatalf("get /%s: %v", assetPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("asset %s: got %d, want 200", assetPath, resp.StatusCode)
	}
}

func TestDevTokenSetsCookieWhenEnabled(t *testing.T) {
	h := dashboard.DevTokenHandler("the-token", true)
	body := strings.NewReader(`{"token":"the-token"}`)
	req := httptest.NewRequest("POST", "/dev-token", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var found *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mneme_token" {
			found = c
			break
		}
	}
	if found == nil || found.Value != "the-token" {
		t.Errorf("cookie not set correctly: %+v", found)
	}
}

func TestDevTokenDisabledReturns404(t *testing.T) {
	h := dashboard.DevTokenHandler("the-token", false)
	body := strings.NewReader(`{"token":"the-token"}`)
	req := httptest.NewRequest("POST", "/dev-token", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("disabled handler should 404; got %d", rec.Code)
	}
}

func TestDevTokenCORSPreflight(t *testing.T) {
	h := dashboard.DevTokenHandler("the-token", true)
	req := httptest.NewRequest("OPTIONS", "/dev-token", bytes.NewReader(nil))
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status: got %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("missing CORS credentials header")
	}
}
