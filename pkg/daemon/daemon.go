package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

// RunOptions configures a daemon process.
type RunOptions struct {
	Home              string
	PID               int
	Version           string
	LogLevel          string
	CronEnabled       bool
	TCPAddr           string
	ShutdownTimeout   time.Duration
	HeartbeatInterval time.Duration
	DevMode           bool // M10a: enables /dev-token endpoint
}

// Run is the daemon entrypoint. Blocks until ctx is cancelled.
func Run(ctx context.Context, opt RunOptions) error {
	if opt.Home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		opt.Home = h
	}
	if opt.ShutdownTimeout == 0 {
		opt.ShutdownTimeout = 10 * time.Second
	}
	if opt.HeartbeatInterval == 0 {
		opt.HeartbeatInterval = 30 * time.Minute
	}

	if err := os.MkdirAll(state.DaemonDir(opt.Home), 0o700); err != nil {
		return err
	}

	pidPath := state.DaemonPidPath(opt.Home)
	pidLock := flock.New(pidPath)
	locked, err := pidLock.TryLock()
	if err != nil {
		return fmt.Errorf("pid lock: %w", err)
	}
	if !locked {
		if data, _ := os.ReadFile(pidPath); len(data) > 0 {
			if pid, perr := strconv.Atoi(string(data)); perr == nil && pid > 0 {
				if proc, perr := os.FindProcess(pid); perr == nil {
					if perr := proc.Signal(syscall.Signal(0)); perr == nil {
						return fmt.Errorf("daemon already running (pid %d)", pid)
					}
				}
			}
		}
		_ = os.Remove(pidPath)
		_ = os.Remove(state.DaemonSocketPath(opt.Home))
		locked, err = pidLock.TryLock()
		if err != nil || !locked {
			return fmt.Errorf("pid lock retry: %w", err)
		}
	}
	defer func() {
		_ = pidLock.Unlock()
		_ = os.Remove(pidPath)
	}()

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(opt.PID)), 0o600); err != nil {
		return err
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	if err := os.WriteFile(state.DaemonTokenPath(opt.Home), []byte(token), 0o600); err != nil {
		return err
	}

	log, err := NewFileLogger(opt.Home, opt.LogLevel, time.Now)
	if err != nil {
		return err
	}
	defer log.Close()
	log.Info("daemon", fmt.Sprintf("starting pid=%d version=%s", opt.PID, opt.Version))

	manifest, err := LoadOrSeedManifest(opt.Home)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	sched := NewScheduler(opt.Home, log)
	if opt.CronEnabled {
		if err := sched.Start(ctx, manifest); err != nil {
			return fmt.Errorf("scheduler start: %w", err)
		}
	}
	defer sched.Stop()

	bus, err := events.NewBus(opt.Home, log)
	if err != nil {
		return fmt.Errorf("event bus: %w", err)
	}
	defer bus.Close()
	sched.SetBus(bus)

	startedAt := time.Now().Unix()
	mux := NewMux(RouteDeps{
		Log:       log,
		Home:      opt.Home,
		PID:       opt.PID,
		Version:   opt.Version,
		StartedAt: startedAt,
		Scheduler: sched,
		Token:     token,
		DevMode:   opt.DevMode,
		Bus:       bus,
	})
	srv := NewServer(ServerConfig{
		SocketPath:      state.DaemonSocketPath(opt.Home),
		TCPAddr:         opt.TCPAddr,
		Token:           token,
		Log:             log,
		Mux:             mux,
		ShutdownTimeout: opt.ShutdownTimeout,
	})

	var hbWG sync.WaitGroup
	hbWG.Add(1)
	go func() {
		defer hbWG.Done()
		RunHeartbeat(ctx, opt.Home, opt.PID, opt.Version, opt.HeartbeatInterval)
	}()

	go runReloadHandler(ctx,
		func() {
			log.Info("daemon", "SIGHUP received: reloading manifest")
			fresh, lerr := LoadManifest(opt.Home)
			if lerr != nil {
				log.Warn("daemon", "manifest reload failed: "+lerr.Error())
				return
			}
			sched.Stop()
			if opt.CronEnabled {
				if err := sched.Start(ctx, fresh); err != nil {
					log.Warn("daemon", "scheduler restart failed: "+err.Error())
				}
			}
		},
		func() {
			log.Info("daemon", "SIGUSR1 received: rotating log")
			_ = log.Rotate()
		},
	)

	runErr := srv.Run(ctx)
	hbWG.Wait()
	srv.Wait()

	log.Info("daemon", "stopped")
	return runErr
}
