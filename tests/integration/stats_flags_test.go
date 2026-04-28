package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupStatsProject creates a temp project with .mneme/ (no git needed).
func setupStatsProject(t *testing.T) (projectDir, homeDir string) {
	t.Helper()
	proj := t.TempDir()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".mneme"), 0755); err != nil {
		t.Fatalf("mkdir .mneme: %v", err)
	}
	// Write a local-id so ReadLedger works without hitting the network.
	localIDPath := filepath.Join(proj, ".mneme", ".local-id")
	if err := os.WriteFile(localIDPath, []byte("test-uuid-stats-1234"), 0644); err != nil {
		t.Fatalf("write .local-id: %v", err)
	}
	return proj, home
}

func runStats(t *testing.T, projectDir, homeDir string, args ...string) (stdout string, code int) {
	t.Helper()
	cmdArgs := append([]string{"stats"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "HOME="+homeDir)
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	if err != nil {
		t.Logf("runStats error: %v", err)
	}
	return string(out), 0
}

func TestStatsWasteFlag(t *testing.T) {
	proj, home := setupStatsProject(t)

	out, code := runStats(t, proj, home, "--waste")
	if code != 0 {
		t.Fatalf("stats --waste exited %d: %s", code, out)
	}
	if !strings.Contains(out, "Waste Patterns") {
		t.Errorf("expected 'Waste Patterns' section in output, got:\n%s", out)
	}
	if !strings.Contains(out, "repeated_reads") {
		t.Errorf("expected 'repeated_reads' pattern name, got:\n%s", out)
	}
	if !strings.Contains(out, "anatomy_miss_rate") {
		t.Errorf("expected 'anatomy_miss_rate' pattern name, got:\n%s", out)
	}
}

func TestStatsJSONFlag(t *testing.T) {
	proj, home := setupStatsProject(t)

	out, code := runStats(t, proj, home, "--json")
	if code != 0 {
		t.Fatalf("stats --json exited %d: %s", code, out)
	}
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stats --json output is not valid JSON: %v\noutput: %s", err, out)
	}
	if _, ok := report["project_id"]; !ok {
		t.Errorf("expected 'project_id' field in JSON, got: %s", out)
	}
	if _, ok := report["totals"]; !ok {
		t.Errorf("expected 'totals' field in JSON, got: %s", out)
	}
	if _, ok := report["memory_rows"]; !ok {
		t.Errorf("expected 'memory_rows' field in JSON, got: %s", out)
	}
}

func TestStatsJSONWithWasteFlag(t *testing.T) {
	proj, home := setupStatsProject(t)

	out, code := runStats(t, proj, home, "--json", "--waste")
	if code != 0 {
		t.Fatalf("stats --json --waste exited %d: %s", code, out)
	}
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stats --json --waste output is not valid JSON: %v\noutput: %s", err, out)
	}
	wasteRaw, ok := report["waste"]
	if !ok {
		t.Fatalf("expected 'waste' field in JSON when --waste is set, got: %s", out)
	}
	wasteSlice, ok := wasteRaw.([]interface{})
	if !ok {
		t.Fatalf("expected 'waste' to be an array, got: %T", wasteRaw)
	}
	if len(wasteSlice) != 5 {
		t.Errorf("expected 5 waste patterns in JSON, got %d", len(wasteSlice))
	}
}
