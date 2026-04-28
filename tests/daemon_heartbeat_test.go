package tests

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/state"
)

func TestHeartbeat_WritesOnStart(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC) }

	if err := daemon.WriteHeartbeat(home, 12345, "0.2.0-m8", clock); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	data, err := os.ReadFile(state.DaemonHeartbeatPath(home))
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	var hb struct {
		TS      string `json:"ts"`
		PID     int    `json:"pid"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &hb); err != nil {
		t.Fatalf("parse heartbeat: %v", err)
	}
	if hb.TS != "2026-04-28T12:00:00Z" {
		t.Errorf("TS = %q, want 2026-04-28T12:00:00Z", hb.TS)
	}
	if hb.PID != 12345 {
		t.Errorf("PID = %d, want 12345", hb.PID)
	}
	if hb.Version != "0.2.0-m8" {
		t.Errorf("Version = %q, want 0.2.0-m8", hb.Version)
	}
}

func TestHeartbeat_RunTicks(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	tickCh := make(chan time.Time, 4)
	tickCh <- time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	tickCh <- time.Date(2026, 4, 28, 12, 30, 0, 0, time.UTC)
	close(tickCh)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		daemon.RunHeartbeatWithTicks(ctx, home, 1, "0.2.0-m8", tickCh)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunHeartbeatWithTicks did not exit after channel close")
	}
	cancel()

	data, err := os.ReadFile(state.DaemonHeartbeatPath(home))
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	if !strings.Contains(string(data), "12:30:00") {
		t.Errorf("heartbeat did not record latest tick: %q", string(data))
	}
}
