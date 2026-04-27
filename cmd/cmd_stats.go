package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ranwei/claude-context/pkg/state"
)

func dispatchStats(args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stats: cannot get cwd:", err)
		os.Exit(1)
	}

	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "stats: not inside an initialized project (run: claude-context init)")
		os.Exit(1)
	}

	l, err := state.ReadLedger(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stats: read ledger:", err)
		os.Exit(1)
	}

	fmt.Printf("project: %s\n", l.ProjectID)
	if l.FirstRecorded != "" {
		fmt.Printf("first recorded: %s\n", l.FirstRecorded)
	}
	fmt.Println()

	fmt.Println("=== Hook Totals ===")
	for _, k := range []string{"pre-read", "pre-write", "post-tool-use", "session-start", "stop"} {
		fmt.Printf("  %-24s %d\n", k+":", l.Totals.HookFired[k])
	}
	fmt.Printf("  %-24s %d\n", "hook_errors:", l.Totals.HookErrors)
	fmt.Printf("  %-24s %d\n", "stdin_parse_failures:", l.Totals.StdinParseFailures)
	fmt.Printf("  %-24s %d\n", "outside_project_skipped:", l.Totals.OutsideProjectSkipped)

	if len(l.Totals.EditPatterns) > 0 {
		fmt.Println()
		fmt.Println("=== Edit Patterns ===")
		cats := make([]string, 0, len(l.Totals.EditPatterns))
		for cat := range l.Totals.EditPatterns {
			cats = append(cats, cat)
		}
		sort.Slice(cats, func(i, j int) bool {
			return l.Totals.EditPatterns[cats[i]] > l.Totals.EditPatterns[cats[j]]
		})
		for _, cat := range cats {
			fmt.Printf("  %-20s %d\n", cat+":", l.Totals.EditPatterns[cat])
		}
	}

	home, _ := os.UserHomeDir()
	rows, err := state.ReadMemory(home)
	if err == nil && len(rows) > 0 {
		fmt.Println()
		fmt.Println("=== Recent Sessions (last 5) ===")
		start := 0
		if len(rows) > 5 {
			start = len(rows) - 5
		}
		for _, r := range rows[start:] {
			patterns := formatPatternCounts(r.PatternCounts)
			fmt.Printf("  %-32s %2d turns  %s\n", r.StartedAt, r.TurnCount, patterns)
		}
		fmt.Println()
		fmt.Println("=== Memory ===")
		fmt.Printf("  rows written:      %d\n", l.Totals.MemoryRowsWritten)
		fmt.Printf("  memory.md:         %s/.claude/claude-context-memory.md\n", home)
	}
}

func formatPatternCounts(m map[string]int) string {
	if len(m) == 0 {
		return "(no edits)"
	}
	cats := make([]string, 0, len(m))
	for cat := range m {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool {
		return m[cats[i]] > m[cats[j]]
	})
	parts := make([]string, 0, len(cats))
	for _, cat := range cats {
		parts = append(parts, fmt.Sprintf("%s×%d", cat, m[cat]))
	}
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, ", ")
}
