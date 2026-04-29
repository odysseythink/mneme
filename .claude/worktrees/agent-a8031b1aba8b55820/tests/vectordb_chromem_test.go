package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

	results, err := store.Search(ctx, emb, 5, "")
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

// TestChromemHashesPerCodebase verifies that each codebase gets its own hash file
// so a single file never grows unbounded across all projects.
func TestChromemHashesPerCodebase(t *testing.T) {
	dir := t.TempDir()
	store := vectordb.NewChromemStore(dir)
	if err := store.Initialize(""); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	emb := []float32{0.1, 0.2, 0.3, 0.4}

	// Index two files in two different codebases.
	for _, tc := range []struct{ cbHash, filePath string }{
		{"cb_aaa", "/proj-a/main.go"},
		{"cb_bbb", "/proj-b/main.go"},
	} {
		if err := store.InsertVector(ctx, pkg.Vector{
			Embedding:    emb,
			Text:         "func A() {}",
			FilePath:     tc.filePath,
			Language:     "go",
			StartLine:    1,
			EndLine:      1,
			CodebaseHash: tc.cbHash,
			FileHash:     "hash_" + tc.cbHash,
		}); err != nil {
			t.Fatalf("InsertVector %s: %v", tc.cbHash, err)
		}
	}

	// Expect two separate files, not one monolithic file_hashes.json.
	if _, err := os.Stat(filepath.Join(dir, "file_hashes.json")); err == nil {
		t.Error("legacy file_hashes.json should not be created by new code")
	}
	for _, cbHash := range []string{"cb_aaa", "cb_bbb"} {
		p := filepath.Join(dir, "hashes_"+cbHash+".json")
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected per-codebase file %s, not found", p)
		}
	}

	// GetFileHashes for one codebase must not return files from the other.
	hashes, err := store.GetFileHashes(ctx, "cb_aaa")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := hashes["/proj-b/main.go"]; ok {
		t.Error("GetFileHashes('cb_aaa') returned a file from cb_bbb")
	}
	if hashes["/proj-a/main.go"] == "" {
		t.Error("GetFileHashes('cb_aaa') did not return /proj-a/main.go")
	}
}

// TestChromemMigratesLegacyHashes verifies that the old monolithic file_hashes.json
// is automatically split into per-codebase files on Initialize.
func TestChromemMigratesLegacyHashes(t *testing.T) {
	dir := t.TempDir()

	// Write a legacy file_hashes.json with entries for two codebases.
	type legacyEntry struct {
		FileHash     string `json:"file_hash"`
		CodebaseHash string `json:"codebase_hash"`
	}
	legacy := map[string]legacyEntry{
		"/proj-a/foo.go": {FileHash: "hash_a", CodebaseHash: "cb_aaa"},
		"/proj-b/bar.go": {FileHash: "hash_b", CodebaseHash: "cb_bbb"},
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "file_hashes.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	store := vectordb.NewChromemStore(dir)
	if err := store.Initialize(""); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Legacy file must be removed after migration.
	if _, err := os.Stat(filepath.Join(dir, "file_hashes.json")); err == nil {
		t.Error("legacy file_hashes.json should have been removed after migration")
	}

	// Per-codebase files must exist.
	for _, cbHash := range []string{"cb_aaa", "cb_bbb"} {
		p := filepath.Join(dir, "hashes_"+cbHash+".json")
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected migrated file %s, not found", p)
		}
	}

	// GetFileHashes must return the migrated data.
	ctx := context.Background()
	hashes, err := store.GetFileHashes(ctx, "cb_aaa")
	if err != nil {
		t.Fatal(err)
	}
	if hashes["/proj-a/foo.go"] != "hash_a" {
		t.Errorf("expected hash_a for /proj-a/foo.go, got %q", hashes["/proj-a/foo.go"])
	}
	if _, ok := hashes["/proj-b/bar.go"]; ok {
		t.Error("cb_aaa should not contain entries from cb_bbb after migration")
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
	results, err := store.Search(ctx, []float32{0.0, 1.0, 0.0, 0.0}, 5, "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (orthogonal vectors filtered out), got %d", len(results))
	}
}
