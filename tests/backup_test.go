package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
)

func TestBackupAndListAndRestore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectID := "proj-1"

	src := t.TempDir()
	rules := filepath.Join(src, "rules.md")
	mneme := filepath.Join(src, "mneme.md")
	os.WriteFile(rules, []byte("rules v1"), 0644)
	os.WriteFile(mneme, []byte("mneme v1"), 0644)

	op := installer.BackupOp{
		ProjectID: projectID,
		Operation: "test",
		Files:     []string{rules, mneme},
		MnemeVer:  "0.1.0-m7",
	}
	bk, err := installer.WriteBackup(op)
	if err != nil {
		t.Fatalf("WriteBackup: %v", err)
	}
	if !strings.HasPrefix(bk.Path, filepath.Join(home, ".mneme", "backups", projectID)) {
		t.Errorf("backup path = %q, want under %s/.mneme/backups/%s", bk.Path, home, projectID)
	}

	os.WriteFile(rules, []byte("rules v2"), 0644)
	os.WriteFile(mneme, []byte("mneme v2"), 0644)

	list, err := installer.ListBackups(projectID)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(list))
	}

	if err := installer.RestoreBackup(bk.Timestamp, projectID); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	r, _ := os.ReadFile(rules)
	if string(r) != "rules v1" {
		t.Errorf("after restore, rules = %q, want %q", string(r), "rules v1")
	}
}

func TestRestoreBackupMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := installer.RestoreBackup("2026-04-28T12:00:00Z", "no-such"); err == nil {
		t.Errorf("expected error for missing backup")
	}
}
