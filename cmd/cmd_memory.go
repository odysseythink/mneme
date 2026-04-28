package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ranwei/claude-context/pkg/consolidator"
)

func dispatchMemory(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: claude-context memory <subcommand>")
		fmt.Fprintln(os.Stderr, "subcommands: consolidate")
		os.Exit(2)
	}

	switch args[0] {
	case "consolidate":
		runMemoryConsolidate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "memory: unknown subcommand %q\n", args[0])
		os.Exit(2)
	}
}

func runMemoryConsolidate(args []string) {
	fs := flag.NewFlagSet("memory consolidate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "memory consolidate: cannot get home dir:", err)
		os.Exit(1)
	}

	n, err := consolidator.Consolidate(home)
	if err != nil {
		fmt.Fprintln(os.Stderr, "memory consolidate:", err)
		os.Exit(1)
	}
	fmt.Printf("consolidated %d session row(s)\n", n)
}
