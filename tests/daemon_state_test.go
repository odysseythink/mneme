package tests

import (
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestDaemonPaths(t *testing.T) {
	home := "/tmp/fake-home"
	got := map[string]string{
		"DaemonDir":           state.DaemonDir(home),
		"DaemonPidPath":       state.DaemonPidPath(home),
		"DaemonSocketPath":    state.DaemonSocketPath(home),
		"DaemonTokenPath":     state.DaemonTokenPath(home),
		"DaemonManifestPath":  state.DaemonManifestPath(home),
		"DaemonStatePath":     state.DaemonStatePath(home),
		"DaemonHeartbeatPath": state.DaemonHeartbeatPath(home),
		"DaemonLogsDir":       state.DaemonLogsDir(home),
	}
	want := map[string]string{
		"DaemonDir":           filepath.Join(home, ".mneme", "daemon"),
		"DaemonPidPath":       filepath.Join(home, ".mneme", "daemon", "pid"),
		"DaemonSocketPath":    filepath.Join(home, ".mneme", "daemon", "socket"),
		"DaemonTokenPath":     filepath.Join(home, ".mneme", "daemon", "token"),
		"DaemonManifestPath":  filepath.Join(home, ".mneme", "daemon", "cron-manifest.json"),
		"DaemonStatePath":     filepath.Join(home, ".mneme", "daemon", "cron-state.json"),
		"DaemonHeartbeatPath": filepath.Join(home, ".mneme", "daemon", "heartbeat.json"),
		"DaemonLogsDir":       filepath.Join(home, ".mneme", "daemon", "logs"),
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
}

func TestDaemonLogPath(t *testing.T) {
	home := "/tmp/fake-home"
	got := state.DaemonLogPath(home, "20260428")
	want := filepath.Join(home, ".mneme", "daemon", "logs", "daemon-20260428.log")
	if got != want {
		t.Errorf("DaemonLogPath = %q, want %q", got, want)
	}
}
