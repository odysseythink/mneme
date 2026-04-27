package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		runMCPServer()
		return
	}

	switch os.Args[1] {
	case "hook":
		dispatchHook(os.Args[2:])
	case "init":
		dispatchInit(os.Args[2:])
	case "stats":
		dispatchStats(os.Args[2:])
	case "version":
		printVersion()
	case "-h", "--help", "help":
		printTopUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n", os.Args[1])
		printTopUsage()
		os.Exit(2)
	}
}

// Stubs — replaced by real implementations in subsequent tasks.
func dispatchInit(args []string)  { os.Exit(0) }
func dispatchStats(args []string) { os.Exit(0) }
func printVersion()               {}
func printTopUsage()              {}
