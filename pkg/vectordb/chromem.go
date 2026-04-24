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

func (s *ChromemStore) Initialize(dbPath string) error { return nil }
func (s *ChromemStore) InsertVector(ctx context.Context, vec pkg.Vector) error { return nil }
func (s *ChromemStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return nil, nil
}
func (s *ChromemStore) Close() error { return nil }
