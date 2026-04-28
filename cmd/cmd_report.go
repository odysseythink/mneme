package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

func dispatchReport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme report <waste>")
		os.Exit(2)
	}
	switch args[0] {
	case "waste":
		runReportWaste(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown report subcommand: %q\n", args[0])
		os.Exit(2)
	}
}

func runReportWaste(args []string) {
	fs := flag.NewFlagSet("report waste", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print report to stdout instead of writing")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}
	home, _ := os.UserHomeDir()
	cfg := config.FromEnv()

	out, err := waste.GenerateReport(waste.ReportInput{
		ProjectRoot: root,
		HomeDir:     home,
		Now:         time.Now().UTC(),
		Threshold:   float64(cfg.WasteThresholdPercent) / 100.0,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ generate report:", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Print(waste.RenderMarkdown(out))
		return
	}
	path, err := waste.WriteReport(root, out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ write report:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ wrote %s\n", path)
}
