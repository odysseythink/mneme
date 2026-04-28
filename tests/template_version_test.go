package tests

import (
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestReadTemplateVersionMissing(t *testing.T) {
	dir := t.TempDir()
	v, err := state.ReadTemplateVersion(dir)
	if err != nil {
		t.Fatalf("ReadTemplateVersion missing: %v", err)
	}
	if v != 0 {
		t.Errorf("missing file should return 0, got %d", v)
	}
}

func TestWriteAndReadTemplateVersion(t *testing.T) {
	dir := t.TempDir()
	if err := state.WriteTemplateVersion(dir, 7); err != nil {
		t.Fatalf("WriteTemplateVersion: %v", err)
	}
	v, err := state.ReadTemplateVersion(dir)
	if err != nil {
		t.Fatalf("ReadTemplateVersion: %v", err)
	}
	if v != 7 {
		t.Errorf("got %d, want 7", v)
	}
}

func TestReadTemplateVersionCorrupt(t *testing.T) {
	dir := t.TempDir()
	mneme := filepath.Join(dir, ".mneme")
	if err := state.WriteTemplateVersionRaw(mneme, "not-a-number\n"); err != nil {
		t.Fatalf("WriteTemplateVersionRaw: %v", err)
	}
	v, err := state.ReadTemplateVersion(dir)
	if err == nil {
		t.Errorf("expected parse error, got value %d", v)
	}
}
