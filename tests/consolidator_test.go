package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/claude-context/pkg/consolidator"
	"github.com/ranwei/claude-context/pkg/state"
)

func setupConsolidatorHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	return home
}

func writeRows(t *testing.T, home string, rows []state.MemoryRow) {
	t.Helper()
	for _, r := range rows {
		if err := state.AppendMemoryRow(home, r); err != nil {
			t.Fatalf("AppendMemoryRow: %v", err)
		}
	}
}

func TestConsolidateOldRows(t *testing.T) {
	home := setupConsolidatorHome(t)

	old := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	writeRows(t, home, []state.MemoryRow{
		{StartedAt: old, TurnCount: 3, PatternCounts: map[string]int{"bugfix": 1}},
		{StartedAt: old, TurnCount: 5, PatternCounts: map[string]int{"feature": 2}},
		{StartedAt: old, TurnCount: 2, PatternCounts: map[string]int{}},
	})

	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3 rows consolidated, got %d", n)
	}

	data, _ := os.ReadFile(filepath.Join(home, ".claude", "claude-context-memory.md"))
	if !strings.Contains(string(data), "> Consolidated") {
		t.Errorf("expected '> Consolidated' in memory.md, got:\n%s", data)
	}
	if !strings.Contains(string(data), "10 actions") {
		t.Errorf("expected '10 actions' in consolidated summary, got:\n%s", data)
	}
}

func TestConsolidateKeepsRecentRows(t *testing.T) {
	home := setupConsolidatorHome(t)

	recent := time.Now().Add(-1 * 24 * time.Hour).UTC().Format(time.RFC3339)
	writeRows(t, home, []state.MemoryRow{
		{StartedAt: recent, TurnCount: 4, PatternCounts: map[string]int{}},
	})

	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows consolidated (all recent), got %d", n)
	}

	rows, _ := state.ReadMemory(home)
	if len(rows) != 1 {
		t.Errorf("expected 1 row remaining, got %d", len(rows))
	}
}

func TestConsolidateNoop(t *testing.T) {
	home := setupConsolidatorHome(t)
	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate on empty: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestConsolidateMixedRows(t *testing.T) {
	home := setupConsolidatorHome(t)

	old := time.Now().Add(-10 * 24 * time.Hour).UTC().Format(time.RFC3339)
	recent := time.Now().Add(-2 * 24 * time.Hour).UTC().Format(time.RFC3339)
	writeRows(t, home, []state.MemoryRow{
		{StartedAt: old, TurnCount: 6, PatternCounts: map[string]int{}},
		{StartedAt: recent, TurnCount: 3, PatternCounts: map[string]int{"feature": 1}},
	})

	n, err := consolidator.Consolidate(home)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 old row consolidated, got %d", n)
	}

	rows, _ := state.ReadMemory(home)
	if len(rows) != 1 {
		t.Errorf("expected 1 remaining recent row, got %d", len(rows))
	}
}
