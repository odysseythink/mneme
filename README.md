# Mneme - Go Edition

[中文文档](README-chs.md)

High-performance semantic code search for Claude Code, powered by Go with pluggable vector storage backends.

## Features

- **Semantic Search** - Find relevant code using natural language queries
- **Multiple Backends** - DuckDB (embedded), chromem-go (pure Go), or Qdrant (high-scale)
- **Multiple Embedding Models** - SiliconFlow and Qwen
- **Single Binary** - No infrastructure required for DuckDB and chromem backends
- **MCP Integration** - Works directly with Claude Code via stdio JSON-RPC
- **Hook System** - Lightweight hooks track edits, sessions, and context usage
- **Anatomy Map** - Incremental file scanner builds a project summary for Claude
- **Session Memory** - Automatically summarizes past sessions to save context tokens
- **Cerebrum** - Project-level coding rules that surface as Claude warnings
- **Buglog** - Per-project bug history matched against incoming edits

## Quick Start

### Prerequisites

- Go 1.21+
- SiliconFlow or Qwen API key

### Build

```bash
go build -o ./bin/mneme ./cmd
```

### Initialize a project

Run inside any git repository:

```bash
cd /path/to/your/project
mneme init --yes
```

This installs five hooks into Claude Code's `settings.json`, scans all source files into an anatomy map (`.mneme/anatomy.md`), and writes project-context rules into `~/.claude/CLAUDE.md`.

### Register with Claude Code (MCP server)

```bash
claude mcp add mneme \
  -e EMBEDDING_API_KEY=$EMBEDDING_API_KEY \
  -e EMBEDDING_PROVIDER=$EMBEDDING_PROVIDER \
  -- /path/to/bin/mneme
```

Once registered, Claude Code can index a codebase and search it semantically without any further configuration.

## Feature Tutorials

### Anatomy Scan

The anatomy map is a Markdown index of every source file: its language, estimated token count, and a one-line description generated from its content. Claude reads this before opening files, so it can decide whether to read a file at all.

**First scan (automatic on `init`):**

```bash
mneme scan
# ✓ Scanned 142 files → .mneme/anatomy.md
```

**Incremental scan (fast, default):**

On subsequent runs, only files modified since the last scan are re-extracted. Unchanged files are read from the cached anatomy. A clean repo typically completes in under 2 seconds.

```bash
mneme scan
# ✓ Scanned 3 files → .mneme/anatomy.md
```

**Force full rescan:**

```bash
mneme scan --force
# ✓ Scanned 142 files → .mneme/anatomy.md
```

Run `scan` after large refactors or file renames to keep the map accurate.

---

### Session Memory

Every time a Claude Code session ends, the hook writes a structured summary row to `~/.claude/mneme-memory.md`. This file is injected into future sessions, giving Claude a compact history of what you have been working on — without re-reading code.

A row looks like:

```
## 2026-04-20T09:15:00Z (12 turns)
Files: cmd/cmd_scan.go (feature×2), pkg/scanner/extractor.go (refactor×1)
Patterns: feature×2, refactor×1
Summary: Added 2 feature(s). Refactored 1 area(s).
```

**View recent sessions:**

```bash
mneme stats
```

The output shows the last five session rows under `=== Recent Sessions ===`.

**Manual consolidation:**

When the memory file grows beyond 50 rows (about two months of daily use), the session-start hook consolidates it automatically. You can also trigger this manually:

```bash
mneme memory consolidate
# consolidated 37 session row(s)
```

Old sessions (>7 days) are folded into a single blockquote to preserve recency without inflating context:

```
> Consolidated session (284 actions from 37 sessions before 2026-04-13)
```

---

### Stats

View hook activity, edit patterns, and memory rows for the current project:

```bash
mneme stats
```

Example output:

```
project: abc123-myrepo

=== Hook Totals ===
  pre-read:                14
  pre-write:               8
  session-start:           3
  stop:                    3
  hook_errors:             0

=== Edit Patterns ===
  feature:             5
  refactor:            3
  bugfix:              1

=== Recent Sessions (last 5) ===
  2026-04-20T09:15:00Z         12 turns  feature×2, refactor×1
  2026-04-19T14:03:22Z          7 turns  bugfix×1
```

**Waste diagnostics** — detect context-wasting patterns (repeated reads, large unchanged files):

```bash
mneme stats --waste
```

**JSON output** for scripts or dashboards:

```bash
mneme stats --json
```

---

### Cerebrum (Coding Rules)

Cerebrum lets you attach regex-based warning rules to a project. When Claude is about to write to a file that matches a rule's pattern, the pre-write hook injects the warning message into the hook response — reminding Claude of project conventions before it edits.

**Add a rule:**

```bash
mneme cerebrum add \
  --pattern "pkg/state/" \
  --message "All state mutations must go through AtomicWrite; never write files directly." \
  --comment "State package write guard"
```

**List rules:**

```bash
mneme cerebrum list
# 1  pkg/state/  →  All state mutations must go through AtomicWrite; never write files directly.
#    (State package write guard)
```

**Remove a rule:**

```bash
mneme cerebrum remove 1 --yes
```

Rules are stored in `.mneme/cerebrum.json` inside the project.

---

### Buglog

Buglog is a per-project log of bugs you have encountered. When Claude is about to write code, the pre-write hook checks whether any of the changed lines match known bug patterns, and if so appends a reminder to the hook output.

**Log a bug manually:**

```bash
mneme buglog add \
  --description "Forgot to hold the lock before writing memory.md" \
  --code "state.AtomicWrite(memPath, data)" \
  --file "pkg/state/memory.go"
```

**List logged bugs:**

```bash
mneme buglog list
# 1  2026-04-18  pkg/state/memory.go
#    Forgot to hold the lock before writing memory.md
```

**Auto-logging from post-write hook:**

The post-write hook automatically upserts a buglog entry whenever Claude writes a commit classified as a bugfix. This builds the bug history passively without manual input.

**Clear the log:**

```bash
mneme buglog clear --yes
```

---

### MCP Tools (advanced)

When running as an MCP server, mneme exposes three additional tools beyond index/search:

| Tool | Description |
|---|---|
| `describe_codebase` | Returns the full anatomy map as structured text |
| `get_project_rules` | Returns active cerebrum rules for the current project |
| `find_similar_bugs` | Searches the buglog for entries matching a code snippet |

These are available automatically once the binary is registered with `claude mcp add`.

---

## Configuration

All configuration is via environment variables or a config file at `~/.mneme/config.yaml`.

**Priority: env var > config file > built-in default**

Example `~/.mneme/config.yaml`:

```yaml
embedding_api_key: sk-xxx
embedding_provider: qwen
embedding_model: text-embedding-v4

db_backend: chromem
chromem_path: ~/.mneme/chromem
```

| Variable | Default | Description |
|---|---|---|
| `EMBEDDING_API_KEY` | (required) | SiliconFlow or Qwen API key |
| `EMBEDDING_PROVIDER` | `siliconflow` | `siliconflow` or `qwen` |
| `EMBEDDING_MODEL` | `BAAI/bge-large-zh-v1.5` | Embedding model ID |
| `DB_BACKEND` | `duckdb` | Vector store backend: `duckdb`, `qdrant`, `chromem` |
| `DB_PATH` | `~/.mneme/db.duckdb` | DuckDB file path (duckdb backend only) |
| `QDRANT_URL` | `http://localhost:6333` | Qdrant service URL (HTTP port; gRPC 6334 used automatically) |
| `QDRANT_COLLECTION` | `mneme` | Qdrant collection name |
| `CHROMEM_PATH` | `~/.mneme/chromem` | chromem-go persistence directory |
| `LOG_LEVEL` | `info` | `info` or `debug` |

## Backends

### DuckDB (default)

Zero-configuration embedded database. Good for single-machine use with moderate codebases.

```bash
export DB_BACKEND=duckdb
export DB_PATH="$HOME/.mneme/db.duckdb"
```

### chromem-go

Pure Go embedded vector store. No external process required. Persists to disk automatically.

```bash
export DB_BACKEND=chromem
export CHROMEM_PATH="$HOME/.mneme/chromem"
```

### Qdrant

External Qdrant service over gRPC. Best for large codebases or shared/multi-process deployments. The collection is created automatically on first insert.

```bash
# Start Qdrant (Docker)
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant

# Configure
export DB_BACKEND=qdrant
export QDRANT_URL="http://localhost:6333"
export QDRANT_COLLECTION="mneme"
```

The HTTP port (6333) is auto-converted to the gRPC port (6334) internally.

## MCP Resources

| Resource | Description |
|---|---|
| `codebase://index` | Index a codebase. POST with `{"codebase_path": "/path/to/repo"}` |
| `codebase://search` | Semantic search. POST with `{"query": "...", "top_k": 10}` |

## Architecture

```
MCP stdio request
  → cmd/main.go           (subcommand dispatch)
  → cmd/mcp_server.go     (JSON-RPC over stdin/stdout)
  → pkg/context/          (indexer.go: split + embed + store;
                            searcher.go: embed query + retrieve)
  → pkg/embedding/        (CachedClient wraps SiliconFlow/Qwen with MD5 cache)
  → pkg/vectordb/         (factory.go selects DuckDB / Qdrant / chromem)
  → pkg/splitter/         (function-boundary splitting for Go/Py/JS; 50-line chunks otherwise)

Hook pipeline (Claude Code events → cmd/hook_*.go):
  session-start → write memory row, lazy consolidate, upsert session
  pre-read      → anatomy lookup, emit description for Claude
  pre-write     → cerebrum rule match, buglog match, emit warnings
  post-write    → classify edit, upsert to ledger and buglog
  stop          → increment stop counter

State files (per-project, inside .mneme/):
  anatomy.md    → file map regenerated by `scan`
  ledger.json   → cumulative hook counters
  session.json  → current session edits
  cerebrum.json → coding rules
  buglog.json   → bug history

Global state (~/.claude/):
  mneme-memory.md → session history, consolidated on demand
```

All cross-package contracts are defined as interfaces in `pkg/types.go` (`Store`, `EmbeddingProvider`, `Indexer`, `Searcher`, `Splitter`).

Package layout:

- `cmd/` - Binary entry point + all subcommands and hooks
- `pkg/` - Core interfaces and types
- `pkg/config/` - Environment-based configuration
- `pkg/context/` - Indexing and search orchestration
- `pkg/embedding/` - SiliconFlow and Qwen providers with caching
- `pkg/vectordb/` - DuckDB, Qdrant, and chromem backends; factory selector
- `pkg/splitter/` - Language-aware code chunking
- `pkg/mcp/` - MCP protocol server
- `pkg/state/` - Ledger, session, anatomy, memory, and lock primitives
- `pkg/scanner/` - File walker and anatomy extractor (incremental)
- `pkg/consolidator/` - Memory row consolidation
- `pkg/hook/` - Hook event parsing helpers
- `pkg/waste/` - Context waste pattern detection

## Development

```bash
# Build
go build -o ./bin/mneme ./cmd

# Run all tests
go test -v ./...

# Run a single test
go test -v ./tests -run TestScanIncrementalSucceeds

# Run Qdrant integration tests (requires a running Qdrant instance)
QDRANT_URL=http://localhost:6333 go test -v -tags qdrant_integration ./tests

# Debug logging
LOG_LEVEL=debug ./bin/mneme

# Uninstall hooks from a project
mneme init --uninstall --yes
```

## License

MIT
