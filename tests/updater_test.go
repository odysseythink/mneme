package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/updater"
)

func setupProject(t *testing.T, home, projectName string, pinnedVersion int) string {
	t.Helper()
	root := filepath.Join(home, "p-"+projectName)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	id, _ := state.ReadOrCreateLocalID(root)
	state.WriteOrigin(id, root)
	state.WriteTemplateVersion(root, pinnedVersion)
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	os.WriteFile(filepath.Join(root, ".mneme", "mneme.md"), []byte("OLD CONTENT\n"), 0644)
	return root
}

func TestUpdateSyncsOutdatedProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rootA := setupProject(t, home, "a", 0)
	rootB := setupProject(t, home, "b", state.TemplateVersion)

	res, err := updater.SyncAll(updater.SyncOptions{})
	if err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("Updated = %d, want 1 (only rootA was outdated)", res.Updated)
	}
	if res.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", res.Skipped)
	}

	a, _ := os.ReadFile(filepath.Join(rootA, ".mneme", "mneme.md"))
	if strings.Contains(string(a), "OLD CONTENT") {
		t.Errorf("rootA mneme.md not refreshed: %q", string(a))
	}
	b, _ := os.ReadFile(filepath.Join(rootB, ".mneme", "mneme.md"))
	if !strings.Contains(string(b), "OLD CONTENT") {
		t.Errorf("rootB mneme.md should be untouched: %q", string(b))
	}

	if v, _ := state.ReadTemplateVersion(rootA); v != state.TemplateVersion {
		t.Errorf("rootA pinned version = %d, want %d", v, state.TemplateVersion)
	}
}

func TestUpdateDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := setupProject(t, home, "dry", 0)

	res, err := updater.SyncAll(updater.SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("SyncAll dry-run: %v", err)
	}
	if res.WouldUpdate != 1 {
		t.Errorf("WouldUpdate = %d, want 1", res.WouldUpdate)
	}
	a, _ := os.ReadFile(filepath.Join(root, ".mneme", "mneme.md"))
	if !strings.Contains(string(a), "OLD CONTENT") {
		t.Errorf("dry-run wrote files: %q", string(a))
	}
}

func TestUpdateProjectFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	rootA := setupProject(t, home, "a", 0)
	setupProject(t, home, "b", 0)

	idA, _ := state.ReadOrCreateLocalID(rootA)
	res, err := updater.SyncAll(updater.SyncOptions{ProjectID: idA})
	if err != nil {
		t.Fatalf("SyncAll filtered: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("Updated = %d, want 1 (only filtered project)", res.Updated)
	}
}
