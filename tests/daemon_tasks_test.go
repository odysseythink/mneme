package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
)

type stubLogger struct {
	lines []string
}

func (s *stubLogger) Debug(c, m string) { s.lines = append(s.lines, "DEBUG "+c+" "+m) }
func (s *stubLogger) Info(c, m string)  { s.lines = append(s.lines, "INFO "+c+" "+m) }
func (s *stubLogger) Warn(c, m string)  { s.lines = append(s.lines, "WARN "+c+" "+m) }
func (s *stubLogger) Error(c, m string) { s.lines = append(s.lines, "ERROR "+c+" "+m) }
func (s *stubLogger) Rotate() error     { return nil }
func (s *stubLogger) Close() error      { return nil }

func TestRegistry_HasBuiltInTasks(t *testing.T) {
	for _, name := range []string{"anatomy-rescan", "consolidate-memory", "prune-backups"} {
		fn := daemon.LookupTask(name)
		if fn == nil {
			t.Errorf("LookupTask(%q) returned nil", name)
		}
	}
	if daemon.LookupTask("unknown-task") != nil {
		t.Errorf("LookupTask should return nil for unknown name")
	}
}

func TestPruneBackups_MissingDirIsNoOp(t *testing.T) {
	home := t.TempDir()
	clock := func() time.Time { return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC) }
	if err := daemon.PruneBackupsAt(home, clock, 14, 10); err != nil {
		t.Errorf("PruneBackupsAt with no backups dir: %v", err)
	}
}

func TestPruneBackups_KeepsRecentDeletesOld(t *testing.T) {
	home := t.TempDir()
	backupRoot := filepath.Join(home, ".mneme", "backups", "proj1")
	if err := os.MkdirAll(backupRoot, 0o700); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	mkSnap := func(rel string, age time.Duration) {
		dir := filepath.Join(backupRoot, rel)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		mtime := now.Add(-age)
		if err := os.Chtimes(dir, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		mkSnap("recent-"+string(rune('a'+i)), time.Duration(i)*24*time.Hour)
	}
	for i := 0; i < 6; i++ {
		mkSnap("ancient-"+string(rune('a'+i)), time.Duration(20+i)*24*time.Hour)
	}

	if err := daemon.PruneBackupsAt(home, clock, 14, 10); err != nil {
		t.Fatalf("PruneBackupsAt: %v", err)
	}

	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Errorf("after prune: %d entries, want 10", len(entries))
	}
}

func TestRunPruneBackups_TaskWrapper(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	lg := &stubLogger{}
	if err := daemon.LookupTask("prune-backups")(context.Background(), lg); err != nil {
		t.Errorf("prune-backups task returned error on empty home: %v", err)
	}
}
