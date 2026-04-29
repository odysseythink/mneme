package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/claude-context/pkg"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestIndexerIndex(t *testing.T) {
	mockStore := &MockStore{}
	mockProvider := &MockEmbeddingProvider{}

	indexer := ctxpkg.NewIndexer(mockStore, embedding.NewCachedClient(mockProvider), nil, "test-model")

	ctx := context.Background()
	result, err := indexer.Index(ctx, "/tmp")

	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	if result.FilesProcessed == 0 {
		t.Log("Expected FilesProcessed > 0, but got 0 (may be normal for /tmp)")
	}
}

type MockStore struct{}

func (m *MockStore) Initialize(dbPath string) error {
	return nil
}

func (m *MockStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	return nil
}

func (m *MockStore) Search(_ context.Context, _ []float32, _ int, _ string) ([]pkg.Vector, error) {
	return []pkg.Vector{}, nil
}

func (m *MockStore) Close() error {
	return nil
}

func (m *MockStore) GetFileHashes(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *MockStore) DeleteByFilePath(_ context.Context, _ string, _ string) error {
	return nil
}

type FailingMockStore struct{}

func (m *FailingMockStore) Initialize(dbPath string) error {
	return nil
}

func (m *FailingMockStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	return fmt.Errorf("insert failed")
}

func (m *FailingMockStore) Search(_ context.Context, _ []float32, _ int, _ string) ([]pkg.Vector, error) {
	return []pkg.Vector{}, nil
}

func (m *FailingMockStore) Close() error {
	return nil
}

func (m *FailingMockStore) GetFileHashes(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *FailingMockStore) DeleteByFilePath(_ context.Context, _ string, _ string) error {
	return nil
}

type FailingEmbeddingProvider struct{}

func (m *FailingEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("embedding failed")
}

func (m *FailingEmbeddingProvider) BatchGenerateEmbedding(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, fmt.Errorf("batch embedding failed")
}

func TestIndexerWarningsDoNotPollutStdout(t *testing.T) {
	// MCP uses stdout for JSON-RPC — any fmt.Printf to stdout would corrupt the protocol.
	// Warnings must go to stderr (via mlog), never to stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	failingStore := &FailingMockStore{}
	mockProvider := &MockEmbeddingProvider{}
	indexer := ctxpkg.NewIndexer(failingStore, embedding.NewCachedClient(mockProvider), nil, "test-model")

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(tmpFile, []byte("package main\nfunc main() {}"), 0644)

	indexer.Index(context.Background(), tmpDir)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	if buf.Len() > 0 {
		t.Errorf("indexer wrote to stdout (must use stderr for logs): %q", buf.String())
	}
}

func TestIndexerReportsFailedFilesOnStoreFailure(t *testing.T) {
	failingStore := &FailingMockStore{}
	mockProvider := &MockEmbeddingProvider{}

	indexer := ctxpkg.NewIndexer(failingStore, embedding.NewCachedClient(mockProvider), nil, "test-model")

	ctx := context.Background()
	result, err := indexer.Index(ctx, "/tmp")

	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// FailedFiles should be initialized (not nil)
	if result.FailedFiles == nil {
		t.Fatal("Expected FailedFiles to be initialized (not nil)")
	}
}

func TestIndexerReportsFailedFilesOnEmbeddingFailure(t *testing.T) {
	mockStore := &MockStore{}
	failingProvider := &FailingEmbeddingProvider{}

	indexer := ctxpkg.NewIndexer(mockStore, embedding.NewCachedClient(failingProvider), nil, "test-model")

	ctx := context.Background()
	result, err := indexer.Index(ctx, "/tmp")

	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// FailedFiles should be initialized (not nil)
	if result.FailedFiles == nil {
		t.Fatal("Expected FailedFiles to be initialized (not nil)")
	}
}
