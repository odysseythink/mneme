# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o ./bin/claude-context ./cmd

# Run all tests
go test -v ./...

# Run a single test
go test -v ./tests -run TestStoreInitialize

# Debug logging
LOG_LEVEL=debug ./bin/claude-context
```

## Configuration

**Priority: env var > config file > built-in default**

Config file is read from `~/.claude-context/config.yaml` by default.
Override the path with `CONFIG_FILE=/path/to/config.yaml`.

Example `~/.claude-context/config.yaml`:

```yaml
embedding_api_key: sk-xxx
embedding_provider: qwen
embedding_model: text-embedding-v4

db_backend: chromem
chromem_path: ~/.claude-context/chromem

# db_path: ~/.claude-context/db.duckdb   # db_backend=duckdb
# qdrant_url: http://localhost:6333       # db_backend=qdrant
# qdrant_collection: claude-context

log_level: info
```

Environment variables (override config file):

| Variable | Default | Description |
|---|---|---|
| `CONFIG_FILE` | `~/.claude-context/config.yaml` | Config file path |
| `EMBEDDING_API_KEY` | (required) | SiliconFlow or Qwen API key |
| `EMBEDDING_PROVIDER` | `siliconflow` | `siliconflow` or `qwen` |
| `EMBEDDING_MODEL` | `BAAI/bge-large-zh-v1.5` | Model ID |
| `DB_PATH` | `~/.claude-context/db.duckdb` | DuckDB file path |
| `DB_BACKEND` | `duckdb` | Vector store backend: `duckdb`, `qdrant`, `chromem` |
| `QDRANT_URL` | `http://localhost:6333` | Qdrant service URL (HTTP port; gRPC 6334 auto-used) |
| `QDRANT_COLLECTION` | `claude-context` | Qdrant collection name |
| `CHROMEM_PATH` | `~/.claude-context/chromem` | chromem-go persistence directory |

## Architecture

All cross-package contracts are defined as interfaces in `pkg/types.go` (`Indexer`, `Searcher`, `EmbeddingProvider`, `Store`, `Splitter`). The data flow is:

```
MCP stdio request
  → pkg/mcp/server.go   (JSON-RPC over stdin/stdout)
  → pkg/context/        (indexer.go orchestrates splitting + embedding + storage;
                          searcher.go generates query embedding + retrieves results)
  → pkg/embedding/      (CachedClient wraps SiliconFlow/Qwen behind an in-memory MD5 cache)
  → pkg/vectordb/       (DuckDB stores vectors as FLOAT[] rows)
  → pkg/splitter/       (function-boundary splitting by string matching, not tree-sitter AST)
```

**Key non-obvious detail:** `vectordb.DuckDBStore.Search()` does a full table scan but does **not** select the `embedding` column, so returned `Vector.Embedding` fields are nil. Cosine similarity is re-computed in `pkg/context/searcher.go` using the query embedding vs. stored embeddings — but because the store doesn't return embeddings, similarity always evaluates to 0 and no results pass the 0.5 threshold. The `DuckDBStore.SearchWithEmbedding()` method does fetch embeddings but is not exposed through the `Store` interface. This is the primary known bug.

**MCP transport:** The server reads newline-delimited JSON from stdin, writes JSON to stdout (`sendResponse` calls `fmt.Println`). Startup uses `os.Stdin.Stat()` to detect terminal mode and exits gracefully instead of blocking.

**Splitter languages:** Go, Python, and JS/TS use function-boundary splitting (brace/indent counting). All other languages fall back to 50-line fixed chunks.

**Tests** live in `tests/` as package `tests`, separate from source packages. Mock types (`MockStore`, `MockEmbeddingProvider`) are defined in multiple test files — watch for redeclaration conflicts.

## Registering with Claude Code

```bash
claude mcp add claude-context \
  -e EMBEDDING_API_KEY=$EMBEDDING_API_KEY \
  -e EMBEDDING_PROVIDER=$EMBEDDING_PROVIDER \
  -- /path/to/bin/claude-context
```

MCP resources exposed: `codebase://index` (POST with `codebase_path`) and `codebase://search` (POST with `query`, optional `top_k`).
