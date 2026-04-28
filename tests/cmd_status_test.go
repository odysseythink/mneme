package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildMnemeBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mneme")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd")
	cmd.Dir = mustRepoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestStatusOutsideProject(t *testing.T) {
	bin := buildMnemeBinary(t)
	dir := t.TempDir()
	cmd := exec.Command(bin, "status")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("status outside project should exit nonzero; output=%s", out)
	}
	if !strings.Contains(string(out), "no mneme project") {
		t.Errorf("expected error mentioning 'no mneme project', got: %s", out)
	}
}

func TestStatusJSONInProject(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()

	mneme := filepath.Join(proj, ".mneme")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, ".local-id"), []byte("test-id\n"), 0644)
	os.WriteFile(filepath.Join(mneme, "anatomy.md"), []byte("<!-- mneme anatomy v1 -->\n"), 0644)

	cmd := exec.Command(bin, "status", "--json")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status --json: %v\n%s", err, out)
	}

	var report map[string]any
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("status --json output not valid JSON: %v\n%s", err, out)
	}
	if report["project_id"] != "test-id" {
		t.Errorf("project_id = %v, want test-id", report["project_id"])
	}
	if _, ok := report["daemon"]; !ok {
		t.Errorf("status --json missing daemon field; got keys: %v", keys(report))
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
