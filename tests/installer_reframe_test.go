package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
)

func TestInstallReframe_CreatesFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mneme"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := installer.InstallReframe(root); err != nil {
		t.Fatalf("InstallReframe: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mneme", "reframe.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, name := range []string{
		"Next.js", "Vite", "Astro", "SvelteKit", "Remix", "Nuxt",
		"Solid Start", "Qwik", "Fresh", "Gatsby", "Ember", "Angular",
	} {
		if !strings.Contains(content, name) {
			t.Errorf("missing framework %q in installed reframe.md", name)
		}
	}
}

func TestInstallReframe_OverwritesExisting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mneme")
	os.MkdirAll(dir, 0o700)
	old := []byte("ancient content")
	os.WriteFile(filepath.Join(dir, "reframe.md"), old, 0o600)

	if err := installer.InstallReframe(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "reframe.md"))
	if string(data) == "ancient content" {
		t.Error("InstallReframe did not overwrite stale file")
	}
}
