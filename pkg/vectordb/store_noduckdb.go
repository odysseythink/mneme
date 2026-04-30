//go:build !duckdb

package vectordb

import (
	"context"
	"errors"

	"github.com/ranwei/mneme/pkg"
)

type DuckDBStore struct{}

// Ensure DuckDBStore satisfies pkg.Store at compile time even in the no-duckdb stub.
var _ pkg.Store = (*DuckDBStore)(nil)

func NewStore() *DuckDBStore { return nil }

func (d *DuckDBStore) Initialize(dbPath string) error {
	return errors.New("duckdb backend not compiled; rebuild with -tags duckdb")
}

func (d *DuckDBStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	return errors.New("duckdb backend not compiled; rebuild with -tags duckdb")
}

func (d *DuckDBStore) Search(ctx context.Context, embedding []float32, topK int, codebaseHash string) ([]pkg.Vector, error) {
	return nil, errors.New("duckdb backend not compiled; rebuild with -tags duckdb")
}

func (d *DuckDBStore) Close() error { return nil }

func (d *DuckDBStore) GetFileHashes(ctx context.Context, codebaseHash string) (map[string]string, error) {
	return nil, errors.New("duckdb backend not compiled; rebuild with -tags duckdb")
}

func (d *DuckDBStore) DeleteByFilePath(ctx context.Context, filePath string, codebaseHash string) error {
	return errors.New("duckdb backend not compiled; rebuild with -tags duckdb")
}
