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
