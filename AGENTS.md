# Mneme Go - Agent Instructions

## Project

Go implementation of a semantic code search MCP server for Claude Code. Single binary with embedded DuckDB.

## Build & Run

```bash
go build -o ./bin/mneme ./cmd/mcp
go test -v ./...
```

## Environment

Required at runtime:
- `EMBEDDING_API_KEY` - SiliconFlow or Qwen API key
- `EMBEDDING_PROVIDER` - `siliconflow` or `qwen` (default: siliconflow)
- `EMBEDDING_MODEL` - Model ID (default: `BAAI/bge-large-zh-v1.5`)
- `DB_PATH` - DuckDB file path (default: `~/.mneme/db.duckdb`)

## Architecture

```
cmd/mcp/main.go          # Entry point
pkg/
├── config/              # Env config
├── context/             # Indexer + Searcher orchestration
├── embedding/           # SiliconFlow/Qwen providers + cache
├── vectordb/            # DuckDB operations
├── splitter/            # Code parsing via tree-sitter
└── mcp/                 # MCP protocol handlers
```

## Key Dependencies

- `github.com/marcboeker/go-duckdb` - Vector storage
- `github.com/tree-sitter/go-tree-sitter` - AST parsing
- `github.com/google/uuid` - ID generation

## Notes

- MCP server communicates via stdio
- Supports Go, Python, JavaScript/TypeScript code splitting
- Embedding dimensions: 1536
- Default search topK: 5, max: 20, similarity threshold: 0.5
