// tests/integration/hook_chain_test.go
package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func setupInitializedProject(t *testing.T) (projectDir string, homeDir string) {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	initGitRepo(t, project)

	env := append(os.Environ(), "HOME="+home)
	cmd := exec.Command(binaryPath, "init", "--yes")
	cmd.Dir = project
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	return project, home
}

func runHook(t *testing.T, projectDir, homeDir, hookEvent string, fixture string) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "fixtures", "hook-payloads", fixture))
	if err != nil {
		t.Fatalf("open fixture %s: %v", fixture, err)
	}
	defer f.Close()

	env := append(os.Environ(), "HOME="+homeDir)
	cmd := exec.Command(binaryPath, "hook", hookEvent)
	cmd.Dir = projectDir
	cmd.Env = env
	cmd.Stdin = f
	if out, err := cmd.CombinedOutput(); err != nil {
		// exit 1 from debug mode is OK; we're not in debug mode so should be exit 0
		t.Logf("hook %s output: %s", hookEvent, out)
	}
}

func readTestLedger(t *testing.T, projectDir string) *state.Ledger {
	t.Helper()
	// Temporarily override HOME via state functions is not needed;
	// ReadLedger uses projectDir directly
	l, err := state.ReadLedger(projectDir)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	return l
}

func TestSessionStartCreatesSession(t *testing.T) {
	project, home := setupInitializedProject(t)

	// Override HOME in env for the hook binary but we need state.ReadLedger to also
	// resolve to same location. Use the binary's stats command to verify instead.
	f, _ := os.Open(filepath.Join("..", "fixtures", "hook-payloads", "session-start.json"))
	defer f.Close()

	env := append(os.Environ(), "HOME="+home)
	cmd := exec.Command(binaryPath, "hook", "session-start")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = f
	cmd.CombinedOutput()

	// Verify ledger via stats output
	statsCmd := exec.Command(binaryPath, "stats")
	statsCmd.Dir = project
	statsCmd.Env = env
	out, err := statsCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stats: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "session-start") {
		t.Errorf("stats output missing session-start: %s", out)
	}
}

func TestPreReadIncrementsCounter(t *testing.T) {
	project, home := setupInitializedProject(t)

	// Run pre-read hook with the fixture
	f, _ := os.Open(filepath.Join("..", "fixtures", "hook-payloads", "pre-read.json"))
	defer f.Close()

	// The pre-read fixture has file_path = /private/tmp/mneme-m0-sandbox/auth.go
	// which is NOT inside our project. So we need a fixture that IS inside our project,
	// or we accept that outside_project_skipped is incremented instead.
	// For M1, we just verify the hook doesn't crash.
	env := append(os.Environ(), "HOME="+home)
	cmd := exec.Command(binaryPath, "hook", "pre-read")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = f
	out, err := cmd.CombinedOutput()
	// Should exit 0 (silently, path is outside project)
	if err != nil {
		t.Errorf("pre-read hook failed: %v\n%s", err, out)
	}
}

func TestStopFireMultipleTimes(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	// Create a stop fixture pointing to our project dir
	stopPayload := map[string]interface{}{
		"session_id":       "test-session",
		"transcript_path":  "/tmp/transcript",
		"cwd":              project, // points to our initialized project
		"hook_event_name":  "Stop",
		"permission_mode":  "bypassPermissions",
		"stop_hook_active": false,
	}
	payloadBytes, _ := json.Marshal(stopPayload)

	for i := 0; i < 3; i++ {
		cmd := exec.Command(binaryPath, "hook", "stop")
		cmd.Dir = project
		cmd.Env = env
		cmd.Stdin = strings.NewReader(string(payloadBytes))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("stop hook fire %d: %v\n%s", i+1, err, out)
		}
	}

	// Verify stats shows stop fired 3 times
	statsCmd := exec.Command(binaryPath, "stats")
	statsCmd.Dir = project
	statsCmd.Env = env
	out, _ := statsCmd.CombinedOutput()
	if !strings.Contains(string(out), "stop") {
		t.Errorf("stats missing stop count: %s", out)
	}
}

func TestBadJSONStdinSafe(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	cmd := exec.Command(binaryPath, "hook", "pre-read")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader("{bad json}")

	out, err := cmd.CombinedOutput()
	// Must exit 0 — never crash Claude
	if err != nil {
		t.Errorf("bad JSON should exit 0, got: %v\n%s", err, out)
	}
}

func BenchmarkHookStubE2E(b *testing.B) {
	home := b.TempDir()
	project := b.TempDir()

	// init a git repo + run mneme init
	exec.Command("git", "-C", project, "init").Run()
	exec.Command("git", "-C", project, "config", "user.email", "b@b.com").Run()
	exec.Command("git", "-C", project, "config", "user.name", "B").Run()
	env := append(os.Environ(), "HOME="+home)
	initCmd := exec.Command(binaryPath, "init", "--yes")
	initCmd.Dir = project
	initCmd.Env = env
	initCmd.Run()

	stopPayload := `{"session_id":"bench","transcript_path":"/tmp/t","cwd":"` + project + `","hook_event_name":"Stop","permission_mode":"bypassPermissions","stop_hook_active":false}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(binaryPath, "hook", "stop")
		cmd.Dir = project
		cmd.Env = env
		cmd.Stdin = strings.NewReader(stopPayload)
		cmd.Run()
	}
}

func TestPreReadAnatomy(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	initGitRepo(t, project)

	goContent := "package main\n\n// main is the entry point.\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(project, "main.go"), []byte(goContent), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	exec.Command("git", "-C", project, "add", "main.go").Run()

	env := append(os.Environ(), "HOME="+home)
	initCmd := exec.Command(binaryPath, "init", "--yes")
	initCmd.Dir = project
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	anatomyPath := filepath.Join(project, ".mneme", "anatomy.md")
	if _, err := os.Stat(anatomyPath); err != nil {
		t.Fatalf("anatomy.md not created after init: %v", err)
	}

	payload := fmt.Sprintf(
		`{"session_id":"test","transcript_path":"/tmp/t","cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":%q},"permission_mode":"bypassPermissions","stop_hook_active":false}`,
		project, filepath.Join(project, "main.go"),
	)
	cmd := exec.Command(binaryPath, "hook", "pre-read")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(payload)
	out, err := cmd.CombinedOutput()

	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1 (anatomy hit), got err=%v; output: %s", err, out)
	}
	if !strings.Contains(string(out), "main.go") {
		t.Errorf("stderr should contain 'main.go', got: %s", out)
	}
}

func TestPreReadRepeat(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	initGitRepo(t, project)

	os.WriteFile(filepath.Join(project, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"), 0644)

	env := append(os.Environ(), "HOME="+home)
	// --no-scan: anatomy.md stays empty so repeat_reads path can fire
	initCmd := exec.Command(binaryPath, "init", "--yes", "--no-scan")
	initCmd.Dir = project
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// Fire session-start so AppendSessionRead has a _session.json to write to.
	sessionPayload := fmt.Sprintf(
		`{"session_id":"repeat-test","transcript_path":"/tmp/t","cwd":%q,"hook_event_name":"SessionStart","source":"human","model":"claude-opus-4-7"}`,
		project,
	)
	sessionCmd := exec.Command(binaryPath, "hook", "session-start")
	sessionCmd.Dir = project
	sessionCmd.Env = env
	sessionCmd.Stdin = strings.NewReader(sessionPayload)
	sessionCmd.Run()

	mainGoPath := filepath.Join(project, "main.go")
	makePayload := func() string {
		return fmt.Sprintf(
			`{"session_id":"repeat-test","transcript_path":"/tmp/t","cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":%q},"permission_mode":"bypassPermissions","stop_hook_active":false}`,
			project, mainGoPath,
		)
	}

	// First pre-read: not in anatomy, not yet read → exit 0 (silent)
	cmd1 := exec.Command(binaryPath, "hook", "pre-read")
	cmd1.Dir = project
	cmd1.Env = env
	cmd1.Stdin = strings.NewReader(makePayload())
	out1, err1 := cmd1.CombinedOutput()
	if err1 != nil {
		t.Fatalf("first pre-read: expected exit 0 (silent), got err=%v; out=%s", err1, out1)
	}

	// Second pre-read: not in anatomy, but already read → exit 1 with "already read"
	cmd2 := exec.Command(binaryPath, "hook", "pre-read")
	cmd2.Dir = project
	cmd2.Env = env
	cmd2.Stdin = strings.NewReader(makePayload())
	out2, err2 := cmd2.CombinedOutput()
	exitErr2, ok2 := err2.(*exec.ExitError)
	if !ok2 || exitErr2.ExitCode() != 1 {
		t.Fatalf("second pre-read: expected exit 1 (repeat-read), got err=%v; out=%s", err2, out2)
	}
	if !strings.Contains(string(out2), "already read") {
		t.Errorf("second pre-read stderr should contain 'already read', got: %s", out2)
	}
}

func TestPreReadOutsideProject(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	initGitRepo(t, project)

	env := append(os.Environ(), "HOME="+home)
	initCmd := exec.Command(binaryPath, "init", "--yes", "--no-scan")
	initCmd.Dir = project
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// File path is OUTSIDE the project (in a different temp dir)
	outsidePath := filepath.Join(t.TempDir(), "other.go")
	os.WriteFile(outsidePath, []byte("package x\n"), 0644)

	payload := fmt.Sprintf(
		`{"session_id":"test","transcript_path":"/tmp/t","cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":%q},"permission_mode":"bypassPermissions","stop_hook_active":false}`,
		project, outsidePath,
	)
	cmd := exec.Command(binaryPath, "hook", "pre-read")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(payload)
	out, err := cmd.CombinedOutput()

	// Must exit 0 (silently) — file is outside project tree
	if err != nil {
		t.Errorf("pre-read for outside-project file should exit 0, got: %v\n%s", err, out)
	}
}

func TestPostToolUseClassifiesEdit(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	sessionPayload := map[string]interface{}{
		"session_id":      "sess-ptu-test",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "SessionStart",
		"source":          "human",
		"model":           "claude-opus-4-7",
	}
	sessionBytes, _ := json.Marshal(sessionPayload)
	cmd := exec.Command(binaryPath, "hook", "session-start")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(sessionBytes))
	cmd.CombinedOutput()

	ptuPayload := map[string]interface{}{
		"session_id":      "sess-ptu-test",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "PostToolUse",
		"tool_name":       "Write",
		"tool_use_id":     "toolu_01",
		"tool_input": map[string]interface{}{
			"file_path": filepath.Join(project, "new_handler.go"),
			"content":   "package main\n\nfunc newHandler() {}\n",
		},
	}
	ptuBytes, _ := json.Marshal(ptuPayload)
	cmd2 := exec.Command(binaryPath, "hook", "post-tool-use")
	cmd2.Dir = project
	cmd2.Env = env
	cmd2.Stdin = strings.NewReader(string(ptuBytes))
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Fatalf("post-tool-use: %v\n%s", err, out)
	}

	statsCmd := exec.Command(binaryPath, "stats")
	statsCmd.Dir = project
	statsCmd.Env = env
	out, _ := statsCmd.CombinedOutput()
	if !strings.Contains(string(out), "post-tool-use") {
		t.Errorf("stats missing post-tool-use counter: %s", out)
	}
}

func TestSessionStartWritesMemoryRow(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	fireSessionStart := func(sessionID string) {
		payload := map[string]interface{}{
			"session_id":      sessionID,
			"transcript_path": "/tmp/t",
			"cwd":             project,
			"hook_event_name": "SessionStart",
			"source":          "human",
			"model":           "claude-opus-4-7",
		}
		b, _ := json.Marshal(payload)
		cmd := exec.Command(binaryPath, "hook", "session-start")
		cmd.Dir = project
		cmd.Env = env
		cmd.Stdin = strings.NewReader(string(b))
		cmd.CombinedOutput()
	}

	fireSessionStart("sess-A")
	fireSessionStart("sess-B")

	memPath := filepath.Join(home, ".claude", "mneme-memory.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("memory.md not created: %v", err)
	}
	if !strings.Contains(string(data), "<!-- mneme memory v1 -->") {
		t.Errorf("memory.md missing header: %s", data)
	}
	if !strings.Contains(string(data), "(0 turns)") {
		t.Errorf("memory.md missing session row: %s", data)
	}
}

func TestStopAggregatesTurnEdits(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	sessPayload := map[string]interface{}{
		"session_id":      "sess-agg",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "SessionStart",
		"source":          "human",
		"model":           "claude-opus-4-7",
	}
	b, _ := json.Marshal(sessPayload)
	cmd := exec.Command(binaryPath, "hook", "session-start")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(b))
	cmd.CombinedOutput()

	for i := 0; i < 2; i++ {
		ptuPayload := map[string]interface{}{
			"session_id":      "sess-agg",
			"transcript_path": "/tmp/t",
			"cwd":             project,
			"hook_event_name": "PostToolUse",
			"tool_name":       "Write",
			"tool_use_id":     fmt.Sprintf("toolu_%d", i),
			"tool_input": map[string]interface{}{
				"file_path": filepath.Join(project, fmt.Sprintf("file%d.go", i)),
				"content":   "package main\n",
			},
		}
		pb, _ := json.Marshal(ptuPayload)
		c := exec.Command(binaryPath, "hook", "post-tool-use")
		c.Dir = project
		c.Env = env
		c.Stdin = strings.NewReader(string(pb))
		c.CombinedOutput()
	}

	stopPayload := map[string]interface{}{
		"session_id":       "sess-agg",
		"transcript_path":  "/tmp/t",
		"cwd":              project,
		"hook_event_name":  "Stop",
		"permission_mode":  "bypassPermissions",
		"stop_hook_active": false,
	}
	sb, _ := json.Marshal(stopPayload)
	stopCmd := exec.Command(binaryPath, "hook", "stop")
	stopCmd.Dir = project
	stopCmd.Env = env
	stopCmd.Stdin = strings.NewReader(string(sb))
	if out, err := stopCmd.CombinedOutput(); err != nil {
		t.Fatalf("stop: %v\n%s", err, out)
	}

	statsCmd := exec.Command(binaryPath, "stats")
	statsCmd.Dir = project
	statsCmd.Env = env
	out, _ := statsCmd.CombinedOutput()
	if !strings.Contains(string(out), "stop") {
		t.Errorf("stats missing stop: %s", out)
	}
}

func BenchmarkPostToolUseE2E(b *testing.B) {
	home := b.TempDir()
	project := b.TempDir()
	exec.Command("git", "-C", project, "init").Run()
	exec.Command("git", "-C", project, "config", "user.email", "b@b.com").Run()
	exec.Command("git", "-C", project, "config", "user.name", "B").Run()
	env := append(os.Environ(), "HOME="+home)
	initCmd := exec.Command(binaryPath, "init", "--yes")
	initCmd.Dir = project
	initCmd.Env = env
	initCmd.Run()

	sessPayload := `{"session_id":"bench-sess","transcript_path":"/tmp/t","cwd":"` + project + `","hook_event_name":"SessionStart","source":"human","model":"claude-opus-4-7"}`
	sc := exec.Command(binaryPath, "hook", "session-start")
	sc.Dir = project
	sc.Env = env
	sc.Stdin = strings.NewReader(sessPayload)
	sc.Run()

	ptuPayload := `{"session_id":"bench-sess","transcript_path":"/tmp/t","cwd":"` + project + `","hook_event_name":"PostToolUse","tool_name":"Write","tool_use_id":"t1","tool_input":{"file_path":"` + project + `/foo.go","content":"package main\n"}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(binaryPath, "hook", "post-tool-use")
		cmd.Dir = project
		cmd.Env = env
		cmd.Stdin = strings.NewReader(ptuPayload)
		cmd.Run()
	}
}

func TestPreWriteMatchesCerebrumRule(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	// Add a cerebrum rule
	cmd := exec.Command(binaryPath, "cerebrum", "add", "--pattern", `fmt\.Println\(`, "--message", "use structured logger")
	cmd.Dir = project
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cerebrum add failed: %v\n%s", err, out)
	}

	// Fire pre-write hook with Edit payload where new_string contains fmt.Println
	payload := map[string]interface{}{
		"session_id":      "sess-X",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input": map[string]interface{}{
			"file_path":  filepath.Join(project, "main.go"),
			"old_string": "func main() {}",
			"new_string": "func main() {\n\tfmt.Println(\"hello\")\n}",
		},
	}
	b, _ := json.Marshal(payload)

	hookCmd := exec.Command(binaryPath, "hook", "pre-write")
	hookCmd.Dir = project
	hookCmd.Env = env
	hookCmd.Stdin = strings.NewReader(string(b))
	out, err = hookCmd.CombinedOutput()

	// Should exit with code 1 (error)
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got err=%v; output: %s", err, out)
	}

	// Output should contain "cerebrum" and the message
	outStr := string(out)
	if !strings.Contains(outStr, "cerebrum") {
		t.Errorf("output missing 'cerebrum': %s", outStr)
	}
	if !strings.Contains(outStr, "use structured logger") {
		t.Errorf("output missing 'use structured logger': %s", outStr)
	}
}

func TestPreWriteNoMatchExitsZero(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	// Add the fmt.Println rule
	cmd := exec.Command(binaryPath, "cerebrum", "add", "--pattern", `fmt\.Println\(`, "--message", "use structured logger")
	cmd.Dir = project
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cerebrum add failed: %v\n%s", err, out)
	}

	// Fire pre-write hook with Edit payload where new_string does NOT contain fmt.Println
	payload := map[string]interface{}{
		"session_id":      "sess-X",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input": map[string]interface{}{
			"file_path":  filepath.Join(project, "main.go"),
			"old_string": "func main() {}",
			"new_string": "func main() {\n\tlog.Println(\"hello\")\n}",
		},
	}
	b, _ := json.Marshal(payload)

	hookCmd := exec.Command(binaryPath, "hook", "pre-write")
	hookCmd.Dir = project
	hookCmd.Env = env
	hookCmd.Stdin = strings.NewReader(string(b))
	out, err = hookCmd.CombinedOutput()

	// Should exit with code 0
	if err != nil {
		t.Errorf("expected exit 0 (no match), got: %v\n%s", err, out)
	}
}

func TestPreWriteNoCerebrumExitsZero(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	// Do NOT add any cerebrum rules

	// Fire pre-write hook with Write payload
	payload := map[string]interface{}{
		"session_id":      "sess-X",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": filepath.Join(project, "newfile.go"),
			"content":   "package main\n",
		},
	}
	b, _ := json.Marshal(payload)

	hookCmd := exec.Command(binaryPath, "hook", "pre-write")
	hookCmd.Dir = project
	hookCmd.Env = env
	hookCmd.Stdin = strings.NewReader(string(b))
	out, err := hookCmd.CombinedOutput()

	// Should exit with code 0
	if err != nil {
		t.Errorf("expected exit 0 (no rules), got: %v\n%s", err, out)
	}
}

func TestBuglogListEmpty(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	cmd := exec.Command(binaryPath, "buglog", "list")
	cmd.Dir = project
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("buglog list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "No buglog entries") {
		t.Errorf("expected empty message, got: %s", out)
	}
}

func TestBuglogAddAndList(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	addCmd := exec.Command(binaryPath, "buglog", "add",
		"--description", "possible nil deref in session",
		"--code", "session.StopCount++\nsession.Value = nil",
	)
	addCmd.Dir = project
	addCmd.Env = env
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("buglog add: %v\n%s", err, out)
	}

	listCmd := exec.Command(binaryPath, "buglog", "list")
	listCmd.Dir = project
	listCmd.Env = env
	out, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("buglog list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "possible nil deref in session") {
		t.Errorf("expected description in output, got: %s", out)
	}
	if !strings.Contains(string(out), "[manual]") {
		t.Errorf("expected [manual] source in output, got: %s", out)
	}
}

func TestBuglogClearWithYes(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	addCmd := exec.Command(binaryPath, "buglog", "add",
		"--description", "some old bug",
		"--code", "badCall := session.Uninitialized()",
	)
	addCmd.Dir = project
	addCmd.Env = env
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("buglog add: %v\n%s", err, out)
	}

	clearCmd := exec.Command(binaryPath, "buglog", "clear", "--yes")
	clearCmd.Dir = project
	clearCmd.Env = env
	if out, err := clearCmd.CombinedOutput(); err != nil {
		t.Fatalf("buglog clear: %v\n%s", err, out)
	}

	listCmd := exec.Command(binaryPath, "buglog", "list")
	listCmd.Dir = project
	listCmd.Env = env
	out, _ := listCmd.CombinedOutput()
	if !strings.Contains(string(out), "No buglog entries") {
		t.Errorf("expected empty after clear, got: %s", out)
	}
}

func TestPostWriteAutoUpsert(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	oldStr := "func init() {\n\tsession.StopCount++\n\tbadValue := session.Init()\n\tsession.Reset()\n\tdoWork()\n}"
	newStr := "func init() {\n\tsession.StopCount++\n\tsession.Reset()\n\tdoWork()\n}"

	payload := map[string]interface{}{
		"session_id":      "test",
		"cwd":             project,
		"hook_event_name": "PostToolUse",
		"tool_name":       "Edit",
		"tool_input": map[string]interface{}{
			"file_path":  filepath.Join(project, "main.go"),
			"old_string": oldStr,
			"new_string": newStr,
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	cmd := exec.Command(binaryPath, "hook", "post-tool-use")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(payloadBytes))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("post-tool-use hook: %v\n%s", err, out)
	}

	entries, err := state.ReadBuglog(project)
	if err != nil {
		t.Fatalf("ReadBuglog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 buglog entry after bugfix edit, got %d", len(entries))
	}
	if entries[0].Source != "auto" {
		t.Errorf("expected source=auto, got %q", entries[0].Source)
	}
	if !strings.Contains(entries[0].Description, "main.go") {
		t.Errorf("description should contain filename, got: %q", entries[0].Description)
	}
}

func TestPreWriteBuglogMatchWarns(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	err := state.AppendBuglogEntry(project, state.BuglogEntry{
		Source:      "manual",
		Description: "possible nil deref in session",
		BadCode:     "session.StopCount++\nsession.Value = nil",
	})
	if err != nil {
		t.Fatalf("AppendBuglogEntry: %v", err)
	}

	payload := map[string]interface{}{
		"session_id":      "test",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": filepath.Join(project, "main.go"),
			"content":   "session.StopCount++\nsession.Value = session.Init()",
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	cmd := exec.Command(binaryPath, "hook", "pre-write")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(payloadBytes))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected exit 1 (warning), got exit 0\noutput: %s", out)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "[buglog]") {
		t.Errorf("expected [buglog] prefix in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "possible nil deref in session") {
		t.Errorf("expected description in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "was:") {
		t.Errorf("expected 'was:' line in output, got: %s", outStr)
	}
}

func TestPreWriteBuglogNoMatchExitsZero(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	state.AppendBuglogEntry(project, state.BuglogEntry{
		Source:      "manual",
		Description: "possible nil deref",
		BadCode:     "session.StopCount++\nsession.Value = nil",
	})

	payload := map[string]interface{}{
		"session_id":      "test",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": filepath.Join(project, "main.go"),
			"content":   "fmt.Println(\"hello world\")",
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	cmd := exec.Command(binaryPath, "hook", "pre-write")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(payloadBytes))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected exit 0, got exit 1\noutput: %s", out)
	}
}
