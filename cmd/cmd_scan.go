package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/scanner"
	"github.com/ranwei/mneme/pkg/state"
)

func dispatchScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	force := fs.Bool("force", false, "re-scan all files (ignore mtime cache)")
	check := fs.Bool("check", false, "verify anatomy.md matches filesystem; exit 1 on drift")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, _ := resolveInitProjectRoot(cwd)
	if _, err := os.Stat(filepath.Join(root, ".mneme")); err != nil {
		fmt.Fprintln(os.Stderr, "scan: project not initialized (run: mneme init)")
		os.Exit(1)
	}

	if *check {
		runScanCheck(root)
		return
	}

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", root)

	paths, err := scanner.Walk(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan: walk:", err)
		os.Exit(1)
	}

	var scanEntries []scanner.FileEntry
	if !*force {
		since, tsErr := state.ReadAnatomyGeneratedTime(root)
		existing, _ := state.ReadAnatomy(root)
		if tsErr == nil && len(existing) > 0 {
			scanEntries, err = scanner.ScanProjectIncremental(root, paths, since, existing)
		} else {
			scanEntries, err = scanner.ExtractAll(root, paths)
		}
	} else {
		scanEntries, err = scanner.ExtractAll(root, paths)
	}
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

	anatomyPath := filepath.Join(root, ".mneme", "anatomy.md")
	if err := state.WriteAnatomy(root, entries); err != nil {
		fmt.Fprintln(os.Stderr, "scan: write anatomy:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Scanned %d files → %s\n", len(entries), anatomyPath)
}

func runScanCheck(root string) {
	existing, err := state.ReadAnatomy(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan --check: read anatomy:", err)
		os.Exit(2)
	}
	paths, err := scanner.Walk(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan --check: walk:", err)
		os.Exit(2)
	}
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	var added, removed []string
	for _, p := range paths {
		if _, ok := existing[p]; !ok {
			added = append(added, p)
		}
	}
	for p := range existing {
		if !pathSet[p] {
			removed = append(removed, p)
		}
	}

	if len(added) == 0 && len(removed) == 0 {
		fmt.Println("anatomy.md in sync — no drift detected")
		return
	}
	fmt.Printf("drift detected: %d added, %d removed\n", len(added), len(removed))
	for _, p := range added {
		fmt.Printf("  + %s\n", p)
	}
	for _, p := range removed {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Println("\nrun: mneme scan")
	os.Exit(1)
}
