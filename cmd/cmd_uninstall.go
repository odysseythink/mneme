package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/ranwei/mneme/pkg/installer"
)

func dispatchUninstall(opts initOpts) {
	cwd, _ := os.Getwd()
	projectRoot, _ := resolveInitProjectRoot(cwd)
	settingsPath := resolveSettingsPath(projectRoot, opts)
	claudeMDPath := filepath.Join(os.Getenv("HOME"), ".claude", "CLAUDE.md")
	rulesPath := filepath.Join(os.Getenv("HOME"), ".claude", "mneme-rules.md")

	// Detect if anything is installed
	data, _ := os.ReadFile(settingsPath)
	md, _ := os.ReadFile(claudeMDPath)
	if !strings.Contains(string(data), `"_managed_by"`) &&
		!strings.Contains(string(md), "mneme-managed") {
		fmt.Fprintln(os.Stderr, "no mneme installation found")
		os.Exit(0)
	}

	if !opts.yes && !opts.dryRun {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "✗ stdin is not a TTY; refusing to apply changes.")
			fmt.Fprintln(os.Stderr, "  Re-run with --yes to confirm or --dry-run to preview.")
			os.Exit(1)
		}
	}

	fmt.Fprintln(os.Stderr, "╭─ Uninstall Plan ───────────────────────────────────────────╮")
	fmt.Fprintf(os.Stderr, "│ Remove managed hooks from %-34s│\n", settingsPath)
	fmt.Fprintf(os.Stderr, "│ Remove @import block from %-35s│\n", claudeMDPath)
	fmt.Fprintf(os.Stderr, "│ Delete %-53s│\n", rulesPath)
	fmt.Fprintln(os.Stderr, "│ Keep <project>/.mneme/ (user data; remove manually)│")
	fmt.Fprintln(os.Stderr, "╰────────────────────────────────────────────────────────────╯")

	if opts.dryRun {
		os.Exit(0)
	}

	if !opts.yes {
		fmt.Fprint(os.Stderr, "Apply these changes? [y/N]: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		reply := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if reply != "y" && reply != "yes" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			os.Exit(0)
		}
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	for _, p := range []string{settingsPath, claudeMDPath} {
		backupFile(p, ts)
	}

	if err := installer.Uninstall(installer.UninstallOpts{
		SettingsPath: settingsPath,
		CLAUDEMDPath: claudeMDPath,
		RulesPath:    rulesPath,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "✗ uninstall:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "✓ Removed managed hooks from %s\n", settingsPath)
	fmt.Fprintf(os.Stderr, "✓ Removed @import from %s\n", claudeMDPath)
	fmt.Fprintf(os.Stderr, "✓ Deleted %s\n", rulesPath)
	fmt.Fprintf(os.Stderr, "! Project state at %s/.mneme/ kept intact.\n", projectRoot)
	fmt.Fprintln(os.Stderr, "  To remove: rm -rf "+projectRoot+"/.mneme/")
}
