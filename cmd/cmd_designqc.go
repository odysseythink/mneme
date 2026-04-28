package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ranwei/mneme/pkg/designqc"
	"github.com/ranwei/mneme/pkg/state"
)

type stringSlice []string

func (s *stringSlice) String() string     { return fmt.Sprint(*s) }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

func dispatchDesignQC(args []string) {
	fs := flag.NewFlagSet("designqc", flag.ExitOnError)
	var routes stringSlice
	fs.Var(&routes, "route", "Capture only this route (repeatable). Skips auto-detection.")
	port := fs.Int("port", 0, "Override dev server port (0 = framework default).")
	quality := fs.Int("quality", 80, "JPEG quality 1-100.")
	maxWidth := fs.Int("max-width", 1440, "Browser viewport width.")
	asJSON := fs.Bool("json", false, "Print report.json to stdout instead of human summary.")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		os.Exit(1)
	}
	root, ok := state.FindGitRoot(cwd)
	if !ok {
		root = cwd
	}
	if _, err := os.Stat(root + "/.mneme"); err != nil {
		fmt.Fprintln(os.Stderr, "designqc: no .mneme/ directory; run `mneme init` first")
		os.Exit(1)
	}
	id, err := state.ReadOrCreateLocalID(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		os.Exit(1)
	}

	report, err := designqc.Run(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     id,
		HomeDir:       home,
		RouteOverride: routes,
		Quality:       *quality,
		MaxWidth:      *maxWidth,
		Port:          *port,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "designqc:", err)
		if _, derr := designqc.DetectChrome(); derr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}

	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		return
	}
	fmt.Printf("mneme designqc — %s project at %s\n", report.Framework, root)
	fmt.Printf("captures: %d routes\n", len(report.Captures))
	for _, c := range report.Captures {
		if c.Error != "" {
			fmt.Printf("  %-10s → ERROR: %s\n", c.Route, c.Error)
			continue
		}
		fmt.Printf("  %-10s → captures/%s (%d×%d)\n", c.Route, c.File, c.Width, c.Height)
	}
	fmt.Printf("report:    %s/.mneme/designqc/%s/report.json\n", home, id)
}
