package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/state"
)

func dispatchCerebrum(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme cerebrum <add|list|remove>")
		os.Exit(2)
	}
	switch args[0] {
	case "add":
		cerebrumAdd(args[1:])
	case "list":
		cerebrumList()
	case "remove":
		cerebrumRemove(args[1:])
	case "review":
		cerebrumReview(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown cerebrum subcommand: %q\n", args[0])
		os.Exit(2)
	}
}

func cerebrumAdd(args []string) {
	fs := flag.NewFlagSet("cerebrum add", flag.ContinueOnError)
	pattern := fs.String("pattern", "", "regex pattern to match")
	message := fs.String("message", "", "warning message shown to Claude")
	comment := fs.String("comment", "", "human-readable label (optional)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	scanner := bufio.NewScanner(os.Stdin)
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	if *comment == "" && isTTY {
		fmt.Fprint(os.Stderr, "Comment (optional, press Enter to skip): ")
		scanner.Scan()
		*comment = strings.TrimSpace(scanner.Text())
	}
	if *pattern == "" {
		if !isTTY {
			fmt.Fprintln(os.Stderr, "✗ --pattern required in non-TTY mode")
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, "Pattern (regex): ")
		scanner.Scan()
		*pattern = strings.TrimSpace(scanner.Text())
	}
	if *message == "" {
		if !isTTY {
			fmt.Fprintln(os.Stderr, "✗ --message required in non-TTY mode")
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, "Warning message: ")
		scanner.Scan()
		*message = strings.TrimSpace(scanner.Text())
	}

	if *pattern == "" || *message == "" {
		fmt.Fprintln(os.Stderr, "✗ pattern and message are required")
		os.Exit(1)
	}
	if _, err := regexp.Compile(*pattern); err != nil {
		fmt.Fprintf(os.Stderr, "✗ invalid regex: %v\n", err)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ cannot get cwd:", err)
		os.Exit(1)
	}
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}
	rule := state.CerebrumRule{Comment: *comment, Pattern: *pattern, Message: *message}
	if err := state.AppendCerebrumRule(root, rule); err != nil {
		fmt.Fprintln(os.Stderr, "✗ write error:", err)
		os.Exit(1)
	}

	if rules, err2 := state.ReadCerebrum(root); err2 == nil {
		fmt.Fprintf(os.Stderr, "✓ Rule added (%d rules total in .mneme/cerebrum.md)\n", len(rules))
	} else {
		fmt.Fprintln(os.Stderr, "✓ Rule added to .mneme/cerebrum.md")
	}
}

func cerebrumList() {
	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}

	rules, err := state.ReadCerebrum(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ read error:", err)
		os.Exit(1)
	}
	if len(rules) == 0 {
		fmt.Println("No cerebrum rules. Run: mneme cerebrum add")
		return
	}

	fmt.Printf("Cerebrum rules (%d):\n", len(rules))
	for i, r := range rules {
		fmt.Printf("  %d. %s\n", i+1, r.Message)
		fmt.Printf("     pattern: %s\n", r.Pattern)
	}
}

func cerebrumRemove(args []string) {
	fs := flag.NewFlagSet("cerebrum remove", flag.ContinueOnError)
	var skipConfirm bool
	fs.BoolVar(&skipConfirm, "yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: mneme cerebrum remove <N> [--yes]")
		os.Exit(2)
	}

	n, err := strconv.Atoi(fs.Arg(0))
	if err != nil || n < 1 {
		fmt.Fprintln(os.Stderr, "✗ N must be a positive integer")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}

	rules, err := state.ReadCerebrum(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ read error:", err)
		os.Exit(1)
	}
	if n > len(rules) {
		fmt.Fprintf(os.Stderr, "✗ no rule %d (have %d rules)\n", n, len(rules))
		os.Exit(1)
	}

	target := rules[n-1]
	if !skipConfirm {
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))
		if !isTTY {
			fmt.Fprintln(os.Stderr, "✗ confirmation required: re-run with --yes to skip")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Remove rule %d: %q? [y/N]: ", n, target.Message)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		reply := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if reply != "y" && reply != "yes" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			os.Exit(0)
		}
	}

	newRules := append(rules[:n-1:n-1], rules[n:]...)
	if err := state.WriteCerebrum(root, newRules); err != nil {
		fmt.Fprintln(os.Stderr, "✗ write error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Removed. %d rules remaining.\n", len(newRules))
}

func cerebrumReview(args []string) {
	fs := flag.NewFlagSet("cerebrum review", flag.ContinueOnError)
	listOnly := fs.Bool("list", false, "print pending candidates without prompting")
	asJSON := fs.Bool("json", false, "JSON output (only with --list)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}

	pending, err := cerebrum.LoadPending(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ load pending:", err)
		os.Exit(1)
	}

	if *listOnly {
		if *asJSON {
			data, _ := json.MarshalIndent(pending, "", "  ")
			fmt.Println(string(data))
			return
		}
		if len(pending) == 0 {
			fmt.Println("No pending cerebrum candidates.")
			return
		}
		for i, c := range pending {
			fmt.Printf("%d. [%s] %s — %s\n", i+1, c.ID[:8], c.Trigger.Phrase, c.DraftRule.Message)
		}
		return
	}

	if len(pending) == 0 {
		fmt.Println("No pending cerebrum candidates.")
		return
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "✗ interactive review requires a TTY; use --list for non-interactive output")
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	remove := make(map[string]bool)
	for i, c := range pending {
		fmt.Printf("\n[%d/%d] phrase=%q hits=%d\n", i+1, len(pending), c.Trigger.Phrase, c.HitCount)
		fmt.Printf("    user: %s\n", c.Trigger.UserMsg)
		fmt.Printf("    prior: %s\n", c.Trigger.PriorAsst)
		fmt.Printf("    draft pattern: %s\n", c.DraftRule.Pattern)
		fmt.Printf("    draft message: %s\n", c.DraftRule.Message)
		fmt.Print("    [a]ccept / [r]eject / [s]kip / [q]uit: ")
		scanner.Scan()
		switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
		case "a", "accept":
			if err := state.AppendCerebrumRule(root, c.DraftRule); err != nil {
				fmt.Fprintln(os.Stderr, "✗ append rule:", err)
				continue
			}
			remove[c.ID] = true
		case "r", "reject":
			if err := cerebrum.AppendRejected(root, c.ID, time.Now().UTC()); err != nil {
				fmt.Fprintln(os.Stderr, "✗ record rejection:", err)
				continue
			}
			remove[c.ID] = true
		case "q", "quit":
			break
		}
	}
	if len(remove) > 0 {
		if err := cerebrum.RemovePending(root, remove); err != nil {
			fmt.Fprintln(os.Stderr, "✗ update pending:", err)
		}
	}
}

// keep regexp/strconv used
var _ = regexp.MustCompile
var _ = strconv.Atoi
