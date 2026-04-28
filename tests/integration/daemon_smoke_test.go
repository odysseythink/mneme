//go:build integration

package integration

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDaemonSmoke(t *testing.T) {
	// Use short temp dir to keep Unix socket path under macOS's 104-byte limit.
	tmp, err := os.MkdirTemp("", "mneme")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	t.Setenv("HOME", tmp)

	bin := filepath.Join(tmp, "mneme")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd")
	cmd.Dir = repoRoot(t)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}

	dctx, dcancel := context.WithCancel(context.Background())
	defer dcancel()
	dcmd := exec.CommandContext(dctx, bin, "daemon", "start")
	dcmd.Env = append(os.Environ(), "HOME="+tmp)
	dcmd.Stdout = os.Stdout
	dcmd.Stderr = os.Stderr
	if err := dcmd.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer dcmd.Process.Kill()

	socket := filepath.Join(tmp, ".mneme", "daemon", "socket")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("unix", socket); err == nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(socket); err != nil {
		t.Fatalf("socket never appeared: %v", err)
	}

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socket)
		},
	}}
	resp, err := client.Get("http://unix/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	if err := dcmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	if err := dcmd.Wait(); err != nil {
		t.Logf("daemon exited: %v", err)
	}

	if _, err := os.Stat(socket); err == nil {
		t.Errorf("socket not removed on shutdown")
	}
	pidPath := filepath.Join(tmp, ".mneme", "daemon", "pid")
	if data, err := os.ReadFile(pidPath); err == nil {
		t.Errorf("pid file not removed: contents=%q", strings.TrimSpace(string(data)))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}
