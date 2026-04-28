package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/suggestions"
)

func dispatchSuggestions(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme suggestions <list|dismiss|refresh>")
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		runSuggestionsList(args[1:])
	case "dismiss":
		runSuggestionsDismiss(args[1:])
	case "refresh":
		runSuggestionsRefresh(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown suggestions subcommand: %q\n", args[0])
		os.Exit(2)
	}
}

func runSuggestionsList(args []string) {
	fs := flag.NewFlagSet("suggestions list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	root := mustProjectRootSuggestions()
	cfg := config.FromEnv()

	got, err := suggestions.List(root, cfg.SuggestionsDismissTTLDays, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ list:", err)
		os.Exit(1)
	}
	if *asJSON {
		data, _ := json.MarshalIndent(got, "", "  ")
		fmt.Println(string(data))
		return
	}
	if len(got) == 0 {
		fmt.Println("No suggestions.")
		return
	}
	for _, s := range got {
		fmt.Println(suggestions.HumanLine(s))
	}
}

func runSuggestionsDismiss(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme suggestions dismiss <id>")
		os.Exit(2)
	}
	root := mustProjectRootSuggestions()
	if err := suggestions.AppendDismissed(root, args[0], time.Now().UTC()); err != nil {
		fmt.Fprintln(os.Stderr, "✗ dismiss:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✓ dismissed", args[0])
}

func runSuggestionsRefresh(args []string) {
	root := mustProjectRootSuggestions()
	got, err := suggestions.Refresh(root, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ refresh:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ generated %d suggestion(s)\n", len(got))
}

func mustProjectRootSuggestions() string {
	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}
	return root
}
