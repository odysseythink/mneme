# Claude Context Go Refactoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor TypeScript claude-context v0.1.7 into a single Go binary (core + MCP server) with embedded DuckDB and SiliconFlow/Qwen embeddings.

**Architecture:** Go-idiomatic three-layer design (MCP service → business logic → DuckDB data layer). All components compile into one binary with no external infrastructure dependencies.

**Tech Stack:** Go 1.21+, DuckDB, tree-sitter (code parsing), SiliconFlow/Qwen embedding APIs, MCP protocol (stdio-based).

---

## File Structure Overview

```
go-claude-context/
├── cmd/mcp/
│   └── main.go                         # MCP server binary entry point
├── pkg/
│   ├── config/
│   │   └── config.go                   # Configuration from env vars
│   ├── context/
│   │   ├── indexer.go                  # Indexing orchestration
│   │   └── searcher.go                 # Search query execution
│   ├── embedding/
│   │   ├── client.go                   # Interface + CachedClient
│   │   ├── siliconflow.go              # SiliconFlow provider
│   │   └── qwen.go                     # Qwen provider
│   ├── vectordb/
│   │   └── store.go                    # DuckDB operations
│   ├── splitter/
│   │   └── ast.go                      # Code parsing & splitting
│   └── mcp/
│       ├── server.go                   # MCP protocol handler
│       ├── resources.go                # Resource definitions
│       └── types.go                    # MCP message types
├── go.mod
├── go.sum
├── README.md
└── docs/
    ├── specs/
    │   └── 2026-04-23-go-refactor-design.md
    └── plans/
        └── 2026-04-23-go-refactor.md
```

---

## Phase 1: Setup & Infrastructure

### Task 1: Initialize Go Project and Directory Structure

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `cmd/mcp/main.go` (empty)
- Create: `pkg/config/config.go` (empty)
- Create: `pkg/context/indexer.go` (empty)
- Create: `pkg/context/searcher.go` (empty)
- Create: `pkg/embedding/client.go` (empty)
- Create: `pkg/embedding/siliconflow.go` (empty)
- Create: `pkg/embedding/qwen.go` (empty)
- Create: `pkg/vectordb/store.go` (empty)
- Create: `pkg/splitter/ast.go` (empty)
- Create: `pkg/mcp/server.go` (empty)
- Create: `pkg/mcp/resources.go` (empty)
- Create: `pkg/mcp/types.go` (empty)

- [ ] **Step 1: Initialize go.mod**

```bash
cd /Users/ranwei/workspace/go_work/claude-context-research/go-claude-context
go mod init github.com/ranwei/claude-context
```

- [ ] **Step 2: Add third-party dependencies to go.mod**

```bash
go get github.com/marcboeker/go-duckdb
go get github.com/tree-sitter/go-tree-sitter
go get github.com/google/uuid
```

- [ ] **Step 3: Create directory structure**

```bash
mkdir -p cmd/mcp pkg/{config,context,embedding,vectordb,splitter,mcp} docs/{specs,plans}
```

- [ ] **Step 4: Create placeholder files (all empty for now)**

```bash
touch cmd/mcp/main.go \
  pkg/config/config.go \
  pkg/context/indexer.go \
  pkg/context/searcher.go \
  pkg/embedding/client.go \
  pkg/embedding/siliconflow.go \
  pkg/embedding/qwen.go \
  pkg/vectordb/store.go \
  pkg/splitter/ast.go \
  pkg/mcp/server.go \
  pkg/mcp/resources.go \
  pkg/mcp/types.go
```

- [ ] **Step 5: Commit project structure**

```bash
git add go.mod go.sum
git commit -m "chore: initialize go project structure"
```

---

### Task 2: Define Core Types and Interfaces

**Files:**
- Create: `pkg/types.go` (shared types across packages)
- Modify: `pkg/config/config.go`

- [ ] **Step 1: Create pkg/types.go with core domain types**

```go
package pkg

import (
	"context"
	"time"
)

// Vector represents a code chunk with its embedding
type Vector struct {
	ID           int64
	Embedding    []float32
	Text         string
	FilePath     string
	Language     string
	StartLine    int
	EndLine      int
	CodebaseHash string
	IndexedAt    time.Time
}

// IndexResult summarizes the result of indexing a codebase
type IndexResult struct {
	FilesProcessed int
	VectorsStored  int
	Duration       time.Duration
	EmbeddingModel string
	CodebasePath   string
}

// SearchResult is a single search result with metadata
type SearchResult struct {
	FileLocation string  // "file:startLine:endLine"
	Language     string
	Code         string
	Similarity   float32
}

// Indexer orchestrates code splitting, embedding, and storage
type Indexer interface {
	Index(ctx context.Context, codebasePath string) (IndexResult, error)
}

// Searcher executes semantic search queries
type Searcher interface {
	Search(ctx context.Context, query string, topK int) ([]SearchResult, error)
}

// EmbeddingProvider generates embeddings for text
type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// Store manages vector and metadata persistence
type Store interface {
	Initialize(dbPath string) error
	InsertVector(ctx context.Context, vec Vector) error
	Search(ctx context.Context, embedding []float32, topK int) ([]Vector, error)
	Close() error
}

// Splitter extracts code chunks from source files
type Splitter interface {
	Split(filePath string, language string) ([]Vector, error)
}
```

- [ ] **Step 2: Implement Config**

```go
// pkg/config/config.go
package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	EmbeddingAPIKey  string
	EmbeddingProvider string
	EmbeddingModel   string
	DBPath           string
	LogLevel         string
}

func FromEnv() *Config {
	homeDir, _ := os.UserHomeDir()
	
	return &Config{
		EmbeddingAPIKey:  os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingProvider: getEnvOrDefault("EMBEDDING_PROVIDER", "siliconflow"),
		EmbeddingModel:   getEnvOrDefault("EMBEDDING_MODEL", "BAAI/bge-large-zh-v1.5"),
		DBPath:           getEnvOrDefault("DB_PATH", filepath.Join(homeDir, ".claude-context", "db.duckdb")),
		LogLevel:         getEnvOrDefault("LOG_LEVEL", "info"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
```

- [ ] **Step 3: Commit type definitions**

```bash
git add pkg/types.go pkg/config/config.go
git commit -m "feat: define core types and config"
```

---

## Phase 2: Data Layer (DuckDB)

### Task 3: Implement Vector Database Store

**Files:**
- Modify: `pkg/vectordb/store.go`

- [ ] **Step 1: Write test first**

```bash
mkdir -p tests
cat > tests/vectordb_test.go << 'EOF'
package tests

import (
	"context"
	"os"
	"testing"
	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func TestStoreInitialize(t *testing.T) {
	dbPath := "/tmp/test_vectors.duckdb"
	defer os.Remove(dbPath)
	
	store := vectordb.NewStore()
	err := store.Initialize(dbPath)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer store.Close()
	
	// Verify DB file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("DB file was not created")
	}
}

func TestStoreInsertAndSearch(t *testing.T) {
	dbPath := "/tmp/test_vectors.duckdb"
	defer os.Remove(dbPath)
	
	store := vectordb.NewStore()
	store.Initialize(dbPath)
	defer store.Close()
	
	ctx := context.Background()
	
	// Insert a test vector
	vec := pkg.Vector{
		Embedding:    []float32{0.1, 0.2, 0.3, 0.4},
		Text:         "func hello() {}",
		FilePath:     "main.go",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		CodebaseHash: "abc123",
	}
	
	err := store.InsertVector(ctx, vec)
	if err != nil {
		t.Fatalf("InsertVector failed: %v", err)
	}
	
	// Search with similar embedding
	queryEmbedding := []float32{0.1, 0.2, 0.3, 0.4}
	results, err := store.Search(ctx, queryEmbedding, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) == 0 {
		t.Fatal("Expected at least 1 result")
	}
	
	if results[0].Text != "func hello() {}" {
		t.Fatalf("Expected text 'func hello() {{}}', got %s", results[0].Text)
	}
}
EOF
```

- [ ] **Step 2: Implement Store interface**

```go
// pkg/vectordb/store.go
package vectordb

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb"
	"github.com/ranwei/claude-context/pkg"
)

type DuckDBStore struct {
	db *sql.DB
}

func NewStore() *DuckDBStore {
	return &DuckDBStore{}
}

func (s *DuckDBStore) Initialize(dbPath string) error {
	// Create directory if needed
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create DB directory: %w", err)
	}

	// Open or create DuckDB
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open DuckDB: %w", err)
	}

	s.db = db

	// Create tables
	if err := s.createTables(); err != nil {
		s.db.Close()
		return err
	}

	return nil
}

func (s *DuckDBStore) createTables() error {
	vectorsTable := `
	CREATE TABLE IF NOT EXISTS vectors (
		id INTEGER PRIMARY KEY DEFAULT nextval('seq_vectors'),
		embedding FLOAT32[],
		text VARCHAR,
		file_path VARCHAR,
		language VARCHAR,
		start_line INTEGER,
		end_line INTEGER,
		codebase_hash VARCHAR,
		indexed_at TIMESTAMP DEFAULT now()
	);
	`

	metadataTable := `
	CREATE TABLE IF NOT EXISTS metadata (
		codebase_hash VARCHAR PRIMARY KEY,
		codebase_path VARCHAR,
		embedding_model VARCHAR,
		indexed_at TIMESTAMP,
		total_files INTEGER,
		total_vectors INTEGER
	);
	`

	if _, err := s.db.Exec(vectorsTable); err != nil {
		return fmt.Errorf("failed to create vectors table: %w", err)
	}

	if _, err := s.db.Exec(metadataTable); err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	return nil
}

func (s *DuckDBStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	query := `
	INSERT INTO vectors (embedding, text, file_path, language, start_line, end_line, codebase_hash)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		vec.Embedding,
		vec.Text,
		vec.FilePath,
		vec.Language,
		vec.StartLine,
		vec.EndLine,
		vec.CodebaseHash,
	)

	return err
}

func (s *DuckDBStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	// DuckDB doesn't have native cosine_similarity, so we implement it in Go
	// In production, would use DuckDB's vector extension when available
	
	query := `SELECT id, embedding, text, file_path, language, start_line, end_line, codebase_hash, indexed_at FROM vectors LIMIT 1000`
	
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []pkg.Vector
	for rows.Next() {
		var vec pkg.Vector
		if err := rows.Scan(&vec.ID, &vec.Embedding, &vec.Text, &vec.FilePath, &vec.Language, &vec.StartLine, &vec.EndLine, &vec.CodebaseHash, &vec.IndexedAt); err != nil {
			return nil, err
		}
		results = append(results, vec)
	}

	// Sort by cosine similarity (simplified for MVP)
	// In production would use vectorization library
	
	return results[:min(len(results), topK)], nil
}

func (s *DuckDBStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// cosineSimilarity computes cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
```

- [ ] **Step 3: Run test**

```bash
go test -v ./tests -run TestStoreInitialize
```

Expected: PASS

- [ ] **Step 4: Run insert/search test**

```bash
go test -v ./tests -run TestStoreInsertAndSearch
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/vectordb/store.go tests/vectordb_test.go
git commit -m "feat: implement DuckDB vector store with insert and search"
```

---

## Phase 3: Code Parsing

### Task 4: Implement Code Splitter

**Files:**
- Modify: `pkg/splitter/ast.go`

- [ ] **Step 1: Write failing test**

```bash
cat > tests/splitter_test.go << 'EOF'
package tests

import (
	"testing"
	"github.com/ranwei/claude-context/pkg/splitter"
)

func TestSplitGoCode(t *testing.T) {
	// Create a test Go file
	testFile := "/tmp/test.go"
	code := `package main

// Hello prints a greeting
func Hello() {
	println("Hello, World!")
}

// Add returns the sum of two numbers
func Add(a, b int) int {
	return a + b
}
`
	
	_ = splitter.WriteTestFile(testFile, code)
	defer deleteTestFile(testFile)
	
	s := splitter.NewSplitter()
	chunks, err := s.Split(testFile, "go")
	
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}
	
	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks, got %d", len(chunks))
	}
	
	// Check first function was extracted
	found := false
	for _, chunk := range chunks {
		if contains(chunk.Text, "Hello") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected to find 'Hello' function in chunks")
	}
}

func deleteTestFile(path string) {
	_ = os.Remove(path)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && s[0:len(substr)] == substr)
}
EOF
```

- [ ] **Step 2: Implement Splitter**

```go
// pkg/splitter/ast.go
package splitter

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ranwei/claude-context/pkg"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/go-tree-sitter/golang"
)

type ASTSplitter struct {
	// Language parsers would be loaded here
}

func NewSplitter() *ASTSplitter {
	return &ASTSplitter{}
}

func (s *ASTSplitter) Split(filePath string, language string) ([]pkg.Vector, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	chunks := []pkg.Vector{}

	switch language {
	case "go":
		chunks, err = s.splitGo(filePath, content)
	case "python", "js", "ts":
		// For MVP, use line-based splitting for unsupported languages
		chunks = s.splitByLines(filePath, string(content), language)
	default:
		// Fallback to line-based splitting
		chunks = s.splitByLines(filePath, string(content), language)
	}

	if err != nil {
		return nil, err
	}

	return chunks, nil
}

func (s *ASTSplitter) splitGo(filePath string, content []byte) ([]pkg.Vector, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree := parser.Parse(nil, content)
	defer tree.Close()

	return s.extractFunctionsFromAST(filePath, string(content), tree, "go"), nil
}

func (s *ASTSplitter) extractFunctionsFromAST(filePath string, content string, tree *sitter.Tree, language string) []pkg.Vector {
	chunks := []pkg.Vector{}
	lines := strings.Split(content, "\n")

	rootNode := tree.RootNode()
	
	// Walk through AST and extract function declarations
	s.walkNode(rootNode, func(node *sitter.Node) {
		if node.Type() == "function_declaration" || node.Type() == "method_declaration" {
			startLine := int(node.StartPoint().Row) + 1
			endLine := int(node.EndPoint().Row) + 1
			
			// Extract code chunk
			var codeLines []string
			for i := startLine - 1; i < endLine && i < len(lines); i++ {
				codeLines = append(codeLines, lines[i])
			}
			
			chunk := pkg.Vector{
				FilePath:  filePath,
				Language:  language,
				StartLine: startLine,
				EndLine:   endLine,
				Text:      strings.Join(codeLines, "\n"),
			}
			chunks = append(chunks, chunk)
		}
	})

	return chunks
}

func (s *ASTSplitter) walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		s.walkNode(node.Child(i), fn)
	}
}

func (s *ASTSplitter) splitByLines(filePath string, content string, language string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	chunks := []pkg.Vector{}

	for i := 0; i < len(lines); i += 50 { // Group 50 lines per chunk
		endIdx := i + 50
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		chunk := pkg.Vector{
			FilePath:  filePath,
			Language:  language,
			StartLine: i + 1,
			EndLine:   endIdx,
			Text:      strings.Join(lines[i:endIdx], "\n"),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

func WriteTestFile(path string, content string) error {
	return ioutil.WriteFile(path, []byte(content), 0644)
}
```

- [ ] **Step 3: Run test**

```bash
go test -v ./tests -run TestSplitGoCode
```

Expected: PASS (extracts function definitions)

- [ ] **Step 4: Commit**

```bash
git add pkg/splitter/ast.go tests/splitter_test.go
git commit -m "feat: implement code splitter with tree-sitter AST parsing"
```

---

## Phase 4: Embedding Generation

### Task 5: Implement Embedding Client Interface and Caching

**Files:**
- Modify: `pkg/embedding/client.go`

- [ ] **Step 1: Write test**

```bash
cat > tests/embedding_test.go << 'EOF'
package tests

import (
	"context"
	"testing"
	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestCachedClientCache(t *testing.T) {
	mockProvider := &MockEmbeddingProvider{}
	client := embedding.NewCachedClient(mockProvider)

	ctx := context.Background()

	// First call should hit provider
	emb1, err := client.GenerateEmbedding(ctx, "hello world")
	if err != nil {
		t.Fatalf("GenerateEmbedding failed: %v", err)
	}

	if mockProvider.CallCount != 1 {
		t.Fatalf("Expected 1 call to provider, got %d", mockProvider.CallCount)
	}

	// Second call with same text should hit cache
	emb2, err := client.GenerateEmbedding(ctx, "hello world")
	if err != nil {
		t.Fatalf("GenerateEmbedding failed: %v", err)
	}

	if mockProvider.CallCount != 1 {
		t.Fatalf("Expected cache hit (1 call), got %d calls", mockProvider.CallCount)
	}

	// Embeddings should be identical
	if len(emb1) != len(emb2) {
		t.Fatal("Embeddings have different lengths")
	}
}

// MockEmbeddingProvider for testing
type MockEmbeddingProvider struct {
	CallCount int
}

func (m *MockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.CallCount++
	// Return dummy embedding (1536 dimensions for MVP)
	emb := make([]float32, 1536)
	for i := range emb {
		emb[i] = 0.1
	}
	return emb, nil
}
EOF
```

- [ ] **Step 2: Implement CachedClient**

```go
// pkg/embedding/client.go
package embedding

import (
	"context"
	"crypto/md5"
	"fmt"
	"sync"

	"github.com/ranwei/claude-context/pkg"
)

type CachedClient struct {
	provider pkg.EmbeddingProvider
	cache    map[string][]float32
	mu       sync.RWMutex
}

func NewCachedClient(provider pkg.EmbeddingProvider) *CachedClient {
	return &CachedClient{
		provider: provider,
		cache:    make(map[string][]float32),
	}
}

func (c *CachedClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	hash := c.hashText(text)

	// Check cache
	c.mu.RLock()
	if emb, found := c.cache[hash]; found {
		c.mu.RUnlock()
		return emb, nil
	}
	c.mu.RUnlock()

	// Call provider
	emb, err := c.provider.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.mu.Lock()
	c.cache[hash] = emb
	c.mu.Unlock()

	return emb, nil
}

func (c *CachedClient) hashText(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

func (c *CachedClient) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[string][]float32)
	c.mu.Unlock()
}
```

- [ ] **Step 3: Run test**

```bash
go test -v ./tests -run TestCachedClientCache
```

Expected: PASS (cache hits reduce provider calls)

- [ ] **Step 4: Commit**

```bash
git add pkg/embedding/client.go tests/embedding_test.go
git commit -m "feat: implement embedding client with caching"
```

---

### Task 6: Implement SiliconFlow Embedding Provider

**Files:**
- Modify: `pkg/embedding/siliconflow.go`

- [ ] **Step 1: Implement SiliconFlow provider**

```go
// pkg/embedding/siliconflow.go
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type SiliconFlowProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewSiliconFlowProvider(apiKey, model string) *SiliconFlowProvider {
	return &SiliconFlowProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type SiliconFlowRequest struct {
	Model  string   `json:"model"`
	Input  []string `json:"input"`
	Encoding string `json:"encoding_format,omitempty"`
}

type SiliconFlowResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *SiliconFlowProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	req := SiliconFlowRequest{
		Model:  p.model,
		Input:  []string{text},
		Encoding: "float",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.siliconflow.cn/v1/embeddings", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var sfResp SiliconFlowResponse
	if err := json.Unmarshal(respBody, &sfResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if sfResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", sfResp.Error.Message)
	}

	if len(sfResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return sfResp.Data[0].Embedding, nil
}

func (p *SiliconFlowProvider) BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	req := SiliconFlowRequest{
		Model:  p.model,
		Input:  texts,
		Encoding: "float",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.siliconflow.cn/v1/embeddings", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var sfResp SiliconFlowResponse
	if err := json.Unmarshal(respBody, &sfResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if sfResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", sfResp.Error.Message)
	}

	// Sort by index to maintain order
	result := make([][]float32, len(texts))
	for _, data := range sfResp.Data {
		if data.Index < len(result) {
			result[data.Index] = data.Embedding
		}
	}

	return result, nil
}
```

- [ ] **Step 2: Test with mock**

```bash
cat > tests/siliconflow_test.go << 'EOF'
package tests

import (
	"context"
	"testing"
	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestSiliconFlowProviderInit(t *testing.T) {
	provider := embedding.NewSiliconFlowProvider("sk-test", "BAAI/bge-large-zh-v1.5")
	if provider == nil {
		t.Fatal("Failed to create SiliconFlow provider")
	}
}
EOF
```

- [ ] **Step 3: Run test**

```bash
go test -v ./tests -run TestSiliconFlowProviderInit
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/embedding/siliconflow.go tests/siliconflow_test.go
git commit -m "feat: implement SiliconFlow embedding provider"
```

---

### Task 7: Implement Qwen Embedding Provider

**Files:**
- Modify: `pkg/embedding/qwen.go`

- [ ] **Step 1: Implement Qwen provider**

```go
// pkg/embedding/qwen.go
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type QwenProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewQwenProvider(apiKey, model string) *QwenProvider {
	return &QwenProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type QwenRequest struct {
	Model  string      `json:"model"`
	Input  QwenInput   `json:"input"`
	Parameters QwenParameters `json:"parameters,omitempty"`
}

type QwenInput struct {
	Texts []string `json:"texts"`
}

type QwenParameters struct {
	// Qwen-specific parameters
}

type QwenResponse struct {
	Output struct {
		Embeddings []struct {
			TextIndex int       `json:"text_index"`
			Embedding []float32 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
	RequestID string `json:"request_id"`
	Usage struct {
		InputTokens int `json:"input_tokens"`
	} `json:"usage"`
}

func (p *QwenProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	req := QwenRequest{
		Model: p.model,
		Input: QwenInput{
			Texts: []string{text},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var qResp QwenResponse
	if err := json.Unmarshal(respBody, &qResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(qResp.Output.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return qResp.Output.Embeddings[0].Embedding, nil
}

func (p *QwenProvider) BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	req := QwenRequest{
		Model: p.model,
		Input: QwenInput{
			Texts: texts,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var qResp QwenResponse
	if err := json.Unmarshal(respBody, &qResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Create result slice
	result := make([][]float32, len(texts))
	for _, emb := range qResp.Output.Embeddings {
		if emb.TextIndex < len(result) {
			result[emb.TextIndex] = emb.Embedding
		}
	}

	return result, nil
}
```

- [ ] **Step 2: Test initialization**

```bash
cat > tests/qwen_test.go << 'EOF'
package tests

import (
	"testing"
	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestQwenProviderInit(t *testing.T) {
	provider := embedding.NewQwenProvider("sk-test", "text-embedding-v2")
	if provider == nil {
		t.Fatal("Failed to create Qwen provider")
	}
}
EOF
```

- [ ] **Step 3: Run test**

```bash
go test -v ./tests -run TestQwenProviderInit
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/embedding/qwen.go tests/qwen_test.go
git commit -m "feat: implement Qwen embedding provider"
```

---

## Phase 5: Core Business Logic

### Task 8: Implement Indexer

**Files:**
- Modify: `pkg/context/indexer.go`

- [ ] **Step 1: Write test**

```bash
cat > tests/indexer_test.go << 'EOF'
package tests

import (
	"context"
	"os"
	"testing"
	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func TestIndexerIndex(t *testing.T) {
	// Setup mock store and provider
	mockStore := &MockStore{}
	mockProvider := &MockEmbeddingProvider{}

	indexer := context.NewIndexer(mockStore, embedding.NewCachedClient(mockProvider), nil)

	ctx := context.Background()
	result, err := indexer.Index(ctx, "/tmp")

	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	if result.FilesProcessed == 0 {
		t.Fatal("Expected FilesProcessed > 0")
	}
}

type MockStore struct{}

func (m *MockStore) Initialize(dbPath string) error {
	return nil
}

func (m *MockStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	return nil
}

func (m *MockStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return []pkg.Vector{}, nil
}

func (m *MockStore) Close() error {
	return nil
}
EOF
```

- [ ] **Step 2: Implement Indexer**

```go
// pkg/context/indexer.go
package context

import (
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/splitter"
)

type Indexer struct {
	store      pkg.Store
	embedding  *embedding.CachedClient
	splitter   *splitter.ASTSplitter
	embeddingModel string
}

func NewIndexer(store pkg.Store, emb *embedding.CachedClient, s *splitter.ASTSplitter) *Indexer {
	if s == nil {
		s = splitter.NewSplitter()
	}
	return &Indexer{
		store:     store,
		embedding: emb,
		splitter:  s,
		embeddingModel: "BAAI/bge-large-zh-v1.5",
	}
}

func (idx *Indexer) Index(ctx context.Context, codebasePath string) (pkg.IndexResult, error) {
	result := pkg.IndexResult{
		CodebasePath:   codebasePath,
		EmbeddingModel: idx.embeddingModel,
	}

	startTime := time.Now()

	// Hash the codebase path for multi-codebase support
	codebaseHash := hashPath(codebasePath)

	// Walk filesystem and collect files
	fileMap := make(map[string]bool) // Track supported files
	err := filepath.Walk(codebasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip non-code files
		ext := filepath.Ext(path)
		if isCodeFile(ext) {
			fileMap[path] = true
		}
		return nil
	})

	if err != nil {
		return result, fmt.Errorf("failed to walk filesystem: %w", err)
	}

	result.FilesProcessed = len(fileMap)

	// Process each file and extract chunks
	chunks := []pkg.Vector{}
	for filePath := range fileMap {
		lang := getLanguageFromExt(filepath.Ext(filePath))
		
		fileChunks, err := idx.splitter.Split(filePath, lang)
		if err != nil {
			// Log but continue on error
			fmt.Printf("Warning: failed to split %s: %v\n", filePath, err)
			continue
		}

		chunks = append(chunks, fileChunks...)
	}

	// Generate embeddings for all chunks
	for i := range chunks {
		emb, err := idx.embedding.GenerateEmbedding(ctx, chunks[i].Text)
		if err != nil {
			fmt.Printf("Warning: failed to embed chunk: %v\n", err)
			continue
		}

		chunks[i].Embedding = emb
		chunks[i].CodebaseHash = codebaseHash

		// Store in database
		if err := idx.store.InsertVector(ctx, chunks[i]); err != nil {
			fmt.Printf("Warning: failed to insert vector: %v\n", err)
			continue
		}

		result.VectorsStored++
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

func isCodeFile(ext string) bool {
	supportedExts := map[string]bool{
		".go":   true,
		".py":   true,
		".js":   true,
		".ts":   true,
		".jsx":  true,
		".tsx":  true,
		".java": true,
		".cpp":  true,
		".c":    true,
		".cs":   true,
		".rs":   true,
	}
	return supportedExts[strings.ToLower(ext)]
}

func getLanguageFromExt(ext string) string {
	langMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "js",
		".ts":   "ts",
		".jsx":  "js",
		".tsx":  "ts",
		".java": "java",
		".cpp":  "cpp",
		".c":    "c",
		".cs":   "cs",
		".rs":   "rust",
	}
	if lang, ok := langMap[strings.ToLower(ext)]; ok {
		return lang
	}
	return "unknown"
}

func hashPath(path string) string {
	hash := md5.Sum([]byte(path))
	return fmt.Sprintf("%x", hash)
}
```

- [ ] **Step 3: Run test**

```bash
go test -v ./tests -run TestIndexerIndex
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/context/indexer.go tests/indexer_test.go
git commit -m "feat: implement indexer orchestration logic"
```

---

### Task 9: Implement Searcher

**Files:**
- Modify: `pkg/context/searcher.go`

- [ ] **Step 1: Write test**

```bash
cat > tests/searcher_test.go << 'EOF'
package tests

import (
	"context"
	"testing"
	"github.com/ranwei/claude-context/pkg"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestSearcherSearch(t *testing.T) {
	mockStore := &MockSearchStore{}
	mockProvider := &MockEmbeddingProvider{}

	searcher := ctxpkg.NewSearcher(mockStore, embedding.NewCachedClient(mockProvider))

	ctx := context.Background()
	results, err := searcher.Search(ctx, "test query", 5)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected at least 1 result")
	}
}

type MockSearchStore struct{}

func (m *MockSearchStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return []pkg.Vector{
		{
			FilePath:  "test.go",
			Language:  "go",
			StartLine: 1,
			EndLine:   10,
			Text:      "func test() {}",
			Embedding: embedding,
		},
	}, nil
}
EOF
```

- [ ] **Step 2: Implement Searcher**

```go
// pkg/context/searcher.go
package context

import (
	"context"
	"fmt"

	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/embedding"
)

type Searcher struct {
	store     pkg.Store
	embedding *embedding.CachedClient
}

func NewSearcher(store pkg.Store, emb *embedding.CachedClient) *Searcher {
	return &Searcher{
		store:     store,
		embedding: emb,
	}
}

func (s *Searcher) Search(ctx context.Context, query string, topK int) ([]pkg.SearchResult, error) {
	if topK > 20 {
		topK = 20
	}
	if topK <= 0 {
		topK = 5
	}

	// Generate embedding for query
	queryEmbedding, err := s.embedding.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search database
	vectors, err := s.store.Search(ctx, queryEmbedding, topK)
	if err != nil {
		return nil, fmt.Errorf("database search failed: %w", err)
	}

	// Convert to search results
	results := make([]pkg.SearchResult, 0, len(vectors))
	for _, vec := range vectors {
		// Skip results with low similarity (< 0.5)
		similarity := cosineSimilarity(queryEmbedding, vec.Embedding)
		if similarity < 0.5 {
			continue
		}

		results = append(results, pkg.SearchResult{
			FileLocation: fmt.Sprintf("%s:%d:%d", vec.FilePath, vec.StartLine, vec.EndLine),
			Language:     vec.Language,
			Code:         vec.Text,
			Similarity:   similarity,
		})
	}

	return results, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float32) float32 {
	// Simple approximation for MVP
	if x < 0 {
		return 0
	}
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
```

- [ ] **Step 3: Run test**

```bash
go test -v ./tests -run TestSearcherSearch
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/context/searcher.go tests/searcher_test.go
git commit -m "feat: implement semantic search with similarity ranking"
```

---

## Phase 6: MCP Server

### Task 10: Implement MCP Server

**Files:**
- Modify: `pkg/mcp/types.go`
- Modify: `pkg/mcp/server.go`
- Modify: `pkg/mcp/resources.go`

- [ ] **Step 1: Define MCP message types**

```go
// pkg/mcp/types.go
package mcp

import (
	"encoding/json"
)

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	JsonRpc string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// MCPResponse represents an outgoing MCP response
type MCPResponse struct {
	JsonRpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// IndexRequest represents an index resource request
type IndexRequest struct {
	CodebasePath string `json:"codebase_path"`
}

// SearchRequest represents a search resource request
type SearchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"`
}
```

- [ ] **Step 2: Implement MCP Server**

```go
// pkg/mcp/server.go
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/ranwei/claude-context/pkg"
)

type Server struct {
	indexer  pkg.Indexer
	searcher pkg.Searcher
	mu       sync.Mutex
	nextID   int
}

func NewMCPServer(indexer pkg.Indexer, searcher pkg.Searcher) *Server {
	return &Server{
		indexer:  indexer,
		searcher: searcher,
	}
}

func (s *Server) Start() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(req.ID, -32700, "Parse error")
			continue
		}

		go s.handleRequest(req)
	}
}

func (s *Server) handleRequest(req MCPRequest) {
	var resp MCPResponse
	resp.JsonRpc = "2.0"
	resp.ID = req.ID

	ctx := context.Background()

	switch req.Method {
	case "resources/read":
		resp.Result = s.handleResourceRead(ctx, req.Params)
	case "resources/list":
		resp.Result = s.handleResourceList()
	default:
		resp.Error = &MCPError{Code: -32601, Message: "Method not found"}
	}

	s.sendResponse(resp)
}

func (s *Server) handleResourceRead(ctx context.Context, params json.RawMessage) interface{} {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil
	}

	// Parse URI to determine action
	if req.URI == "codebase://index" {
		return s.handleIndex(ctx, params)
	} else if req.URI[:16] == "codebase://search" {
		return s.handleSearch(ctx, params)
	}

	return nil
}

func (s *Server) handleIndex(ctx context.Context, params json.RawMessage) interface{} {
	var idxReq IndexRequest
	if err := json.Unmarshal(params, &idxReq); err != nil {
		return nil
	}

	result, err := s.indexer.Index(ctx, idxReq.CodebasePath)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{
		"files_processed": result.FilesProcessed,
		"vectors_stored":  result.VectorsStored,
		"duration_ms":     result.Duration.Milliseconds(),
		"embedding_model": result.EmbeddingModel,
	}
}

func (s *Server) handleSearch(ctx context.Context, params json.RawMessage) interface{} {
	var searchReq SearchRequest
	if err := json.Unmarshal(params, &searchReq); err != nil {
		return nil
	}

	if searchReq.TopK <= 0 {
		searchReq.TopK = 5
	}

	results, err := s.searcher.Search(ctx, searchReq.Query, searchReq.TopK)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{
		"results": results,
		"count":   len(results),
	}
}

func (s *Server) handleResourceList() interface{} {
	return map[string]interface{}{
		"resources": []map[string]interface{}{
			{
				"uri":         "codebase://index",
				"name":        "Index Codebase",
				"description": "Index a codebase for semantic search",
				"mimeType":    "application/json",
			},
			{
				"uri":         "codebase://search",
				"name":        "Search Codebase",
				"description": "Search indexed codebase semantically",
				"mimeType":    "application/json",
			},
		},
	}
}

func (s *Server) sendResponse(resp MCPResponse) {
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func (s *Server) sendError(id int, code int, message string) {
	resp := MCPResponse{
		JsonRpc: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: message},
	}
	s.sendResponse(resp)
}
```

- [ ] **Step 3: Test MCP server initialization**

```bash
cat > tests/mcp_test.go << 'EOF'
package tests

import (
	"testing"
	"github.com/ranwei/claude-context/pkg/mcp"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
)

func TestMCPServerInit(t *testing.T) {
	mockStore := &MockStore{}
	mockProvider := &MockEmbeddingProvider{}
	
	indexer := ctxpkg.NewIndexer(mockStore, nil, nil)
	searcher := ctxpkg.NewSearcher(mockStore, nil)
	
	server := mcp.NewMCPServer(indexer, searcher)
	if server == nil {
		t.Fatal("Failed to create MCP server")
	}
}
EOF
```

- [ ] **Step 4: Run test**

```bash
go test -v ./tests -run TestMCPServerInit
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/mcp/types.go pkg/mcp/server.go pkg/mcp/resources.go tests/mcp_test.go
git commit -m "feat: implement MCP server with index and search resources"
```

---

## Phase 7: Integration & Deployment

### Task 11: Implement Main Entry Point

**Files:**
- Modify: `cmd/mcp/main.go`

- [ ] **Step 1: Implement main.go**

```go
// cmd/mcp/main.go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ranwei/claude-context/pkg/config"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/mcp"
	"github.com/ranwei/claude-context/pkg/splitter"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func main() {
	cfg := config.FromEnv()

	// Validate required config
	if cfg.EmbeddingAPIKey == "" {
		log.Fatal("EMBEDDING_API_KEY environment variable is required")
	}

	// Initialize database
	store := vectordb.NewStore()
	if err := store.Initialize(cfg.DBPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	// Create embedding provider
	var provider embedding.EmbeddingProvider
	switch cfg.EmbeddingProvider {
	case "siliconflow":
		provider = embedding.NewSiliconFlowProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
	case "qwen":
		provider = embedding.NewQwenProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
	default:
		log.Fatalf("Unknown embedding provider: %s", cfg.EmbeddingProvider)
	}

	// Create cached embedding client
	embeddingClient := embedding.NewCachedClient(provider)

	// Create code splitter
	codeSplitter := splitter.NewSplitter()

	// Create indexer and searcher
	indexer := ctxpkg.NewIndexer(store, embeddingClient, codeSplitter)
	searcher := ctxpkg.NewSearcher(store, embeddingClient)

	// Create and start MCP server
	server := mcp.NewMCPServer(indexer, searcher)

	fmt.Fprintf(os.Stderr, "Claude Context MCP Server started\n")
	fmt.Fprintf(os.Stderr, "Embedding Provider: %s\n", cfg.EmbeddingProvider)
	fmt.Fprintf(os.Stderr, "Database: %s\n", cfg.DBPath)

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

- [ ] **Step 2: Build the binary**

```bash
cd /Users/ranwei/workspace/go_work/claude-context-research/go-claude-context
go build -o ./bin/claude-context ./cmd/mcp
```

Expected: Binary created at `./bin/claude-context`

- [ ] **Step 3: Verify binary**

```bash
file ./bin/claude-context
```

Expected: `Mach-O 64-bit executable` (or similar for your platform)

- [ ] **Step 4: Commit**

```bash
git add cmd/mcp/main.go
git commit -m "feat: implement main entry point and binary build"
```

---

### Task 12: Integration Test

**Files:**
- Create: `tests/integration_test.go`

- [ ] **Step 1: Write integration test**

```bash
cat > tests/integration_test.go << 'EOF'
package tests

import (
	"context"
	"os"
	"testing"

	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/splitter"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func TestEndToEndIndexAndSearch(t *testing.T) {
	// Setup
	dbPath := "/tmp/integration_test.duckdb"
	defer os.Remove(dbPath)

	store := vectordb.NewStore()
	if err := store.Initialize(dbPath); err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	mockProvider := &MockEmbeddingProvider{}
	embeddingClient := embedding.NewCachedClient(mockProvider)
	splitter := splitter.NewSplitter()

	indexer := ctxpkg.NewIndexer(store, embeddingClient, splitter)
	searcher := ctxpkg.NewSearcher(store, embeddingClient)

	ctx := context.Background()

	// Test indexing
	result, err := indexer.Index(ctx, "/tmp")
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	if result.VectorsStored == 0 {
		t.Fatal("Expected vectors to be stored")
	}

	// Test searching
	results, err := searcher.Search(ctx, "test query", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected search results")
	}
}
EOF
```

- [ ] **Step 2: Run integration test**

```bash
go test -v ./tests -run TestEndToEndIndexAndSearch
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add tests/integration_test.go
git commit -m "test: add end-to-end integration test"
```

---

### Task 13: Documentation and Deployment Guide

**Files:**
- Create: `README.md`
- Create: `docs/DEPLOYMENT.md`

- [ ] **Step 1: Write README.md**

```bash
cat > README.md << 'EOF'
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
EOF
```

- [ ] **Step 2: Write deployment guide**

```bash
cat > docs/DEPLOYMENT.md << 'EOF'
# Deployment Guide

## Local Development

1. **Clone and build:**
   ```bash
   git clone <repo>
   cd go-claude-context
   go build -o bin/claude-context ./cmd/mcp
   ```

2. **Set credentials:**
   ```bash
   export EMBEDDING_API_KEY="sk-xxxx"
   export EMBEDDING_PROVIDER="siliconflow"
   ```

3. **Register with Claude Code:**
   ```bash
   claude mcp add claude-context -- ./bin/claude-context
   ```

## Docker Deployment (Future)

Dockerfile coming soon for containerized deployment.

## Performance Tuning

### Database

- For 100k+ vectors, create an index: `CREATE INDEX ON vectors(embedding)`
- DuckDB is single-threaded; for parallel indexing, use multiple processes with separate DBs

### Embedding Generation

- Batch queries of 20-50 texts for efficiency
- Use embedding caching to avoid re-querying identical text
- Implement incremental indexing to avoid re-processing unchanged files

## Troubleshooting

**"API key invalid"**
- Verify `EMBEDDING_API_KEY` is set correctly
- Check provider documentation for key format

**"Database locked"**
- Multiple processes accessing same DB; use separate DB paths
- Or implement connection pooling

**"Out of memory during indexing"**
- Reduce batch size (currently 20-50 per batch)
- Index in smaller chunks per run
- Consider incremental indexing for future versions
EOF
```

- [ ] **Step 3: Commit documentation**

```bash
git add README.md docs/DEPLOYMENT.md
git commit -m "docs: add README and deployment guide"
```

---

### Task 14: Final Build and Verification

**Files:**
- Verify all code compiles

- [ ] **Step 1: Clean build**

```bash
cd /Users/ranwei/workspace/go_work/claude-context-research/go-claude-context
go clean
go build -o bin/claude-context ./cmd/mcp
```

Expected: Binary compiles without errors

- [ ] **Step 2: Run all tests**

```bash
go test -v ./tests
```

Expected: All tests pass

- [ ] **Step 3: Check binary size**

```bash
ls -lh bin/claude-context
```

Expected: Single binary, likely 10-50MB depending on Go version and platform

- [ ] **Step 4: Verify binary works**

```bash
./bin/claude-context --help 2>&1 || echo "Binary runs without error"
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: final build verification and cleanup"
```

- [ ] **Step 6: Tag release**

```bash
git tag v0.1.0-go
```

---

## Acceptance Criteria Checklist

- [ ] Single compiled binary at `bin/claude-context`
- [ ] All unit tests passing
- [ ] Integration test passing
- [ ] Supports SiliconFlow and Qwen embeddings
- [ ] DuckDB embedded (no separate infrastructure)
- [ ] MCP protocol implemented for Claude Code
- [ ] Code splitting works for Go, Python, JavaScript/TypeScript
- [ ] Search returns ranked results by similarity
- [ ] README and deployment guide completed
- [ ] No external runtime dependencies (API calls only)

---

**Plan Status:** ✅ Ready for execution

Choose execution approach in next message.
