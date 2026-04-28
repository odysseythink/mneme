package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"syscall"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestServer_UnixSocketRoundTrip(t *testing.T) {
	home := t.TempDir()
	socketPath := filepath.Join(home, "test.sock")

	srv := daemon.NewServer(daemon.ServerConfig{
		SocketPath: socketPath,
		TCPAddr:    "",
		Token:      "",
		Log:        &stubLogger{},
		Mux: daemon.NewMux(daemon.RouteDeps{
			Log:     &stubLogger{},
			PID:     999,
			Version: "test",
		}),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := net.Dial("unix", socketPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}}
	resp, err := client.Get("http://unix/health")
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, body=%s", resp.StatusCode, body)
	}

	cancel()
	srv.Wait()
}

// keep imports referenced
var _ = httptest.NewRequest
var _ = bytes.NewReader
var _ = strings.Contains
var _ = json.Marshal

func TestDaemonRun_StartsAndStops(t *testing.T) {
	// Use a short temp dir because Unix socket paths are limited to
	// ~104 bytes on macOS and t.TempDir()'s default is long enough to overflow.
	home, err := os.MkdirTemp("", "mneme")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	t.Setenv("HOME", home)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.RunOptions{
			Home:    home,
			PID:     syscall.Getpid(),
			Version: "0.2.0-m8-test",
		})
	}()

	socketPath := filepath.Join(home, ".mneme", "daemon", "socket")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, dErr := net.Dial("unix", socketPath); dErr == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("socket never created: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}

	if _, err := os.Stat(socketPath); err == nil {
		t.Errorf("socket not removed on shutdown")
	}
}
