package tests

import (
	"crypto/subtle"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
)

func TestRecoverMW_CatchesPanic(t *testing.T) {
	h := daemon.RecoverMW(&stubLogger{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("oops")
	}))
	req := httptest.NewRequest("GET", "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal") {
		t.Errorf("body should contain error: %q", rec.Body.String())
	}
}

func TestAuthMW_BypassesUnixSocket(t *testing.T) {
	h := daemon.AuthMW("the-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/x", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (unix bypass)", rec.Code)
	}
}

func TestAuthMW_TCPRequiresToken(t *testing.T) {
	token := "abc123"
	h := daemon.AuthMW(token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"no-header", "", 401},
		{"wrong-token", "Bearer wrong", 401},
		{"right-token", "Bearer abc123", 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/x", nil)
			req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportTCP))
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Errorf("status = %d, want %d", rec.Code, c.want)
			}
		})
	}
}

// Regression: pre-fix, ?token= on GET / authorized the HTML response but did
// not set the session cookie, so every asset request that followed (no query
// param, no bearer header, path != "/") 401'd and the page rendered blank.
func TestAuthMW_BootstrapQueryTokenSetsCookie(t *testing.T) {
	const token = "the-token"
	h := daemon.AuthMW(token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/?token="+token, nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportTCP))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mneme_token" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("mneme_token cookie not set after bootstrap query token")
	}
	if cookie.Value != token {
		t.Errorf("cookie value = %q, want %q", cookie.Value, token)
	}
	if cookie.Path != "/" {
		t.Errorf("cookie path = %q, want /", cookie.Path)
	}
}

// Bootstrap query token must NOT set a cookie when the token is wrong —
// otherwise an attacker could force-set an invalid cookie value.
func TestAuthMW_BootstrapWrongTokenNoCookie(t *testing.T) {
	h := daemon.AuthMW("the-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/?token=wrong", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportTCP))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mneme_token" {
			t.Errorf("cookie should not be set on wrong token, got %+v", c)
		}
	}
}

func TestBodyLimitMW_413OnLargeBody(t *testing.T) {
	h := daemon.BodyLimitMW(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too big", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/x", strings.NewReader(strings.Repeat("a", 2048)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

func TestSubtleConstantTimeCompare_Used(t *testing.T) {
	if subtle.ConstantTimeCompare([]byte("a"), []byte("a")) != 1 {
		t.Fatal("subtle package broken")
	}
}
