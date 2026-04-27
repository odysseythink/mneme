# M4c: MCP Local-State Tools — Design Spec

## Overview

M4c adds three MCP tools that expose per-project local state to Claude without requiring an embedding API. They read files created by M2 (anatomy map), M3 (memory rows), M4a (cerebrum rules), and M4b (buglog).

Three deliverables:
- `pkg/mcp/local_tools.go` — `callDescribeCodebase`, `callGetProjectRules`, `callFindSimilarBugs` methods on `*Server`
- Modifications to `pkg/mcp/server.go` — add 3 tools to list/dispatch; nil-guard embedding tools
- Modifications to `cmd/mcp_server.go` — remove fatal on missing API key; allow nil indexer/searcher

M4a (cerebrum) and M4b (buglog) are prerequisites. M2 (anatomy) and M3 (memory) are soft prerequisites — their files may be absent, which is handled gracefully.

---

## 1. Architecture & Data Flow

The `Server` struct gains no new fields. Each local-state tool call resolves the project root fresh from the `cwd` parameter.

**File map:**

| File | Change |
|---|---|
| `pkg/mcp/local_tools.go` | New — 3 call methods on `*Server` |
| `pkg/mcp/server.go` | Modified — tool list, dispatch, nil-guards for embedding tools |
| `cmd/mcp_server.go` | Modified — remove `Fatal` on missing API key; allow nil indexer/searcher/embClient |

**Data flow:**

```
tools/call → handleToolCall(ctx, params) → switch req.Name:

"describe_codebase" (cwd string):
  state.FindProjectRoot(cwd) → root (error if not found)
  state.ReadAnatomy(root)    → map[string]AnatomyEntry  (nil if absent)
  homeDir, _ := os.UserHomeDir()
  state.ReadMemory(homeDir)  → []MemoryRow  (nil if absent)
  → format text block → ToolCallResult

"get_project_rules" (cwd string):
  state.FindProjectRoot(cwd) → root
  state.ReadCerebrum(root)   → []CerebrumRule  (nil if absent)
  → numbered list → ToolCallResult

"find_similar_bugs" (cwd string, query string):
  state.FindProjectRoot(cwd) → root
  state.ReadBuglog(root)     → []BuglogEntry  (nil if absent)
  match.Tokenize(query)      → queryTokens
  for each entry:
    overlap := match.TokenOverlap(queryTokens, match.Tokenize(entry.BadCode))
    if overlap >= 3 → match
  sort matches by overlap descending, cap at 10
  → formatted results → ToolCallResult
```

---

## 2. Tool Specifications

### Input schemas

All three tools require `cwd` (string, required) — the caller's working directory. Claude passes its primary working directory so the tool locates the current project.

`find_similar_bugs` additionally requires `query` (string, required) — either a code snippet or a natural language description.

### `describe_codebase`

```json
{
  "name": "describe_codebase",
  "description": "Show the anatomy (file structure with descriptions) and recent session history for the current project. Pass cwd from your system context. No embedding API required.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "cwd": {"type": "string", "description": "Working directory of the current project"}
    },
    "required": ["cwd"]
  }
}
```

**Output format:**

```
Project root: go-claude-context (/path/to/project)

=== Anatomy (37 files) ===
cmd/
  cmd_hook.go         hook event dispatcher (~450 tok)
  cmd_init.go         init command (~280 tok)
  main.go             subcommand dispatcher (~120 tok)
pkg/state/
  anatomy.go          anatomy state I/O (~200 tok)
  cerebrum.go         cerebrum rules storage (~150 tok)
  ledger.go           ledger/counter management (~370 tok)

=== Recent Sessions (last 5) ===
  2026-04-27T16:05Z  7 turns  bugfix×2, feature×1 — Fixed 2 bugs, added 1 feature.
  2026-04-27T14:32Z  3 turns  refactor×1, new_file×1
```

**Rules:**
- Anatomy entries are grouped by directory (lexicographic), each file indented with 2 spaces.
- If anatomy has > 40 entries, show the 40 with the highest `EstTokens`.
- Sessions: last 5 from `ReadMemory`, most-recent first.
- If anatomy absent: replace anatomy section with `(No anatomy map. Run: claude-context scan)`
- If memory absent or empty: replace sessions section with `(No session history yet.)`

---

### `get_project_rules`

```json
{
  "name": "get_project_rules",
  "description": "Return the cerebrum rules for the current project — coding conventions that Claude should follow. Pass cwd from your system context. No embedding API required.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "cwd": {"type": "string", "description": "Working directory of the current project"}
    },
    "required": ["cwd"]
  }
}
```

**Output format:**

```
Cerebrum rules (2):
  1. prefer := for short variable declarations
     pattern: \bvar\s+\w+\s*=
  2. use a structured logger instead of fmt.Println
     pattern: fmt\.Println\(
```

If a rule has a non-empty `Comment`, it is shown as the label; otherwise `Message` is used. `Pattern` is always shown.

If no rules: `No cerebrum rules. Run: claude-context cerebrum add`

---

### `find_similar_bugs`

```json
{
  "name": "find_similar_bugs",
  "description": "Search the project's buglog for previously fixed bugs similar to the given query (code snippet or description). Returns matches by token overlap. Pass cwd from your system context. No embedding API required.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "cwd":   {"type": "string", "description": "Working directory of the current project"},
      "query": {"type": "string", "description": "Code snippet or description to match against known bugs"}
    },
    "required": ["cwd", "query"]
  }
}
```

**Matching:** `match.Tokenize(query)` → queryTokens; for each `BuglogEntry`, compute `TokenOverlap(queryTokens, Tokenize(entry.BadCode))`; keep entries with overlap ≥ 3; sort descending by overlap; cap at 10.

**Output format:**

```
Similar bugs (1 match):
  1. [auto] possible re-introduction of bugfix in session.go  (overlap: 4 tokens)
     was: session.StopCount++

No matches found. (if entries exist but none overlap >= 3)
No buglog entries. Run: claude-context buglog add (if file absent)
```

---

## 3. Server Startup Changes

**Problem:** `cmd/mcp_server.go` currently calls `mlog.Fatal` if `EmbeddingAPIKey == ""`, blocking the server from starting even when only local-state tools are needed.

**Fix:** Remove the fatal check. Allow the server to start without a key. Embedding-dependent tools (`index_codebase`, `search_codebase`, `ping_embedding`) return a helpful tool error if called without a configured indexer/searcher/embedding client.

**`cmd/mcp_server.go` change:** conditionally initialize embedding components:

```go
var (
    store     pkg.Store
    indexer   pkg.Indexer
    searcher  pkg.Searcher
    embClient *embedding.CachedClient
)

if cfg.EmbeddingAPIKey != "" {
    // initialize store, provider, indexer, searcher, embClient as before
} else {
    mlog.Infof("EMBEDDING_API_KEY not set — embedding tools unavailable; local-state tools active")
}

server := mcp.NewMCPServer(indexer, searcher, embClient)
```

**`pkg/mcp/server.go` change:** nil-guard in each embedding tool dispatch:

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

The 3 new local-state tools have no nil-guard — they always proceed to file reads.

---

## 4. Error Handling & Safety

| Error | Behavior |
|---|---|
| `cwd` param missing or empty | Return tool error: `"cwd is required"` |
| Project root not found from `cwd` | Return tool error: `"no claude-context project found at <cwd> (run: claude-context init)"` |
| Anatomy/cerebrum/buglog file absent | Return partial/empty result with explanatory message — never a tool error |
| `find_similar_bugs` no matches | Return `"No matches found."` — not an error |
| `os.UserHomeDir()` fails | Omit sessions section from `describe_codebase`, continue |
| `query` param missing for `find_similar_bugs` | Return tool error: `"query is required"` |
| Any panic in call method | Recovered by existing `handleRequest` goroutine; server stays alive |

---

## 5. Testing Strategy

**Unit/integration tests** in `tests/mcp_local_tools_test.go`:

Tests call `server.HandleRequest(req)` directly (no subprocess) to inspect `ToolCallResult.Content[0].Text`.

```go
// helpers used across tests
func newTestServer() *mcp.Server {
    return mcp.NewMCPServer(nil, nil, nil)
}
func toolsCall(t *testing.T, s *mcp.Server, name string, args map[string]string) mcp.ToolCallResult
```

- `TestDescribeCodebaseNoCwd` — missing cwd → tool error containing `"cwd is required"`
- `TestDescribeCodebaseNoProject` — cwd = `/tmp` (not initialized) → tool error containing `"no claude-context project found"`
- `TestDescribeCodebaseEmpty` — initialized project, no anatomy, no memory → output contains `"No anatomy"` and `"No session history"`
- `TestDescribeCodebaseWithAnatomy` — project with 2 anatomy entries → output contains directory grouping and token estimates
- `TestGetProjectRulesEmpty` — no cerebrum.md → output contains `"No cerebrum rules"`
- `TestGetProjectRulesWithRules` — 2 rules via `state.AppendCerebrumRule` → numbered list, both patterns visible
- `TestFindSimilarBugsNoQuery` — missing `query` param → tool error containing `"query is required"`
- `TestFindSimilarBugsNoBuglog` — no buglog.json → output contains `"No buglog entries"`
- `TestFindSimilarBugsMatch` — 2 entries, query overlaps one with 4 tokens → 1 match, overlap count shown
- `TestFindSimilarBugsNoMatch` — query overlaps 0 entries → `"No matches found"`
- `TestServerEmbeddingToolsWithoutKey` — `NewMCPServer(nil, nil, nil)` + call `index_codebase` → tool error containing `"embedding tools unavailable"`, `IsError: true`, no panic

---

## 6. Out of Scope (M4c)

- Authentication or rate-limiting on local-state tools
- Streaming results (all responses are single text blocks)
- `describe_codebase` showing buglog or cerebrum rules (those have their own tools)
- Fuzzy / semantic matching for `find_similar_bugs` (M5+ with embeddings)
- Auto-refresh anatomy on `describe_codebase` call (user must run `claude-context scan`)
