# Claude Context Go Refactoring Design

**Date:** 2026-04-23  
**Scope:** Refactor TypeScript monorepo (claude-context v0.1.7) → Go MVP (Core + MCP Server)  
**Goals:** Better performance/resource efficiency, simplified deployment (single binary)

---

## 1. Project Overview

**Source:** TypeScript monorepo with code indexing tool, semantic search, vector DB integration  
**Target:** Go implementation focusing on MVP with essential features only

### What's Included
- Core indexing engine (code splitting, embedding, vector search)
- MCP server for Claude Code integration
- Embedded DuckDB (no separate infrastructure)
- SiliconFlow or Qwen embeddings
- Single compiled binary

### What's Excluded (v1)
- VS Code extension (can be built later as thin wrapper)
- Chrome extension (future phase)
- Multiple embedding models (start with SiliconFlow/Qwen only)
- Multiple vector DB backends (DuckDB only)

---

## 2. Architecture

### Three-Layer Design

**Service Layer** — MCP server handling Claude Code protocol, request routing, resource management

**Business Logic** — Core indexing engine orchestrating code splitting, embedding generation, semantic search

**Data Layer** — DuckDB for persistent vector storage and metadata

All three compile into a single statically-linked binary.

### Information Flow

```
User (Claude Code)
    ↓
MCP Server (cmd/mcp)
    ├→ [Index Request]  → Indexer → Splitter → Embedding API → DuckDB
    └→ [Search Request] → Searcher → Embedding API → DuckDB → Results
```

---

## 3. Package Structure

```
go-claude-context/
├── cmd/mcp/
│   └── main.go                    # MCP server entry point
├── pkg/
│   ├── context/
│   │   ├── indexer.go            # Orchestrates index pipeline
│   │   └── searcher.go           # Executes semantic search queries
│   ├── embedding/
│   │   ├── client.go             # Interface + SiliconFlow/Qwen implementations
│   │   └── cache.go              # Local embedding cache
│   ├── vectordb/
│   │   └── store.go              # DuckDB initialization, insert, search
│   ├── splitter/
│   │   └── ast.go                # Code splitting via tree-sitter
│   └── mcp/
│       ├── server.go             # MCP protocol handlers
│       └── resources.go          # MCP resource definitions
├── go.mod
├── go.sum
├── README.md
└── docs/
    └── specs/
        └── 2026-04-23-go-refactor-design.md
```

---

## 4. Core Components

### 4.1 Indexer (`pkg/context/indexer.go`)

**Responsibility:** Orchestrate the indexing pipeline.

**Interface:**
```go
type Indexer interface {
    Index(ctx context.Context, codebasePath string) (IndexResult, error)
}

type IndexResult struct {
    FilesProcessed   int
    VectorsStored    int
    Duration         time.Duration
    EmbeddingModel   string
    CodebasePath     string
}
```

**Behavior:**
1. Scan codebase for code files (Go, Python, JavaScript, TypeScript, etc.)
2. Parse each file using tree-sitter, extract semantic chunks (functions, classes, blocks)
3. For each chunk, generate embedding via embedding client
4. Store embedding + metadata (file path, language, line range) in DuckDB
5. Return summary stats

**Constraints:**
- Max chunk size: 2000 tokens (to fit embedding limits)
- Batch API calls (group embeddings by 20-50 per request for efficiency)
- Skip non-code files (.git, node_modules, vendor, etc.)

### 4.2 Searcher (`pkg/context/searcher.go`)

**Responsibility:** Execute semantic search queries.

**Interface:**
```go
type Searcher interface {
    Search(ctx context.Context, query string, topK int) ([]SearchResult, error)
}

type SearchResult struct {
    FileLocation string  // file:startLine:endLine
    Language     string
    Code         string
    Similarity   float32
}
```

**Behavior:**
1. Generate embedding for query string
2. Find K nearest neighbors in DuckDB vector space (cosine similarity)
3. Retrieve metadata and code snippets
4. Rank by similarity score
5. Return top K results

**Constraints:**
- Default K=5, maximum K=20
- Only return results with similarity > 0.5 threshold

### 4.3 Embedding Client (`pkg/embedding/client.go`)

**Responsibility:** Abstract embedding generation and caching.

**Interface:**
```go
type EmbeddingProvider interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

type CachedClient struct {
    provider EmbeddingProvider
    cache    map[string][]float32  // hash(text) → embedding
}
```

**Implementations:**
- `SiliconFlowProvider` — calls SiliconFlow API endpoint
- `QwenProvider` — calls Qwen (Alibaba) API endpoint

**Behavior:**
- Cache embeddings in memory during run to avoid re-querying identical text
- Batch API requests (combine multiple texts into single call for efficiency)
- Handle rate limiting with exponential backoff

### 4.4 Code Splitter (`pkg/splitter/ast.go`)

**Responsibility:** Parse code and extract semantic chunks.

**Dependencies:** tree-sitter via `github.com/tree-sitter/go-tree-sitter` bindings

**Supported Languages:** Go, Python, JavaScript, TypeScript, Rust, Java, C++, C# (extensible)

**Behavior:**
1. Use tree-sitter to parse file into AST
2. Extract function/method/class definitions as chunks
3. Include docstrings/comments in chunk
4. Fall back to fixed-size line-based splitting if language not supported
5. Return list of chunks with metadata (language, start_line, end_line)

**Constraints:**
- Min chunk size: 10 tokens
- Max chunk size: 2000 tokens
- Each chunk must be a complete, parseable unit if possible

### 4.5 Vector Database Store (`pkg/vectordb/store.go`)

**Responsibility:** Initialize DuckDB, insert vectors, execute searches.

**Database Schema:**
```sql
CREATE TABLE vectors (
    id INTEGER PRIMARY KEY DEFAULT nextval('seq_vectors'),
    embedding FLOAT32[],           -- DuckDB native vector type
    text VARCHAR,                  -- code chunk content
    file_path VARCHAR,             -- absolute or relative path
    language VARCHAR,              -- programming language
    start_line INTEGER,
    end_line INTEGER,
    codebase_hash VARCHAR,         -- for multi-codebase support
    indexed_at TIMESTAMP DEFAULT now()
);

CREATE TABLE metadata (
    codebase_hash VARCHAR PRIMARY KEY,
    codebase_path VARCHAR,
    embedding_model VARCHAR,
    indexed_at TIMESTAMP,
    total_files INTEGER,
    total_vectors INTEGER
);
```

**Interface:**
```go
type Store interface {
    Initialize(dbPath string) error
    InsertVector(ctx context.Context, vec Vector) error
    Search(ctx context.Context, embedding []float32, topK int) ([]Vector, error)
    Close() error
}
```

**Behavior:**
- Create DB file on first run (auto-create `~/.claude-context/db.duckdb`)
- Use DuckDB's native vector type and cosine_similarity() function
- Index embeddings for fast search
- Support multi-codebase indexing (keyed by codebase hash)

### 4.6 MCP Server (`pkg/mcp/server.go`)

**Responsibility:** Implement Model Context Protocol for Claude Code.

**Resources Exposed:**
- `codebase://index` — POST with codebase_path → triggers indexing
- `codebase://search?query=<text>` — GET → returns search results

**Behavior:**
1. Listen on stdio (MCP standard)
2. Parse incoming MCP requests
3. Route to Indexer or Searcher
4. Format responses as MCP-compliant resources
5. Stream large results if needed

**Error Handling:**
- Invalid paths → 400 Bad Request
- API rate limits → 429 Retry-After
- DB errors → 500 Internal Server Error

---

## 5. Configuration & Deployment

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_API_KEY` | (required) | API key for embedding provider |
| `EMBEDDING_PROVIDER` | `siliconflow` | `siliconflow` or `qwen` |
| `EMBEDDING_MODEL` | `BAAI/bge-large-zh-v1.5` | Model ID. SiliconFlow: `BAAI/bge-large-zh-v1.5` (1536 dims). Qwen: `text-embedding-v2` (1536 dims) |
| `DB_PATH` | `~/.claude-context/db.duckdb` | Path to DuckDB file |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Build & Deployment

**Build:**
```bash
go build -o claude-context ./cmd/mcp
```

**Register with Claude Code:**
```bash
claude mcp add claude-context \
  -e EMBEDDING_API_KEY=sk-xxx \
  -e EMBEDDING_PROVIDER=siliconflow \
  -- /path/to/claude-context
```

**Single Binary:** Includes Go runtime, DuckDB, tree-sitter grammars. No external dependencies except API calls to embedding provider.

---

## 6. Data Flow Examples

### Indexing Flow
```
User: "Index my /home/user/myproject"
  ↓
MCP: POST codebase://index { codebase_path: "/home/user/myproject" }
  ↓
Indexer.Index():
  1. Walk filesystem, find .go, .py, .ts files
  2. For each file:
     a. Parse with tree-sitter
     b. Extract functions/classes
     c. Generate embeddings (batch 20 at a time)
     d. Store in DuckDB with metadata
  3. Return { FilesProcessed: 150, VectorsStored: 1250, Duration: 45s }
  ↓
Claude Code: "Indexed 1250 vectors in 45 seconds"
```

### Search Flow
```
User: "Find code related to 'authentication middleware'"
  ↓
Claude Code: GET codebase://search?query=authentication%20middleware
  ↓
MCP → Searcher.Search("authentication middleware", topK=5):
  1. Generate embedding for query
  2. SELECT * FROM vectors WHERE cosine_similarity(embedding, query_embedding) > 0.5
     ORDER BY similarity DESC LIMIT 5
  3. Return top 5 results with file locations and code snippets
  ↓
Claude Code: Displays results in context window
  ↓
User: "Modify this code in auth.go:45:60 to add logging"
  ↓
Claude: Operates on context-aware code
```

---

## 7. Error Handling & Resilience

| Scenario | Handling |
|----------|----------|
| Embedding API rate limit | Exponential backoff, retry after 60s |
| Embedding API timeout | Fail chunk, log error, continue |
| DuckDB disk full | Return error, halt indexing |
| Invalid codebase path | Return 400, log error |
| Unsupported language | Fall back to line-based splitting |
| Network error | Retry up to 3 times, then fail |

---

## 8. Testing Strategy

**Unit Tests:**
- Splitter: verify correct chunk extraction for each language
- Embedding client: mock API, verify cache hit/miss
- Searcher: mock DB, verify ranking by similarity

**Integration Tests:**
- Index real small codebase, verify DuckDB population
- Search, verify results match expected snippets
- MCP server: verify request/response format

**Performance Tests:**
- Index large codebase (10k+ files), measure time and memory
- Search latency at scale

---

## 9. Future Extensions (Post-MVP)

- Multiple embedding models (OpenAI, Gemini, Ollama)
- Incremental indexing (only re-index changed files)
- Web UI for search and management
- CLI tool for standalone indexing
- Persist embedding cache to DuckDB
- Async indexing (background job)

---

## 10. Dependencies

**Go Standard Library:** context, encoding/json, fmt, log, os, path/filepath, time

**Third-Party Libraries:**
- `github.com/tree-sitter/go-tree-sitter` — code parsing (AST extraction)
- `github.com/marcboeker/go-duckdb` — DuckDB driver
- `github.com/google/uuid` — unique IDs for vectors
- (MCP library: TBD based on Go ecosystem standard, likely based on Anthropic's reference implementation)

**External APIs:**
- SiliconFlow embedding API (https://api.siliconflow.cn/)
- Qwen embedding API (via DashScope API)

---

## 11. Success Criteria

✅ Single compiled binary, no external dependencies except embedding API  
✅ Index 5000+ files in <2 minutes  
✅ Search query returns results in <500ms  
✅ DuckDB file < 1GB for 100k vectors  
✅ MCP protocol correctly implemented for Claude Code  
✅ Support Go, Python, JavaScript/TypeScript code splitting  

---

**Design Status:** ✅ Ready for implementation planning
