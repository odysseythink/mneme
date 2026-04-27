package tests

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/mcp"
	"github.com/ranwei/claude-context/pkg/state"
)

func newTestServer() *mcp.Server {
	return mcp.NewMCPServer(nil, nil, nil)
}

func toolsCall(t *testing.T, s *mcp.Server, name string, argsMap map[string]interface{}) mcp.ToolCallResult {
	t.Helper()
	args, _ := json.Marshal(argsMap)
	params, _ := json.Marshal(map[string]interface{}{
		"name":      name,
		"arguments": json.RawMessage(args),
	})
	req := mcp.MCPRequest{
		Method: "tools/call",
		Params: params,
	}
	resp := s.HandleRequest(req)
	data, _ := json.Marshal(resp.Result)
	var result mcp.ToolCallResult
	json.Unmarshal(data, &result)
	return result
}

func TestServerEmbeddingToolsWithoutKey(t *testing.T) {
	s := newTestServer()

	for _, toolName := range []string{"index_codebase", "search_codebase", "ping_embedding"} {
		result := toolsCall(t, s, toolName, map[string]interface{}{"codebase_path": "/tmp"})
		if !result.IsError {
			t.Errorf("%s: expected IsError=true when embedding nil, got false", toolName)
		}
		if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "EMBEDDING_API_KEY") {
			t.Errorf("%s: expected EMBEDDING_API_KEY mention in error, got: %v", toolName, result.Content)
		}
	}
}

func setupMCPProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/.claude-context", 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return dir
}

func TestDescribeCodebaseNoCwd(t *testing.T) {
	s := newTestServer()
	result := toolsCall(t, s, "describe_codebase", map[string]interface{}{})
	if !result.IsError {
		t.Errorf("expected IsError=true for missing cwd")
	}
	if !strings.Contains(result.Content[0].Text, "cwd is required") {
		t.Errorf("expected 'cwd is required', got: %s", result.Content[0].Text)
	}
}

func TestDescribeCodebaseNoProject(t *testing.T) {
	s := newTestServer()
	result := toolsCall(t, s, "describe_codebase", map[string]interface{}{
		"cwd": t.TempDir(),
	})
	if !result.IsError {
		t.Errorf("expected IsError=true for non-project cwd")
	}
	if !strings.Contains(result.Content[0].Text, "no claude-context project found") {
		t.Errorf("expected 'no claude-context project found', got: %s", result.Content[0].Text)
	}
}

func TestDescribeCodebaseEmpty(t *testing.T) {
	dir := setupMCPProject(t)
	s := newTestServer()
	result := toolsCall(t, s, "describe_codebase", map[string]interface{}{"cwd": dir})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "No anatomy map") {
		t.Errorf("expected 'No anatomy map' in output, got: %s", text)
	}
	if !strings.Contains(text, "No session history") {
		t.Errorf("expected 'No session history' in output, got: %s", text)
	}
}

func TestDescribeCodebaseWithAnatomy(t *testing.T) {
	dir := setupMCPProject(t)
	entries := []state.AnatomyEntry{
		{Path: "cmd/main.go", Description: "subcommand dispatcher", EstTokens: 120, Language: "go"},
		{Path: "pkg/state/session.go", Description: "session state management", EstTokens: 190, Language: "go"},
	}
	if err := state.WriteAnatomy(dir, entries); err != nil {
		t.Fatalf("WriteAnatomy: %v", err)
	}

	s := newTestServer()
	result := toolsCall(t, s, "describe_codebase", map[string]interface{}{"cwd": dir})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Anatomy") {
		t.Errorf("expected 'Anatomy' section in output, got: %s", text)
	}
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected 'main.go' in output, got: %s", text)
	}
	if !strings.Contains(text, "subcommand dispatcher") {
		t.Errorf("expected description in output, got: %s", text)
	}
	if !strings.Contains(text, "~120 tok") {
		t.Errorf("expected token estimate in output, got: %s", text)
	}
}

func TestGetProjectRulesNoCwd(t *testing.T) {
	s := newTestServer()
	result := toolsCall(t, s, "get_project_rules", map[string]interface{}{})
	if !result.IsError || !strings.Contains(result.Content[0].Text, "cwd is required") {
		t.Errorf("expected 'cwd is required' error, got: %v", result)
	}
}

func TestGetProjectRulesEmpty(t *testing.T) {
	dir := setupMCPProject(t)
	s := newTestServer()
	result := toolsCall(t, s, "get_project_rules", map[string]interface{}{"cwd": dir})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "No cerebrum rules") {
		t.Errorf("expected 'No cerebrum rules', got: %s", result.Content[0].Text)
	}
}

func TestGetProjectRulesWithRules(t *testing.T) {
	dir := setupMCPProject(t)
	state.AppendCerebrumRule(dir, state.CerebrumRule{
		Comment: "prefer := over var",
		Pattern: `\bvar\s+\w+\s*=`,
		Message: "prefer := for short variable declarations",
	})
	state.AppendCerebrumRule(dir, state.CerebrumRule{
		Pattern: `fmt\.Println\(`,
		Message: "use a structured logger instead of fmt.Println",
	})

	s := newTestServer()
	result := toolsCall(t, s, "get_project_rules", map[string]interface{}{"cwd": dir})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Cerebrum rules (2)") {
		t.Errorf("expected 'Cerebrum rules (2)', got: %s", text)
	}
	if !strings.Contains(text, "prefer := over var") {
		t.Errorf("expected comment label in output, got: %s", text)
	}
	if !strings.Contains(text, `fmt\.Println\(`) {
		t.Errorf("expected pattern in output, got: %s", text)
	}
}

func TestFindSimilarBugsNoQuery(t *testing.T) {
	dir := setupMCPProject(t)
	s := newTestServer()
	result := toolsCall(t, s, "find_similar_bugs", map[string]interface{}{"cwd": dir})
	if !result.IsError || !strings.Contains(result.Content[0].Text, "query is required") {
		t.Errorf("expected 'query is required' error, got: %v", result)
	}
}

func TestFindSimilarBugsNoBuglog(t *testing.T) {
	dir := setupMCPProject(t)
	s := newTestServer()
	result := toolsCall(t, s, "find_similar_bugs", map[string]interface{}{
		"cwd":   dir,
		"query": "session.StopCount++",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "No buglog entries") {
		t.Errorf("expected 'No buglog entries', got: %s", result.Content[0].Text)
	}
}

func TestFindSimilarBugsMatch(t *testing.T) {
	dir := setupMCPProject(t)
	state.AppendBuglogEntry(dir, state.BuglogEntry{
		Source:      "manual",
		Description: "possible nil deref in session",
		BadCode:     "session.StopCount++\nsession.Value = nil",
	})
	state.AppendBuglogEntry(dir, state.BuglogEntry{
		Source:      "manual",
		Description: "unrelated bug",
		BadCode:     "completely different code here",
	})

	s := newTestServer()
	result := toolsCall(t, s, "find_similar_bugs", map[string]interface{}{
		"cwd":   dir,
		"query": "session.StopCount++\nsession.Value = session.Init()",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Similar bugs (1 match") {
		t.Errorf("expected 1 match, got: %s", text)
	}
	if !strings.Contains(text, "possible nil deref in session") {
		t.Errorf("expected description in output, got: %s", text)
	}
	if !strings.Contains(text, "overlap:") {
		t.Errorf("expected overlap count in output, got: %s", text)
	}
}

func TestFindSimilarBugsNoMatch(t *testing.T) {
	dir := setupMCPProject(t)
	state.AppendBuglogEntry(dir, state.BuglogEntry{
		Source:      "manual",
		Description: "session nil deref",
		BadCode:     "session.StopCount++\nsession.Value = nil",
	})

	s := newTestServer()
	result := toolsCall(t, s, "find_similar_bugs", map[string]interface{}{
		"cwd":   dir,
		"query": "fmt.Println(\"hello\")",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "No matches found") {
		t.Errorf("expected 'No matches found', got: %s", result.Content[0].Text)
	}
}
