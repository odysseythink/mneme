# M4c: MCP Local-State Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three MCP tools (`describe_codebase`, `get_project_rules`, `find_similar_bugs`) that read local project state files and work without an embedding API key.

**Architecture:** New methods on `*mcp.Server` live in `pkg/mcp/local_tools.go`. The existing server startup in `cmd/mcp_server.go` is refactored to start without a key (embedding tools nil-guarded, local-state tools always available). Tool list and dispatch in `pkg/mcp/server.go` gain 3 new entries.

**Tech Stack:** Go standard library, `pkg/state` (ReadAnatomy/ReadCerebrum/ReadBuglog/ReadMemory), `pkg/match` (Tokenize/TokenOverlap from M4b), `pkg/mcp` (existing MCP server).

**Prerequisites:**
- **M4a** must be implemented — `state.ReadCerebrum` and `state.CerebrumRule` must exist in `pkg/state/cerebrum.go`
- **M4b** must be implemented — `state.ReadBuglog`, `state.BuglogEntry`, `match.Tokenize`, `match.TokenOverlap` must exist
- **M2** and **M3** are soft prerequisites — their state files may be absent, which is handled gracefully (tools still return partial results)

---

### Task 1: Server graceful degradation — start without API key

**Files:**
- Modify: `cmd/mcp_server.go`
- Modify: `pkg/mcp/server.go`
- Test: `tests/mcp_local_tools_test.go`

- [ ] **Step 1: Create `tests/mcp_local_tools_test.go` with the nil-key test**

```go
package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/mcp"
)

// newTestServer creates a Server with no embedding dependencies (nil indexer/searcher/embedding).
func newTestServer() *mcp.Server {
	return mcp.NewMCPServer(nil, nil, nil)
}

// toolsCall fires a tools/call request and returns the ToolCallResult.
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./tests/ -run TestServerEmbeddingToolsWithoutKey -v
```

Expected: FAIL — `index_codebase` panics or returns wrong result because `s.indexer` is nil with no nil-guard.

- [ ] **Step 3: Add nil-guards to `pkg/mcp/server.go` `handleToolCall`**

Replace the three embedding tool cases in `handleToolCall`:

```go
case "index_codebase":
    if s.indexer == nil {
        return toolError("embedding tools unavailable: set EMBEDDING_API_KEY to use index_codebase")
    }
    return s.callIndexCodbase(ctx, req.Arguments)
case "search_codebase":
    if s.searcher == nil {
        return toolError("embedding tools unavailable: set EMBEDDING_API_KEY to use search_codebase")
    }
    return s.callSearchCodebase(ctx, req.Arguments)
case "ping_embedding":
    if s.embedding == nil {
        return toolError("embedding tools unavailable: set EMBEDDING_API_KEY to use ping_embedding")
    }
    return s.callPingEmbedding(ctx)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./tests/ -run TestServerEmbeddingToolsWithoutKey -v
```

Expected: PASS.

- [ ] **Step 5: Refactor `cmd/mcp_server.go` to start without API key**

Replace the entire `runMCPServer` function with:

```go
func runMCPServer() {
	cfg := config.FromEnv()
	initLogger(cfg.LogLevel)

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Println("Mneme MCP Server")
		fmt.Println("Usage: Set EMBEDDING_API_KEY and run via Claude Code MCP")
		fmt.Printf("Provider: %s, Model: %s\n", cfg.EmbeddingProvider, cfg.EmbeddingModel)
		os.Exit(0)
	}

	var (
		indexer   pkg.Indexer
		searcher  pkg.Searcher
		embClient *embedding.CachedClient
	)

	if cfg.EmbeddingAPIKey != "" {
		store, err := vectordb.NewStoreFromConfig(cfg)
		if err != nil {
			mlog.Fatalf("Invalid DB_BACKEND: %v", err)
		}
		if err := store.Initialize(cfg.DBPath); err != nil {
			mlog.Fatalf("Failed to initialize database: %v", err)
		}
		defer store.Close()

		var provider pkg.EmbeddingProvider
		switch cfg.EmbeddingProvider {
		case "siliconflow":
			provider = embedding.NewSiliconFlowProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
		case "qwen":
			provider = embedding.NewQwenProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
		default:
			mlog.Fatalf("Unknown embedding provider: %s", cfg.EmbeddingProvider)
		}

		embClient = embedding.NewCachedClient(provider)
		codeSplitter := splitter.NewSplitter()
		indexer = ctxpkg.NewIndexer(store, embClient, codeSplitter, cfg.EmbeddingModel)
		searcher = ctxpkg.NewSearcher(store, embClient)

		keyHint := ""
		if len(cfg.EmbeddingAPIKey) > 10 {
			keyHint = cfg.EmbeddingAPIKey[:10] + "..."
		}
		configSrc := "defaults"
		if cfg.ConfigSource != "" {
			configSrc = cfg.ConfigSource
		}
		mlog.Infof("Mneme MCP Server started: provider=%s model=%s backend=%s key=%s config=%s",
			cfg.EmbeddingProvider, cfg.EmbeddingModel, cfg.DBBackend, keyHint, configSrc)
	} else {
		mlog.Infof("Mneme MCP Server started: EMBEDDING_API_KEY not set — embedding tools unavailable; local-state tools active")
	}

	server := mcp.NewMCPServer(indexer, searcher, embClient)
	if err := server.Start(); err != nil {
		mlog.Fatalf("Server error: %v", err)
	}
}
```

- [ ] **Step 6: Build to verify no compile errors**

```bash
go build -o ./bin/mneme ./cmd
```

Expected: success.

- [ ] **Step 7: Run full test suite**

```bash
go test ./...
```

Expected: all pass (including `TestServerEmbeddingToolsWithoutKey`).

- [ ] **Step 8: Commit**

```bash
git add cmd/mcp_server.go pkg/mcp/server.go tests/mcp_local_tools_test.go
git commit -m "feat(m4c): allow MCP server to start without embedding API key"
```

---

### Task 2: `pkg/mcp/local_tools.go` — 3 local-state tools + wiring

**Files:**
- Create: `pkg/mcp/local_tools.go`
- Modify: `pkg/mcp/server.go` (tool list + dispatch)
- Modify: `tests/mcp_local_tools_test.go` (add remaining tests)

- [ ] **Step 1: Add all remaining tests to `tests/mcp_local_tools_test.go`**

Append after `TestServerEmbeddingToolsWithoutKey`:

```go
// setupMCPProject creates a temp dir with a .mneme/ subdirectory.
// state.FindProjectRoot will locate it via the walk-up fallback (no git needed).
func setupMCPProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/.mneme", 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return dir
}

// --- describe_codebase ---

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
	// t.TempDir() has no .mneme/ and is not a git repo
	result := toolsCall(t, s, "describe_codebase", map[string]interface{}{
		"cwd": t.TempDir(),
	})
	if !result.IsError {
		t.Errorf("expected IsError=true for non-project cwd")
	}
	if !strings.Contains(result.Content[0].Text, "no mneme project found") {
		t.Errorf("expected 'no mneme project found', got: %s", result.Content[0].Text)
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

// --- get_project_rules ---

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

// --- find_similar_bugs ---

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
	// Entry with 4 overlapping tokens with the query
	state.AppendBuglogEntry(dir, state.BuglogEntry{
		Source:      "manual",
		Description: "possible nil deref in session",
		BadCode:     "session.StopCount++\nsession.Value = nil",
	})
	// Entry that won't match
	state.AppendBuglogEntry(dir, state.BuglogEntry{
		Source:      "manual",
		Description: "unrelated bug",
		BadCode:     "completely different code here",
	})

	s := newTestServer()
	// Query overlaps with first entry: session(×3) + stopcount(×1) = 4 >= 3
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./tests/ -run 'TestDescribeCodebase|TestGetProjectRules|TestFindSimilarBugs' -v
```

Expected: FAIL with `unknown tool: describe_codebase` (or similar).

- [ ] **Step 3: Add the 3 tools to `handleToolList` in `pkg/mcp/server.go`**

In `handleToolList`, append these 3 entries to the `"tools"` slice after `ping_embedding`:

```go
{
    "name":        "describe_codebase",
    "description": "Show the anatomy (file structure with descriptions) and recent session history for the current project. Pass cwd from your system context. No embedding API required.",
    "inputSchema": map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "cwd": map[string]interface{}{
                "type":        "string",
                "description": "Working directory of the current project",
            },
        },
        "required": []string{"cwd"},
    },
},
{
    "name":        "get_project_rules",
    "description": "Return the cerebrum rules for the current project — coding conventions Claude should follow. Pass cwd from your system context. No embedding API required.",
    "inputSchema": map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "cwd": map[string]interface{}{
                "type":        "string",
                "description": "Working directory of the current project",
            },
        },
        "required": []string{"cwd"},
    },
},
{
    "name":        "find_similar_bugs",
    "description": "Search the project buglog for previously fixed bugs similar to the given query (code snippet or description). Pass cwd from your system context. No embedding API required.",
    "inputSchema": map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "cwd": map[string]interface{}{
                "type":        "string",
                "description": "Working directory of the current project",
            },
            "query": map[string]interface{}{
                "type":        "string",
                "description": "Code snippet or description to match against known bugs",
            },
        },
        "required": []string{"cwd", "query"},
    },
},
```

- [ ] **Step 4: Add the 3 dispatch cases to `handleToolCall` in `pkg/mcp/server.go`**

In `handleToolCall`, after `case "ping_embedding":` block, add:

```go
case "describe_codebase":
    return s.callDescribeCodebase(ctx, req.Arguments)
case "get_project_rules":
    return s.callGetProjectRules(ctx, req.Arguments)
case "find_similar_bugs":
    return s.callFindSimilarBugs(ctx, req.Arguments)
```

- [ ] **Step 5: Build to verify it fails with "undefined" (methods not yet defined)**

```bash
go build -o ./bin/mneme ./cmd 2>&1
```

Expected: compile error: `s.callDescribeCodebase undefined`.

- [ ] **Step 6: Create `pkg/mcp/local_tools.go`**

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ranwei/mneme/pkg/match"
	"github.com/ranwei/mneme/pkg/state"
)

type localArgs struct {
	Cwd   string `json:"cwd"`
	Query string `json:"query,omitempty"`
}

func (s *Server) callDescribeCodebase(_ context.Context, args json.RawMessage) interface{} {
	var a localArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Cwd == "" {
		return toolError("cwd is required")
	}

	root, ok := state.FindProjectRoot(a.Cwd)
	if !ok {
		return toolError(fmt.Sprintf("no mneme project found at %s (run: mneme init)", a.Cwd))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Project root: %s (%s)\n", filepath.Base(root), root)

	// Anatomy section
	anatomy, _ := state.ReadAnatomy(root)
	if len(anatomy) == 0 {
		fmt.Fprintf(&sb, "\n(No anatomy map. Run: mneme scan)\n")
	} else {
		entries := make([]state.AnatomyEntry, 0, len(anatomy))
		for _, e := range anatomy {
			entries = append(entries, e)
		}
		if len(entries) > 40 {
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].EstTokens > entries[j].EstTokens
			})
			entries = entries[:40]
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Path < entries[j].Path
		})

		fmt.Fprintf(&sb, "\n=== Anatomy (%d files) ===\n", len(anatomy))
		currentDir := ""
		for _, e := range entries {
			dir := filepath.Dir(e.Path)
			if dir == "." {
				dir = ""
			}
			if dir != currentDir {
				if dir == "" {
					fmt.Fprintf(&sb, "(root)/\n")
				} else {
					fmt.Fprintf(&sb, "%s/\n", dir)
				}
				currentDir = dir
			}
			name := filepath.Base(e.Path)
			fmt.Fprintf(&sb, "  %-22s %s (~%d tok)\n", name, e.Description, e.EstTokens)
		}
	}

	// Sessions section
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(&sb, "\n(No session history: cannot determine home directory)\n")
	} else {
		rows, _ := state.ReadMemory(homeDir)
		if len(rows) == 0 {
			fmt.Fprintf(&sb, "\n(No session history yet.)\n")
		} else {
			count := min(5, len(rows))
			fmt.Fprintf(&sb, "\n=== Recent Sessions (last %d) ===\n", count)
			start := len(rows) - count
			for i := len(rows) - 1; i >= start; i-- {
				r := rows[i]
				line := fmt.Sprintf("  %s  %d turns", r.StartedAt, r.TurnCount)
				patterns := make([]string, 0, len(r.PatternCounts))
				for k, v := range r.PatternCounts {
					patterns = append(patterns, fmt.Sprintf("%s×%d", k, v))
				}
				sort.Strings(patterns)
				if len(patterns) > 0 {
					line += "  " + strings.Join(patterns, ", ")
				}
				if r.Summary != "" {
					line += " — " + r.Summary
				}
				fmt.Fprintln(&sb, line)
			}
		}
	}

	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: sb.String()}}}
}

func (s *Server) callGetProjectRules(_ context.Context, args json.RawMessage) interface{} {
	var a localArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Cwd == "" {
		return toolError("cwd is required")
	}

	root, ok := state.FindProjectRoot(a.Cwd)
	if !ok {
		return toolError(fmt.Sprintf("no mneme project found at %s (run: mneme init)", a.Cwd))
	}

	rules, _ := state.ReadCerebrum(root)
	if len(rules) == 0 {
		return ToolCallResult{Content: []ToolContent{{Type: "text", Text: "No cerebrum rules. Run: mneme cerebrum add"}}}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Cerebrum rules (%d):\n", len(rules))
	for i, r := range rules {
		label := r.Message
		if r.Comment != "" {
			label = r.Comment
		}
		fmt.Fprintf(&sb, "  %d. %s\n     pattern: %s\n", i+1, label, r.Pattern)
	}
	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: sb.String()}}}
}

func (s *Server) callFindSimilarBugs(_ context.Context, args json.RawMessage) interface{} {
	var a localArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Cwd == "" {
		return toolError("cwd is required")
	}
	if a.Query == "" {
		return toolError("query is required")
	}

	root, ok := state.FindProjectRoot(a.Cwd)
	if !ok {
		return toolError(fmt.Sprintf("no mneme project found at %s (run: mneme init)", a.Cwd))
	}

	entries, _ := state.ReadBuglog(root)
	if len(entries) == 0 {
		return ToolCallResult{Content: []ToolContent{{Type: "text", Text: "No buglog entries. Run: mneme buglog add"}}}
	}

	queryTokens := match.Tokenize(a.Query)

	type bugMatch struct {
		entry   state.BuglogEntry
		overlap int
	}
	var matches []bugMatch
	for _, e := range entries {
		overlap := match.TokenOverlap(queryTokens, match.Tokenize(e.BadCode))
		if overlap >= 3 {
			matches = append(matches, bugMatch{e, overlap})
		}
	}

	if len(matches) == 0 {
		return ToolCallResult{Content: []ToolContent{{Type: "text", Text: "No matches found."}}}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].overlap > matches[j].overlap
	})
	if len(matches) > 10 {
		matches = matches[:10]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Similar bugs (%d match(es)):\n", len(matches))
	for i, m := range matches {
		firstLine := m.entry.BadCode
		if idx := strings.Index(firstLine, "\n"); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		fmt.Fprintf(&sb, "  %d. [%s] %s  (overlap: %d tokens)\n     was: %s\n",
			i+1, m.entry.Source, m.entry.Description, m.overlap, firstLine)
	}
	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: sb.String()}}}
}
```

- [ ] **Step 7: Build to verify it compiles**

```bash
go build -o ./bin/mneme ./cmd
```

Expected: success.

- [ ] **Step 8: Run the local tool tests**

```bash
go test ./tests/ -run 'TestDescribeCodebase|TestGetProjectRules|TestFindSimilarBugs' -v
```

Expected: PASS (10 tests).

- [ ] **Step 9: Run the full test suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 10: Commit**

```bash
git add pkg/mcp/local_tools.go pkg/mcp/server.go tests/mcp_local_tools_test.go
git commit -m "feat(m4c): add describe_codebase, get_project_rules, find_similar_bugs MCP tools"
```

---

## Self-Review

**Spec coverage:**
- ✅ `pkg/mcp/local_tools.go` with 3 methods — Task 2
- ✅ `pkg/mcp/server.go` nil-guards + tool list/dispatch — Tasks 1 and 2
- ✅ `cmd/mcp_server.go` graceful degradation — Task 1
- ✅ `describe_codebase`: anatomy (top 40 by EstTokens if >40, grouped by dir) + sessions (last 5) — Task 2
- ✅ `describe_codebase` absent anatomy → "No anatomy map" message — Task 2
- ✅ `describe_codebase` absent memory → "No session history yet." — Task 2
- ✅ `get_project_rules`: numbered list with Comment-over-Message label + pattern — Task 2
- ✅ `get_project_rules` no rules → "No cerebrum rules" message — Task 2
- ✅ `find_similar_bugs`: token-overlap ≥ 3, sorted desc, capped at 10 — Task 2
- ✅ `find_similar_bugs` no buglog → "No buglog entries" — Task 2
- ✅ `find_similar_bugs` no match → "No matches found." — Task 2
- ✅ Missing `cwd` → tool error "cwd is required" — Task 2
- ✅ Non-project cwd → tool error "no mneme project found" — Task 2
- ✅ Missing `query` for find_similar_bugs → "query is required" — Task 2
- ✅ `TestServerEmbeddingToolsWithoutKey` — Task 1
- ✅ All 9 remaining local-tool tests — Task 2

**Placeholder scan:** No TBD/TODO in any step.

**Type consistency:**
- `state.AnatomyEntry` used in Task 2 matches M2 spec fields (`Path`, `Description`, `EstTokens`, `Language`) ✓
- `state.CerebrumRule` used in Task 2 matches M4a spec fields (`Comment`, `Pattern`, `Message`) ✓
- `state.BuglogEntry` used in Task 2 matches M4b spec fields (`Source`, `Description`, `BadCode`) ✓
- `state.MemoryRow` used in Task 2 matches M3 spec fields (`StartedAt`, `TurnCount`, `PatternCounts`, `Summary`) ✓
- `match.Tokenize` / `match.TokenOverlap` match M4b `pkg/match/tokens.go` signatures ✓
- `localArgs` struct defined and used only within `local_tools.go` ✓
- `toolError` helper already exists in `server.go` — used in `local_tools.go` (same package) ✓
