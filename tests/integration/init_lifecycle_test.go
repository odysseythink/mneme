// tests/integration/init_lifecycle_test.go
package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "cc-integration")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	bin := filepath.Join(tmp, "claude-context")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", bin,
		"github.com/ranwei/claude-context/cmd").CombinedOutput()
	if err != nil {
		panic("build failed: " + string(out))
	}
	binaryPath = bin
	os.Exit(m.Run())
}

func TestInitFreshThenUninstall(t *testing.T) {
	// Set up a temp HOME and a temp git project
	home := t.TempDir()
	project := t.TempDir()
	initGitRepo(t, project)

	// Run init --yes
	env := append(os.Environ(),
		"HOME="+home,
		"EMBEDDING_API_KEY=test-key",
	)
	cmd := exec.Command(binaryPath, "init", "--yes")
	cmd.Dir = project
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init --yes failed: %v\n%s", err, out)
	}

	// Verify settings.json was created with 5 hooks
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	hooks, ok := result["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("no hooks in settings.json")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse not in hooks")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("SessionStart not in hooks")
	}
	if _, ok := hooks["Stop"]; !ok {
		t.Error("Stop not in hooks")
	}

	// Verify rules.md was created
	rulesPath := filepath.Join(home, ".claude", "claude-context-rules.md")
	if _, err := os.Stat(rulesPath); err != nil {
		t.Error("rules.md not created")
	}

	// Verify CLAUDE.md has @import
	claudeMD, _ := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(string(claudeMD), "claude-context-managed BEGIN") {
		t.Error("CLAUDE.md missing boundary marker")
	}

	// Verify .claude-context/ and .local-id
	if _, err := os.Stat(filepath.Join(project, ".claude-context", ".local-id")); err != nil {
		t.Error(".local-id not created")
	}

	// Run uninstall --yes
	cmd2 := exec.Command(binaryPath, "init", "--uninstall", "--yes")
	cmd2.Dir = project
	cmd2.Env = env
	out2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		t.Fatalf("uninstall failed: %v\n%s", err2, out2)
	}

	// Verify settings.json no longer has managed hooks
	afterData, _ := os.ReadFile(settingsPath)
	if strings.Contains(string(afterData), `"_managed_by"`) {
		t.Errorf("_managed_by still present after uninstall: %s", afterData)
	}

	// Verify CLAUDE.md no longer has boundary markers
	afterMD, _ := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if strings.Contains(string(afterMD), "claude-context-managed") {
		t.Errorf("CLAUDE.md still has markers after uninstall: %s", afterMD)
	}

	// Verify rules.md deleted
	if _, err := os.Stat(rulesPath); !os.IsNotExist(err) {
		t.Error("rules.md should be deleted after uninstall")
	}
}

func TestInitNonTTYRefusal(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	initGitRepo(t, project)

	env := append(os.Environ(), "HOME="+home)
	// No --yes, no --dry-run → should refuse on non-TTY stdin (which pipe is)
	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader("") // piped → not a TTY

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, got 0\noutput: %s", out)
	}
	if !strings.Contains(string(out), "not a TTY") {
		t.Errorf("expected TTY error message, got: %s", out)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
}
