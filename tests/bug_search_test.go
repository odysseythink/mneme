package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func seedBuglog(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	entries := []state.BuglogEntry{
		{Source: "manual", File: "auth.go", Description: "use crypto/subtle for token compare", BadCode: "if got == want { return true }"},
		{Source: "auto", File: "db.go", Description: "missing context cancellation", BadCode: "rows, _ := db.Query(\"SELECT *\")"},
		{Source: "manual", File: "ui.tsx", Description: "useState in render path causes re-render loop", BadCode: "const [x] = useState(compute())"},
	}
	for _, e := range entries {
		state.AppendBuglogEntry(dir, e)
	}
}

func TestBuglogSearchTokenOverlap(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	os.WriteFile(filepath.Join(proj, ".mneme/.local-id"), []byte("id\n"), 0644)
	seedBuglog(t, proj)

	cmd := exec.Command(bin, "buglog", "search", "context cancellation")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "missing context cancellation") {
		t.Errorf("expected match for 'missing context cancellation', got:\n%s", s)
	}
	if strings.Contains(s, "useState in render path") {
		t.Errorf("unexpected unrelated match in output:\n%s", s)
	}
}

func TestBuglogSearchEmptyResult(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := t.TempDir()
	os.MkdirAll(filepath.Join(proj, ".mneme"), 0755)
	os.WriteFile(filepath.Join(proj, ".mneme/.local-id"), []byte("id\n"), 0644)

	cmd := exec.Command(bin, "buglog", "search", "nothing matches")
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "no matches") {
		t.Errorf("expected 'no matches' message, got: %s", out)
	}
}
