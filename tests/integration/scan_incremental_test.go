package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupScanProject(t *testing.T) string {
	t.Helper()
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".claude-context"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, ".claude-context", ".local-id"), []byte("test-scan-incr-1234"), 0644); err != nil {
		t.Fatalf("write .local-id: %v", err)
	}
	// init a git repo so Walk() works
	exec.Command("git", "-C", proj, "init").Run()
	exec.Command("git", "-C", proj, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", proj, "config", "user.name", "Test").Run()
	return proj
}

func runScan(t *testing.T, projDir string, args ...string) (string, int) {
	t.Helper()
	cmdArgs := append([]string{"scan"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...)
	cmd.Dir = projDir
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	return string(out), 0
}

func TestScanIncrementalSucceeds(t *testing.T) {
	proj := setupScanProject(t)

	// Add a file and commit it
	os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", proj, "add", ".").Run()
	exec.Command("git", "-C", proj, "commit", "-m", "init").Run()

	// First scan (full — no anatomy.md yet)
	out, code := runScan(t, proj)
	if code != 0 {
		t.Fatalf("first scan failed (code %d): %s", code, out)
	}
	if !strings.Contains(out, "Scanned") {
		t.Errorf("expected 'Scanned' in first scan output, got: %s", out)
	}

	// Second scan (incremental — anatomy.md exists)
	out2, code2 := runScan(t, proj)
	if code2 != 0 {
		t.Fatalf("second scan failed (code %d): %s", code2, out2)
	}
	if !strings.Contains(out2, "Scanned") {
		t.Errorf("expected 'Scanned' in second scan output, got: %s", out2)
	}
}

func TestScanForceFlag(t *testing.T) {
	proj := setupScanProject(t)
	os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\n"), 0644)
	exec.Command("git", "-C", proj, "add", ".").Run()
	exec.Command("git", "-C", proj, "commit", "-m", "init").Run()

	// First full scan
	runScan(t, proj)

	// --force should complete without error
	out, code := runScan(t, proj, "--force")
	if code != 0 {
		t.Fatalf("--force scan failed (code %d): %s", code, out)
	}
	if !strings.Contains(out, "Scanned") {
		t.Errorf("expected 'Scanned' in --force output, got: %s", out)
	}
}
