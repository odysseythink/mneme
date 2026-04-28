//go:build !windows

package daemon

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// runReloadHandler watches SIGHUP and SIGUSR1 and dispatches to callbacks.
func runReloadHandler(ctx context.Context, onHUP, onUSR1 func()) {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGUSR1)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				if onHUP != nil {
					onHUP()
				}
			case syscall.SIGUSR1:
				if onUSR1 != nil {
					onUSR1()
				}
			}
		}
	}
}
