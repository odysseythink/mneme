package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ranwei/claude-context/pkg/consolidator"
	"github.com/ranwei/claude-context/pkg/hook"
	"github.com/ranwei/claude-context/pkg/state"
)

func runSessionStart(stdin io.Reader) {
	defer recoverAndLog("session-start")
	ev := parseOrExit(stdin, "session-start")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.session-start")

	prev, err := state.ReadSession(root)
	if err == nil && prev != nil {
		home, _ := os.UserHomeDir()
		row := buildMemoryRow(prev)
		if err := state.AppendMemoryRow(home, row); err != nil {
			hook.WriteStderr("session-start: memory write: " + err.Error())
		} else {
			state.IncrementSafe(root, "memory_rows_written")
			// Lazy consolidation: only run when memory is getting large.
			if rows, _ := state.ReadMemory(home); len(rows) > 50 {
				if _, cerr := consolidator.Consolidate(home); cerr != nil {
					hook.WriteStderr("session-start: consolidate: " + cerr.Error())
				}
			}
		}
	}

	s := state.Session{
		SessionID:       ev.SessionID,
		ClaudeCodeModel: ev.Model,
	}
	if err := state.UpsertSession(root, s); err != nil {
		hook.WriteStderr("session-start: upsert session: " + err.Error())
	}

	exitHook("session-start", root)
}

func buildMemoryRow(s *state.Session) state.MemoryRow {
	fileCounts := map[string]map[string]int{}
	patternCounts := map[string]int{}

	collectEdits := func(edits []state.TurnEdit) {
		for _, e := range edits {
			if fileCounts[e.File] == nil {
				fileCounts[e.File] = map[string]int{}
			}
			fileCounts[e.File][e.Category]++
			patternCounts[e.Category]++
		}
	}

	for _, ts := range s.TurnSummaries {
		collectEdits(ts.Edits)
	}
	collectEdits(s.TurnEdits)

	files := make([]string, 0, len(fileCounts))
	for f := range fileCounts {
		files = append(files, f)
	}
	sort.Strings(files)

	fileSummary := make([]state.FileStat, 0, len(files))
	for _, f := range files {
		cats := make([]string, 0, len(fileCounts[f]))
		for cat, n := range fileCounts[f] {
			cats = append(cats, fmt.Sprintf("%s×%d", cat, n))
		}
		sort.Strings(cats)
		fileSummary = append(fileSummary, state.FileStat{File: f, Categories: cats})
	}

	return state.MemoryRow{
		StartedAt:     s.StartedAt,
		TurnCount:     s.StopCount,
		FileSummary:   fileSummary,
		PatternCounts: patternCounts,
		Summary:       generateSummary(patternCounts),
	}
}

var summaryTemplates = map[string]string{
	"new_file":       "Created %d new file(s).",
	"bugfix":         "Fixed %d bug(s).",
	"feature":        "Added %d feature(s).",
	"refactor":       "Refactored %d area(s).",
	"test":           "Updated %d test(s).",
	"docs":           "Updated %d doc(s).",
	"config":         "Modified %d config file(s).",
	"dependency":     "Updated %d dependenc(y/ies).",
	"types":          "Updated %d type definition(s).",
	"style":          "Applied %d style change(s).",
	"import":         "Updated %d import(s).",
	"delete_content": "Removed content from %d file(s).",
}

func generateSummary(patternCounts map[string]int) string {
	cats := make([]string, 0, len(patternCounts))
	for cat := range patternCounts {
		if cat != "unknown" {
			cats = append(cats, cat)
		}
	}
	if len(cats) == 0 {
		return ""
	}
	sort.Strings(cats)
	parts := make([]string, 0, len(cats))
	for _, cat := range cats {
		if tmpl, ok := summaryTemplates[cat]; ok {
			parts = append(parts, fmt.Sprintf(tmpl, patternCounts[cat]))
		}
	}
	return strings.Join(parts, " ")
}
