package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/installer"
)

func TestScaffoldProject(t *testing.T) {
	dir := t.TempDir()

	id, err := installer.ScaffoldProject(dir)
	if err != nil {
		t.Fatalf("ScaffoldProject: %v", err)
	}
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d (val=%q)", len(id), id)
	}

	if _, err := os.Stat(filepath.Join(dir, ".claude-context")); err != nil {
		t.Error(".claude-context/ not created")
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".claude-context", ".gitignore"))
	if !strings.Contains(string(gi), "_session.json") {
		t.Errorf(".gitignore missing _session.json, got: %q", string(gi))
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude-context", ".local-id")); err != nil {
		t.Error(".local-id not created")
	}
}

func TestScaffoldProjectIdempotent(t *testing.T) {
	dir := t.TempDir()
	id1, err1 := installer.ScaffoldProject(dir)
	id2, err2 := installer.ScaffoldProject(dir)
	if err1 != nil || err2 != nil {
		t.Fatalf("ScaffoldProject errors: %v, %v", err1, err2)
	}
	if id1 != id2 {
		t.Errorf("re-scaffold changed ID: %q → %q", id1, id2)
	}
}

func TestMergeEmptySettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	err := installer.MergeHooks(path, "/usr/local/bin/claude-context")
	if err != nil {
		t.Fatalf("MergeHooks: %v", err)
	}

	data, _ := os.ReadFile(path)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	hooks, ok := result["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'hooks' key in settings.json")
	}
	preToolUse, ok := hooks["PreToolUse"].([]interface{})
	if !ok {
		t.Fatal("expected PreToolUse array")
	}
	if len(preToolUse) != 2 {
		t.Errorf("expected 2 PreToolUse matchers (Read+Write), got %d", len(preToolUse))
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("expected SessionStart key")
	}
	if _, ok := hooks["Stop"]; !ok {
		t.Error("expected Stop key")
	}
}

func TestMergeExistingUserHookPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"my-logger"}]}]}}`
	os.WriteFile(path, []byte(existing), 0644)

	installer.MergeHooks(path, "/usr/local/bin/claude-context")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "my-logger") {
		t.Error("user's existing hook was removed")
	}
	if !strings.Contains(string(data), "hook pre-read") {
		t.Error("our hook not added")
	}
}

func TestMergeUpgradeIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	installer.MergeHooks(path, "/old/claude-context")
	installer.MergeHooks(path, "/new/claude-context")

	data, _ := os.ReadFile(path)
	if strings.Count(string(data), "hook pre-read") != 1 {
		t.Errorf("expected exactly 1 pre-read entry after upgrade, count=%d, data=%s",
			strings.Count(string(data), "hook pre-read"), data)
	}
	if !strings.Contains(string(data), "/new/claude-context") {
		t.Error("expected new binary path after upgrade")
	}
}

func TestUninstallByMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	installer.MergeHooks(path, "/usr/local/bin/claude-context")
	installer.UninstallHooks(path)

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), `"_managed_by"`) {
		t.Errorf("found _managed_by after uninstall: %s", data)
	}
}

func TestUninstallEmptyHooksKeyRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	installer.MergeHooks(path, "/usr/local/bin/claude-context")
	installer.UninstallHooks(path)

	data, _ := os.ReadFile(path)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	if _, ok := result["hooks"]; ok {
		t.Errorf("hooks key should be absent after full uninstall of all managed entries, got: %s", data)
	}
}

func TestUninstallPreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"my-logger"}]}]}}`
	os.WriteFile(path, []byte(existing), 0644)

	installer.MergeHooks(path, "/usr/local/bin/claude-context")
	installer.UninstallHooks(path)

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "my-logger") {
		t.Error("user hook removed during uninstall")
	}
}

func TestRulesMDWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.md")

	if err := installer.WriteRules(path); err != nil {
		t.Fatalf("WriteRules: %v", err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Error("rules.md is empty")
	}
	if !strings.Contains(string(data), "claude-context") {
		t.Error("rules.md should mention claude-context")
	}
}

func TestInjectCLAUDEMD(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CLAUDE.md")
	rulesPath := filepath.Join(dir, "claude-context-rules.md")

	os.WriteFile(mdPath, []byte("# existing\n"), 0644)

	if err := installer.InjectCLAUDEMD(mdPath, rulesPath); err != nil {
		t.Fatalf("InjectCLAUDEMD: %v", err)
	}

	data, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(data), "claude-context-managed BEGIN") {
		t.Error("missing BEGIN marker")
	}
	if !strings.Contains(string(data), "claude-context-managed END") {
		t.Error("missing END marker")
	}
	if !strings.Contains(string(data), "@") {
		t.Error("missing @import line")
	}
	// existing content preserved
	if !strings.Contains(string(data), "# existing") {
		t.Error("existing content removed")
	}
}

func TestInjectCLAUDEMDIdempotent(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CLAUDE.md")
	rulesPath := filepath.Join(dir, "rules.md")

	installer.InjectCLAUDEMD(mdPath, rulesPath)
	installer.InjectCLAUDEMD(mdPath, rulesPath)

	data, _ := os.ReadFile(mdPath)
	if strings.Count(string(data), "claude-context-managed BEGIN") != 1 {
		t.Errorf("expected exactly 1 BEGIN marker, got: %s", data)
	}
}

func TestRemoveCLAUDEMDBlock(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CLAUDE.md")
	rulesPath := filepath.Join(dir, "rules.md")

	os.WriteFile(mdPath, []byte("# existing\n"), 0644)
	installer.InjectCLAUDEMD(mdPath, rulesPath)
	installer.RemoveCLAUDEMDBlock(mdPath)

	data, _ := os.ReadFile(mdPath)
	if strings.Contains(string(data), "claude-context-managed") {
		t.Errorf("markers still present after removal: %s", data)
	}
	if !strings.Contains(string(data), "# existing") {
		t.Error("existing content removed")
	}
}
