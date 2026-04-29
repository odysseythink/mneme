# Claude Context - Go Edition

High-performance semantic code search for Claude Code, powered by Go with pluggable vector storage backends.

## Features

- **Semantic Search** - Find relevant code using natural language queries
- **Multiple Backends** - DuckDB (embedded), chromem-go (pure Go), or Qdrant (high-scale)
- **Multiple Embedding Models** - SiliconFlow and Qwen
- **Single Binary** - No infrastructure required for DuckDB and chromem backends
- **MCP Integration** - Works directly with Claude Code via stdio JSON-RPC

## Quick Start

### Prerequisites

- Go 1.21+
- SiliconFlow or Qwen API key

### Build

```bash
go build -o ./bin/claude-context ./cmd/mcp
```

### Register with Claude Code

```bash
claude mcp add claude-context \
  -e EMBEDDING_API_KEY=$EMBEDDING_API_KEY \
  -e EMBEDDING_PROVIDER=$EMBEDDING_PROVIDER \
  -- /path/to/bin/claude-context
```

Once registered, Claude Code can index a codebase and search it semantically without any further configuration.

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `EMBEDDING_API_KEY` | (required) | SiliconFlow or Qwen API key |
| `EMBEDDING_PROVIDER` | `siliconflow` | `siliconflow` or `qwen` |
| `EMBEDDING_MODEL` | `BAAI/bge-large-zh-v1.5` | Embedding model ID |
| `DB_BACKEND` | `duckdb` | Vector store backend: `duckdb`, `qdrant`, `chromem` |
| `DB_PATH` | `~/.claude-context/db.duckdb` | DuckDB file path (duckdb backend only) |
| `QDRANT_URL` | `http://localhost:6333` | Qdrant service URL (HTTP port; gRPC 6334 used automatically) |
| `QDRANT_COLLECTION` | `claude-context` | Qdrant collection name |
| `CHROMEM_PATH` | `~/.claude-context/chromem` | chromem-go persistence directory |
| `LOG_LEVEL` | `info` | `info` or `debug` |

## Backends

### DuckDB (default)

Zero-configuration embedded database. Good for single-machine use with moderate codebases.

```bash
export DB_BACKEND=duckdb
export DB_PATH="$HOME/.claude-context/db.duckdb"
```

### chromem-go

Pure Go embedded vector store. No external process required. Persists to disk automatically.

```bash
export DB_BACKEND=chromem
export CHROMEM_PATH="$HOME/.claude-context/chromem"
```

### Qdrant

External Qdrant service over gRPC. Best for large codebases or shared/multi-process deployments. The collection is created automatically on first insert.

```bash
# Start Qdrant (Docker)
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant

# Configure
export DB_BACKEND=qdrant
export QDRANT_URL="http://localhost:6333"
export QDRANT_COLLECTION="claude-context"
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
  → pkg/mcp/server.go     (JSON-RPC over stdin/stdout)
  → pkg/context/          (indexer.go: split + embed + store;
                            searcher.go: embed query + retrieve)
  → pkg/embedding/        (CachedClient wraps SiliconFlow/Qwen with MD5 cache)
  → pkg/vectordb/         (factory.go selects DuckDB / Qdrant / chromem)
  → pkg/splitter/         (function-boundary splitting for Go/Py/JS; 50-line chunks otherwise)
```

All cross-package contracts are defined as interfaces in `pkg/types.go` (`Store`, `EmbeddingProvider`, `Indexer`, `Searcher`, `Splitter`).

Package layout:

- `cmd/mcp/` - Binary entry point
- `pkg/` - Core interfaces and types
- `pkg/config/` - Environment-based configuration
- `pkg/context/` - Indexing and search orchestration
- `pkg/embedding/` - SiliconFlow and Qwen providers with caching
- `pkg/vectordb/` - DuckDB, Qdrant, and chromem backends; factory selector
- `pkg/splitter/` - Language-aware code chunking
- `pkg/mcp/` - MCP protocol server

## Development

```bash
# Build
go build -o ./bin/claude-context ./cmd/mcp

# Run all tests
go test -v ./...

# Run Qdrant integration tests (requires a running Qdrant instance)
QDRANT_URL=http://localhost:6333 go test -v -tags qdrant_integration ./tests

# Run a single test
go test -v ./tests -run TestChromemStoreInsertAndSearch

# Debug logging
LOG_LEVEL=debug ./bin/claude-context
```

## License

MIT
