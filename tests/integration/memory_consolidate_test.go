package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/claude-context/pkg/state"
)

func setupMemoryProject(t *testing.T) (projDir, homeDir string) {
	t.Helper()
	proj := t.TempDir()
	home := t.TempDir()
	os.MkdirAll(filepath.Join(proj, ".claude-context"), 0755)
	os.WriteFile(filepath.Join(proj, ".claude-context", ".local-id"), []byte("test-mem-uuid-9876"), 0644)
	return proj, home
}

func runMemory(t *testing.T, projDir, homeDir string, args ...string) (string, int) {
	t.Helper()
	cmdArgs := append([]string{"memory"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...)
	cmd.Dir = projDir
	cmd.Env = append(os.Environ(), "HOME="+homeDir)
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	return string(out), 0
}

func TestMemoryConsolidateNoOp(t *testing.T) {
	proj, home := setupMemoryProject(t)
	out, code := runMemory(t, proj, home, "consolidate")
	if code != 0 {
		t.Fatalf("memory consolidate exit %d: %s", code, out)
	}
	if !strings.Contains(out, "0") {
		t.Errorf("expected '0' in output, got: %s", out)
	}
}

func TestMemoryConsolidateOldRows(t *testing.T) {
	proj, home := setupMemoryProject(t)

	old := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	for i := 0; i < 3; i++ {
		state.AppendMemoryRow(home, state.MemoryRow{
			StartedAt:     old,
			TurnCount:     4,
			PatternCounts: map[string]int{},
		})
	}

	out, code := runMemory(t, proj, home, "consolidate")
	if code != 0 {
		t.Fatalf("memory consolidate exit %d: %s", code, out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("expected '3' consolidated in output, got: %s", out)
	}
}
