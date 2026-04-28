package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
)

func TestScheduler_PublishesCronTick(t *testing.T) {
	bus, _ := events.NewBus(t.TempDir(), &stubLogger{})
	defer bus.Close()

	sub, unsub := bus.Subscribe()
	defer unsub()

	deps := daemon.SchedulerDeps{
		LookupTask: func(name string) daemon.TaskFunc {
			return func(ctx context.Context, log daemon.Logger) error { return nil }
		},
	}
	sched := daemon.NewSchedulerWithDeps(t.TempDir(), &stubLogger{}, deps)
	sched.SetBus(bus)

	if err := sched.RunOnce(context.Background(), "noop"); err != nil {
		t.Fatal(err)
	}

	got := drain(sub, 100*time.Millisecond)
	types := []string{}
	for _, e := range got {
		types = append(types, e.Type)
	}
	if len(types) < 2 {
		t.Fatalf("expected at least 2 events (start+ok), got %v", types)
	}
}

func TestScheduler_PublishesFailureStatus(t *testing.T) {
	bus, _ := events.NewBus(t.TempDir(), &stubLogger{})
	defer bus.Close()
	sub, unsub := bus.Subscribe()
	defer unsub()

	deps := daemon.SchedulerDeps{
		LookupTask: func(name string) daemon.TaskFunc {
			return func(ctx context.Context, log daemon.Logger) error {
				return errors.New("kaboom")
			}
		},
		RetryDelayFn: func(d time.Duration, fn func()) daemon.CancelFn {
			// drop retries so the test finishes quickly
			return func() bool { return true }
		},
	}
	sched := daemon.NewSchedulerWithDeps(t.TempDir(), &stubLogger{}, deps)
	sched.SetBus(bus)
	_ = sched.RunOnce(context.Background(), "noop")

	got := drain(sub, 100*time.Millisecond)
	sawFailed := false
	for _, e := range got {
		if e.Type == "cron.tick" {
			// data contains "failed"
			if string(e.Data) != "" && contains(string(e.Data), `"failed"`) {
				sawFailed = true
			}
		}
	}
	if !sawFailed {
		t.Errorf("expected a cron.tick with status=failed, got %d events", len(got))
	}
}

func drain(ch <-chan events.Event, d time.Duration) []events.Event {
	out := []events.Event{}
	deadline := time.After(d)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, e)
		case <-deadline:
			return out
		}
	}
}
