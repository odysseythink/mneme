package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
	"github.com/ranwei/mneme/pkg/state"
)

func TestRestoreList(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)

	t.Setenv("HOME", home)

	mneme := filepath.Join(proj, ".mneme", "mneme.md")
	os.WriteFile(mneme, []byte("v1"), 0644)
	installer.WriteBackup(installer.BackupOp{ProjectID: id, Operation: "test", Files: []string{mneme}, MnemeVer: "0.1"})

	cmd := exec.Command(bin, "restore", "--list")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restore --list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), id) && !strings.Contains(string(out), "T") {
		t.Errorf("restore --list missing project/timestamp; got: %s", out)
	}
}

func TestRestoreLatest(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)
	t.Setenv("HOME", home)

	mneme := filepath.Join(proj, ".mneme", "mneme.md")
	os.WriteFile(mneme, []byte("v1"), 0644)
	installer.WriteBackup(installer.BackupOp{ProjectID: id, Operation: "test", Files: []string{mneme}, MnemeVer: "0.1"})

	os.WriteFile(mneme, []byte("v2"), 0644)

	cmd := exec.Command(bin, "restore", "--latest", "--yes")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restore --latest: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(mneme)
	if string(got) != "v1" {
		t.Errorf("post-restore mneme.md = %q, want %q", string(got), "v1")
	}
}
