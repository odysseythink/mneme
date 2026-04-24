package vectordb

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	chromem "github.com/philippgille/chromem-go"
	"github.com/odysseythink/mlog"
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
	// chromem-go requires nResults <= number of documents in collection
	count := s.collection.Count()
	nResults := topK
	if nResults > count {
		nResults = count
	}
	if nResults == 0 {
		return []pkg.Vector{}, nil
	}

	results, err := s.collection.QueryEmbedding(ctx, embedding, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem search failed: %w", err)
	}

	var vectors []pkg.Vector
	for _, r := range results {
		if r.Similarity < 0.5 {
			continue
		}
		startLine, err := strconv.Atoi(r.Metadata["start_line"])
		if err != nil {
			mlog.Warningf("chromem: failed to parse start_line %q: %v", r.Metadata["start_line"], err)
		}
		endLine, err := strconv.Atoi(r.Metadata["end_line"])
		if err != nil {
			mlog.Warningf("chromem: failed to parse end_line %q: %v", r.Metadata["end_line"], err)
		}
		indexedAt, err := time.Parse(time.RFC3339, r.Metadata["indexed_at"])
		if err != nil {
			mlog.Warningf("chromem: failed to parse indexed_at %q: %v", r.Metadata["indexed_at"], err)
		}

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
