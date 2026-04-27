package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/ranwei/claude-context/pkg/installer"
	"github.com/ranwei/claude-context/pkg/state"
)

type initOpts struct {
	yes       bool
	dryRun    bool
	printMode bool
	project   bool
	local     bool
	noScan    bool
	uninstall bool
}

func dispatchInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	var opts initOpts
	fs.BoolVar(&opts.yes, "yes", false, "skip confirmation prompt")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "show plan without writing")
	fs.BoolVar(&opts.printMode, "print", false, "print final file contents to stdout")
	fs.BoolVar(&opts.project, "project", false, "use project-level settings.json")
	fs.BoolVar(&opts.local, "local", false, "use project-local settings.local.json")
	fs.BoolVar(&opts.noScan, "no-scan", false, "skip initial anatomy scan")
	fs.BoolVar(&opts.uninstall, "uninstall", false, "remove all managed entries")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if opts.uninstall {
		dispatchUninstall(opts)
		return
	}

	cwd, _ := os.Getwd()
	projectRoot, _ := resolveInitProjectRoot(cwd)
	settingsPath := resolveSettingsPath(projectRoot, opts)
	claudeMDPath := filepath.Join(os.Getenv("HOME"), ".claude", "CLAUDE.md")
	rulesPath := filepath.Join(os.Getenv("HOME"), ".claude", "claude-context-rules.md")

	binaryPath, _ := os.Executable()

	// Check TTY / flag mode
	if !opts.yes && !opts.dryRun && !opts.printMode {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "✗ stdin is not a TTY; refusing to apply changes.")
			fmt.Fprintln(os.Stderr, "  Re-run with --yes to confirm, --dry-run to preview, or --print to see final files.")
			os.Exit(1)
		}
	}

	// --print mode: show final file contents and exit
	if opts.printMode {
		fmt.Printf("==> %s\n", settingsPath)
		fmt.Println("(would contain merged hooks — run without --print to apply)")
		os.Exit(0)
	}

	// Show plan
	printInitPlan(projectRoot, settingsPath, claudeMDPath, rulesPath)

	if opts.dryRun {
		os.Exit(0)
	}

	// Confirm
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

	// Execute
	runInit(projectRoot, settingsPath, claudeMDPath, rulesPath, binaryPath, opts.noScan)
}

func resolveInitProjectRoot(cwd string) (string, bool) {
	if root, ok := state.FindGitRoot(cwd); ok {
		return root, true
	}
	return cwd, false
}

func resolveSettingsPath(projectRoot string, opts initOpts) string {
	home := os.Getenv("HOME")
	if opts.project {
		return filepath.Join(projectRoot, ".claude", "settings.json")
	}
	if opts.local {
		return filepath.Join(projectRoot, ".claude", "settings.local.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func printInitPlan(projectRoot, settingsPath, claudeMDPath, rulesPath string) {
	fmt.Fprintln(os.Stderr, "╭─ Plan ─────────────────────────────────────────────────────╮")
	fmt.Fprintf(os.Stderr, "│ Will modify %-47s│\n", settingsPath)
	fmt.Fprintln(os.Stderr, "│   + add 5 hook entries (PreToolUse:Read, Write; PostTool... │")
	fmt.Fprintf(os.Stderr, "│ Will write %-49s│\n", rulesPath)
	fmt.Fprintf(os.Stderr, "│ Will append @import block to %-31s│\n", claudeMDPath)
	fmt.Fprintf(os.Stderr, "│ Will init %-50s│\n", projectRoot+"/.claude-context/")
	fmt.Fprintln(os.Stderr, "│ Backups: <each-path>.bak.<timestamp>                       │")
	fmt.Fprintln(os.Stderr, "╰────────────────────────────────────────────────────────────╯")
}

func runInit(projectRoot, settingsPath, claudeMDPath, rulesPath, binaryPath string, noScan bool) {
	ts := time.Now().UTC().Format("20060102T150405Z")

	// Step 1: backups
	for _, p := range []string{settingsPath, claudeMDPath} {
		backupFile(p, ts)
	}

	// Step 2: merge settings.json
	if err := installer.MergeHooks(settingsPath, binaryPath); err != nil {
		fmt.Fprintln(os.Stderr, "✗ settings.json:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Registered 5 hooks in %s\n", settingsPath)

	// Step 3: write rules.md
	if err := installer.WriteRules(rulesPath); err != nil {
		fmt.Fprintln(os.Stderr, "✗ rules.md:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Wrote rules to %s\n", rulesPath)

	// Step 4: inject CLAUDE.md
	if err := installer.InjectCLAUDEMD(claudeMDPath, rulesPath); err != nil {
		fmt.Fprintln(os.Stderr, "✗ CLAUDE.md:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Added @import to %s\n", claudeMDPath)

	// Step 5-7: scaffold project dir
	id, err := installer.ScaffoldProject(projectRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ project scaffold:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Initialized %s/.claude-context/ (project ID: %s)\n", projectRoot, id)

	// Step 8: auto-scan (build anatomy map)
	if !noScan {
		fmt.Fprintln(os.Stderr, "Running initial scan...")
		dispatchScan(nil)
	}

	fmt.Fprintln(os.Stderr, "\nBackups stored at *.bak."+ts)
	fmt.Fprintln(os.Stderr, "To verify: start a new Claude Code session and run `claude-context stats` after.")
	fmt.Fprintln(os.Stderr, "To uninstall: claude-context init --uninstall")
}

func backupFile(path, ts string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // nothing to back up
	}
	os.WriteFile(path+".bak."+ts, data, 0644)
}
