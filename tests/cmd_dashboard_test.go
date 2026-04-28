package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/dashboard"
)

func TestDashboardPreflightAllOK(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	mneme := filepath.Join(tmp, ".mneme", "daemon")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, "token"), []byte("the-token\n"), 0600)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Write([]byte(`{"pid":1,"uptime_s":1,"version":"x"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	hostPort := strings.TrimPrefix(srv.URL, "http://")
	host, port := splitHP(hostPort)

	var openedURL string
	deps := dashboard.CLIDeps{
		TCPHost:       host,
		TCPPort:       port,
		TokenPath:     filepath.Join(mneme, "token"),
		Opener:        func(u string) error { openedURL = u; return nil },
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		SkipUnixCheck: true,
	}
	exit := dashboard.RunCLI(deps, nil)
	if exit != 0 {
		t.Errorf("exit: got %d, want 0", exit)
	}
	if !strings.Contains(openedURL, "?token=the-token") {
		t.Errorf("opener URL missing token: %s", openedURL)
	}
}

func TestDashboardPreflightTCPDown(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	mneme := filepath.Join(tmp, ".mneme", "daemon")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, "token"), []byte("the-token\n"), 0600)

	deps := dashboard.CLIDeps{
		TCPHost:       "127.0.0.1",
		TCPPort:       1,
		TokenPath:     filepath.Join(mneme, "token"),
		Opener:        func(u string) error { t.Errorf("opener called: %s", u); return nil },
		Stdout:        os.Stdout,
		Stderr:        new(strings.Builder),
		SkipUnixCheck: true,
	}
	exit := dashboard.RunCLI(deps, nil)
	if exit != 1 {
		t.Errorf("exit: got %d, want 1", exit)
	}
}

func splitHP(hp string) (host string, port int) {
	parts := strings.Split(hp, ":")
	if len(parts) != 2 {
		return "", 0
	}
	for _, c := range parts[1] {
		port = port*10 + int(c-'0')
	}
	return parts[0], port
}
