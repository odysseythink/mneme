package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

type daemonStatus struct {
	Running bool   `json:"running"`
	Note    string `json:"note,omitempty"`
}

type statusReport struct {
	ProjectID       string         `json:"project_id"`
	ProjectRoot     string         `json:"project_root"`
	AnatomyAge      string         `json:"anatomy_age,omitempty"`
	AnatomyMissing  bool           `json:"anatomy_missing,omitempty"`
	Daemon          daemonStatus   `json:"daemon"`
	HookFiredTotals map[string]int `json:"hook_fired_totals,omitempty"`
	LastHookUpdate  string         `json:"last_hook_update,omitempty"`
	MnemeVersion    string         `json:"mneme_version"`
	ReleaseChannel  string         `json:"release_channel"`
}

func dispatchStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "status: cannot get cwd:", err)
		os.Exit(1)
	}
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "status: no mneme project here (run: mneme init)")
		os.Exit(1)
	}

	report := buildStatusReport(root)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "status: encode:", err)
			os.Exit(1)
		}
		return
	}
	printStatusHuman(report)
}

func buildStatusReport(root string) statusReport {
	r := statusReport{
		ProjectRoot:    root,
		MnemeVersion:   version,
		ReleaseChannel: releaseChannel,
		Daemon:         probeDaemon(),
	}
	id, _ := state.ReadOrCreateLocalID(root)
	r.ProjectID = id

	if generated, err := state.ReadAnatomyGeneratedTime(root); err == nil {
		r.AnatomyAge = time.Since(generated).Round(time.Second).String()
	} else {
		r.AnatomyMissing = true
	}

	if l, err := state.ReadLedger(root); err == nil {
		r.HookFiredTotals = l.Totals.HookFired
		r.LastHookUpdate = l.LastUpdated
	}
	return r
}

func probeDaemon() daemonStatus {
	home, _ := os.UserHomeDir()
	socket := filepath.Join(home, ".mneme", "daemon", "socket")
	conn, err := net.DialTimeout("unix", socket, 200*time.Millisecond)
	if err != nil {
		return daemonStatus{Running: false, Note: "no daemon socket — start with `mneme daemon start` (M8+)"}
	}
	conn.Close()
	return daemonStatus{Running: true}
}

func printStatusHuman(r statusReport) {
	fmt.Printf("project: %s\n", r.ProjectID)
	fmt.Printf("root:    %s\n", r.ProjectRoot)
	fmt.Printf("mneme:   %s (%s channel)\n", r.MnemeVersion, r.ReleaseChannel)
	fmt.Println()
	if r.AnatomyMissing {
		fmt.Println("anatomy.md: missing (run: mneme scan)")
	} else {
		fmt.Printf("anatomy.md age: %s\n", r.AnatomyAge)
	}
	fmt.Println()
	if r.Daemon.Running {
		fmt.Println("daemon: running")
	} else {
		fmt.Printf("daemon: %s\n", r.Daemon.Note)
	}
	if len(r.HookFiredTotals) > 0 {
		fmt.Println()
		fmt.Println("hook fires:")
		for _, k := range []string{"pre-read", "pre-write", "post-tool-use", "session-start", "stop"} {
			fmt.Printf("  %-15s %d\n", k+":", r.HookFiredTotals[k])
		}
		if r.LastHookUpdate != "" {
			fmt.Printf("last update:    %s\n", r.LastHookUpdate)
		}
	}
}
