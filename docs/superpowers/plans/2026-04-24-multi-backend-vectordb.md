# Multi-Backend Vector Database Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Qdrant (HTTP/gRPC) and chromem-go (embedded) as selectable vector store backends alongside the existing DuckDB implementation.

**Architecture:** A factory function `vectordb.NewStoreFromConfig(cfg)` selects the backend based on `DB_BACKEND` env var. Each backend implements the existing `pkg.Store` interface unchanged. DuckDB remains the default.

**Tech Stack:** `github.com/philippgille/chromem-go`, `github.com/qdrant/go-client/qdrant`, existing DuckDB + Go module.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `pkg/config/config.go` | Modify | Add 4 new env var fields |
| `pkg/vectordb/factory.go` | Create | Backend selector, `NewStoreFromConfig` |
| `pkg/vectordb/chromem.go` | Create | chromem-go Store implementation |
| `pkg/vectordb/qdrant.go` | Create | Qdrant Store implementation |
| `cmd/mcp/main.go` | Modify | Use factory instead of `NewStore()` |
| `CLAUDE.md` | Modify | Update env var table |
| `tests/vectordb_factory_test.go` | Create | Factory selection tests |
| `tests/vectordb_chromem_test.go` | Create | chromem integration tests |
| `tests/vectordb_qdrant_test.go` | Create | Qdrant integration tests (build-tagged) |

---

## Task 1: Install Dependencies and Update Config

**Files:**
- Modify: `pkg/config/config.go`
- Create: `tests/config_backend_test.go`

- [ ] **Step 1: Install dependencies**

```bash
go get github.com/philippgille/chromem-go
go get github.com/qdrant/go-client/qdrant
```

Expected: both added to `go.mod` and `go.sum`.

- [ ] **Step 2: Write failing config test**

Create `tests/config_backend_test.go`:

```go
package tests

import (
	"os"
	"testing"

	"github.com/ranwei/claude-context/pkg/config"
)

func TestConfigBackendDefaults(t *testing.T) {
	os.Unsetenv("DB_BACKEND")
	os.Unsetenv("QDRANT_URL")
	os.Unsetenv("QDRANT_COLLECTION")
	os.Unsetenv("CHROMEM_PATH")

	cfg := config.FromEnv()

	if cfg.DBBackend != "duckdb" {
		t.Errorf("expected DBBackend='duckdb', got %q", cfg.DBBackend)
	}
	if cfg.QdrantURL != "http://localhost:6333" {
		t.Errorf("expected QdrantURL='http://localhost:6333', got %q", cfg.QdrantURL)
	}
	if cfg.QdrantCollection != "claude-context" {
		t.Errorf("expected QdrantCollection='claude-context', got %q", cfg.QdrantCollection)
	}
}

func TestConfigBackendFromEnv(t *testing.T) {
	os.Setenv("DB_BACKEND", "qdrant")
	os.Setenv("QDRANT_URL", "http://myserver:6333")
	os.Setenv("QDRANT_COLLECTION", "my-collection")
	os.Setenv("CHROMEM_PATH", "/tmp/chromem")
	defer func() {
		os.Unsetenv("DB_BACKEND")
		os.Unsetenv("QDRANT_URL")
		os.Unsetenv("QDRANT_COLLECTION")
		os.Unsetenv("CHROMEM_PATH")
	}()

	cfg := config.FromEnv()

	if cfg.DBBackend != "qdrant" {
		t.Errorf("expected DBBackend='qdrant', got %q", cfg.DBBackend)
	}
	if cfg.QdrantURL != "http://myserver:6333" {
		t.Errorf("expected QdrantURL='http://myserver:6333', got %q", cfg.QdrantURL)
	}
	if cfg.ChromemPath != "/tmp/chromem" {
		t.Errorf("expected ChromemPath='/tmp/chromem', got %q", cfg.ChromemPath)
	}
}
```

- [ ] **Step 3: Run test to confirm it fails**

```bash
go test -v ./tests -run TestConfigBackend
```

Expected: compilation error — `cfg.DBBackend` undefined.

- [ ] **Step 4: Add fields to Config**

Edit `pkg/config/config.go` — replace the entire file:

```go
package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	EmbeddingAPIKey   string
	EmbeddingProvider string
	EmbeddingModel    string
	DBPath            string
	LogLevel          string
	DBBackend         string
	QdrantURL         string
	QdrantCollection  string
	ChromemPath       string
}

func FromEnv() *Config {
	homeDir, _ := os.UserHomeDir()

	return &Config{
		EmbeddingAPIKey:   os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingProvider: getEnvOrDefault("EMBEDDING_PROVIDER", "siliconflow"),
		EmbeddingModel:    getEnvOrDefault("EMBEDDING_MODEL", "BAAI/bge-large-zh-v1.5"),
		DBPath:            getEnvOrDefault("DB_PATH", filepath.Join(homeDir, ".claude-context", "db.duckdb")),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
		DBBackend:         getEnvOrDefault("DB_BACKEND", "duckdb"),
		QdrantURL:         getEnvOrDefault("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection:  getEnvOrDefault("QDRANT_COLLECTION", "claude-context"),
		ChromemPath:       getEnvOrDefault("CHROMEM_PATH", filepath.Join(homeDir, ".claude-context", "chromem")),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
```

- [ ] **Step 5: Run test to confirm it passes**

```bash
go test -v ./tests -run TestConfigBackend
go build ./...
```

Expected: both pass with no errors.

- [ ] **Step 6: Commit**

```bash
git add pkg/config/config.go tests/config_backend_test.go
git commit -m "feat: add DB_BACKEND, QDRANT_URL, QDRANT_COLLECTION, CHROMEM_PATH config fields"
```

---

## Task 2: Factory Function

**Files:**
- Create: `pkg/vectordb/factory.go`
- Create: `tests/vectordb_factory_test.go`

- [ ] **Step 1: Write failing factory tests**

Create `tests/vectordb_factory_test.go`:

```go
package tests

import (
	"testing"

	"github.com/ranwei/claude-context/pkg/config"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func TestFactoryDefaultsToDuckDB(t *testing.T) {
	cfg := &config.Config{DBBackend: ""}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if _, ok := store.(*vectordb.DuckDBStore); !ok {
		t.Errorf("expected *vectordb.DuckDBStore, got %T", store)
	}
}

func TestFactoryExplicitDuckDB(t *testing.T) {
	cfg := &config.Config{DBBackend: "duckdb"}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*vectordb.DuckDBStore); !ok {
		t.Errorf("expected *vectordb.DuckDBStore, got %T", store)
	}
}

func TestFactoryChromemBackend(t *testing.T) {
	cfg := &config.Config{DBBackend: "chromem", ChromemPath: t.TempDir()}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*vectordb.ChromemStore); !ok {
		t.Errorf("expected *vectordb.ChromemStore, got %T", store)
	}
}

func TestFactoryQdrantBackend(t *testing.T) {
	cfg := &config.Config{
		DBBackend:        "qdrant",
		QdrantURL:        "http://localhost:6333",
		QdrantCollection: "test",
	}
	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*vectordb.QdrantStore); !ok {
		t.Errorf("expected *vectordb.QdrantStore, got %T", store)
	}
}

func TestFactoryUnknownBackendReturnsError(t *testing.T) {
	cfg := &config.Config{DBBackend: "postgres"}
	_, err := vectordb.NewStoreFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for unknown backend, got nil")
	}
}
```

- [ ] **Step 2: Run to confirm compilation fails**

```bash
go test -v ./tests -run TestFactory
```

Expected: compilation error — `vectordb.NewStoreFromConfig` undefined.

- [ ] **Step 3: Export DuckDBStore type**

`DuckDBStore` in `pkg/vectordb/store.go` is already exported (capital D). Confirm with:

```bash
grep "type DuckDBStore" pkg/vectordb/store.go
```

Expected output: `type DuckDBStore struct {`

If it shows `type duckDBStore` (lowercase), rename it to `DuckDBStore` throughout `store.go` and `tests/vectordb_test.go`.

- [ ] **Step 4: Create factory with stub types**

Create `pkg/vectordb/factory.go`:

```go
package vectordb

import (
	"fmt"

	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/config"
)

// NewStoreFromConfig selects and constructs the vector store backend from config.
// Valid DB_BACKEND values: "duckdb" (default), "qdrant", "chromem".
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

At this point `NewQdrantStore` and `NewChromemStore` don't exist yet. Add stub files so the package compiles:

Create `pkg/vectordb/chromem.go`:

```go
package vectordb

import (
	"context"

	"github.com/ranwei/claude-context/pkg"
)

type ChromemStore struct {
	path string
}

func NewChromemStore(path string) *ChromemStore {
	return &ChromemStore{path: path}
}

func (s *ChromemStore) Initialize(dbPath string) error           { return nil }
func (s *ChromemStore) InsertVector(ctx context.Context, vec pkg.Vector) error { return nil }
func (s *ChromemStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return nil, nil
}
func (s *ChromemStore) Close() error { return nil }
```

Create `pkg/vectordb/qdrant.go`:

```go
package vectordb

import (
	"context"

	"github.com/ranwei/claude-context/pkg"
)

type QdrantStore struct {
	url        string
	collection string
}

func NewQdrantStore(url, collection string) *QdrantStore {
	return &QdrantStore{url: url, collection: collection}
}

func (s *QdrantStore) Initialize(dbPath string) error           { return nil }
func (s *QdrantStore) InsertVector(ctx context.Context, vec pkg.Vector) error { return nil }
func (s *QdrantStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return nil, nil
}
func (s *QdrantStore) Close() error { return nil }
```

- [ ] **Step 5: Run factory tests to confirm they pass**

```bash
go test -v ./tests -run TestFactory
go build ./...
```

Expected: all 5 factory tests PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/vectordb/factory.go pkg/vectordb/chromem.go pkg/vectordb/qdrant.go tests/vectordb_factory_test.go
git commit -m "feat: add store factory and stub chromem/qdrant backends"
```

---

## Task 3: chromem-go Backend

**Files:**
- Modify: `pkg/vectordb/chromem.go` (replace stubs with real implementation)
- Create: `tests/vectordb_chromem_test.go`

- [ ] **Step 1: Write failing chromem tests**

Create `tests/vectordb_chromem_test.go`:

```go
package tests

import (
	"context"
	"testing"

	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func TestChromemStoreInitialize(t *testing.T) {
	store := vectordb.NewChromemStore(t.TempDir())
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()
}

func TestChromemStoreInsertAndSearch(t *testing.T) {
	store := vectordb.NewChromemStore(t.TempDir())
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	emb := []float32{0.1, 0.2, 0.3, 0.4}

	vec := pkg.Vector{
		ID:           1,
		Embedding:    emb,
		Text:         "func hello() {}",
		FilePath:     "main.go",
		Language:     "go",
		StartLine:    1,
		EndLine:      3,
		CodebaseHash: "abc123",
	}

	if err := store.InsertVector(ctx, vec); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	results, err := store.Search(ctx, emb, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}
	if results[0].Text != "func hello() {}" {
		t.Errorf("expected text 'func hello() {}', got %q", results[0].Text)
	}
	if results[0].Embedding == nil {
		t.Error("expected Embedding to be populated, got nil")
	}
}

func TestChromemStoreSearchFiltersLowSimilarity(t *testing.T) {
	store := vectordb.NewChromemStore(t.TempDir())
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Insert a vector pointing in one direction
	if err := store.InsertVector(ctx, pkg.Vector{
		ID:        1,
		Embedding: []float32{1.0, 0.0, 0.0, 0.0},
		Text:      "unrelated code",
		FilePath:  "other.go",
		Language:  "go",
		StartLine: 1,
		EndLine:   1,
	}); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	// Search with an orthogonal query (similarity = 0)
	results, err := store.Search(ctx, []float32{0.0, 1.0, 0.0, 0.0}, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (orthogonal vectors filtered out), got %d", len(results))
	}
}
```

- [ ] **Step 2: Run to confirm tests fail with stub**

```bash
go test -v ./tests -run TestChromemStore
```

Expected: `TestChromemStoreInsertAndSearch` FAILS — "expected at least 1 result, got 0" (stub returns nil).

- [ ] **Step 3: Implement chromem-go backend**

Replace `pkg/vectordb/chromem.go` entirely:

```go
package vectordb

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	chromem "github.com/philippgille/chromem-go"
	"github.com/ranwei/claude-context/pkg"
)

type ChromemStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	path       string
}

func NewChromemStore(path string) *ChromemStore {
	return &ChromemStore{path: path}
}

func (s *ChromemStore) Initialize(_ string) error {
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return fmt.Errorf("failed to create chromem dir: %w", err)
	}

	db, err := chromem.NewPersistentDB(s.path, false)
	if err != nil {
		return fmt.Errorf("failed to open chromem db: %w", err)
	}
	s.db = db

	col, err := db.GetOrCreateCollection("vectors", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get/create collection: %w", err)
	}
	s.collection = col
	return nil
}

func (s *ChromemStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	id := fmt.Sprintf("%s:%d:%d", vec.FilePath, vec.StartLine, vec.EndLine)
	doc := chromem.Document{
		ID:        id,
		Embedding: vec.Embedding,
		Metadata: map[string]string{
			"text":          vec.Text,
			"file_path":     vec.FilePath,
			"language":      vec.Language,
			"start_line":    strconv.Itoa(vec.StartLine),
			"end_line":      strconv.Itoa(vec.EndLine),
			"codebase_hash": vec.CodebaseHash,
			"indexed_at":    vec.IndexedAt.Format(time.RFC3339),
		},
	}
	return s.collection.AddDocument(ctx, doc)
}

func (s *ChromemStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	results, err := s.collection.QueryEmbedding(ctx, embedding, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem search failed: %w", err)
	}

	var vectors []pkg.Vector
	for _, r := range results {
		if r.Similarity < 0.5 {
			continue
		}
		startLine, _ := strconv.Atoi(r.Metadata["start_line"])
		endLine, _ := strconv.Atoi(r.Metadata["end_line"])
		indexedAt, _ := time.Parse(time.RFC3339, r.Metadata["indexed_at"])

		vectors = append(vectors, pkg.Vector{
			Embedding:    r.Embedding,
			Text:         r.Metadata["text"],
			FilePath:     r.Metadata["file_path"],
			Language:     r.Metadata["language"],
			StartLine:    startLine,
			EndLine:      endLine,
			CodebaseHash: r.Metadata["codebase_hash"],
			IndexedAt:    indexedAt,
		})
	}
	return vectors, nil
}

func (s *ChromemStore) Close() error {
	return nil // chromem flushes synchronously on each write
}
```

- [ ] **Step 4: Run chromem tests**

```bash
go test -v ./tests -run TestChromemStore
go build ./...
```

Expected: all 3 chromem tests PASS.

- [ ] **Step 5: Run full suite to check no regressions**

```bash
go test -v ./...
```

Expected: all existing tests still PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/vectordb/chromem.go tests/vectordb_chromem_test.go
git commit -m "feat: implement chromem-go vector store backend"
```

---

## Task 4: Qdrant Backend

**Files:**
- Modify: `pkg/vectordb/qdrant.go` (replace stubs with real implementation)
- Create: `tests/vectordb_qdrant_test.go`

- [ ] **Step 1: Write Qdrant integration tests (build-tagged)**

Create `tests/vectordb_qdrant_test.go`:

```go
//go:build qdrant_integration

package tests

import (
	"context"
	"os"
	"testing"

	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func qdrantURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("QDRANT_URL")
	if url == "" {
		t.Skip("QDRANT_URL not set; skipping Qdrant integration tests")
	}
	return url
}

func TestQdrantStoreInitialize(t *testing.T) {
	store := vectordb.NewQdrantStore(qdrantURL(t), "test-init")
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()
}

func TestQdrantStoreInsertAndSearch(t *testing.T) {
	store := vectordb.NewQdrantStore(qdrantURL(t), "test-search")
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	emb := []float32{0.1, 0.2, 0.3, 0.4}

	vec := pkg.Vector{
		ID:           1,
		Embedding:    emb,
		Text:         "func hello() {}",
		FilePath:     "main.go",
		Language:     "go",
		StartLine:    1,
		EndLine:      3,
		CodebaseHash: "abc123",
	}

	if err := store.InsertVector(ctx, vec); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	results, err := store.Search(ctx, emb, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}
	if results[0].Text != "func hello() {}" {
		t.Errorf("expected text 'func hello() {}', got %q", results[0].Text)
	}
	if results[0].Embedding == nil {
		t.Error("expected Embedding to be populated, got nil")
	}
}

func TestQdrantStoreSearchFiltersLowSimilarity(t *testing.T) {
	store := vectordb.NewQdrantStore(qdrantURL(t), "test-filter")
	if err := store.Initialize(""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	if err := store.InsertVector(ctx, pkg.Vector{
		ID:        1,
		Embedding: []float32{1.0, 0.0, 0.0, 0.0},
		Text:      "unrelated",
		FilePath:  "other.go",
		Language:  "go",
		StartLine: 1,
		EndLine:   1,
	}); err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}

	results, err := store.Search(ctx, []float32{0.0, 1.0, 0.0, 0.0}, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (orthogonal vectors filtered out), got %d", len(results))
	}
}
```

- [ ] **Step 2: Confirm build-tagged tests are excluded from normal run**

```bash
go test -v ./tests -run TestQdrant
```

Expected: `[no test files]` or `testing: warning: no tests to run` — the build tag excludes them.

- [ ] **Step 3: Implement Qdrant backend**

Replace `pkg/vectordb/qdrant.go` entirely:

```go
package vectordb

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/odysseythink/mlog"
	qdrantpb "github.com/qdrant/go-client/qdrant"
	"github.com/ranwei/claude-context/pkg"
)

type QdrantStore struct {
	client     *qdrantpb.Client
	collection string
	rawURL     string
	mu         sync.Mutex
	vectorSize uint64
	ready      bool // true once collection confirmed/created
}

func NewQdrantStore(rawURL, collection string) *QdrantStore {
	return &QdrantStore{rawURL: rawURL, collection: collection}
}

func (s *QdrantStore) Initialize(_ string) error {
	host, port, err := parseQdrantURL(s.rawURL)
	if err != nil {
		return fmt.Errorf("invalid QDRANT_URL: %w", err)
	}

	client, err := qdrantpb.NewClient(&qdrantpb.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Qdrant at %s: %w", s.rawURL, err)
	}
	s.client = client

	// Check if collection already exists
	ctx := context.Background()
	collections, err := client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list Qdrant collections: %w", err)
	}
	for _, c := range collections {
		if c == s.collection {
			s.ready = true
			mlog.Infof("Qdrant: using existing collection %q", s.collection)
			return nil
		}
	}
	// Collection will be created on first InsertVector when vector size is known
	mlog.Infof("Qdrant: collection %q will be created on first insert", s.collection)
	return nil
}

func (s *QdrantStore) ensureCollection(ctx context.Context, vectorSize uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ready {
		return nil
	}
	_, err := s.client.CreateCollection(ctx, &qdrantpb.CreateCollection{
		CollectionName: s.collection,
		VectorsConfig: qdrantpb.NewVectorsConfig(&qdrantpb.VectorParams{
			Size:     vectorSize,
			Distance: qdrantpb.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to create Qdrant collection %q: %w", s.collection, err)
	}
	s.vectorSize = vectorSize
	s.ready = true
	return nil
}

func (s *QdrantStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	if err := s.ensureCollection(ctx, uint64(len(vec.Embedding))); err != nil {
		return err
	}

	id := pointID(vec)
	_, err := s.client.Upsert(ctx, &qdrantpb.UpsertPoints{
		CollectionName: s.collection,
		Points: []*qdrantpb.PointStruct{
			{
				Id:      qdrantpb.NewIDNum(id),
				Vectors: qdrantpb.NewVectors(vec.Embedding...),
				Payload: qdrantpb.NewValueMap(map[string]any{
					"text":          vec.Text,
					"file_path":     vec.FilePath,
					"language":      vec.Language,
					"start_line":    int64(vec.StartLine),
					"end_line":      int64(vec.EndLine),
					"codebase_hash": vec.CodebaseHash,
					"indexed_at":    vec.IndexedAt.Format(time.RFC3339),
				}),
			},
		},
	})
	return err
}

func (s *QdrantStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	if !s.ready {
		return nil, nil // no vectors yet
	}

	limit := uint64(topK)
	threshold := float32(0.5)
	withVectors := true
	results, err := s.client.Query(ctx, &qdrantpb.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrantpb.NewQuery(embedding...),
		Limit:          &limit,
		WithVectors:    &qdrantpb.WithVectorsSelector{SelectorOptions: &qdrantpb.WithVectorsSelector_Enable{Enable: withVectors}},
		ScoreThreshold: &threshold,
	})
	if err != nil {
		return nil, fmt.Errorf("Qdrant search failed: %w", err)
	}

	vectors := make([]pkg.Vector, 0, len(results))
	for _, r := range results {
		payload := r.GetPayload()
		startLine := int(payload["start_line"].GetIntegerValue())
		endLine := int(payload["end_line"].GetIntegerValue())
		indexedAt, _ := time.Parse(time.RFC3339, payload["indexed_at"].GetStringValue())

		var emb []float32
		if v := r.GetVectors(); v != nil {
			emb = v.GetVector().GetData()
		}

		vectors = append(vectors, pkg.Vector{
			Embedding:    emb,
			Text:         payload["text"].GetStringValue(),
			FilePath:     payload["file_path"].GetStringValue(),
			Language:     payload["language"].GetStringValue(),
			StartLine:    startLine,
			EndLine:      endLine,
			CodebaseHash: payload["codebase_hash"].GetStringValue(),
			IndexedAt:    indexedAt,
		})
	}
	return vectors, nil
}

func (s *QdrantStore) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// parseQdrantURL extracts host and gRPC port from an HTTP URL.
// Qdrant HTTP port is 6333; gRPC port is 6334. If port 6333 is given, it is
// automatically converted to 6334.
func parseQdrantURL(rawURL string) (host string, port int, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, err
	}
	host = u.Hostname()
	portStr := u.Port()
	if portStr == "" {
		return host, 6334, nil
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port in URL: %w", err)
	}
	if p == 6333 {
		p = 6334 // auto-convert HTTP port to gRPC port
	}
	return host, p, nil
}

// pointID generates a stable uint64 ID from vector coordinates.
func pointID(vec pkg.Vector) uint64 {
	s := fmt.Sprintf("%s:%d:%d", vec.FilePath, vec.StartLine, vec.EndLine)
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
```

- [ ] **Step 4: Build to confirm compilation**

```bash
go build ./...
```

If any Qdrant API method names differ from the installed version, the compiler will tell you. Fix any method name mismatches by checking the installed package docs:

```bash
go doc github.com/qdrant/go-client/qdrant Client
```

- [ ] **Step 5: Run normal test suite (Qdrant tests are skipped)**

```bash
go test -v ./...
```

Expected: all existing tests PASS; Qdrant integration tests not run.

- [ ] **Step 6: Run Qdrant integration tests (requires running Qdrant)**

Only run if you have Qdrant available locally:

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant &
QDRANT_URL=http://localhost:6333 go test -v -tags qdrant_integration ./tests -run TestQdrantStore
```

Expected: all 3 Qdrant tests PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/vectordb/qdrant.go tests/vectordb_qdrant_test.go
git commit -m "feat: implement Qdrant vector store backend"
```

---

## Task 5: Wire Factory into main.go and Update Docs

**Files:**
- Modify: `cmd/mcp/main.go`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update main.go**

In `cmd/mcp/main.go`, replace:

```go
store := vectordb.NewStore()
if err := store.Initialize(cfg.DBPath); err != nil {
    mlog.Fatalf("Failed to initialize database: %v", err)
}
```

with:

```go
store, err := vectordb.NewStoreFromConfig(cfg)
if err != nil {
    mlog.Fatalf("Invalid DB_BACKEND: %v", err)
}
if err := store.Initialize(cfg.DBPath); err != nil {
    mlog.Fatalf("Failed to initialize database: %v", err)
}
```

Also update the startup log line from:

```go
mlog.Infof("Claude Context MCP Server started (provider=%s, db=%s)", cfg.EmbeddingProvider, cfg.DBPath)
```

to:

```go
mlog.Infof("Claude Context MCP Server started (provider=%s, backend=%s)", cfg.EmbeddingProvider, cfg.DBBackend)
```

- [ ] **Step 2: Build to confirm main compiles**

```bash
go build -o ./bin/claude-context ./cmd/mcp
```

Expected: builds without error.

- [ ] **Step 3: Update CLAUDE.md environment variables table**

In `CLAUDE.md`, find the `## Required Environment Variables` table and add the new rows:

```markdown
| `DB_BACKEND` | `duckdb` | Vector store backend: `duckdb`, `qdrant`, `chromem` |
| `QDRANT_URL` | `http://localhost:6333` | Qdrant service URL (HTTP port; gRPC 6334 auto-used) |
| `QDRANT_COLLECTION` | `claude-context` | Qdrant collection name |
| `CHROMEM_PATH` | `~/.claude-context/chromem` | chromem-go persistence directory |
```

- [ ] **Step 4: Run full test suite**

```bash
go test -v ./...
```

Expected: all tests PASS, no regressions.

- [ ] **Step 5: Commit**

```bash
git add cmd/mcp/main.go CLAUDE.md
git commit -m "feat: wire store factory into main, update CLAUDE.md with new env vars"
```

---

## Verification Checklist

After all tasks are complete:

- [ ] `go build ./...` succeeds
- [ ] `go test -v ./...` all green
- [ ] `DB_BACKEND=chromem` selects ChromemStore (verified by factory test)
- [ ] `DB_BACKEND=unknown` returns error at startup
- [ ] `DB_BACKEND` unset still uses DuckDB (backward compatible)
- [ ] Qdrant integration tests pass when `QDRANT_URL` is set and Qdrant is running
- [ ] chromem tests pass without any external services
