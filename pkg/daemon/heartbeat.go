package daemon

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

// WriteHeartbeat atomically writes ~/.mneme/daemon/heartbeat.json.
func WriteHeartbeat(home string, pid int, version string, clock func() time.Time) error {
	if clock == nil {
		clock = time.Now
	}
	hb := struct {
		TS      string `json:"ts"`
		PID     int    `json:"pid"`
		Version string `json:"version"`
	}{
		TS:      clock().UTC().Format(time.RFC3339),
		PID:     pid,
		Version: version,
	}
	data, err := json.MarshalIndent(hb, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(state.DaemonHeartbeatPath(home), data)
}

// RunHeartbeat writes the heartbeat once on start, then every interval until ctx is cancelled.
func RunHeartbeat(ctx context.Context, home string, pid int, version string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	tickCh := make(chan time.Time)
	go func() {
		defer close(tickCh)
		tickCh <- time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				tickCh <- t
			}
		}
	}()
	RunHeartbeatWithTicks(ctx, home, pid, version, tickCh)
}

// RunHeartbeatWithTicks consumes a tick channel (used by tests for deterministic firing).
func RunHeartbeatWithTicks(ctx context.Context, home string, pid int, version string, tickCh <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-tickCh:
			if !ok {
				return
			}
			clock := func() time.Time { return t }
			_ = WriteHeartbeat(home, pid, version, clock)
		}
	}
}
