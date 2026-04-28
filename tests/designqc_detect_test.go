package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/designqc"
)

func TestDetectFramework_DefaultPort(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "vite.config.ts"),
		[]byte("export default { plugins: [] }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fw, err := designqc.DetectFramework(root)
	if err != nil {
		t.Fatalf("DetectFramework: %v", err)
	}
	if fw.Name != "vite" || fw.DefaultPort != 5173 {
		t.Errorf("got %+v, want vite/5173", fw)
	}
}

func TestDetectFramework_ExplicitPort(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "vite.config.js"),
		[]byte("export default {\n  server: { port: 4000 },\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fw, err := designqc.DetectFramework(root)
	if err != nil {
		t.Fatal(err)
	}
	if fw.DefaultPort != 4000 {
		t.Errorf("DefaultPort = %d, want 4000", fw.DefaultPort)
	}
}

func TestDetectFramework_NotFound(t *testing.T) {
	_, err := designqc.DetectFramework(t.TempDir())
	if err == nil {
		t.Error("expected error when no Vite config present")
	}
}

func TestDetectFramework_DynamicPortFallsBack(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "vite.config.mjs"),
		[]byte("export default { server: { port: process.env.PORT } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fw, err := designqc.DetectFramework(root)
	if err != nil {
		t.Fatal(err)
	}
	if fw.DefaultPort != 5173 {
		t.Errorf("non-literal port: got %d, want default 5173", fw.DefaultPort)
	}
}
