package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/updater"
)

func dispatchUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	binaryFlag := fs.Bool("binary", false, "self-update the mneme binary from GitHub Releases")
	listFlag := fs.Bool("list", false, "list known projects without acting")
	dryRun := fs.Bool("dry-run", false, "show what would be updated without writing")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	project := fs.String("project", "", "limit sync to a single project ID")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	_ = yes

	if *binaryFlag {
		runBinaryUpdate()
		return
	}
	if *listFlag {
		runListProjects()
		return
	}

	res, err := updater.SyncAll(updater.SyncOptions{
		DryRun:    *dryRun,
		ProjectID: *project,
		Verbose:   true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "update:", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Printf("\nDry run: %d would update.\n", res.WouldUpdate)
		return
	}
	fmt.Printf("\nDone: %d updated, %d skipped, %d failed.\n", res.Updated, res.Skipped, res.Failed)
	for _, f := range res.Failures {
		fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", f.Path, f.Err)
	}
	if res.Failed > 0 {
		os.Exit(1)
	}
}

func runBinaryUpdate() {
	if releaseChannel != "github" || releaseRepo == "" {
		fmt.Fprintln(os.Stderr, "no GitHub release configured for this build.")
		fmt.Fprintln(os.Stderr, "Build with: -ldflags=\"-X main.releaseChannel=github -X main.releaseRepo=owner/mneme\"")
		os.Exit(1)
	}
	tmpPath, ver, err := updater.DownloadLatestBinary(updater.DefaultGitHubAPIBase, releaseRepo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "update --binary:", err)
		os.Exit(1)
	}
	target, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "update --binary: locate self:", err)
		os.Exit(1)
	}
	if err := updater.ReplaceBinary(tmpPath, target); err != nil {
		fmt.Fprintln(os.Stderr, "update --binary: replace:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Updated mneme to %s\n", ver)
	fmt.Println("Now running template sync …")
	res, err := updater.SyncAll(updater.SyncOptions{Verbose: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "post-update sync:", err)
		os.Exit(1)
	}
	fmt.Printf("\nDone: %d updated, %d skipped.\n", res.Updated, res.Skipped)
}

func runListProjects() {
	origins, err := state.ListOrigins()
	if err != nil {
		fmt.Fprintln(os.Stderr, "list:", err)
		os.Exit(1)
	}
	if len(origins) == 0 {
		fmt.Println("No initialized projects.")
		return
	}
	fmt.Printf("%-40s  template-version  origin\n", "project-id")
	for id, root := range origins {
		v, _ := state.ReadTemplateVersion(root)
		fmt.Printf("%-40s  %-16d  %s\n", id, v, root)
	}
}
