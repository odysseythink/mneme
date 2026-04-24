# Multi-Backend Vector Database Design

**Date:** 2026-04-24  
**Status:** Approved  
**Scope:** Add Qdrant (HTTP) and chromem-go (embedded) as alternative vector store backends alongside the existing DuckDB implementation.

---

## Problem

The current DuckDB store uses a single file with an exclusive OS lock. When multiple claude-context MCP server instances point at the same database path, the second instance fails to open the file. Users running multiple Claude Code windows against the same codebase hit this limit.

Additionally, some users want a production-grade vector database (Qdrant) with proper concurrent access and richer search capabilities.

---

## Goals

- Add Qdrant support (connects to a running local or remote Qdrant service)
- Add chromem-go support (zero-dependency embedded pure-Go store)
- Keep DuckDB as the default; existing users need zero config changes
- Backend selection via a single `DB_BACKEND` environment variable

---

## Non-Goals

- Qdrant embedded mode (Qdrant has no Go embedded library)
- Migration tooling between backends
- Multi-backend fan-out (write to multiple backends simultaneously)

---

## Architecture

```
cmd/mcp/main.go
    └─ vectordb.NewStoreFromConfig(cfg)
           ├─ "duckdb"  → DuckDBStore  (pkg/vectordb/store.go, existing)
           ├─ "qdrant"  → QdrantStore  (pkg/vectordb/qdrant.go, new)
           └─ "chromem" → ChromemStore (pkg/vectordb/chromem.go, new)

pkg/vectordb/
    ├─ store.go      (DuckDB, existing, unchanged)
    ├─ qdrant.go     (new)
    ├─ chromem.go    (new)
    └─ factory.go    (new)
```

The `Store` interface in `pkg/types.go` is **not changed**. All three backends implement the same four methods: `Initialize`, `InsertVector`, `Search`, `Close`.

---

## Configuration

New environment variables added to `pkg/config/config.go`:

| Variable | Default | Backend | Description |
|----------|---------|---------|-------------|
| `DB_BACKEND` | `duckdb` | all | Backend selector: `duckdb`, `qdrant`, `chromem` |
| `DB_PATH` | `~/.claude-context/db.duckdb` | duckdb | DuckDB file path (unchanged) |
| `QDRANT_URL` | `http://localhost:6333` | qdrant | Qdrant service address |
| `QDRANT_COLLECTION` | `claude-context` | qdrant | Qdrant collection name |
| `CHROMEM_PATH` | `~/.claude-context/chromem` | chromem | chromem persistence directory |

Each backend has its own path/connection config. `DB_PATH` remains DuckDB-only to avoid semantic ambiguity (DuckDB uses a file; chromem uses a directory).

---

## Component Details

### Factory (`pkg/vectordb/factory.go`)

```go
func NewStoreFromConfig(cfg *config.Config) (pkg.Store, error) {
    switch cfg.DBBackend {
    case "qdrant":
        return NewQdrantStore(cfg.QdrantURL, cfg.QdrantCollection), nil
    case "chromem":
        return NewChromemStore(cfg.ChromemPath), nil
    case "duckdb", "":
        return NewStore(), nil
    default:
        return nil, fmt.Errorf("unknown DB_BACKEND: %q (valid: duckdb, qdrant, chromem)", cfg.DBBackend)
    }
}
```

`main.go` replaces the hardcoded `vectordb.NewStore()` call with `vectordb.NewStoreFromConfig(cfg)`.

### Qdrant Backend (`pkg/vectordb/qdrant.go`)

**Dependency:** `github.com/qdrant/go-client` (official gRPC SDK)

**Struct:**
```go
type QdrantStore struct {
    client     *qdrant.Client
    collection string
    vectorSize uint64  // inferred on first insert
}
```

**Initialize():** Connect to Qdrant via gRPC. Create the collection if it does not exist, using cosine distance. Vector size is deferred until the first `InsertVector` call (at which point the collection is (re)created with the correct dimension). If the collection already exists with the correct size, no-op.

**InsertVector():** Map `pkg.Vector` to a Qdrant Point:
- Vector: `vec.Embedding`
- Payload: `text`, `file_path`, `language`, `start_line`, `end_line`, `codebase_hash`, `indexed_at`
- ID: `uint64(vec.ID)` (Qdrant supports uint64 point IDs natively)

**Search():** Call Qdrant's Search API with `with_vectors: true` and cosine similarity. Filter results to score ≥ 0.5. Populate `vec.Embedding` from the returned point vector so that `searcher.go` can recompute cosine similarity correctly.

**Close():** Close the gRPC connection.

### chromem-go Backend (`pkg/vectordb/chromem.go`)

**Dependency:** `github.com/philippgille/chromem-go`

**Struct:**
```go
type ChromemStore struct {
    db         *chromem.DB
    collection *chromem.Collection
    path       string
}
```

**Initialize():** Open or create a persistent chromem DB at `path`. Load or create a collection named `"claude-context"`.

**InsertVector():** Convert `pkg.Vector` to a `chromem.Document`:
- `ID`: string form of `vec.ID` (or hash of content)
- `Embedding`: `vec.Embedding`
- `Metadata`: map of all other fields (text, file_path, language, start_line, end_line, codebase_hash)

**Search():** Call `collection.QueryEmbedding()` with the query embedding and `topK` limit. Filter results to cosine similarity ≥ 0.5. Reconstruct `[]pkg.Vector` from document metadata and populate `vec.Embedding` from the document's embedding so that `searcher.go` can recompute similarity correctly.

**Close():** No-op (chromem flushes on write; no persistent connection).

---

## Data Flow

```
index_codebase tool call
    → indexer.Index()
        → store.InsertVector()  ← dispatches to chosen backend
            DuckDB:  SQL INSERT with FLOAT[] string
            Qdrant:  gRPC UpsertPoints
            chromem: collection.AddDocument()

search_codebase tool call
    → searcher.Search()
        → embedding.GenerateEmbedding(query)
        → store.Search(queryEmbedding, topK)  ← dispatches to chosen backend
            DuckDB:  full table scan + in-process cosine similarity
            Qdrant:  native cosine search (server-side)
            chromem: in-memory cosine search
        → filter similarity ≥ 0.5 (done by each backend)
        → return SearchResult[]
```

---

## Error Handling

- `NewStoreFromConfig` with unknown backend returns an error immediately (fast fail at startup)
- Qdrant connection failure in `Initialize` surfaces as error → `mlog.Fatalf` in main
- chromem directory creation failure in `Initialize` surfaces as error → same
- Both new backends log warnings via `mlog.Warningf` for non-fatal per-operation errors (consistent with DuckDB behavior)

---

## Testing

### chromem tests (`tests/vectordb_chromem_test.go`)
No build tags. Run with standard `go test ./...`.
- `TestChromemStoreInitialize` — creates store in temp dir
- `TestChromemStoreInsertAndSearch` — insert vector, search with identical embedding, verify result
- `TestChromemStoreSearchFiltersLowSimilarity` — insert dissimilar vector, verify score < 0.5 excluded

### Qdrant tests (`tests/vectordb_qdrant_test.go`)
Build tag: `//go:build qdrant_integration`. Requires `QDRANT_URL` env var.
- `TestQdrantStoreInitialize`
- `TestQdrantStoreInsertAndSearch`
- `TestQdrantStoreSearchFiltersLowSimilarity`

### Factory tests (`tests/vectordb_factory_test.go`)
No build tags.
- `TestFactoryDefaultsToDuckDB` — empty `DB_BACKEND` returns DuckDB store
- `TestFactoryUnknownBackendReturnsError` — unknown value returns error
- `TestFactoryChromemBackend` — returns ChromemStore type

### Existing tests unchanged
All current DuckDB tests (`TestStoreInitialize`, `TestStoreInsertAndSearch`, `TestStoreSearchReturnsEmbeddings`) remain valid.

---

## Dependencies to Add

```
github.com/qdrant/go-client      (Qdrant gRPC SDK)
github.com/philippgille/chromem-go (embedded vector store)
```

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/config/config.go` | Add 4 new fields + env var reads |
| `pkg/vectordb/factory.go` | New: backend selector |
| `pkg/vectordb/qdrant.go` | New: Qdrant Store implementation |
| `pkg/vectordb/chromem.go` | New: chromem Store implementation |
| `cmd/mcp/main.go` | Replace `NewStore()` with `NewStoreFromConfig(cfg)` |
| `CLAUDE.md` | Update environment variables table |
| `tests/vectordb_factory_test.go` | New: factory tests |
| `tests/vectordb_chromem_test.go` | New: chromem integration tests |
| `tests/vectordb_qdrant_test.go` | New: Qdrant integration tests (build-tagged) |
