package consolidator

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranwei/claude-context/pkg/state"
)

const staleDays = 7

// Consolidate reads claude-context-memory.md, folds rows older than staleDays
// into a single blockquote, and writes the result back atomically.
// Returns the number of rows folded.
func Consolidate(homeDir string) (int, error) {
	rows, err := state.ReadMemory(homeDir)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	cutoff := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	var oldRows, recentRows []state.MemoryRow
	for _, r := range rows {
		ts, parseErr := time.Parse(time.RFC3339, r.StartedAt)
		if parseErr != nil {
			recentRows = append(recentRows, r)
			continue
		}
		if ts.Before(cutoff) {
			oldRows = append(oldRows, r)
		} else {
			recentRows = append(recentRows, r)
		}
	}

	if len(oldRows) == 0 {
		return 0, nil
	}

	totalActions := 0
	for _, r := range oldRows {
		totalActions += r.TurnCount
	}

	var sb strings.Builder
	sb.WriteString("<!-- claude-context memory v1 -->\n")
	fmt.Fprintf(&sb, "\n> Consolidated session (%d actions from %d sessions before %s)\n",
		totalActions, len(oldRows), cutoff.UTC().Format("2006-01-02"))

	for _, r := range recentRows {
		sb.WriteString("\n")
		sb.WriteString(formatRow(r))
	}

	memPath := filepath.Join(homeDir, ".claude", "claude-context-memory.md")
	lockPath := filepath.Join(homeDir, ".claude", "claude-context-memory.lock")
	release, lockErr := state.AcquireLock(lockPath, 2*time.Second)
	if lockErr != nil {
		return 0, fmt.Errorf("consolidate: acquire lock: %w", lockErr)
	}
	defer release()

	if err := state.AtomicWrite(memPath, []byte(sb.String())); err != nil {
		return 0, fmt.Errorf("consolidate: write: %w", err)
	}
	return len(oldRows), nil
}

func formatRow(r state.MemoryRow) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%d turns)\n", r.StartedAt, r.TurnCount))

	if len(r.FileSummary) > 0 {
		parts := make([]string, 0, len(r.FileSummary))
		for _, fs := range r.FileSummary {
			parts = append(parts, fmt.Sprintf("%s (%s)", fs.File, strings.Join(fs.Categories, ", ")))
		}
		sb.WriteString("Files: " + strings.Join(parts, ", ") + "\n")
	}

	if len(r.PatternCounts) > 0 {
		parts := make([]string, 0, len(r.PatternCounts))
		for cat, n := range r.PatternCounts {
			parts = append(parts, fmt.Sprintf("%s×%d", cat, n))
		}
		sb.WriteString("Patterns: " + strings.Join(parts, ", ") + "\n")
	}

	if r.Summary != "" {
		sb.WriteString("Summary: " + r.Summary + "\n")
	}
	return sb.String()
}
