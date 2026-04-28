package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
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

func TestManifest_RoundTrip(t *testing.T) {
	home := t.TempDir()
	m := daemon.Manifest{
		Version: 1,
		Tasks: []daemon.ManifestTask{
			{Name: "anatomy-rescan", Schedule: "@every 6h", Enabled: true},
			{Name: "consolidate-memory", Schedule: "0 3 * * *", Enabled: true},
		},
	}
	if err := daemon.SaveManifest(home, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	got, err := daemon.LoadManifest(home)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got.Version != 1 || len(got.Tasks) != 2 || got.Tasks[0].Name != "anatomy-rescan" {
		t.Errorf("LoadManifest got %+v", got)
	}
}

func TestManifest_AutoSeedOnMissing(t *testing.T) {
	home := t.TempDir()
	m, err := daemon.LoadOrSeedManifest(home)
	if err != nil {
		t.Fatalf("LoadOrSeedManifest: %v", err)
	}
	if len(m.Tasks) != 3 {
		t.Errorf("seeded manifest has %d tasks, want 3", len(m.Tasks))
	}
	names := map[string]bool{}
	for _, task := range m.Tasks {
		names[task.Name] = true
	}
	for _, want := range []string{"anatomy-rescan", "consolidate-memory", "prune-backups"} {
		if !names[want] {
			t.Errorf("seeded manifest missing task %q", want)
		}
	}
	if _, err := os.Stat(state.DaemonManifestPath(home)); err != nil {
		t.Errorf("manifest not persisted: %v", err)
	}
}

func TestCronState_RoundTrip(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	st := daemon.CronState{
		Version: 1,
		Tasks: map[string]daemon.TaskState{
			"anatomy-rescan": {
				LastRun:             "2026-04-28T12:00:00Z",
				LastSuccess:         "2026-04-28T12:00:00Z",
				ConsecutiveFailures: 0,
			},
		},
	}
	if err := daemon.SaveCronState(home, st); err != nil {
		t.Fatalf("SaveCronState: %v", err)
	}
	got, err := daemon.LoadCronState(home)
	if err != nil {
		t.Fatalf("LoadCronState: %v", err)
	}
	js, _ := json.Marshal(got)
	if !strings.Contains(string(js), "anatomy-rescan") {
		t.Errorf("LoadCronState output missing key: %s", js)
	}
	_ = filepath.Join // keep import
}

func TestCronState_LoadMissingReturnsEmpty(t *testing.T) {
	home := t.TempDir()
	st, err := daemon.LoadCronState(home)
	if err != nil {
		t.Fatalf("LoadCronState on missing file: %v", err)
	}
	if st.Version != 1 || len(st.Tasks) != 0 {
		t.Errorf("missing state should yield empty CronState v1, got %+v", st)
	}
}
