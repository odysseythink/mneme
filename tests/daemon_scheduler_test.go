package tests

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
)

func TestScheduler_RunOnceSuccess(t *testing.T) {
	home := t.TempDir()
	var ran atomic.Int32
	taskFn := func(ctx context.Context, log daemon.Logger) error {
		ran.Add(1)
		return nil
	}
	sch := daemon.NewSchedulerWithDeps(home, &stubLogger{}, daemon.SchedulerDeps{
		LookupTask: func(name string) daemon.TaskFunc { return taskFn },
		RetryDelayFn: func(d time.Duration, fn func()) daemon.CancelFn {
			return func() bool { return true }
		},
	})

	if err := sch.RunOnce(context.Background(), "anatomy-rescan"); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if ran.Load() != 1 {
		t.Errorf("task ran %d times, want 1", ran.Load())
	}

	st, err := daemon.LoadCronState(home)
	if err != nil {
		t.Fatal(err)
	}
	if st.Tasks["anatomy-rescan"].LastSuccess == "" {
		t.Errorf("LastSuccess not recorded: %+v", st.Tasks["anatomy-rescan"])
	}
}

func TestScheduler_RetryThenDeadLetter(t *testing.T) {
	home := t.TempDir()
	var attempts atomic.Int32
	taskFn := func(ctx context.Context, log daemon.Logger) error {
		attempts.Add(1)
		return errors.New("boom")
	}

	syncRetry := func(d time.Duration, fn func()) daemon.CancelFn {
		fn()
		return func() bool { return false }
	}

	sch := daemon.NewSchedulerWithDeps(home, &stubLogger{}, daemon.SchedulerDeps{
		LookupTask:   func(name string) daemon.TaskFunc { return taskFn },
		RetryDelayFn: syncRetry,
	})

	_ = sch.RunOnce(context.Background(), "anatomy-rescan")

	if got := attempts.Load(); got != 4 {
		t.Errorf("attempts = %d, want 4", got)
	}
	st, err := daemon.LoadCronState(home)
	if err != nil {
		t.Fatal(err)
	}
	if st.Tasks["anatomy-rescan"].DeadLetteredAt == "" {
		t.Errorf("expected dead-letter, got %+v", st.Tasks["anatomy-rescan"])
	}
}

func TestScheduler_PanicCaughtAsError(t *testing.T) {
	home := t.TempDir()
	var attempts atomic.Int32
	taskFn := func(ctx context.Context, log daemon.Logger) error {
		attempts.Add(1)
		panic("oops")
	}

	noRetry := func(d time.Duration, fn func()) daemon.CancelFn {
		return func() bool { return true }
	}

	sch := daemon.NewSchedulerWithDeps(home, &stubLogger{}, daemon.SchedulerDeps{
		LookupTask:   func(name string) daemon.TaskFunc { return taskFn },
		RetryDelayFn: noRetry,
	})

	_ = sch.RunOnce(context.Background(), "anatomy-rescan")
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1 (panic should not loop)", attempts.Load())
	}
	st, _ := daemon.LoadCronState(home)
	if st.Tasks["anatomy-rescan"].LastError == "" {
		t.Errorf("panic should record LastError, got %+v", st.Tasks["anatomy-rescan"])
	}
}

func TestScheduler_RetryRetriesAreCancellable(t *testing.T) {
	home := t.TempDir()
	taskFn := func(ctx context.Context, log daemon.Logger) error {
		return errors.New("boom")
	}

	var sched []func()
	stash := func(d time.Duration, fn func()) daemon.CancelFn {
		sched = append(sched, fn)
		return func() bool { sched = nil; return true }
	}

	sch := daemon.NewSchedulerWithDeps(home, &stubLogger{}, daemon.SchedulerDeps{
		LookupTask:   func(name string) daemon.TaskFunc { return taskFn },
		RetryDelayFn: stash,
	})

	_ = sch.RunOnce(context.Background(), "anatomy-rescan")
	if len(sched) != 1 {
		t.Errorf("expected 1 retry scheduled, got %d", len(sched))
	}
}

func TestScheduler_ParsesScheduleExpressions(t *testing.T) {
	cases := []string{"@every 6h", "0 3 * * *", "30 3 * * 0"}
	for _, expr := range cases {
		if err := daemon.ValidateSchedule(expr); err != nil {
			t.Errorf("ValidateSchedule(%q): %v", expr, err)
		}
	}
	if err := daemon.ValidateSchedule("not-a-schedule"); err == nil {
		t.Errorf("ValidateSchedule should reject garbage")
	}
}
