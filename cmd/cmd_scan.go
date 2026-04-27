package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ranwei/claude-context/pkg/scanner"
	"github.com/ranwei/claude-context/pkg/state"
)

func dispatchScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	force := fs.Bool("force", false, "re-scan even if anatomy.md exists (reserved for M6 incremental)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, _ := resolveInitProjectRoot(cwd)
	if _, err := os.Stat(filepath.Join(root, ".claude-context")); err != nil {
		fmt.Fprintln(os.Stderr, "scan: project not initialized (run: claude-context init)")
		os.Exit(1)
	}

	_ = force

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", root)
	scanEntries, err := scanner.ScanProject(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(1)
	}

	entries := make([]state.AnatomyEntry, len(scanEntries))
	for i, e := range scanEntries {
		entries[i] = state.AnatomyEntry{
			Path:        e.Path,
			Description: e.Description,
			EstTokens:   e.EstTokens,
			Language:    e.Language,
		}
	}

	anatomyPath := filepath.Join(root, ".claude-context", "anatomy.md")
	if err := state.WriteAnatomy(root, entries); err != nil {
		fmt.Fprintln(os.Stderr, "scan: write anatomy:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Scanned %d files → %s\n", len(entries), anatomyPath)
}
