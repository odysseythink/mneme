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

func (s *QdrantStore) Initialize(dbPath string) error { return nil }
func (s *QdrantStore) InsertVector(ctx context.Context, vec pkg.Vector) error { return nil }
func (s *QdrantStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	return nil, nil
}
func (s *QdrantStore) Close() error { return nil }
