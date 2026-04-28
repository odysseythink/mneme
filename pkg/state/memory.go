package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FileStat struct {
	File       string
	Categories []string
}

type MemoryRow struct {
	StartedAt     string
	TurnCount     int
	FileSummary   []FileStat
	PatternCounts map[string]int
	Summary       string
}

// AppendMemoryRow appends a session summary row to ~/.claude/mneme-memory.md.
func AppendMemoryRow(homeDir string, row MemoryRow) error {
	dir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "mneme-memory.lock")
	release, err := AcquireLock(lockPath, 1*time.Second)
	if err != nil {
		return err
	}
	defer release()

	memPath := filepath.Join(dir, "mneme-memory.md")
	existing, err := os.ReadFile(memPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	rowText := formatMemoryRow(row)

	var content string
	if len(existing) == 0 {
		content = "<!-- mneme memory v1 -->\n\n" + rowText
	} else {
		content = string(existing) + "\n" + rowText
	}
	return AtomicWrite(memPath, []byte(content))
}

// ReadMemory parses ~/.claude/mneme-memory.md into rows.
// Returns nil, nil if the file does not exist.
func ReadMemory(homeDir string) ([]MemoryRow, error) {
	path := filepath.Join(homeDir, ".claude", "mneme-memory.md")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var rows []MemoryRow
	var cur *MemoryRow
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			rows = append(rows, parseMemoryHeader(line))
			cur = &rows[len(rows)-1]
		} else if cur != nil && strings.HasPrefix(line, "Patterns: ") {
			cur.PatternCounts = parsePatternLine(strings.TrimPrefix(line, "Patterns: "))
		}
	}
	return rows, nil
}

func formatMemoryRow(row MemoryRow) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%d turns)\n", row.StartedAt, row.TurnCount))

	if len(row.FileSummary) > 0 {
		parts := make([]string, 0, len(row.FileSummary))
		for _, fs := range row.FileSummary {
			parts = append(parts, fmt.Sprintf("%s (%s)", fs.File, strings.Join(fs.Categories, ", ")))
		}
		sb.WriteString("Files: " + strings.Join(parts, ", ") + "\n")
	}

	if len(row.PatternCounts) > 0 {
		cats := make([]string, 0, len(row.PatternCounts))
		for cat := range row.PatternCounts {
			cats = append(cats, cat)
		}
		sort.Strings(cats)
		parts := make([]string, 0, len(cats))
		for _, cat := range cats {
			parts = append(parts, fmt.Sprintf("%s×%d", cat, row.PatternCounts[cat]))
		}
		sb.WriteString("Patterns: " + strings.Join(parts, ", ") + "\n")
	}

	if row.Summary != "" {
		sb.WriteString("Summary: " + row.Summary + "\n")
	}
	return sb.String()
}

func parseMemoryHeader(line string) MemoryRow {
	line = strings.TrimPrefix(line, "## ")
	var row MemoryRow
	idx := strings.LastIndex(line, " (")
	if idx < 0 {
		row.StartedAt = line
		return row
	}
	row.StartedAt = line[:idx]
	rest := line[idx+2:]
	if n := strings.Index(rest, " "); n > 0 {
		row.TurnCount, _ = strconv.Atoi(rest[:n])
	}
	return row
}

func parsePatternLine(line string) map[string]int {
	m := make(map[string]int)
	for _, part := range strings.Split(line, ", ") {
		idx := strings.LastIndex(part, "×")
		if idx < 0 {
			continue
		}
		cat := part[:idx]
		n, _ := strconv.Atoi(part[idx+len("×"):])
		m[cat] = n
	}
	return m
}
