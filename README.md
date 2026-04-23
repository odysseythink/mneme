# Claude Context - Go Edition

High-performance semantic code search for Claude Code, powered by Go and DuckDB.

## Features

- **Single Binary Deployment** - No external infrastructure needed
- **Semantic Search** - Find relevant code using natural language queries
- **Embedded Database** - DuckDB for zero-configuration deployment
- **Multiple Embedding Models** - Support for SiliconFlow and Qwen
- **MCP Integration** - Works directly with Claude Code

## Quick Start

### Prerequisites

- Go 1.21+
- SiliconFlow or Qwen API key

### Build

```bash
go build -o claude-context ./cmd/mcp
```

### Configuration

Set environment variables:

```bash
export EMBEDDING_API_KEY="your-api-key"
export EMBEDDING_PROVIDER="siliconflow"  # or "qwen"
export EMBEDDING_MODEL="BAAI/bge-large-zh-v1.5"
export DB_PATH="$HOME/.claude-context/db.duckdb"
```

### Register with Claude Code

```bash
claude mcp add claude-context \
  -e EMBEDDING_API_KEY=$EMBEDDING_API_KEY \
  -e EMBEDDING_PROVIDER=$EMBEDDING_PROVIDER \
  -- /path/to/claude-context
```

## Usage

Once registered, Claude Code can:

1. **Index a codebase** - Send `codebase://index` with codebase path
2. **Search** - Send `codebase://search?query=<text>` to find relevant code

## Architecture

- `cmd/mcp/` - MCP server entry point
- `pkg/context/` - Indexing and search logic
- `pkg/embedding/` - Embedding generation (SiliconFlow, Qwen)
- `pkg/vectordb/` - DuckDB integration
- `pkg/splitter/` - Code parsing and chunking
- `pkg/mcp/` - MCP protocol implementation

## Development

Run tests:

```bash
go test -v ./...
```

Build with debug logging:

```bash
LOG_LEVEL=debug ./claude-context
```

## License

MIT
