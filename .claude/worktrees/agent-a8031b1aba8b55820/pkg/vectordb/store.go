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
		file_hash VARCHAR DEFAULT '',
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

	// Migrate existing databases: add file_hash column if missing
	if _, err := s.db.Exec(`ALTER TABLE vectors ADD COLUMN IF NOT EXISTS file_hash VARCHAR DEFAULT ''`); err != nil {
		return fmt.Errorf("failed to migrate vectors schema: %w", err)
	}

	// Index for efficient per-codebase filtering
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_codebase ON vectors(codebase_hash)`); err != nil {
		return fmt.Errorf("failed to create codebase index: %w", err)
	}

	return nil
}

func (s *DuckDBStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	query := `
	INSERT INTO vectors (id, embedding, text, file_path, language, start_line, end_line, codebase_hash, file_hash)
	VALUES (nextval('seq_vectors'), ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		floatSliceToString(vec.Embedding),
		vec.Text,
		vec.FilePath,
		vec.Language,
		vec.StartLine,
		vec.EndLine,
		vec.CodebaseHash,
		vec.FileHash,
	)

	return err
}

func (s *DuckDBStore) GetFileHashes(ctx context.Context, codebaseHash string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT file_path, file_hash FROM vectors WHERE codebase_hash = ?`,
		codebaseHash,
	)
	if err != nil {
		return nil, fmt.Errorf("GetFileHashes query failed: %w", err)
	}
	defer rows.Close()

	hashes := make(map[string]string)
	for rows.Next() {
		var filePath, fileHash string
		if err := rows.Scan(&filePath, &fileHash); err != nil {
			return nil, err
		}
		hashes[filePath] = fileHash
	}
	return hashes, nil
}

func (s *DuckDBStore) DeleteByFilePath(ctx context.Context, filePath string, codebaseHash string) error {
	query := `DELETE FROM vectors WHERE file_path = ?`
	args := []interface{}{filePath}
	if codebaseHash != "" {
		query += ` AND codebase_hash = ?`
		args = append(args, codebaseHash)
	}
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func floatSliceToString(f []float32) string {
	parts := make([]string, len(f))
	for i, v := range f {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (s *DuckDBStore) Search(ctx context.Context, embedding []float32, topK int, codebaseHash string) ([]pkg.Vector, error) {
	query := `SELECT id, embedding, text, file_path, language, start_line, end_line, codebase_hash, indexed_at FROM vectors`
	var args []interface{}
	if codebaseHash != "" {
		query += ` WHERE codebase_hash = ?`
		args = append(args, codebaseHash)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []pkg.Vector
	for rows.Next() {
		var vec pkg.Vector
		var embInterface interface{}
		if err := rows.Scan(&vec.ID, &embInterface, &vec.Text, &vec.FilePath, &vec.Language, &vec.StartLine, &vec.EndLine, &vec.CodebaseHash, &vec.IndexedAt); err != nil {
			return nil, err
		}
		vec.Embedding = embeddingFromInterface(embInterface)
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

// embeddingFromInterface converts DuckDB's []interface{} FLOAT[] result to []float32.
// DuckDB returns float64 values even for FLOAT[] columns, so both types are handled.
func embeddingFromInterface(v interface{}) []float32 {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]float32, len(raw))
	for i, elem := range raw {
		switch f := elem.(type) {
		case float32:
			out[i] = f
		case float64:
			out[i] = float32(f)
		}
	}
	return out
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
