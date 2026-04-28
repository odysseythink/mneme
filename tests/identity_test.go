package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/installer"
)

func TestDetectProjectMetadataGo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# foo\n\nA tiny Go service.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := installer.DetectProjectMetadata(dir)
	if meta.Language != "Go" {
		t.Errorf("Language = %q, want Go", meta.Language)
	}
	if !strings.Contains(meta.Intent, "tiny Go service") {
		t.Errorf("Intent missing README excerpt: %q", meta.Intent)
	}
}

func TestDetectProjectMetadataNode(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name":"web","scripts":{"dev":"vite"},"dependencies":{"react":"^18"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)
	os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfile\n"), 0644)
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte(""), 0644)

	meta := installer.DetectProjectMetadata(dir)
	if meta.Language != "JavaScript/TypeScript" {
		t.Errorf("Language = %q", meta.Language)
	}
	if meta.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q", meta.PackageManager)
	}
	if meta.Framework != "Vite" {
		t.Errorf("Framework = %q", meta.Framework)
	}
}

func TestWriteIdentityMD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n"), 0644)
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	if err := installer.WriteIdentity(dir); err != nil {
		t.Fatalf("WriteIdentity: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".mneme", "identity.md"))
	if err != nil {
		t.Fatalf("read identity.md: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "# Project:") {
		t.Errorf("identity.md missing header: %q", s)
	}
	if !strings.Contains(s, "Go") {
		t.Errorf("identity.md missing language: %q", s)
	}
}

func TestWriteMnemeMD(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	if err := installer.WriteMnemeMD(dir); err != nil {
		t.Fatalf("WriteMnemeMD: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".mneme", "mneme.md"))
	if err != nil {
		t.Fatalf("read mneme.md: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "mneme operational guide") {
		t.Errorf("mneme.md missing header")
	}
	if !strings.Contains(s, "<!-- mneme:user-section BEGIN -->") {
		t.Errorf("mneme.md missing user-section fence")
	}
}

func TestWriteMnemeMDPreservesUserSection(t *testing.T) {
	dir := t.TempDir()
	mneme := filepath.Join(dir, ".mneme")
	os.MkdirAll(mneme, 0755)
	original := `before
<!-- mneme:user-section BEGIN -->
my custom guidance
keep this verbatim
<!-- mneme:user-section END -->
after`
	os.WriteFile(filepath.Join(mneme, "mneme.md"), []byte(original), 0644)

	if err := installer.WriteMnemeMD(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(mneme, "mneme.md"))
	s := string(data)
	if !strings.Contains(s, "my custom guidance") {
		t.Errorf("user section dropped: %q", s)
	}
	if !strings.Contains(s, "keep this verbatim") {
		t.Errorf("user section partially dropped: %q", s)
	}
}
