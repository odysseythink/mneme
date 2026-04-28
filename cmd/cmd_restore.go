package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/ranwei/mneme/pkg/installer"
	"github.com/ranwei/mneme/pkg/state"
)

func dispatchRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	listFlag := fs.Bool("list", false, "list backups for current project")
	latest := fs.Bool("latest", false, "restore the most recent backup")
	yes := fs.Bool("yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	root := requireProjectRoot()
	id, err := state.ReadOrCreateLocalID(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "restore: read project id:", err)
		os.Exit(1)
	}

	backups, err := installer.ListBackups(id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "restore: list backups:", err)
		os.Exit(1)
	}
	if len(backups) == 0 {
		fmt.Println("No backups available for this project.")
		return
	}

	if *listFlag {
		fmt.Printf("%-32s  path\n", "timestamp (newest first)")
		for _, b := range backups {
			fmt.Printf("%-32s  %s\n", b.Timestamp, b.Path)
		}
		return
	}

	var chosen string
	switch {
	case *latest:
		chosen = backups[0].Timestamp
	case len(fs.Args()) >= 1:
		chosen = fs.Arg(0)
	default:
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "restore: no TTY; pass --latest or a timestamp explicitly")
			os.Exit(2)
		}
		fmt.Println("Available backups:")
		for i, b := range backups {
			fmt.Printf("  [%d] %s\n", i+1, b.Timestamp)
		}
		fmt.Print("Select [1]: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		choice := 1
		if n := strings.TrimSpace(line); n != "" {
			fmt.Sscanf(n, "%d", &choice)
		}
		if choice < 1 || choice > len(backups) {
			fmt.Fprintln(os.Stderr, "invalid selection")
			os.Exit(1)
		}
		chosen = backups[choice-1].Timestamp
	}

	if !*yes {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "restore: confirmation required (re-run with --yes)")
			os.Exit(2)
		}
		fmt.Printf("Restore backup %s? Existing files will be overwritten. [y/N]: ", chosen)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	if err := installer.RestoreBackup(chosen, id); err != nil {
		fmt.Fprintln(os.Stderr, "restore:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Restored from %s\n", chosen)
}
