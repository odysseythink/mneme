//go:build windows

package daemon

import "context"

// runReloadHandler is a no-op on Windows.
func runReloadHandler(ctx context.Context, _ func(), _ func()) {
	<-ctx.Done()
}
