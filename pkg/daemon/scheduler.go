package daemon

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// CancelFn cancels a pending retry. Returns true if cancellation prevented the
// retry from firing, false if it had already fired.
type CancelFn func() bool

// RetryDelayFn schedules fn to fire after d.
type RetryDelayFn func(d time.Duration, fn func()) CancelFn

// SchedulerDeps wires the scheduler to its dependencies.
type SchedulerDeps struct {
	LookupTask   func(name string) TaskFunc
	RetryDelayFn RetryDelayFn
}

var retryDelays = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
}

// Scheduler runs cron tasks from the manifest with retry + dead-letter.
type Scheduler struct {
	home string
	log  Logger
	deps SchedulerDeps

	mu      sync.Mutex
	cron    *cron.Cron
	pending map[string][]CancelFn
}

func NewScheduler(home string, log Logger) *Scheduler {
	return NewSchedulerWithDeps(home, log, SchedulerDeps{
		LookupTask: LookupTask,
		RetryDelayFn: func(d time.Duration, fn func()) CancelFn {
			t := time.AfterFunc(d, fn)
			return t.Stop
		},
	})
}

func NewSchedulerWithDeps(home string, log Logger, deps SchedulerDeps) *Scheduler {
	if deps.LookupTask == nil {
		deps.LookupTask = LookupTask
	}
	if deps.RetryDelayFn == nil {
		deps.RetryDelayFn = func(d time.Duration, fn func()) CancelFn {
			t := time.AfterFunc(d, fn)
			return t.Stop
		}
	}
	return &Scheduler{
		home:    home,
		log:     log,
		deps:    deps,
		pending: map[string][]CancelFn{},
	}
}

func (s *Scheduler) Start(ctx context.Context, m Manifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		return fmt.Errorf("scheduler already started")
	}

	c := cron.New(cron.WithLocation(time.Local))
	for _, t := range m.Tasks {
		if !t.Enabled {
			continue
		}
		if s.deps.LookupTask(t.Name) == nil {
			s.log.Warn("sched", fmt.Sprintf("manifest references unknown task %q; skipping", t.Name))
			continue
		}
		name := t.Name
		_, err := c.AddJob(t.Schedule, cron.NewChain(cron.SkipIfStillRunning(cronAdapter{s.log})).Then(cron.FuncJob(func() {
			_ = s.RunOnce(ctx, name)
		})))
		if err != nil {
			s.log.Warn("sched", fmt.Sprintf("invalid schedule for %s: %v", name, err))
		}
	}
	c.Start()
	s.cron = c
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		<-s.cron.Stop().Done()
		s.cron = nil
	}
	for _, cancels := range s.pending {
		for _, c := range cancels {
			c()
		}
	}
	s.pending = map[string][]CancelFn{}
}

func (s *Scheduler) RunOnce(ctx context.Context, name string) error {
	fn := s.deps.LookupTask(name)
	if fn == nil {
		return fmt.Errorf("unknown task: %s", name)
	}
	s.runAttempt(ctx, name, fn, 0)
	return nil
}

func (s *Scheduler) runAttempt(ctx context.Context, name string, fn TaskFunc, attempt int) {
	s.log.Info("sched", fmt.Sprintf("task=%s status=start attempt=%d", name, attempt+1))

	err := func() (rerr error) {
		defer func() {
			if r := recover(); r != nil {
				rerr = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
			}
		}()
		return fn(ctx, s.log)
	}()

	if err == nil {
		s.recordSuccess(name)
		s.log.Info("sched", fmt.Sprintf("task=%s status=ok attempt=%d", name, attempt+1))
		return
	}

	s.recordFailure(name, err)
	s.log.Warn("sched", fmt.Sprintf("task=%s status=fail attempt=%d err=%q", name, attempt+1, err.Error()))

	if attempt < len(retryDelays) {
		delay := retryDelays[attempt]
		next := attempt + 1
		cancel := s.deps.RetryDelayFn(delay, func() {
			s.runAttempt(ctx, name, fn, next)
		})
		s.mu.Lock()
		s.pending[name] = append(s.pending[name], cancel)
		s.mu.Unlock()
		return
	}

	s.recordDeadLetter(name)
	s.log.Error("sched", fmt.Sprintf("task=%s status=dead_lettered failures=%d", name, attempt+1))
}

func (s *Scheduler) recordSuccess(name string) {
	st, _ := LoadCronState(s.home)
	now := time.Now().UTC().Format(time.RFC3339)
	t := st.Tasks[name]
	t.LastRun = now
	t.LastSuccess = now
	t.LastError = ""
	t.ConsecutiveFailures = 0
	st.Tasks[name] = t
	_ = SaveCronState(s.home, st)
}

func (s *Scheduler) recordFailure(name string, err error) {
	st, _ := LoadCronState(s.home)
	now := time.Now().UTC().Format(time.RFC3339)
	t := st.Tasks[name]
	t.LastRun = now
	t.LastError = err.Error()
	t.ConsecutiveFailures++
	st.Tasks[name] = t
	_ = SaveCronState(s.home, st)
}

func (s *Scheduler) recordDeadLetter(name string) {
	st, _ := LoadCronState(s.home)
	t := st.Tasks[name]
	t.DeadLetteredAt = time.Now().UTC().Format(time.RFC3339)
	st.Tasks[name] = t
	_ = SaveCronState(s.home, st)
}

func (s *Scheduler) ClearDeadLetter(name string) error {
	st, err := LoadCronState(s.home)
	if err != nil {
		return err
	}
	t, ok := st.Tasks[name]
	if !ok {
		return fmt.Errorf("no state for task %s", name)
	}
	t.DeadLetteredAt = ""
	t.ConsecutiveFailures = 0
	st.Tasks[name] = t
	return SaveCronState(s.home, st)
}

// ValidateSchedule returns nil if the expression is parseable.
func ValidateSchedule(expr string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(expr)
	return err
}

type cronAdapter struct{ log Logger }

func (c cronAdapter) Info(msg string, keysAndValues ...interface{}) {
	c.log.Debug("cron", fmt.Sprintf(msg+" %v", keysAndValues))
}

func (c cronAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	c.log.Warn("cron", fmt.Sprintf("%s: %v", msg, err))
}
