package tests

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/daemonclient"
)

func TestClient_TryDial_NoSocketReturnsErrUnavailable(t *testing.T) {
	home := t.TempDir()
	_, err := daemonclient.TryDial(home)
	if !errors.Is(err, daemonclient.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestClient_HealthRoundTrip(t *testing.T) {
	// Short temp dir to keep socket path under macOS's 104-byte limit.
	home, err := os.MkdirTemp("", "mneme-cli")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	socketPath := filepath.Join(home, ".mneme", "daemon", "socket")
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		t.Fatal(err)
	}

	srv := daemon.NewServer(daemon.ServerConfig{
		SocketPath: socketPath,
		Log:        &stubLogger{},
		Mux: daemon.NewMux(daemon.RouteDeps{
			Log: &stubLogger{}, PID: 4242, Version: "test-v",
		}),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Run(ctx) }()
	defer srv.Wait()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn, dErr := net.Dial("unix", socketPath); dErr == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	c, err := daemonclient.TryDial(home)
	if err != nil {
		t.Fatalf("TryDial: %v", err)
	}
	hb, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hb.PID != 4242 || hb.Version != "test-v" {
		t.Errorf("Health = %+v", hb)
	}

	cancel()
}
