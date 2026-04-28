package main

import (
	"context"
	"os"
	"time"

	"github.com/ranwei/mneme/pkg/daemonclient"
)

// publishHookFired is best-effort. If the daemon is not running the call
// is silently dropped — the local-state increment in the caller is the
// authoritative record. Time-boxed at 200 ms to keep hook startup fast.
func publishHookFired(hookName, projectID string, extra map[string]interface{}) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	c, err := daemonclient.TryDial(home)
	if err != nil {
		return
	}
	data := map[string]interface{}{"hook": hookName}
	for k, v := range extra {
		data[k] = v
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = c.PublishEvent(ctx, "hook.fired", projectID, data)
}
