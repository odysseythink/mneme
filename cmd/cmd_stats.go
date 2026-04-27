package main

import (
	"fmt"
	"os"

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
	fmt.Println("hook_fired:")
	for _, k := range []string{"pre-read", "pre-write", "post-write", "session-start", "stop"} {
		fmt.Printf("  %-20s %d\n", k, l.Totals.HookFired[k])
	}
	fmt.Printf("hook_errors:          %d\n", l.Totals.HookErrors)
	fmt.Printf("stdin_parse_failures: %d\n", l.Totals.StdinParseFailures)
	fmt.Printf("outside_project_skipped: %d\n", l.Totals.OutsideProjectSkipped)
	fmt.Printf("anatomy_hits:            %d\n", l.Totals.AnatomyHits)
	fmt.Printf("repeat_reads:            %d\n", l.Totals.RepeatReads)
	fmt.Printf("scan_count:              %d\n", l.Totals.ScanCount)
	if l.Totals.HookFired["stop"] > 0 {
		fmt.Println()
		fmt.Println("note: stop counts include per-turn fires (per M0 finding); session-end semantics deferred to M3")
	}
}
