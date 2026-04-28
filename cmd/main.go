package main

import (
	"fmt"
	"os"
)

// Build-time variables. Override via:
//   go build -ldflags="-X main.releaseRepo=ranwei/mneme -X main.releaseChannel=github" ./cmd
var (
	releaseRepo    = ""        // GitHub "owner/repo" — empty disables --binary
	releaseChannel = "source"  // "source" | "github"
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
	case "scan":
		dispatchScan(os.Args[2:])
	case "stats":
		dispatchStats(os.Args[2:])
	case "status":
		dispatchStatus(os.Args[2:])
	case "cerebrum":
		dispatchCerebrum(os.Args[2:])
	case "buglog":
		dispatchBuglog(os.Args[2:])
	case "memory":
		dispatchMemory(os.Args[2:])
	case "restore":
		dispatchRestore(os.Args[2:])
	case "update":
		dispatchUpdate(os.Args[2:])
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
