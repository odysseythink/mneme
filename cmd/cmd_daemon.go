package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/daemonclient"
	"github.com/ranwei/mneme/pkg/state"
)

func dispatchDaemon(args []string) {
	if len(args) == 0 {
		printDaemonUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "start":
		runDaemonStart(args[1:])
	case "stop":
		runDaemonStop(args[1:])
	case "restart":
		runDaemonRestart(args[1:])
	case "status":
		runDaemonStatus(args[1:])
	case "logs":
		runDaemonLogs(args[1:])
	case "install":
		runDaemonInstall(args[1:])
	case "uninstall":
		runDaemonUninstall(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon subcommand: %q\n", args[0])
		printDaemonUsage()
		os.Exit(2)
	}
}

func printDaemonUsage() {
	fmt.Fprintln(os.Stderr, `usage: mneme daemon <start|stop|restart|status|logs|install|uninstall>`)
}

func runDaemonStart(args []string) {
	home, _ := os.UserHomeDir()
	cfg := config.FromEnv()

	tcp := ""
	if cfg.DaemonDashboardPortEnabled {
		tcp = "127.0.0.1:" + strconv.Itoa(cfg.DaemonDashboardPort)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		cancel()
	}()

	err := daemon.Run(ctx, daemon.RunOptions{
		Home:            home,
		PID:             os.Getpid(),
		Version:         daemonVersion(),
		LogLevel:        cfg.LogLevel,
		CronEnabled:     cfg.DaemonCronEnabled,
		TCPAddr:         tcp,
		ShutdownTimeout: time.Duration(cfg.DaemonShutdownTimeoutSeconds) * time.Second,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon:", err)
		os.Exit(1)
	}
}

func runDaemonStop(args []string) {
	home, _ := os.UserHomeDir()
	pidPath := state.DaemonPidPath(home)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon stop: no pid file (not running?)")
		os.Exit(1)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon stop: bad pid file:", err)
		os.Exit(1)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon stop:", err)
		os.Exit(1)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintln(os.Stderr, "daemon stop: signal:", err)
		os.Exit(1)
	}

	socketPath := state.DaemonSocketPath(home)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "daemon stopped")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "daemon stop: still running after 15s; check logs")
	os.Exit(2)
}

func runDaemonRestart(args []string) {
	runDaemonStop(args)
	time.Sleep(200 * time.Millisecond)
	runDaemonStart(args)
}

func runDaemonStatus(args []string) {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}
	home, _ := os.UserHomeDir()
	c, err := daemonclient.TryDial(home)
	if err != nil {
		if jsonOut {
			b, _ := json.Marshal(map[string]string{"status": "unreachable"})
			fmt.Println(string(b))
		} else {
			fmt.Println("daemon: unreachable")
		}
		os.Exit(2)
	}
	hb, err := c.Health(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon status:", err)
		os.Exit(1)
	}
	if jsonOut {
		b, _ := json.Marshal(hb)
		fmt.Println(string(b))
	} else {
		fmt.Printf("daemon ok pid=%d version=%s uptime_s=%d\n", hb.PID, hb.Version, hb.UptimeS)
	}
}

func runDaemonLogs(args []string) {
	follow := false
	date := time.Now().Format("20060102")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--follow":
			follow = true
		case "--date":
			if i+1 < len(args) {
				date = args[i+1]
				i++
			}
		}
	}
	home, _ := os.UserHomeDir()
	path := state.DaemonLogPath(home, date)
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon logs: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			fmt.Print(line)
		}
		if err != nil {
			if !follow {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func daemonVersion() string {
	return version
}

func runDaemonInstall(args []string) {
	if err := daemon.Install(daemonInstallHome()); err != nil {
		fmt.Fprintln(os.Stderr, "daemon install:", err)
		os.Exit(1)
	}
	fmt.Println("daemon install: ok")
}

func runDaemonUninstall(args []string) {
	if err := daemon.Uninstall(daemonInstallHome()); err != nil {
		fmt.Fprintln(os.Stderr, "daemon uninstall:", err)
		os.Exit(1)
	}
	fmt.Println("daemon uninstall: ok")
}

func daemonInstallHome() string {
	h, _ := os.UserHomeDir()
	return filepath.Clean(h)
}
