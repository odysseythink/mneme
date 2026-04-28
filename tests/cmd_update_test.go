package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestUpdateCLISyncs(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()

	// Set HOME before calling state functions (they use os.UserHomeDir())
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer func() {
		if oldHome != "" {
			os.Setenv("HOME", oldHome)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)
	state.WriteTemplateVersion(proj, 0)
	os.WriteFile(filepath.Join(proj, ".mneme", "mneme.md"), []byte("OLD"), 0644)

	cmd := exec.Command(bin, "update", "--yes")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("update: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "updated") {
		t.Errorf("expected 'updated' in output: %s", out)
	}
	v, _ := state.ReadTemplateVersion(proj)
	if v != state.TemplateVersion {
		t.Errorf("post-update template version = %d, want %d", v, state.TemplateVersion)
	}
}

func TestUpdateBinaryWithoutReleaseChannel(t *testing.T) {
	bin := buildMnemeBinary(t)
	cmd := exec.Command(bin, "update", "--binary")
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "no GitHub release configured") {
		t.Errorf("expected configuration error, got: %s", out)
	}
}

func TestUpdateDryRunDoesNotMutate(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()

	// Set HOME before calling state functions (they use os.UserHomeDir())
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer func() {
		if oldHome != "" {
			os.Setenv("HOME", oldHome)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	proj := filepath.Join(home, "p")
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(proj)
	state.WriteOrigin(id, proj)
	state.WriteTemplateVersion(proj, 0)
	os.WriteFile(filepath.Join(proj, ".mneme", "mneme.md"), []byte("OLD"), 0644)

	cmd := exec.Command(bin, "update", "--dry-run")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "would update") {
		t.Errorf("expected 'would update' in dry-run output, got: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(proj, ".mneme", "mneme.md"))
	if string(data) != "OLD" {
		t.Errorf("dry-run mutated mneme.md: %q", string(data))
	}
}
