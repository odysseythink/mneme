package vectordb

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create DB directory: %w", err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open DuckDB: %w", err)
	}

	s.db = db

	if err := s.createTables(); err != nil {
		s.db.Close()
		return err
	}

	return nil
}

func (s *DuckDBStore) createTables() error {
	vectorsTable := `
	CREATE TABLE IF NOT EXISTS vectors (
		id INTEGER PRIMARY KEY,
		embedding FLOAT[],
		text VARCHAR,
		file_path VARCHAR,
		language VARCHAR,
		start_line INTEGER,
		end_line INTEGER,
		codebase_hash VARCHAR,
		indexed_at TIMESTAMP DEFAULT now()
	);
	CREATE SEQUENCE IF NOT EXISTS seq_vectors START 1;
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
	INSERT INTO vectors (id, embedding, text, file_path, language, start_line, end_line, codebase_hash)
	VALUES (nextval('seq_vectors'), ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		floatSliceToString(vec.Embedding),
		vec.Text,
		vec.FilePath,
		vec.Language,
		vec.StartLine,
		vec.EndLine,
		vec.CodebaseHash,
	)

	return err
}

func floatSliceToString(f []float32) string {
	parts := make([]string, len(f))
	for i, v := range f {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (s *DuckDBStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	query := `SELECT id, text, file_path, language, start_line, end_line, codebase_hash, indexed_at FROM vectors`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []pkg.Vector
	for rows.Next() {
		var vec pkg.Vector
		if err := rows.Scan(&vec.ID, &vec.Text, &vec.FilePath, &vec.Language, &vec.StartLine, &vec.EndLine, &vec.CodebaseHash, &vec.IndexedAt); err != nil {
			return nil, err
		}
		results = append(results, vec)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func (s *DuckDBStore) SearchWithEmbedding(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	query := `SELECT id, embedding, text, file_path, language, start_line, end_line, codebase_hash, indexed_at FROM vectors`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	type scoredVector struct {
		vec    pkg.Vector
		similarity float32
	}

	var scored []scoredVector
	for rows.Next() {
		var vec pkg.Vector
		var embInterface interface{}
		if err := rows.Scan(&vec.ID, &embInterface, &vec.Text, &vec.FilePath, &vec.Language, &vec.StartLine, &vec.EndLine, &vec.CodebaseHash, &vec.IndexedAt); err != nil {
			return nil, err
		}

		if embSlice, ok := embInterface.([]float32); ok {
			vec.Embedding = embSlice
			sim := cosineSimilarity(embedding, vec.Embedding)
			if sim >= 0.5 {
				scored = append(scored, scoredVector{vec: vec, similarity: sim})
			}
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].similarity > scored[j].similarity
	})

	results := make([]pkg.Vector, 0, topK)
	for i := 0; i < len(scored) && i < topK; i++ {
		results = append(results, scored[i].vec)
	}

	return results, nil
}

func (s *DuckDBStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
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
