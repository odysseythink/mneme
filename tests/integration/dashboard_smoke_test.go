//go:build integration

package integration

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
)

// TestDashboardEndToEnd exercises the M10a auth pipeline using an
// httptest.Server that wraps the real daemon mux + AuthMW. This avoids
// spawning a daemon process while still validating cookie + query token
// path through the genuine middleware.
func TestDashboardEndToEnd(t *testing.T) {
	const token = "test-token-abc123"

	mux := daemon.NewMux(daemon.RouteDeps{
		Token:     token,
		DevMode:   false,
		PID:       1,
		Version:   "test",
		StartedAt: 0,
	})

	wrapped := daemon.RecoverMW(&silentLogger{})(daemon.AuthMW(token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(daemon.WithTransport(r.Context(), daemon.TransportTCP)))
	})))

	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: GET /?token=T (bootstrap path) — accepted by AuthMW for `/`.
	resp, err := client.Get(srv.URL + "/?token=" + token)
	if err != nil {
		t.Fatalf("bootstrap GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status: got %d, want 200", resp.StatusCode)
	}

	// Step 2: simulate browser cookie set (the JS would do this).
	parsed, _ := url.Parse(srv.URL)
	jar.SetCookies(parsed, []*http.Cookie{{
		Name:     "mneme_token",
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	}})

	// Step 3: GET /health using cookie only.
	resp, err = client.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("/health GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/health with cookie: got %d, want 200", resp.StatusCode)
	}

	// Step 4: GET /api/no-such with query token (should reject — query token
	// is bootstrap-only on `/`).
	resp, err = client.Get(srv.URL + "/api/no-such?token=" + token)
	if err != nil {
		t.Fatalf("/api/x GET: %v", err)
	}
	resp.Body.Close()
	// With cookie set, request will succeed via cookie path; that's fine.
	// Without cookie it would 401. Just sanity-check it isn't 5xx.
	if resp.StatusCode >= 500 {
		t.Errorf("unexpected 5xx: %d", resp.StatusCode)
	}

	// Strip cookie + query token to verify rejection.
	jar2, _ := cookiejar.New(nil)
	bareClient := &http.Client{Jar: jar2}
	resp2, err := bareClient.Get(srv.URL + "/api/x?token=" + token)
	if err != nil {
		t.Fatalf("bare GET: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("query token on /api/x without cookie should be 401, got %d", resp2.StatusCode)
	}

	_ = strings.Contains // keep import
}

type silentLogger struct{}

func (silentLogger) Debug(c, m string) {}
func (silentLogger) Info(c, m string)  {}
func (silentLogger) Warn(c, m string)  {}
func (silentLogger) Error(c, m string) {}
func (silentLogger) Rotate() error     { return nil }
func (silentLogger) Close() error      { return nil }
