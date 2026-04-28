package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ranwei/mneme/pkg/match"
	"github.com/ranwei/mneme/pkg/state"
)

type localArgs struct {
	Cwd   string `json:"cwd"`
	Query string `json:"query,omitempty"`
}

func (s *Server) callDescribeCodebase(_ context.Context, args json.RawMessage) interface{} {
	var a localArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Cwd == "" {
		return toolError("cwd is required")
	}

	root, ok := state.FindProjectRoot(a.Cwd)
	if !ok {
		return toolError(fmt.Sprintf("no mneme project found at %s (run: mneme init)", a.Cwd))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Project root: %s (%s)\n", filepath.Base(root), root)

	anatomy, _ := state.ReadAnatomy(root)
	if len(anatomy) == 0 {
		fmt.Fprintf(&sb, "\n(No anatomy map. Run: mneme scan)\n")
	} else {
		entries := make([]state.AnatomyEntry, 0, len(anatomy))
		for _, e := range anatomy {
			entries = append(entries, e)
		}
		if len(entries) > 40 {
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].EstTokens > entries[j].EstTokens
			})
			entries = entries[:40]
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Path < entries[j].Path
		})

		fmt.Fprintf(&sb, "\n=== Anatomy (%d files) ===\n", len(anatomy))
		currentDir := ""
		for _, e := range entries {
			dir := filepath.Dir(e.Path)
			if dir == "." {
				dir = ""
			}
			if dir != currentDir {
				if dir == "" {
					fmt.Fprintf(&sb, "(root)/\n")
				} else {
					fmt.Fprintf(&sb, "%s/\n", dir)
				}
				currentDir = dir
			}
			name := filepath.Base(e.Path)
			fmt.Fprintf(&sb, "  %-22s %s (~%d tok)\n", name, e.Description, e.EstTokens)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(&sb, "\n(No session history: cannot determine home directory)\n")
	} else {
		rows, _ := state.ReadMemory(homeDir)
		if len(rows) == 0 {
			fmt.Fprintf(&sb, "\n(No session history yet.)\n")
		} else {
			count := min(5, len(rows))
			fmt.Fprintf(&sb, "\n=== Recent Sessions (last %d) ===\n", count)
			start := len(rows) - count
			for i := len(rows) - 1; i >= start; i-- {
				r := rows[i]
				line := fmt.Sprintf("  %s  %d turns", r.StartedAt, r.TurnCount)
				patterns := make([]string, 0, len(r.PatternCounts))
				for k, v := range r.PatternCounts {
					patterns = append(patterns, fmt.Sprintf("%s×%d", k, v))
				}
				sort.Strings(patterns)
				if len(patterns) > 0 {
					line += "  " + strings.Join(patterns, ", ")
				}
				if r.Summary != "" {
					line += " — " + r.Summary
				}
				fmt.Fprintln(&sb, line)
			}
		}
	}

	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: sb.String()}}}
}

func (s *Server) callGetProjectRules(_ context.Context, args json.RawMessage) interface{} {
	var a localArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Cwd == "" {
		return toolError("cwd is required")
	}

	root, ok := state.FindProjectRoot(a.Cwd)
	if !ok {
		return toolError(fmt.Sprintf("no mneme project found at %s (run: mneme init)", a.Cwd))
	}

	rules, _ := state.ReadCerebrum(root)
	if len(rules) == 0 {
		return ToolCallResult{Content: []ToolContent{{Type: "text", Text: "No cerebrum rules. Run: mneme cerebrum add"}}}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Cerebrum rules (%d):\n", len(rules))
	for i, r := range rules {
		label := r.Message
		if r.Comment != "" {
			label = r.Comment
		}
		fmt.Fprintf(&sb, "  %d. %s\n     pattern: %s\n", i+1, label, r.Pattern)
	}
	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: sb.String()}}}
}

func (s *Server) callFindSimilarBugs(_ context.Context, args json.RawMessage) interface{} {
	var a localArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Cwd == "" {
		return toolError("cwd is required")
	}
	if a.Query == "" {
		return toolError("query is required")
	}

	root, ok := state.FindProjectRoot(a.Cwd)
	if !ok {
		return toolError(fmt.Sprintf("no mneme project found at %s (run: mneme init)", a.Cwd))
	}

	entries, _ := state.ReadBuglog(root)
	if len(entries) == 0 {
		return ToolCallResult{Content: []ToolContent{{Type: "text", Text: "No buglog entries. Run: mneme buglog add"}}}
	}

	queryTokens := match.Tokenize(a.Query)

	type bugMatch struct {
		entry   state.BuglogEntry
		overlap int
	}
	var matches []bugMatch
	for _, e := range entries {
		overlap := match.TokenOverlap(queryTokens, match.Tokenize(e.BadCode))
		if overlap >= 3 {
			matches = append(matches, bugMatch{e, overlap})
		}
	}

	if len(matches) == 0 {
		return ToolCallResult{Content: []ToolContent{{Type: "text", Text: "No matches found."}}}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].overlap > matches[j].overlap
	})
	if len(matches) > 10 {
		matches = matches[:10]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Similar bugs (%d match(es)):\n", len(matches))
	for i, m := range matches {
		firstLine := m.entry.BadCode
		if idx := strings.Index(firstLine, "\n"); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		fmt.Fprintf(&sb, "  %d. [%s] %s  (overlap: %d tokens)\n     was: %s\n",
			i+1, m.entry.Source, m.entry.Description, m.overlap, firstLine)
	}
	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: sb.String()}}}
}
