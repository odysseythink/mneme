package main

import (
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/dashboard"
)

func dispatchDashboard(args []string) {
	home, _ := os.UserHomeDir()
	cfg := config.FromEnv()

	deps := dashboard.CLIDeps{
		UnixSocket: filepath.Join(home, ".mneme", "daemon", "socket"),
		TCPHost:    "127.0.0.1",
		TCPPort:    cfg.DaemonDashboardPort,
		TokenPath:  filepath.Join(home, ".mneme", "daemon", "token"),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
	os.Exit(dashboard.RunCLI(deps, args))
}
