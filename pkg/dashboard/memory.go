package dashboard

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type MemoryRow struct {
	StartedAt string `json:"started_at"`
	TurnCount int    `json:"turn_count"`
	Summary   string `json:"summary"`
}

func MemoryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		home, err := os.UserHomeDir()
		if err != nil {
			http.Error(w, `{"error":"home"}`, http.StatusInternalServerError)
			return
		}
		path := filepath.Join(home, ".claude", "mneme-memory.md")
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			http.Error(w, `{"error":"read"}`, http.StatusInternalServerError)
			return
		}
		raw := string(data)
		rows := parseMemoryRows(raw)
		writeJSON(w, 200, map[string]any{"rows": rows, "raw": raw})
	}
}

func parseMemoryRows(raw string) []MemoryRow {
	out := []MemoryRow{}
	if raw == "" {
		return out
	}
	sections := strings.Split(raw, "\n## ")
	if len(sections) > 0 && strings.HasPrefix(sections[0], "## ") {
		sections[0] = strings.TrimPrefix(sections[0], "## ")
	} else if len(sections) > 0 {
		sections = sections[1:]
	}
	for _, s := range sections {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		lines := strings.SplitN(s, "\n", 2)
		ts := strings.TrimSpace(lines[0])
		if ts == "" {
			continue
		}
		row := MemoryRow{StartedAt: ts}
		body := ""
		if len(lines) > 1 {
			body = lines[1]
		}
		if strings.HasPrefix(body, "turns: ") {
			rest := strings.SplitN(body, "\n", 2)
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(rest[0], "turns: "))); err == nil {
				row.TurnCount = n
			}
			if len(rest) > 1 {
				body = rest[1]
			} else {
				body = ""
			}
		}
		row.Summary = strings.TrimSpace(body)
		out = append(out, row)
	}
	return out
}
