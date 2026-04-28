package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/envcheck"
)

func TestDetectPackageManagerPnpm(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.PackageManager != "pnpm" {
		t.Errorf("PackageManager: got %q, want pnpm", r.PackageManager)
	}
}

func TestDetectFrameworkNext(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "next.config.js"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.Framework != "next" {
		t.Errorf("Framework: got %q, want next", r.Framework)
	}
	if r.DevServerPort != 3000 {
		t.Errorf("DevServerPort: got %d, want 3000", r.DevServerPort)
	}
}

func TestDetectFrameworkVite(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "vite.config.ts"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.Framework != "vite" {
		t.Errorf("Framework: got %q, want vite", r.Framework)
	}
	if r.DevServerPort != 5173 {
		t.Errorf("DevServerPort: got %d, want 5173", r.DevServerPort)
	}
}

func TestDetectFrameworkAstro(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "astro.config.mjs"), []byte(""), 0644)
	r := envcheck.Detect(root)
	if r.Framework != "astro" {
		t.Errorf("Framework: got %q, want astro", r.Framework)
	}
}

func TestDetectChromePathFromEnv(t *testing.T) {
	root := t.TempDir()
	chrome := filepath.Join(root, "chrome-stub")
	os.WriteFile(chrome, []byte(""), 0755)
	t.Setenv("CHROME_PATH", chrome)
	r := envcheck.Detect(root)
	if r.ChromePath != chrome {
		t.Errorf("ChromePath: got %q, want %q", r.ChromePath, chrome)
	}
}
