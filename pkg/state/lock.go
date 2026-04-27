package state

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// AcquireLock acquires an exclusive file lock on lockPath with a timeout.
// Returns a release function. Caller must call release() when done.
func AcquireLock(lockPath string, timeout time.Duration) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("lock dir: %w", err)
	}
	fl := flock.New(lockPath)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	locked, err := fl.TryLockContext(ctx, 5*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("lock acquire: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("lock timeout after %v: %s", timeout, lockPath)
	}
	return func() { fl.Unlock() }, nil
}
