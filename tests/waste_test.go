package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

// setupWasteProject returns a temp project root (with .mneme/) and a temp home dir.
func setupWasteProject(t *testing.T) (projectRoot, homeDir string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mneme"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return dir, t.TempDir()
}

func findPattern(patterns []waste.WastePattern, name string) (waste.WastePattern, bool) {
	for _, p := range patterns {
		if p.Name == name {
			return p, true
		}
	}
	return waste.WastePattern{}, false
}

// --- Pattern 1: repeated_reads ---

func TestWasteRepeatedReadsDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	for i := 0; i < 3; i++ {
		state.IncrementSafe(dir, "repeat_reads")
	}

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "repeated_reads")
	if !ok {
		t.Fatal("pattern 'repeated_reads' not in result")
	}
	if !p.Detected {
		t.Errorf("expected Detected=true for repeated_reads=3")
	}
	if p.Details == "" {
		t.Errorf("expected non-empty Details when detected")
	}
}

func TestWasteRepeatedReadsNotDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	// RepeatReads stays 0

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "repeated_reads")
	if !ok {
		t.Fatal("pattern 'repeated_reads' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false when RepeatReads=0")
	}
}

// --- Pattern 2: large_reads_with_anatomy ---

func TestWasteLargeReadsWithAnatomyDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	// anatomy has a large entry
	state.WriteAnatomy(dir, []state.AnatomyEntry{
		{Path: "big.go", Description: "large file", EstTokens: 2000, Language: "go"},
		{Path: "small.go", Description: "small file", EstTokens: 50, Language: "go"},
	})
	// anatomy was consulted
	state.IncrementSafe(dir, "anatomy_hits")

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "large_reads_with_anatomy")
	if !ok {
		t.Fatal("pattern 'large_reads_with_anatomy' not in result")
	}
	if !p.Detected {
		t.Errorf("expected Detected=true: anatomy has large entry and AnatomyHits>0")
	}
}

func TestWasteLargeReadsWithAnatomyNotDetectedNoHits(t *testing.T) {
	dir, home := setupWasteProject(t)
	// anatomy has large entry but AnatomyHits=0
	state.WriteAnatomy(dir, []state.AnatomyEntry{
		{Path: "big.go", Description: "large file", EstTokens: 2000, Language: "go"},
	})

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "large_reads_with_anatomy")
	if !ok {
		t.Fatal("pattern 'large_reads_with_anatomy' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false when AnatomyHits=0")
	}
}

func TestWasteLargeReadsWithAnatomyNotDetectedSmallFiles(t *testing.T) {
	dir, home := setupWasteProject(t)
	// anatomy has only small entries
	state.WriteAnatomy(dir, []state.AnatomyEntry{
		{Path: "small.go", Description: "small file", EstTokens: 100, Language: "go"},
	})
	state.IncrementSafe(dir, "anatomy_hits")

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "large_reads_with_anatomy")
	if !ok {
		t.Fatal("pattern 'large_reads_with_anatomy' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false when all files have EstTokens<=500")
	}
}

// --- Pattern 3: memory_bloat ---

func TestWasteMemoryBloatDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	// append 51 rows
	for i := 0; i < 51; i++ {
		state.AppendMemoryRow(home, state.MemoryRow{
			StartedAt:     "2026-04-01T10:00:00Z",
			TurnCount:     1,
			PatternCounts: map[string]int{},
		})
	}

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "memory_bloat")
	if !ok {
		t.Fatal("pattern 'memory_bloat' not in result")
	}
	if !p.Detected {
		t.Errorf("expected Detected=true for 51 memory rows")
	}
}

func TestWasteMemoryBloatNotDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	// append only 5 rows
	for i := 0; i < 5; i++ {
		state.AppendMemoryRow(home, state.MemoryRow{
			StartedAt:     "2026-04-01T10:00:00Z",
			TurnCount:     1,
			PatternCounts: map[string]int{},
		})
	}

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "memory_bloat")
	if !ok {
		t.Fatal("pattern 'memory_bloat' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false for 5 memory rows")
	}
}

// --- Pattern 4: cerebrum_staleness ---

func TestWasteCerebrumStalenessDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	// write cerebrum.md
	state.AppendCerebrumRule(dir, state.CerebrumRule{
		Pattern: `fmt\.Println\(`,
		Message: "use structured logger",
	})
	// backdate mtime to 15 days ago
	cerPath := filepath.Join(dir, ".mneme", "cerebrum.md")
	old := time.Now().Add(-15 * 24 * time.Hour)
	os.Chtimes(cerPath, old, old)

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "cerebrum_staleness")
	if !ok {
		t.Fatal("pattern 'cerebrum_staleness' not in result")
	}
	if !p.Detected {
		t.Errorf("expected Detected=true for cerebrum.md 15 days old")
	}
}

func TestWasteCerebrumStalenessNotDetectedRecent(t *testing.T) {
	dir, home := setupWasteProject(t)
	state.AppendCerebrumRule(dir, state.CerebrumRule{
		Pattern: `fmt\.Println\(`,
		Message: "use structured logger",
	})
	// mtime is now (fresh)

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "cerebrum_staleness")
	if !ok {
		t.Fatal("pattern 'cerebrum_staleness' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false for fresh cerebrum.md")
	}
}

func TestWasteCerebrumStalenessNotDetectedMissing(t *testing.T) {
	dir, home := setupWasteProject(t)
	// cerebrum.md does not exist

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "cerebrum_staleness")
	if !ok {
		t.Fatal("pattern 'cerebrum_staleness' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false when cerebrum.md is absent")
	}
}

// --- Pattern 5: anatomy_miss_rate ---

func TestWasteAnatomyMissRateDetected(t *testing.T) {
	dir, home := setupWasteProject(t)
	// 20 pre-reads, 10 anatomy hits → 50% hit rate < 80%
	for i := 0; i < 20; i++ {
		state.IncrementSafe(dir, "hook_fired.pre-read")
	}
	for i := 0; i < 10; i++ {
		state.IncrementSafe(dir, "anatomy_hits")
	}

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "anatomy_miss_rate")
	if !ok {
		t.Fatal("pattern 'anatomy_miss_rate' not in result")
	}
	if !p.Detected {
		t.Errorf("expected Detected=true: 10/20=50%% hit rate < 80%%")
	}
}

func TestWasteAnatomyMissRateNotDetectedHighHitRate(t *testing.T) {
	dir, home := setupWasteProject(t)
	// 20 pre-reads, 18 anatomy hits → 90% hit rate > 80%
	for i := 0; i < 20; i++ {
		state.IncrementSafe(dir, "hook_fired.pre-read")
	}
	for i := 0; i < 18; i++ {
		state.IncrementSafe(dir, "anatomy_hits")
	}

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "anatomy_miss_rate")
	if !ok {
		t.Fatal("pattern 'anatomy_miss_rate' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false: 18/20=90%% hit rate >= 80%%")
	}
}

func TestWasteAnatomyMissRateNotDetectedFewReads(t *testing.T) {
	dir, home := setupWasteProject(t)
	// only 5 pre-reads (below minimum of 10) — not enough data
	for i := 0; i < 5; i++ {
		state.IncrementSafe(dir, "hook_fired.pre-read")
	}

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	p, ok := findPattern(patterns, "anatomy_miss_rate")
	if !ok {
		t.Fatal("pattern 'anatomy_miss_rate' not in result")
	}
	if p.Detected {
		t.Errorf("expected Detected=false: < 10 total pre-reads, insufficient data")
	}
}

// --- All 5 patterns always present ---

func TestWasteDetectAlwaysReturns5Patterns(t *testing.T) {
	dir, home := setupWasteProject(t)

	patterns, err := waste.Detect(dir, home)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(patterns) != 5 {
		t.Errorf("expected exactly 5 patterns, got %d", len(patterns))
	}
	names := map[string]bool{
		"repeated_reads":          false,
		"large_reads_with_anatomy": false,
		"memory_bloat":            false,
		"cerebrum_staleness":      false,
		"anatomy_miss_rate":       false,
	}
	for _, p := range patterns {
		names[p.Name] = true
	}
	for name, present := range names {
		if !present {
			t.Errorf("expected pattern %q in result", name)
		}
	}
}
