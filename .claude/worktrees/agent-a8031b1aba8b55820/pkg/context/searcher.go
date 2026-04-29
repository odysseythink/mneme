package context

import (
	"context"
	"fmt"
	"math"
	"sort"

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

func (s *Searcher) Search(ctx context.Context, query string, topK int, codebasePath string) ([]pkg.SearchResult, error) {
	if topK > 20 {
		topK = 20
	}
	if topK <= 0 {
		topK = 5
	}

	queryEmbedding, err := s.embedding.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	codebaseHash := ""
	if codebasePath != "" {
		codebaseHash = hashPath(codebasePath)
	}

	vectors, err := s.store.Search(ctx, queryEmbedding, 100, codebaseHash)
	if err != nil {
		return nil, fmt.Errorf("database search failed: %w", err)
	}

	type scoredResult struct {
		result     pkg.SearchResult
		similarity float32
	}

	var scored []scoredResult
	for _, vec := range vectors {
		similarity := cosineSimilarity(queryEmbedding, vec.Embedding)
		if similarity >= 0.5 {
			scored = append(scored, scoredResult{
				result: pkg.SearchResult{
					FileLocation: fmt.Sprintf("%s:%d:%d", vec.FilePath, vec.StartLine, vec.EndLine),
					Language:     vec.Language,
					Code:         vec.Text,
					Similarity:   similarity,
				},
				similarity: similarity,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].similarity > scored[j].similarity
	})

	results := make([]pkg.SearchResult, 0, topK)
	for i := 0; i < len(scored) && i < topK; i++ {
		results = append(results, scored[i].result)
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

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
