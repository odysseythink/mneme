package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/ranwei/mneme/pkg/match"
	"github.com/ranwei/mneme/pkg/state"
)

func dispatchBuglog(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: mneme buglog <add|list|search|clear>")
		os.Exit(2)
	}
	switch args[0] {
	case "add":
		buglogAdd(args[1:])
	case "list":
		buglogList()
	case "search":
		buglogSearch(args[1:])
	case "clear":
		buglogClear(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown buglog subcommand: %q\n", args[0])
		os.Exit(2)
	}
}

func requireProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot determine working directory")
		os.Exit(1)
	}
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "no mneme project found (run: mneme init)")
		os.Exit(1)
	}
	return root
}

func buglogAdd(args []string) {
	var description, code, file string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--description":
			if i+1 < len(args) {
				description = args[i+1]
				i++
			}
		case "--code":
			if i+1 < len(args) {
				code = args[i+1]
				i++
			}
		case "--file":
			if i+1 < len(args) {
				file = args[i+1]
				i++
			}
		}
	}

	if description == "" || code == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "description and code are required; use --description and --code flags")
			os.Exit(1)
		}
		reader := bufio.NewReader(os.Stdin)
		if description == "" {
			fmt.Fprint(os.Stdout, "Description: ")
			line, _ := reader.ReadString('\n')
			description = strings.TrimSpace(line)
		}
		if code == "" {
			fmt.Fprintln(os.Stdout, "Bad code snippet (paste, then Ctrl-D):")
			rest, _ := io.ReadAll(reader)
			code = strings.TrimSpace(string(rest))
		}
	}

	if description == "" || code == "" {
		fmt.Fprintln(os.Stderr, "description and code are required")
		os.Exit(1)
	}

	root := requireProjectRoot()
	entry := state.BuglogEntry{
		Source:      "manual",
		File:        file,
		Description: description,
		BadCode:     code,
	}
	if err := state.AppendBuglogEntry(root, entry); err != nil {
		fmt.Fprintf(os.Stderr, "buglog add: %v\n", err)
		os.Exit(1)
	}
	if entries, err := state.ReadBuglog(root); err == nil {
		fmt.Printf("✓ Entry added (%d entries total in .mneme/buglog.json)\n", len(entries))
	} else {
		fmt.Println("✓ Entry added to .mneme/buglog.json")
	}
}

func buglogList() {
	root := requireProjectRoot()
	entries, err := state.ReadBuglog(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buglog list: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No buglog entries. Run: mneme buglog add")
		return
	}
	fmt.Printf("Buglog entries (%d):\n", len(entries))
	for i, e := range entries {
		ts := e.CreatedAt
		if len(ts) > 16 {
			ts = ts[:16] + "Z"
		}
		fmt.Printf("  %d. [%s] %s  (%s)\n", i+1, e.Source, e.Description, ts)
		if e.File != "" {
			fmt.Printf("     file: %s\n", e.File)
		}
		firstLine := e.BadCode
		if idx := strings.Index(firstLine, "\n"); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		fmt.Printf("     was: %s\n", firstLine)
	}
}

func buglogClear(args []string) {
	skipConfirm := false
	for _, a := range args {
		if a == "--yes" {
			skipConfirm = true
		}
	}

	root := requireProjectRoot()
	entries, err := state.ReadBuglog(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buglog clear: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No buglog entries to clear.")
		return
	}

	if !skipConfirm {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "confirmation required: re-run with --yes to skip")
			os.Exit(1)
		}
		fmt.Printf("Remove all %d buglog entries? [y/N]: ", len(entries))
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	if err := state.WriteBuglog(root, nil); err != nil {
		fmt.Fprintf(os.Stderr, "buglog clear: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Cleared.")
}

func buglogSearch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: mneme buglog search <term>")
		os.Exit(2)
	}
	query := strings.Join(args, " ")
	root := requireProjectRoot()
	entries, err := state.ReadBuglog(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buglog search: %v\n", err)
		os.Exit(1)
	}

	queryTokens := match.Tokenize(query)
	queryLower := strings.ToLower(query)

	type hit struct {
		idx   int
		entry state.BuglogEntry
		score int
	}
	var hits []hit
	for i, e := range entries {
		corpus := strings.ToLower(e.Description + " " + e.BadCode + " " + e.File)
		score := 0
		if strings.Contains(corpus, queryLower) {
			score += 5
		}
		score += match.TokenOverlap(queryTokens, match.Tokenize(corpus))
		if score > 0 {
			hits = append(hits, hit{idx: i, entry: e, score: score})
		}
	}
	if len(hits) == 0 {
		fmt.Println("no matches")
		return
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })

	fmt.Printf("%d matches for %q:\n", len(hits), query)
	for _, h := range hits {
		ts := h.entry.CreatedAt
		if len(ts) > 16 {
			ts = ts[:16] + "Z"
		}
		fmt.Printf("  [score=%d] [%s] %s  (%s)\n", h.score, h.entry.Source, h.entry.Description, ts)
		if h.entry.File != "" {
			fmt.Printf("            file: %s\n", h.entry.File)
		}
		firstLine := h.entry.BadCode
		if idx := strings.Index(firstLine, "\n"); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		fmt.Printf("            was:  %s\n", firstLine)
	}
}
