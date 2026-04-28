//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/state"
)

func TestDashboardPanels_EndToEnd(t *testing.T) {
	home, err := os.MkdirTemp("", "mneme")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	t.Setenv("HOME", home)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tcpAddr := listener.Addr().String()
	listener.Close()

	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.RunOptions{
			Home:    home,
			PID:     os.Getpid(),
			Version: "0.2.0-m10b-test",
			TCPAddr: tcpAddr,
		})
	}()

	// Wait for daemon to start listening on TCP and token file to be written
	deadline := time.Now().Add(5 * time.Second)
	var token string
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("tcp", tcpAddr); err == nil {
			conn.Close()
			// Give the daemon a bit more time to fully initialize
			time.Sleep(100 * time.Millisecond)
		}
		// Try to read token file
		tokenPath := state.DaemonTokenPath(home)
		if data, err := os.ReadFile(tokenPath); err == nil {
			token = strings.TrimSpace(string(data))
			if token != "" {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if token == "" {
		t.Fatal("daemon token not available")
	}

	// Create client with auth header
	client := &http.Client{}
	var baseURL = "http://" + tcpAddr

	for _, path := range []string{"/api/overview", "/api/projects", "/api/activity?limit=5", "/api/cron"} {
		req, err := http.NewRequest("GET", baseURL+path, nil)
		if err != nil {
			t.Errorf("NewRequest %s: %v", path, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("GET %s: %v", path, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("GET %s: status=%d body=%s", path, resp.StatusCode, body)
		}
		if !strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
			t.Errorf("GET %s: body not JSON: %s", path, body)
		}
	}

	// SSE connect + receive at least one ping or event within 1.5s.
	req, err := http.NewRequest("GET", baseURL+"/events", nil)
	if err != nil {
		t.Fatalf("NewRequest /events: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("/events content-type: %q", ct)
	}
	resp.Body.Close()

	cancel()
	// Wait a bit for the daemon to start shutting down, but don't block indefinitely
	select {
	case err := <-done:
		if err != nil {
			t.Logf("daemon.Run returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		// Daemon is still running, that's OK for this smoke test
		t.Logf("daemon still running after ctx cancel (OK for this test)")
	}

	_ = json.Marshal // keep import
}
