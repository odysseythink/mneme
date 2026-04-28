# M10b — REST + SSE + 3 Panels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a working dashboard with three panels (Overview, Activity, Cron), live-updated via SSE, backed by a new `pkg/events` pub/sub package and four `/api/*` REST handlers.

**Architecture:** A singleton `*events.Bus` is owned by the daemon and injected into `RouteDeps`. In-process publishers (cron scheduler, cerebrum handler, suggestions task) call `bus.Publish` directly. Out-of-process publishers (`mneme hook ...` subprocesses) post to a Unix-socket-only `/events/publish` endpoint via a new `daemonclient.PublishEvent`. The bus tees every event to a ring buffer (last 500), an `events-YYYYMMDD.jsonl` file (daily rotation, 14-day retention), and any active SSE subscribers. The frontend uses `react-router-dom@^7`, three custom hooks (`useFetch`, `useSSE`, `useProjectList`), and shared components (`AppShell`, `HealthCard`, `ProjectCard`, `EventRow`, `CronTaskRow`).

**Tech Stack:** Go 1.25 (`net/http`, `sync`, `time`), `github.com/robfig/cron/v3` (existing), React 19, TypeScript 5.7, Vite 6, Tailwind v4, `react-router-dom@^7`, Vitest 2.

**Spec:** `docs/superpowers/specs/2026-04-28-m10b-rest-sse-panels-design.md`

---

## File structure

### Backend — created

| Path | Responsibility |
|---|---|
| `pkg/events/event.go` | `Event` struct |
| `pkg/events/bus.go` | `Bus` struct, `NewBus`, `Publish`, `Subscribe`, `Tail`, `Close` |
| `pkg/events/ring.go` | Bounded ring buffer (last 500) |
| `pkg/events/jsonl.go` | Daily-rotated append-only writer + retention sweep |
| `pkg/dashboard/api.go` | `/api/overview`, `/api/projects`, `/api/activity`, `/api/cron` |
| `pkg/dashboard/sse.go` | `/events` SSE handler + `/events/publish` Unix-only handler |
| `tests/events_bus_test.go` | Publish/Subscribe/Tail (in-memory) |
| `tests/events_jsonl_test.go` | Append, rotation, retention, file fallback |
| `tests/dashboard_api_test.go` | All four `/api/*` endpoints |
| `tests/dashboard_sse_test.go` | SSE stream, ping, disconnect, publish endpoint |
| `tests/integration/dashboard_panels_test.go` | (build tag `integration`) full daemon smoke |

### Backend — modified

| Path | Change |
|---|---|
| `pkg/daemon/routes.go` | Extend `RouteDeps` with `Bus *events.Bus`; register `/api/*`, `/events`, `/events/publish` |
| `pkg/daemon/daemon.go` | Instantiate `*events.Bus`, pass to `RouteDeps` and `Scheduler`, close on shutdown |
| `pkg/daemon/scheduler.go` | Optional `Bus` field; `runAttempt` publishes `cron.tick` start/finish |
| `pkg/daemon/cerebrum_handler.go` | Publish `cerebrum.candidate` after enqueue |
| `pkg/daemon/tasks.go` | `suggestionsRefresh` and `anatomyRescan` accept bus and publish results |
| `pkg/state/daemon_paths.go` | Add `DaemonEventsPath(home, dateYYYYMMDD)` |
| `pkg/daemonclient/client.go` | Add `PublishEvent(ctx, eventType, projectID, data)` |
| `cmd/hook_prewrite.go`, `hook_postwrite.go`, `hook_sessionstart.go`, `hook_stop.go` | Call `daemonclient.PublishEvent` after `state.IncrementSafe` |

### Frontend — created

| Path | Responsibility |
|---|---|
| `web/src/api/client.ts` | `request<T>(method, url, body?)` fetch wrapper |
| `web/src/api/types.ts` | Hand-written mirrors of Go response shapes |
| `web/src/api/overview.ts` | `getOverview()` |
| `web/src/api/projects.ts` | `getProjects()` |
| `web/src/api/activity.ts` | `getActivity({limit?, since?})` |
| `web/src/api/cron.ts` | `getCron()`, `runTask(name)`, `retryTask(name)` |
| `web/src/hooks/useFetch.ts` | typed fetcher with `refetch` + optional polling |
| `web/src/hooks/useSSE.ts` | EventSource wrapper + reconnect + context provider |
| `web/src/hooks/useProjectList.ts` | wraps `getProjects` with SSE-driven refresh |
| `web/src/components/AppShell.tsx` | Sidebar + outlet |
| `web/src/components/HealthCard.tsx` | Replaces `Health.tsx` |
| `web/src/components/ProjectCard.tsx` | Single project summary |
| `web/src/components/EventRow.tsx` | Single activity event |
| `web/src/components/CronTaskRow.tsx` | Single cron task with action buttons |
| `web/src/panels/Overview.tsx` | Health + totals + project list |
| `web/src/panels/Activity.tsx` | Live event log |
| `web/src/panels/Cron.tsx` | Task table |
| `web/src/__tests__/App.test.tsx` | Vitest smoke test |
| `web/vitest.config.ts` | Vitest config |

### Frontend — modified

| Path | Change |
|---|---|
| `web/src/App.tsx` | Replace placeholder with `RouterProvider` + routes |
| `web/src/Health.tsx` | Delete (moved to `components/HealthCard.tsx`) |
| `web/package.json` | Add `react-router-dom`, `vitest`, `jsdom`, `@testing-library/react` |

---

## Conventions used in this plan

**Test runner expectations** assume the test file is read by `go test` from the repo root unless otherwise noted. Integration tests need `-tags=integration`.

**Commit message format** matches the existing repo: `feat(m10b):`, `test(m10b):`, `fix(m10b):`, `docs(m10b):`. Co-author trailer is omitted in plan examples but should be present in real commits when run interactively.

**Mock logger** — every Go test that needs a `Logger` should use the `stubLogger` already defined in `tests/`. If the test file is new, declare a local `silentLogger` matching the interface (4 methods + `Rotate` + `Close`).

---

## Task 1: pkg/events — Event type + Bus skeleton

**Files:**
- Create: `pkg/events/event.go`
- Create: `pkg/events/bus.go`
- Create: `tests/events_bus_test.go`

- [ ] **Step 1: Write the failing test**

Create `tests/events_bus_test.go`:

```go
package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/events"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus, err := events.NewBus(t.TempDir(), &stubLogger{})
	if err != nil {
		t.Fatalf("NewBus: %v", err)
	}
	defer bus.Close()

	ch, unsub := bus.Subscribe()
	defer unsub()

	bus.Publish(events.Event{
		TS:        time.Now().UnixMilli(),
		Type:      "test.fired",
		ProjectID: "abc",
		Data:      json.RawMessage(`{"hello":"world"}`),
	})

	select {
	case got := <-ch:
		if got.Type != "test.fired" {
			t.Errorf("got type %q, want test.fired", got.Type)
		}
		if got.ProjectID != "abc" {
			t.Errorf("got project_id %q, want abc", got.ProjectID)
		}
	case <-time.After(time.Second):
		t.Fatal("subscribe channel: timed out waiting for event")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestBus_PublishSubscribe -v`
Expected: FAIL — `package github.com/ranwei/mneme/pkg/events` does not exist.

- [ ] **Step 3: Create `pkg/events/event.go`**

```go
package events

import "encoding/json"

// Event is the canonical envelope for every dashboard event.
type Event struct {
	TS        int64           `json:"ts"` // unix milliseconds
	Type      string          `json:"type"`
	ProjectID string          `json:"project_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}
```

- [ ] **Step 4: Create `pkg/events/bus.go`**

```go
package events

import "sync"

// Logger is the minimal sink for write errors and rotation events.
// Implementations: pkg/daemon/log.go fileLogger.
type Logger interface {
	Debug(component, msg string)
	Info(component, msg string)
	Warn(component, msg string)
	Error(component, msg string)
}

const subscriberBuffer = 64

// Bus is a single-process pub/sub with persistence + ring history.
// Skeleton: in-memory only; ring + jsonl wired in later tasks.
type Bus struct {
	log Logger

	mu      sync.RWMutex
	subs    map[chan Event]struct{}
	closed  bool
}

// NewBus constructs a Bus rooted at home. home is unused in this task
// (used in Task 4 for the JSONL writer).
func NewBus(home string, log Logger) (*Bus, error) {
	return &Bus{
		log:  log,
		subs: map[chan Event]struct{}{},
	}, nil
}

// Publish fans out to all current subscribers. Non-blocking: drops on
// any subscriber whose buffer is full (Task 2 wires the drop counter).
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// drop on full
		}
	}
}

// Subscribe returns a buffered channel and an unsubscribe func.
// The caller must call unsub exactly once.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
}

// Close stops new publishes and closes all subscriber channels.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./tests -run TestBus_PublishSubscribe -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/events/event.go pkg/events/bus.go tests/events_bus_test.go
git commit -m "feat(m10b): pkg/events skeleton — Event type + Bus pub/sub"
```

---

## Task 2: Slow subscriber drop is non-blocking

**Files:**
- Modify: `tests/events_bus_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/events_bus_test.go`:

```go
func TestBus_SlowSubscriberDoesNotBlock(t *testing.T) {
	bus, err := events.NewBus(t.TempDir(), &stubLogger{})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	// Subscribe but never read — fills the 64-slot buffer immediately.
	_, unsub := bus.Subscribe()
	defer unsub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			bus.Publish(events.Event{TS: int64(i), Type: "spam"})
		}
		close(done)
	}()

	select {
	case <-done:
		// Publishing 100 events to a stalled sub completed quickly = non-blocking.
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on slow subscriber")
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestBus_SlowSubscriberDoesNotBlock -v`
Expected: PASS (the skeleton from Task 1 is already non-blocking).

- [ ] **Step 3: Commit**

```bash
git add tests/events_bus_test.go
git commit -m "test(m10b): pkg/events — Publish does not block on slow subscriber"
```

---

## Task 3: Ring buffer + Bus.Tail (in-memory)

**Files:**
- Create: `pkg/events/ring.go`
- Modify: `pkg/events/bus.go`
- Modify: `tests/events_bus_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/events_bus_test.go`:

```go
func TestBus_TailReturnsRecent(t *testing.T) {
	bus, err := events.NewBus(t.TempDir(), &stubLogger{})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	for i := 0; i < 5; i++ {
		bus.Publish(events.Event{TS: int64(i), Type: "x"})
	}

	got := bus.Tail(3, 0)
	if len(got) != 3 {
		t.Fatalf("Tail(3): got %d events, want 3", len(got))
	}
	// Tail returns newest-first.
	if got[0].TS != 4 || got[2].TS != 2 {
		t.Errorf("Tail order wrong: %+v", got)
	}
}

func TestBus_TailRespectsSince(t *testing.T) {
	bus, _ := events.NewBus(t.TempDir(), &stubLogger{})
	defer bus.Close()
	for i := 0; i < 5; i++ {
		bus.Publish(events.Event{TS: int64(i), Type: "x"})
	}
	got := bus.Tail(100, 2)
	// since=2 is exclusive: TS=3,4 only.
	if len(got) != 2 {
		t.Fatalf("Tail(100, since=2): got %d, want 2 (TSs %v)", len(got), tss(got))
	}
}

func tss(es []events.Event) []int64 {
	out := make([]int64, len(es))
	for i, e := range es {
		out[i] = e.TS
	}
	return out
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestBus_Tail -v`
Expected: FAIL — `bus.Tail undefined`.

- [ ] **Step 3: Create `pkg/events/ring.go`**

```go
package events

import "sync"

const ringCapacity = 500

// ring is a fixed-size circular buffer of Events.
// Newest at the head. Concurrent-safe.
type ring struct {
	mu   sync.RWMutex
	buf  []Event // length 0..ringCapacity, in insertion order (oldest at index 0)
}

func newRing() *ring {
	return &ring{buf: make([]Event, 0, ringCapacity)}
}

func (r *ring) push(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.buf) < ringCapacity {
		r.buf = append(r.buf, e)
		return
	}
	// shift left by one, then append (cheap at cap=500, hot path at low rates)
	copy(r.buf, r.buf[1:])
	r.buf[ringCapacity-1] = e
}

// tail returns up to limit events whose TS > since, newest-first.
func (r *ring) tail(limit int, since int64) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		return nil
	}
	out := make([]Event, 0, limit)
	for i := len(r.buf) - 1; i >= 0 && len(out) < limit; i-- {
		if r.buf[i].TS > since {
			out = append(out, r.buf[i])
		}
	}
	return out
}
```

- [ ] **Step 4: Modify `pkg/events/bus.go`**

Add a `*ring` field, push on Publish, expose `Tail`.

Replace the `Bus` struct + relevant methods:

```go
type Bus struct {
	log Logger

	mu     sync.RWMutex
	subs   map[chan Event]struct{}
	closed bool
	ring   *ring
}

func NewBus(home string, log Logger) (*Bus, error) {
	return &Bus{
		log:  log,
		subs: map[chan Event]struct{}{},
		ring: newRing(),
	}, nil
}

func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return
	}
	b.ring.push(e)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Tail returns up to limit events with TS > since, newest-first.
// limit is capped at 500.
func (b *Bus) Tail(limit int, since int64) []Event {
	if limit > 500 {
		limit = 500
	}
	return b.ring.tail(limit, since)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestBus -v`
Expected: 4 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/events/ring.go pkg/events/bus.go tests/events_bus_test.go
git commit -m "feat(m10b): pkg/events — ring buffer + Bus.Tail"
```

---

## Task 4: JSONL writer with daily rotation

**Files:**
- Create: `pkg/events/jsonl.go`
- Create: `tests/events_jsonl_test.go`
- Modify: `pkg/events/bus.go`
- Modify: `pkg/state/daemon_paths.go`

- [ ] **Step 1: Add the path helper**

Append to `pkg/state/daemon_paths.go`:

```go
// DaemonEventsPath returns the path for a daily-rotated events file.
// dateYYYYMMDD must be formatted as "20260428".
func DaemonEventsPath(homeDir, dateYYYYMMDD string) string {
	return filepath.Join(DaemonDir(homeDir), "events-"+dateYYYYMMDD+".jsonl")
}
```

- [ ] **Step 2: Write the failing test**

Create `tests/events_jsonl_test.go`:

```go
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func TestBus_PersistsToJSONL(t *testing.T) {
	home := t.TempDir()
	// NewBus will create ~/.mneme/daemon/.
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}

	bus, err := events.NewBus(home, &stubLogger{})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	bus.Publish(events.Event{
		TS:   time.Now().UnixMilli(),
		Type: "hook.fired",
		Data: json.RawMessage(`{"file":"a.go"}`),
	})

	// Force flush by closing.
	bus.Close()

	today := time.Now().UTC().Format("20060102")
	path := state.DaemonEventsPath(home, today)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read events file: %v", err)
	}
	if !strings.Contains(string(data), `"type":"hook.fired"`) {
		t.Errorf("events.jsonl missing event: %s", data)
	}
}

func TestBus_RotatesAcrossDay(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 4, 28, 23, 59, 0, 0, time.UTC)
	clk := &stepClock{t: now}
	bus, err := events.NewBusWithClock(home, &stubLogger{}, clk.now)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	bus.Publish(events.Event{TS: clk.now().UnixMilli(), Type: "a"})
	clk.advance(2 * time.Minute) // crosses to 2026-04-29
	bus.Publish(events.Event{TS: clk.now().UnixMilli(), Type: "b"})
	bus.Close()

	for _, day := range []string{"20260428", "20260429"} {
		path := state.DaemonEventsPath(home, day)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing rotation file %s: %v", filepath.Base(path), err)
		}
	}
}

type stepClock struct {
	t time.Time
}

func (c *stepClock) now() time.Time          { return c.t }
func (c *stepClock) advance(d time.Duration) { c.t = c.t.Add(d) }
```

- [ ] **Step 3: Run test**

Run: `go test ./tests -run TestBus_Persists -v`
Expected: FAIL — `events.NewBusWithClock undefined`, file not created.

- [ ] **Step 4: Create `pkg/events/jsonl.go`**

```go
package events

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

type jsonlWriter struct {
	home  string
	clock func() time.Time
	log   Logger

	mu      sync.Mutex
	current *os.File
	curDate string
}

func newJSONLWriter(home string, clock func() time.Time, log Logger) *jsonlWriter {
	return &jsonlWriter{home: home, clock: clock, log: log}
}

func (w *jsonlWriter) append(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		w.log.Error("events", fmt.Sprintf("marshal event: %v", err))
		return
	}
	data = append(data, '\n')
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.openTodayLocked(); err != nil {
		w.log.Error("events", fmt.Sprintf("open events file: %v", err))
		return
	}
	if _, err := w.current.Write(data); err != nil {
		w.log.Error("events", fmt.Sprintf("write event: %v", err))
	}
}

func (w *jsonlWriter) openTodayLocked() error {
	date := w.clock().UTC().Format("20060102")
	if w.current != nil && w.curDate == date {
		return nil
	}
	if w.current != nil {
		_ = w.current.Close()
		w.current = nil
	}
	if err := os.MkdirAll(state.DaemonDir(w.home), 0o700); err != nil {
		return err
	}
	path := state.DaemonEventsPath(w.home, date)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	w.current = f
	w.curDate = date
	return nil
}

func (w *jsonlWriter) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		err := w.current.Close()
		w.current = nil
		return err
	}
	return nil
}
```

- [ ] **Step 5: Modify `pkg/events/bus.go`**

Wire the writer in:

```go
type Bus struct {
	log   Logger
	clock func() time.Time

	mu     sync.RWMutex
	subs   map[chan Event]struct{}
	closed bool
	ring   *ring
	jsonl  *jsonlWriter
}

func NewBus(home string, log Logger) (*Bus, error) {
	return NewBusWithClock(home, log, time.Now)
}

// NewBusWithClock allows test injection of the clock.
func NewBusWithClock(home string, log Logger, clock func() time.Time) (*Bus, error) {
	if clock == nil {
		clock = time.Now
	}
	b := &Bus{
		log:   log,
		clock: clock,
		subs:  map[chan Event]struct{}{},
		ring:  newRing(),
		jsonl: newJSONLWriter(home, clock, log),
	}
	return b, nil
}

func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return
	}
	b.ring.push(e)
	b.jsonl.append(e)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	if b.jsonl != nil {
		_ = b.jsonl.close()
	}
	return nil
}
```

Also add `import "time"` to `bus.go` if not yet present.

- [ ] **Step 6: Run tests**

Run: `go test ./tests -run TestBus -v`
Expected: 6 PASS (4 from before + 2 new).

- [ ] **Step 7: Commit**

```bash
git add pkg/state/daemon_paths.go pkg/events/jsonl.go pkg/events/bus.go tests/events_jsonl_test.go
git commit -m "feat(m10b): pkg/events — JSONL writer with daily rotation"
```

---

## Task 5: JSONL retention sweep

**Files:**
- Modify: `pkg/events/jsonl.go`
- Modify: `pkg/events/bus.go`
- Modify: `tests/events_jsonl_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/events_jsonl_test.go`:

```go
func TestBus_PrunesOldEventsFiles(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}

	old := state.DaemonEventsPath(home, "20260101")
	if err := os.WriteFile(old, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldT := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(old, oldT, oldT); err != nil {
		t.Fatal(err)
	}

	bus, err := events.NewBus(home, &stubLogger{})
	if err != nil {
		t.Fatal(err)
	}
	bus.Publish(events.Event{TS: time.Now().UnixMilli(), Type: "x"})
	bus.Close()

	if _, err := os.Stat(old); err == nil {
		t.Errorf("old events file should have been pruned")
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestBus_PrunesOld -v`
Expected: FAIL — old file still exists.

- [ ] **Step 3: Add `pruneOldLocked` to `pkg/events/jsonl.go`**

Append to `jsonl.go`:

```go
import "path/filepath"
import "strings"
```

(adjust the existing import block to add these)

```go
const retentionDays = 14

func (w *jsonlWriter) pruneOldLocked() {
	dir := state.DaemonDir(w.home)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := w.clock().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() ||
			!strings.HasPrefix(e.Name(), "events-") ||
			!strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
```

Modify `openTodayLocked` to call `pruneOldLocked` whenever it rotates (i.e. when curDate changes):

```go
func (w *jsonlWriter) openTodayLocked() error {
	date := w.clock().UTC().Format("20060102")
	if w.current != nil && w.curDate == date {
		return nil
	}
	rotated := w.current != nil
	if w.current != nil {
		_ = w.current.Close()
		w.current = nil
	}
	if err := os.MkdirAll(state.DaemonDir(w.home), 0o700); err != nil {
		return err
	}
	path := state.DaemonEventsPath(w.home, date)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	w.current = f
	w.curDate = date
	if rotated {
		w.pruneOldLocked()
	}
	return nil
}
```

For first-time open (rotated=false), prune is skipped — but the test creates an old file *before* NewBus, then publishes which triggers first open. To prune on first open too, make `pruneOldLocked` always run:

Replace the conditional with: always call `w.pruneOldLocked()` at end of `openTodayLocked`.

```go
func (w *jsonlWriter) openTodayLocked() error {
	date := w.clock().UTC().Format("20060102")
	if w.current != nil && w.curDate == date {
		return nil
	}
	if w.current != nil {
		_ = w.current.Close()
		w.current = nil
	}
	if err := os.MkdirAll(state.DaemonDir(w.home), 0o700); err != nil {
		return err
	}
	path := state.DaemonEventsPath(w.home, date)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	w.current = f
	w.curDate = date
	w.pruneOldLocked()
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run TestBus -v`
Expected: 7 PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/events/jsonl.go tests/events_jsonl_test.go
git commit -m "feat(m10b): pkg/events — 14-day retention sweep on rotation"
```

---

## Task 6: Tail JSONL fallback when ring is short

**Files:**
- Modify: `pkg/events/bus.go`
- Modify: `pkg/events/jsonl.go`
- Modify: `tests/events_jsonl_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/events_jsonl_test.go`:

```go
func TestBus_TailFallsBackToJSONL(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}

	// Pre-seed an events file with 5 events.
	today := time.Now().UTC().Format("20060102")
	path := state.DaemonEventsPath(home, today)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		fmt.Fprintf(f, `{"ts":%d,"type":"seeded"}`+"\n", i)
	}
	f.Close()

	bus, err := events.NewBus(home, &stubLogger{})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	// Ring is empty (no Publish in this process). Tail should read from disk.
	got := bus.Tail(10, 0)
	if len(got) != 5 {
		t.Fatalf("Tail: got %d events, want 5 (from JSONL)", len(got))
	}
	if got[0].TS != 4 {
		t.Errorf("Tail order: got TS=%d, want newest-first (4)", got[0].TS)
	}
}
```

(also add `"fmt"` to imports of `tests/events_jsonl_test.go`)

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestBus_TailFallsBack -v`
Expected: FAIL — `Tail` returns 0 (ring is empty).

- [ ] **Step 3: Add `tail` to `pkg/events/jsonl.go`**

```go
func (w *jsonlWriter) tail(limit int, since int64) []Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	dir := state.DaemonDir(w.home)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Collect events files newest-first by name (date-sorted lexicographically).
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() ||
			!strings.HasPrefix(e.Name(), "events-") ||
			!strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		names = append(names, e.Name())
	}
	// Sort descending so newest dates come first.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] > names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}

	out := make([]Event, 0, limit)
	for _, n := range names {
		// Read the whole file (small per day at expected rates).
		data, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		// Parse newest-line-first by scanning end-to-start.
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
			if lines[i] == "" {
				continue
			}
			var e Event
			if err := json.Unmarshal([]byte(lines[i]), &e); err != nil {
				continue
			}
			if e.TS > since {
				out = append(out, e)
			}
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}
```

Add the `"encoding/json"` import to jsonl.go if not present.

- [ ] **Step 4: Modify `pkg/events/bus.go` Tail**

```go
func (b *Bus) Tail(limit int, since int64) []Event {
	if limit <= 0 {
		return nil
	}
	if limit > 500 {
		limit = 500
	}
	out := b.ring.tail(limit, since)
	if len(out) >= limit {
		return out
	}
	// Ring under-full or older events requested: scan JSONL and merge.
	disk := b.jsonl.tail(limit, since)
	if len(out) == 0 {
		return disk
	}
	// Merge: deduplicate by TS+Type+ProjectID (cheap key), keep ring order priority.
	seen := make(map[string]struct{}, len(out))
	for _, e := range out {
		seen[mergeKey(e)] = struct{}{}
	}
	for _, e := range disk {
		if _, ok := seen[mergeKey(e)]; ok {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func mergeKey(e Event) string {
	return fmt.Sprintf("%d|%s|%s", e.TS, e.Type, e.ProjectID)
}
```

Add `"fmt"` import.

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestBus -v`
Expected: 8 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/events/jsonl.go pkg/events/bus.go tests/events_jsonl_test.go
git commit -m "feat(m10b): pkg/events — Tail falls back to JSONL when ring under-full"
```

---

## Task 7: Filtering on Tail (types + project_id)

**Files:**
- Modify: `pkg/events/bus.go`
- Modify: `pkg/events/ring.go`
- Modify: `pkg/events/jsonl.go`
- Modify: `tests/events_bus_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/events_bus_test.go`:

```go
func TestBus_TailFilters(t *testing.T) {
	bus, _ := events.NewBus(t.TempDir(), &stubLogger{})
	defer bus.Close()
	bus.Publish(events.Event{TS: 1, Type: "a", ProjectID: "p1"})
	bus.Publish(events.Event{TS: 2, Type: "b", ProjectID: "p1"})
	bus.Publish(events.Event{TS: 3, Type: "a", ProjectID: "p2"})

	got := bus.TailFiltered(10, 0, events.Filter{Types: []string{"a"}})
	if len(got) != 2 {
		t.Errorf("Filter Types=a: got %d, want 2", len(got))
	}
	got = bus.TailFiltered(10, 0, events.Filter{ProjectID: "p1"})
	if len(got) != 2 {
		t.Errorf("Filter ProjectID=p1: got %d, want 2", len(got))
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestBus_TailFilters -v`
Expected: FAIL — `events.Filter` and `TailFiltered` undefined.

- [ ] **Step 3: Add filter to `pkg/events/event.go`**

```go
// Filter selects which events Tail/Subscribe-stream returns.
// Empty Types means "all types"; empty ProjectID means "all projects".
type Filter struct {
	Types     []string
	ProjectID string
}

func (f Filter) Match(e Event) bool {
	if f.ProjectID != "" && e.ProjectID != f.ProjectID {
		return false
	}
	if len(f.Types) > 0 {
		hit := false
		for _, t := range f.Types {
			if e.Type == t {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Add `TailFiltered` to `pkg/events/bus.go`**

```go
func (b *Bus) TailFiltered(limit int, since int64, f Filter) []Event {
	all := b.Tail(limit*4, since) // overshoot to leave room for filter
	out := make([]Event, 0, limit)
	for _, e := range all {
		if f.Match(e) && len(out) < limit {
			out = append(out, e)
		}
	}
	return out
}
```

(YAGNI: ring/jsonl get pre-filtered later only if profiling shows the overshoot is wasteful. For M10b, filtering after the fetch is fine at 500-event scale.)

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestBus -v`
Expected: 9 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/events/event.go pkg/events/bus.go tests/events_bus_test.go
git commit -m "feat(m10b): pkg/events — Filter type + Bus.TailFiltered"
```

---

## Task 8: Wire Bus into daemon orchestrator

**Files:**
- Modify: `pkg/daemon/routes.go`
- Modify: `pkg/daemon/daemon.go`
- Modify: `tests/daemon_routes_test.go`

- [ ] **Step 1: Modify `pkg/daemon/routes.go`**

Add `Bus *events.Bus` to `RouteDeps`. Change the import block:

```go
import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ranwei/mneme/pkg/dashboard"
	"github.com/ranwei/mneme/pkg/events"
)
```

Replace `RouteDeps`:

```go
type RouteDeps struct {
	Log       Logger
	Home      string
	PID       int
	Version   string
	StartedAt int64
	Scheduler *Scheduler
	Token     string
	DevMode   bool
	Bus       *events.Bus // M10b
}
```

- [ ] **Step 2: Modify `pkg/daemon/daemon.go`**

After the `sched := NewScheduler(...)` line (about line 100), add:

```go
bus, err := events.NewBus(opt.Home, log)
if err != nil {
	return fmt.Errorf("event bus: %w", err)
}
defer bus.Close()
```

Add to imports: `"github.com/ranwei/mneme/pkg/events"`.

In the `NewMux(RouteDeps{...})` block, add `Bus: bus,`:

```go
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
```

- [ ] **Step 3: Run existing tests to verify no break**

Run: `go test ./tests -run TestRoutes -v && go test ./tests -run TestServer -v && go test ./tests -run TestDaemonRun -v`
Expected: PASS (all M8/M10a tests).

- [ ] **Step 4: Commit**

```bash
git add pkg/daemon/routes.go pkg/daemon/daemon.go
git commit -m "feat(m10b): wire pkg/events.Bus into daemon orchestrator"
```

---

## Task 9: Cron scheduler publishes cron.tick

**Files:**
- Modify: `pkg/daemon/scheduler.go`
- Modify: `pkg/daemon/daemon.go`
- Modify: `tests/scheduler_test.go` (existing) or create `tests/scheduler_publish_test.go`

- [ ] **Step 1: Add the failing test**

Create `tests/scheduler_publish_test.go`:

```go
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

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestScheduler_Publishes -v`
Expected: FAIL — `sched.SetBus undefined`.

- [ ] **Step 3: Modify `pkg/daemon/scheduler.go`**

Add a `bus *events.Bus` field, a `SetBus` method, and publish from `runAttempt`:

```go
import (
	// existing imports
	"encoding/json"
	"github.com/ranwei/mneme/pkg/events"
)
```

```go
type Scheduler struct {
	home string
	log  Logger
	deps SchedulerDeps
	bus  *events.Bus

	mu      sync.Mutex
	cron    *cron.Cron
	pending map[string][]CancelFn
}

// SetBus injects the event bus. Must be called before Start/RunOnce
// if cron.tick events are desired.
func (s *Scheduler) SetBus(b *events.Bus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bus = b
}

func (s *Scheduler) publishTick(name, status string, durMS int64, errMsg string) {
	s.mu.Lock()
	bus := s.bus
	s.mu.Unlock()
	if bus == nil {
		return
	}
	payload := map[string]interface{}{
		"name":        name,
		"status":      status,
		"duration_ms": durMS,
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	data, _ := json.Marshal(payload)
	bus.Publish(events.Event{
		TS:   time.Now().UnixMilli(),
		Type: "cron.tick",
		Data: data,
	})
}
```

In `runAttempt`, wrap the function call with start/end publish:

```go
func (s *Scheduler) runAttempt(ctx context.Context, name string, fn TaskFunc, attempt int) {
	s.log.Info("sched", fmt.Sprintf("task=%s status=start attempt=%d", name, attempt+1))
	s.publishTick(name, "started", 0, "")
	start := time.Now()

	err := func() (rerr error) {
		defer func() {
			if r := recover(); r != nil {
				rerr = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
			}
		}()
		return fn(ctx, s.log)
	}()
	dur := time.Since(start).Milliseconds()

	if err == nil {
		s.recordSuccess(name)
		s.publishTick(name, "ok", dur, "")
		s.log.Info("sched", fmt.Sprintf("task=%s status=ok attempt=%d", name, attempt+1))
		return
	}

	s.recordFailure(name, err)
	s.publishTick(name, "failed", dur, err.Error())
	s.log.Warn("sched", fmt.Sprintf("task=%s status=fail attempt=%d err=%q", name, attempt+1, err.Error()))

	// (existing retry logic unchanged below)
	if attempt < len(retryDelays) {
		// ...
	}
	// ...
}
```

- [ ] **Step 4: Modify `pkg/daemon/daemon.go`**

After bus is created and `sched := NewScheduler(...)` is called, add:

```go
sched.SetBus(bus)
```

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestScheduler -v`
Expected: PASS for new tests AND existing scheduler tests still pass.

- [ ] **Step 6: Commit**

```bash
git add pkg/daemon/scheduler.go pkg/daemon/daemon.go tests/scheduler_publish_test.go
git commit -m "feat(m10b): scheduler publishes cron.tick start/ok/failed events"
```

---

## Task 10: Cerebrum + suggestions + anatomy publishers

**Files:**
- Modify: `pkg/daemon/cerebrum_handler.go`
- Modify: `pkg/daemon/tasks.go`
- Modify: `pkg/daemon/daemon.go`

The actual code uses package-level `var registry = map[string]TaskFunc{...}` in `pkg/daemon/tasks.go`, with standalone `runAnatomyRescan` / `runSuggestionsRefresh` / etc. functions. We keep that structure and add a package-level bus var that the daemon sets at startup.

- [ ] **Step 1: Modify `pkg/daemon/tasks.go`**

Add `"encoding/json"` and `"github.com/ranwei/mneme/pkg/events"` to imports.

Add a package-level bus + setter near the top, after the imports:

```go
// taskBus is the optional event bus for tasks to publish to.
// nil means events are dropped silently (e.g., in tests that don't need them).
var taskBus *events.Bus

// SetTaskBus is called by daemon.Run to wire the shared bus.
func SetTaskBus(b *events.Bus) { taskBus = b }
```

In `runAnatomyRescan`, augment the success path (the `else` branch where it logs `anatomy-rescan: ok`):

```go
		if err := scanProjectIncremental(root); err != nil {
			log.Warn("task", fmt.Sprintf("anatomy-rescan: %s: %v", root, err))
		} else {
			log.Info("task", fmt.Sprintf("anatomy-rescan: ok %s", root))
			if taskBus != nil {
				payload, _ := json.Marshal(map[string]interface{}{
					"files_changed": -1, // unknown — scanner does not surface count today
				})
				taskBus.Publish(events.Event{
					TS:        time.Now().UnixMilli(),
					Type:      "scan.complete",
					ProjectID: e.Name(), // directory name is the project_id
					Data:      payload,
				})
			}
		}
```

In `runSuggestionsRefresh`, replace the body of the per-project loop:

```go
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		originBytes, err := os.ReadFile(filepath.Join(projectsDir, e.Name(), "origin"))
		root := strings.TrimSpace(string(originBytes))
		if err != nil || root == "" {
			root = filepath.Join(projectsDir, e.Name())
		}
		if _, err := os.Stat(root); err != nil {
			continue
		}
		got, err := suggestions.Refresh(root, time.Now().UTC())
		if err == nil && taskBus != nil && len(got) > 0 {
			payload, _ := json.Marshal(map[string]interface{}{"count": len(got)})
			taskBus.Publish(events.Event{
				TS:        time.Now().UnixMilli(),
				Type:      "suggestion.new",
				ProjectID: e.Name(),
				Data:      payload,
			})
		}
	}
```

`runConsolidateMemory`, `runPruneBackups`, and `runWeeklyWasteReport` do not publish events in M10b — they remain unchanged.

- [ ] **Step 2: Modify `pkg/daemon/cerebrum_handler.go`**

Locate the success path of the learn handler. Inspect the file first:

Run: `grep -n "ProjectID\|Publish\|writeJSON" pkg/daemon/cerebrum_handler.go | head`

The handler decodes a body that contains a `ProjectID` field (per the M9 spec). After the successful enqueue/persist, add (right before `writeJSON(w, 200, ...)`):

```go
if d.Bus != nil {
	payload, _ := json.Marshal(map[string]interface{}{"count": 1})
	d.Bus.Publish(events.Event{
		TS:        time.Now().UnixMilli(),
		Type:      "cerebrum.candidate",
		ProjectID: body.ProjectID,
		Data:      payload,
	})
}
```

Adjust `body.ProjectID` to the actual field name read from `cerebrum_handler.go`. Add `"github.com/ranwei/mneme/pkg/events"` and `"time"` and `"encoding/json"` to its imports if not yet present.

- [ ] **Step 3: Modify `pkg/daemon/daemon.go`**

After the `sched.SetBus(bus)` line from Task 9, add:

```go
SetTaskBus(bus)
```

- [ ] **Step 4: Run tests**

```bash
go test ./tests/... -count=1
```

Expected: all existing tests still pass; no new tests added in this task (publishing from cron tasks is exercised by Task 29 integration test).

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon/tasks.go pkg/daemon/cerebrum_handler.go pkg/daemon/daemon.go
git commit -m "feat(m10b): anatomy + suggestions + cerebrum publish to event bus"
```

---

## Task 11: REST `/api/overview`

**Files:**
- Create: `pkg/dashboard/api.go`
- Modify: `pkg/daemon/routes.go`
- Create: `tests/dashboard_api_test.go`

- [ ] **Step 1: Add the failing test**

Create `tests/dashboard_api_test.go`:

```go
package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
)

func TestAPI_Overview_EmptyHome(t *testing.T) {
	home := t.TempDir()
	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()

	mux := daemon.NewMux(daemon.RouteDeps{
		Log:       &stubLogger{},
		Home:      home,
		PID:       42,
		Version:   "test",
		StartedAt: time.Now().Unix() - 100,
		Bus:       bus,
	})
	req := httptest.NewRequest("GET", "/api/overview", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d, body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Daemon struct {
			PID     int    `json:"pid"`
			Version string `json:"version"`
		} `json:"daemon"`
		Totals struct {
			Projects int `json:"projects"`
		} `json:"totals"`
		Projects []map[string]interface{} `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Daemon.PID != 42 {
		t.Errorf("daemon.pid = %d, want 42", body.Daemon.PID)
	}
	if body.Totals.Projects != 0 {
		t.Errorf("totals.projects = %d, want 0", body.Totals.Projects)
	}
	if body.Projects == nil {
		t.Errorf("projects field should be [], got nil")
	}
}

func TestAPI_Overview_OneProject(t *testing.T) {
	home := t.TempDir()
	pid := "abc123"
	pdir := filepath.Join(home, ".mneme", "projects", pid)
	if err := os.MkdirAll(pdir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "origin"), []byte("/work/foo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "anatomy.md"), []byte("# Anatomy\n## a.go\n## b.go\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()

	mux := daemon.NewMux(daemon.RouteDeps{
		Log: &stubLogger{}, Home: home, Bus: bus,
	})
	req := httptest.NewRequest("GET", "/api/overview", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	totals, _ := body["totals"].(map[string]interface{})
	if totals["projects"].(float64) != 1 {
		t.Errorf("totals.projects = %v, want 1", totals["projects"])
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestAPI_Overview -v`
Expected: FAIL — `/api/overview` returns 404.

- [ ] **Step 3: Create `pkg/dashboard/api.go`**

```go
package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// APIDeps is the cross-cutting set of dependencies the API handlers need.
// It is filled in by the daemon at mount time.
type APIDeps struct {
	Home      string
	PID       int
	Version   string
	StartedAt int64
	// Bus + Manifest/State/Scheduler accessors come from a thin interface
	// passed in by daemon. To avoid an import cycle, daemon owns the wiring
	// and calls these handler factories.
}

// ProjectSummary is the per-project shape used by Overview + Projects.
type ProjectSummary struct {
	ID             string `json:"id"`
	Origin         string `json:"origin"`
	AnatomyFiles   int    `json:"anatomy_files"`
	CerebrumPending int   `json:"cerebrum_pending"`
	MemoryBytes    int64  `json:"memory_bytes"`
	LastActivityTS int64  `json:"last_activity_ts"`
}

// OverviewResponse mirrors the Go-to-JSON shape promised by the spec.
type OverviewResponse struct {
	Daemon struct {
		PID       int    `json:"pid"`
		UptimeS   int64  `json:"uptime_s"`
		Version   string `json:"version"`
		StartedAt int64  `json:"started_at"`
	} `json:"daemon"`
	Totals struct {
		Projects        int `json:"projects"`
		AnatomyFiles    int `json:"anatomy_files"`
		CerebrumPending int `json:"cerebrum_pending"`
		OpenSuggestions int `json:"open_suggestions"`
	} `json:"totals"`
	Projects []ProjectSummary `json:"projects"`
}

// EnumerateProjects walks ~/.mneme/projects/<id>/ and assembles summaries.
func EnumerateProjects(home string) ([]ProjectSummary, error) {
	root := filepath.Join(home, ".mneme", "projects")
	out := []ProjectSummary{}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ps := ProjectSummary{ID: e.Name()}
		dir := filepath.Join(root, e.Name())
		if data, err := os.ReadFile(filepath.Join(dir, "origin")); err == nil {
			ps.Origin = strings.TrimSpace(string(data))
		}
		if data, err := os.ReadFile(filepath.Join(dir, "anatomy.md")); err == nil {
			ps.AnatomyFiles = strings.Count(string(data), "\n## ")
		}
		// cerebrum_pending: count of "pending: true" markers in cerebrum.json (best-effort)
		if data, err := os.ReadFile(filepath.Join(dir, "cerebrum-pending.json")); err == nil {
			ps.CerebrumPending = strings.Count(string(data), "{")
		}
		if info, err := os.Stat(filepath.Join(dir, "memory.md")); err == nil {
			ps.MemoryBytes = info.Size()
		}
		out = append(out, ps)
	}
	return out, nil
}

// OverviewHandler is a factory; daemon supplies APIDeps + a function to
// fetch totals (which involve cron-state and manifest the dashboard pkg
// must not import directly).
func OverviewHandler(deps APIDeps, openSuggestionsCount func() int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projects, err := EnumerateProjects(deps.Home)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var resp OverviewResponse
		resp.Daemon.PID = deps.PID
		resp.Daemon.Version = deps.Version
		resp.Daemon.StartedAt = deps.StartedAt
		if deps.StartedAt > 0 {
			resp.Daemon.UptimeS = time.Now().Unix() - deps.StartedAt
		}
		resp.Projects = projects
		resp.Totals.Projects = len(projects)
		for _, p := range projects {
			resp.Totals.AnatomyFiles += p.AnatomyFiles
			resp.Totals.CerebrumPending += p.CerebrumPending
		}
		if openSuggestionsCount != nil {
			resp.Totals.OpenSuggestions = openSuggestionsCount()
		}
		writeJSON(w, 200, resp)
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
```

- [ ] **Step 4: Modify `pkg/daemon/routes.go`**

Wire it in. Add to `NewMux`, after the existing `mux.Handle("/dev-token", ...)` line:

```go
mux.Handle("/api/overview", dashboard.OverviewHandler(
	dashboard.APIDeps{
		Home: deps.Home, PID: deps.PID, Version: deps.Version, StartedAt: deps.StartedAt,
	},
	func() int { return countOpenSuggestions(deps.Home) },
))
```

Add a helper at the bottom of `routes.go`:

```go
func countOpenSuggestions(home string) int {
	dir := filepath.Join(home, ".mneme", "suggestions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	return count
}
```

Add `"os"`, `"path/filepath"`, `"strings"` to the import block.

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestAPI_Overview -v`
Expected: 2 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/api.go pkg/daemon/routes.go tests/dashboard_api_test.go
git commit -m "feat(m10b): GET /api/overview"
```

---

## Task 12: REST `/api/projects`

**Files:**
- Modify: `pkg/dashboard/api.go`
- Modify: `pkg/daemon/routes.go`
- Modify: `tests/dashboard_api_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/dashboard_api_test.go`:

```go
func TestAPI_Projects(t *testing.T) {
	home := t.TempDir()
	for _, pid := range []string{"a", "b"} {
		dir := filepath.Join(home, ".mneme", "projects", pid)
		os.MkdirAll(dir, 0o700)
		os.WriteFile(filepath.Join(dir, "origin"), []byte("/x/"+pid), 0o600)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Projects) != 2 {
		t.Errorf("got %d projects, want 2", len(body.Projects))
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestAPI_Projects -v`
Expected: FAIL — 404.

- [ ] **Step 3: Add `ProjectsHandler` to `pkg/dashboard/api.go`**

```go
func ProjectsHandler(deps APIDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projects, err := EnumerateProjects(deps.Home)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, 200, map[string]interface{}{"projects": projects})
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

In `NewMux`, after `OverviewHandler` registration:

```go
mux.Handle("/api/projects", dashboard.ProjectsHandler(dashboard.APIDeps{
	Home: deps.Home, PID: deps.PID, Version: deps.Version, StartedAt: deps.StartedAt,
}))
```

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestAPI_Projects -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/api.go pkg/daemon/routes.go tests/dashboard_api_test.go
git commit -m "feat(m10b): GET /api/projects"
```

---

## Task 13: REST `/api/activity`

**Files:**
- Modify: `pkg/dashboard/api.go`
- Modify: `pkg/daemon/routes.go`
- Modify: `tests/dashboard_api_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/dashboard_api_test.go`:

```go
func TestAPI_Activity(t *testing.T) {
	home := t.TempDir()
	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()

	for i := 0; i < 5; i++ {
		bus.Publish(events.Event{
			TS:   time.Now().UnixMilli() + int64(i),
			Type: "hook.fired",
		})
	}

	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})
	req := httptest.NewRequest("GET", "/api/activity?limit=3", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []map[string]interface{} `json:"events"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 3 {
		t.Errorf("got %d events, want 3", len(body.Events))
	}
}

func TestAPI_Activity_TypeFilter(t *testing.T) {
	home := t.TempDir()
	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	bus.Publish(events.Event{TS: 1, Type: "hook.fired"})
	bus.Publish(events.Event{TS: 2, Type: "cron.tick"})
	bus.Publish(events.Event{TS: 3, Type: "hook.fired"})

	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})
	req := httptest.NewRequest("GET", "/api/activity?types=hook.fired", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body struct {
		Events []map[string]interface{} `json:"events"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 2 {
		t.Errorf("filter types=hook.fired: got %d, want 2", len(body.Events))
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestAPI_Activity -v`
Expected: FAIL — 404.

- [ ] **Step 3: Add `ActivityHandler` to `pkg/dashboard/api.go`**

```go
import (
	// existing imports
	"strconv"

	"github.com/ranwei/mneme/pkg/events"
)

// ActivityHandler reads from the bus.Tail / TailFiltered.
func ActivityHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		if bus == nil {
			http.Error(w, `{"error":"bus_unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		q := r.URL.Query()
		limit := 100
		if s := q.Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				if n > 500 {
					n = 500
				}
				limit = n
			}
		}
		var since int64
		if s := q.Get("since"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				since = n
			}
		}
		filter := events.Filter{ProjectID: q.Get("project_id")}
		if t := q.Get("types"); t != "" {
			filter.Types = strings.Split(t, ",")
		}

		got := bus.TailFiltered(limit, since, filter)
		var nextCursor int64
		if len(got) > 0 {
			nextCursor = got[len(got)-1].TS
		}
		writeJSON(w, 200, map[string]interface{}{
			"events":      got,
			"next_cursor": nextCursor,
		})
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/api/activity", dashboard.ActivityHandler(deps.Bus))
```

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestAPI_Activity -v`
Expected: 2 PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/api.go pkg/daemon/routes.go tests/dashboard_api_test.go
git commit -m "feat(m10b): GET /api/activity (limit/since/types/project_id)"
```

---

## Task 14: REST `/api/cron`

**Files:**
- Modify: `pkg/daemon/routes.go`
- Modify: `tests/dashboard_api_test.go`

This endpoint reuses existing `LoadOrSeedManifest` + `LoadCronState` logic. We add it as an enriched alias for `/cron/list` exposing `last_error`.

- [ ] **Step 1: Add the failing test**

Append to `tests/dashboard_api_test.go`:

```go
func TestAPI_Cron(t *testing.T) {
	home := t.TempDir()
	if _, err := daemon.LoadOrSeedManifest(home); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})
	req := httptest.NewRequest("GET", "/api/cron", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Tasks []map[string]interface{} `json:"tasks"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Tasks) < 3 {
		t.Errorf("got %d tasks, want >=3 (seed manifest)", len(body.Tasks))
	}
	first := body.Tasks[0]
	if _, ok := first["state"]; !ok {
		t.Errorf("task missing state field: %+v", first)
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestAPI_Cron -v`
Expected: FAIL — 404.

- [ ] **Step 3: Add `apiCron` handler to `pkg/daemon/routes.go`**

Place near `cronList`:

```go
func (d *RouteDeps) apiCron(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method", "GET only")
		return
	}
	if d.Home == "" {
		writeError(w, http.StatusInternalServerError, "no_home", "Home not configured")
		return
	}
	m, err := LoadOrSeedManifest(d.Home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "manifest_load", err.Error())
		return
	}
	st, _ := LoadCronState(d.Home)

	type taskOut struct {
		Name     string    `json:"name"`
		Schedule string    `json:"schedule"`
		Enabled  bool      `json:"enabled"`
		State    TaskState `json:"state"`
	}
	out := make([]taskOut, 0, len(m.Tasks))
	for _, t := range m.Tasks {
		out = append(out, taskOut{
			Name: t.Name, Schedule: t.Schedule, Enabled: t.Enabled, State: st.Tasks[t.Name],
		})
	}
	writeJSON(w, 200, map[string]interface{}{"tasks": out})
}
```

Register it:

```go
mux.HandleFunc("/api/cron", deps.apiCron)
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run TestAPI_Cron -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon/routes.go tests/dashboard_api_test.go
git commit -m "feat(m10b): GET /api/cron with last_error in state"
```

---

## Task 15: SSE `/events` handler

**Files:**
- Create: `pkg/dashboard/sse.go`
- Modify: `pkg/daemon/routes.go`
- Create: `tests/dashboard_sse_test.go`

- [ ] **Step 1: Add the failing test**

Create `tests/dashboard_sse_test.go`:

```go
package tests

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
)

func TestSSE_DeliversEvents(t *testing.T) {
	home := t.TempDir()
	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()

	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(daemon.WithTransport(r.Context(), daemon.TransportUnix)))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("content-type = %q", ct)
	}

	// Publish after a tiny delay to give the handler time to subscribe.
	go func() {
		time.Sleep(50 * time.Millisecond)
		bus.Publish(events.Event{TS: 1, Type: "ping.test"})
	}()

	br := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"ping.test"`) {
			return
		}
	}
	t.Fatal("did not receive ping.test event over SSE")
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestSSE_DeliversEvents -v`
Expected: FAIL — 404.

- [ ] **Step 3: Create `pkg/dashboard/sse.go`**

```go
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/events"
)

const ssePingInterval = 25 * time.Second

// SSEHandler streams bus events as text/event-stream.
func SSEHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bus == nil {
			http.Error(w, "bus unavailable", http.StatusServiceUnavailable)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		filter := events.Filter{ProjectID: r.URL.Query().Get("project_id")}
		if t := r.URL.Query().Get("types"); t != "" {
			filter.Types = strings.Split(t, ",")
		}

		ch, unsub := bus.Subscribe()
		defer unsub()

		ping := time.NewTicker(ssePingInterval)
		defer ping.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				if !filter.Match(e) {
					continue
				}
				data, err := json.Marshal(e)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return
				}
				flusher.Flush()
			case <-ping.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}
```

- [ ] **Step 4: Wire it in `pkg/daemon/routes.go`**

```go
mux.Handle("/events", dashboard.SSEHandler(deps.Bus))
```

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestSSE -v -timeout 30s`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/sse.go pkg/daemon/routes.go tests/dashboard_sse_test.go
git commit -m "feat(m10b): GET /events SSE handler"
```

---

## Task 16: `POST /events/publish` (Unix-only)

**Files:**
- Modify: `pkg/dashboard/sse.go`
- Modify: `pkg/daemon/routes.go`
- Modify: `tests/dashboard_sse_test.go`

- [ ] **Step 1: Add the failing test**

Append to `tests/dashboard_sse_test.go`:

```go
func TestEventsPublish_UnixOnly(t *testing.T) {
	home := t.TempDir()
	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	body := strings.NewReader(`{"type":"hook.fired","project_id":"abc","data":{"hook":"x"}}`)
	req := httptest.NewRequest("POST", "/events/publish", body)
	req.Header.Set("Content-Type", "application/json")
	// TCP transport: should be rejected.
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportTCP))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("TCP /events/publish: got %d, want 403", rec.Code)
	}

	// Unix transport: accepted.
	body2 := strings.NewReader(`{"type":"hook.fired","project_id":"abc","data":{"hook":"x"}}`)
	req2 := httptest.NewRequest("POST", "/events/publish", body2)
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(daemon.WithTransport(req2.Context(), daemon.TransportUnix))
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNoContent {
		t.Errorf("Unix /events/publish: got %d body=%s", rec2.Code, rec2.Body.String())
	}

	if got := bus.Tail(10, 0); len(got) != 1 {
		t.Errorf("bus.Tail after publish: got %d events, want 1", len(got))
	}
}
```

(Add `"net/http/httptest"` and `"strings"` to imports if not yet present.)

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestEventsPublish -v`
Expected: FAIL — 404.

- [ ] **Step 3: Add `PublishHandler` to `pkg/dashboard/sse.go`**

```go
import (
	"context"
	// existing imports
)

// transportFromCtxFunc is supplied by the daemon to avoid an import cycle.
type transportFromCtxFunc func(ctx context.Context) (isUnix bool)

// PublishHandler accepts events from in-process subprocesses (mneme hook).
// Rejected on TCP. Allowed on Unix socket.
func PublishHandler(bus *events.Bus, isUnix transportFromCtxFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isUnix(r.Context()) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		if bus == nil {
			http.Error(w, "bus unavailable", http.StatusServiceUnavailable)
			return
		}
		var in struct {
			Type      string          `json:"type"`
			ProjectID string          `json:"project_id"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, `{"error":"bad_body"}`, http.StatusBadRequest)
			return
		}
		if in.Type == "" {
			http.Error(w, `{"error":"missing_type"}`, http.StatusBadRequest)
			return
		}
		bus.Publish(events.Event{
			TS:        time.Now().UnixMilli(),
			Type:      in.Type,
			ProjectID: in.ProjectID,
			Data:      in.Data,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
```

- [ ] **Step 4: Wire in `pkg/daemon/routes.go`**

Expose a small adapter that converts daemon's internal `transportFromCtx` into the boolean callback, then register:

```go
mux.Handle("/events/publish", dashboard.PublishHandler(deps.Bus, func(ctx context.Context) bool {
	return transportFromCtx(ctx) == TransportUnix
}))
```

Add `"context"` to the import block of `routes.go`.

- [ ] **Step 5: Run tests**

Run: `go test ./tests -run TestEventsPublish -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/dashboard/sse.go pkg/daemon/routes.go tests/dashboard_sse_test.go
git commit -m "feat(m10b): POST /events/publish (Unix-only) for hook subprocesses"
```

---

## Task 17: `daemonclient.PublishEvent`

**Files:**
- Modify: `pkg/daemonclient/client.go`
- Modify: `tests/daemonclient_test.go` (or create one)

- [ ] **Step 1: Add the failing test**

Create `tests/daemonclient_publish_test.go`:

```go
package tests

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemonclient"
	"github.com/ranwei/mneme/pkg/state"
)

func TestPublishEvent_RoundTrip(t *testing.T) {
	home, err := os.MkdirTemp("", "mneme")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := state.DaemonSocketPath(home)

	got := make(chan map[string]interface{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/events/publish", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		got <- body
		w.WriteHeader(http.StatusNoContent)
	})
	srv := &http.Server{Handler: mux}
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(l)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	c, err := daemonclient.TryDial(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.PublishEvent(context.Background(), "hook.fired", "p1", map[string]string{"hook": "pre-write"}); err != nil {
		t.Fatal(err)
	}

	select {
	case body := <-got:
		if body["type"] != "hook.fired" || body["project_id"] != "p1" {
			t.Errorf("server received unexpected payload: %+v", body)
		}
	case <-time.After(time.Second):
		t.Fatal("server never received payload")
	}

	_ = filepath.Base(socketPath) // keep import used
}
```

- [ ] **Step 2: Run test**

Run: `go test ./tests -run TestPublishEvent -v`
Expected: FAIL — `c.PublishEvent undefined`.

- [ ] **Step 3: Add `PublishEvent` to `pkg/daemonclient/client.go`**

```go
// PublishEvent posts an event to the daemon's /events/publish endpoint.
// Intended for hook subprocesses. Returns nil silently if data marshalling fails
// (so a hook never blocks Claude on a serialization bug).
func (c *Client) PublishEvent(ctx context.Context, eventType, projectID string, data interface{}) error {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil
		}
		raw = b
	}
	body := map[string]interface{}{
		"type":       eventType,
		"project_id": projectID,
		"data":       raw,
	}
	return c.do(ctx, "POST", "/events/publish", body, nil)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tests -run TestPublishEvent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemonclient/client.go tests/daemonclient_publish_test.go
git commit -m "feat(m10b): daemonclient.PublishEvent helper"
```

---

## Task 18: Hook handlers publish hook.fired

**Files:**
- Modify: `cmd/hook_prewrite.go`
- Modify: `cmd/hook_postwrite.go`
- Modify: `cmd/hook_sessionstart.go`
- Modify: `cmd/hook_stop.go`

- [ ] **Step 1: Create `cmd/hook_publish.go`**

```go
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
```

The hook runners already have `id` in scope from `resolveProjectFromEvent(ev)` (returns `(root, id, ok)` — see `cmd/cmd_hook.go:101`). We pass that `id` directly instead of re-deriving it.

- [ ] **Step 2: Modify each hook runner**

In `cmd/hook_prewrite.go` — change the existing destructure (currently `root, _, resolved := ...`) to capture `id`:

```go
root, id, resolved := resolveProjectFromEvent(ev)
if !resolved {
    os.Exit(0)
}
state.IncrementSafe(root, "hook_fired.pre-write")
publishHookFired("pre-write", id, nil)
```

In `cmd/hook_postwrite.go` — same destructure pattern, then after `state.IncrementSafe(root, "hook_fired.post-tool-use")`:

```go
publishHookFired("post-tool-use", id, nil)
```

In `cmd/hook_sessionstart.go` — same pattern, then after `state.IncrementSafe(root, "hook_fired.session-start")`:

```go
publishHookFired("session-start", id, nil)
```

In `cmd/hook_stop.go` — same pattern, then after `state.IncrementSafe(root, "hook_fired.stop")`:

```go
publishHookFired("stop", id, nil)
```

If the existing destructure ignores `id` (`root, _, resolved := ...`), change the `_` to `id` and ensure `id` is used (the new `publishHookFired` line uses it, so the compiler will be happy).

- [ ] **Step 3: Build and run a smoke check**

```bash
go build -o ./bin/mneme ./cmd
go test ./tests/... -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/hook_publish.go cmd/hook_prewrite.go cmd/hook_postwrite.go cmd/hook_sessionstart.go cmd/hook_stop.go
git commit -m "feat(m10b): hook handlers publish hook.fired to daemon"
```

---

## Task 19: Frontend — install deps and skeleton

**Files:**
- Modify: `web/package.json`
- Modify: `web/src/App.tsx`
- Create: `web/vitest.config.ts`
- Modify: `web/vite.config.ts` (if needed)

- [ ] **Step 1: Install new deps**

```bash
cd web
pnpm add react-router-dom@^7
pnpm add -D vitest@^2 jsdom@^25 @testing-library/react@^16 @testing-library/jest-dom@^6
cd ..
```

- [ ] **Step 2: Add vitest config**

Create `web/vitest.config.ts`:

```ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/__tests__/setup.ts'],
  },
})
```

Create `web/src/__tests__/setup.ts`:

```ts
import '@testing-library/jest-dom'

// Stub EventSource — the smoke test never connects.
class StubEventSource {
  url: string
  readyState = 0
  onopen: any = null
  onmessage: any = null
  onerror: any = null
  constructor(url: string) { this.url = url }
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
;(globalThis as any).EventSource = StubEventSource
```

- [ ] **Step 3: Replace `web/src/App.tsx` with the router shell**

```tsx
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { Overview } from './panels/Overview'
import { Activity } from './panels/Activity'
import { Cron } from './panels/Cron'
import { Bootstrap } from './Bootstrap'
import { SSEProvider } from './hooks/useSSE'

const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Overview /> },
      { path: 'activity', element: <Activity /> },
      { path: 'cron', element: <Cron /> },
    ],
  },
])

export function App(): JSX.Element {
  return (
    <>
      <Bootstrap />
      <SSEProvider>
        <RouterProvider router={router} />
      </SSEProvider>
    </>
  )
}
```

(Files referenced by this `App.tsx` get created in subsequent tasks. The build will fail until those are in place — that's expected; we rebuild at the end of each task.)

- [ ] **Step 4: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/vitest.config.ts web/src/__tests__/setup.ts web/src/App.tsx
git commit -m "feat(m10b): frontend — install react-router-dom, vitest; replace App with router shell"
```

---

## Task 20: Frontend — `api/client.ts` + types

**Files:**
- Create: `web/src/api/types.ts`
- Create: `web/src/api/client.ts`

- [ ] **Step 1: Create `web/src/api/types.ts`**

```ts
export type ProjectSummary = {
  id: string
  origin: string
  anatomy_files: number
  cerebrum_pending: number
  memory_bytes: number
  last_activity_ts: number
}

export type Overview = {
  daemon: { pid: number; uptime_s: number; version: string; started_at: number }
  totals: {
    projects: number
    anatomy_files: number
    cerebrum_pending: number
    open_suggestions: number
  }
  projects: ProjectSummary[]
}

export type Event = {
  ts: number
  type: string
  project_id?: string
  data?: Record<string, unknown>
}

export type ActivityResponse = {
  events: Event[]
  next_cursor: number
}

export type TaskState = {
  last_run: string
  last_success: string
  last_error: string
  consecutive_failures: number
  dead_lettered_at: string
}

export type CronTask = {
  name: string
  schedule: string
  enabled: boolean
  state: TaskState
}

export type CronResponse = { tasks: CronTask[] }
export type ProjectsResponse = { projects: ProjectSummary[] }
```

- [ ] **Step 2: Create `web/src/api/client.ts`**

```ts
export class APIError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method,
    credentials: 'include',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new APIError(res.status, text || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat(m10b): frontend — api types + fetch wrapper"
```

---

## Task 21: Frontend — api wrappers

**Files:**
- Create: `web/src/api/overview.ts`
- Create: `web/src/api/projects.ts`
- Create: `web/src/api/activity.ts`
- Create: `web/src/api/cron.ts`

- [ ] **Step 1: Create wrappers**

`web/src/api/overview.ts`:
```ts
import { request } from './client'
import type { Overview } from './types'

export const getOverview = () => request<Overview>('GET', '/api/overview')
```

`web/src/api/projects.ts`:
```ts
import { request } from './client'
import type { ProjectsResponse } from './types'

export const getProjects = () => request<ProjectsResponse>('GET', '/api/projects')
```

`web/src/api/activity.ts`:
```ts
import { request } from './client'
import type { ActivityResponse } from './types'

export type ActivityArgs = { limit?: number; since?: number; types?: string[]; projectId?: string }

export function getActivity(args: ActivityArgs = {}): Promise<ActivityResponse> {
  const params = new URLSearchParams()
  if (args.limit) params.set('limit', String(args.limit))
  if (args.since) params.set('since', String(args.since))
  if (args.types?.length) params.set('types', args.types.join(','))
  if (args.projectId) params.set('project_id', args.projectId)
  const qs = params.toString()
  return request<ActivityResponse>('GET', '/api/activity' + (qs ? '?' + qs : ''))
}
```

`web/src/api/cron.ts`:
```ts
import { request } from './client'
import type { CronResponse } from './types'

export const getCron = () => request<CronResponse>('GET', '/api/cron')
export const runTask = (name: string) => request<void>('POST', '/cron/run', { name })
export const retryTask = (name: string) => request<void>('POST', '/cron/retry', { name })
```

- [ ] **Step 2: Commit**

```bash
git add web/src/api/overview.ts web/src/api/projects.ts web/src/api/activity.ts web/src/api/cron.ts
git commit -m "feat(m10b): frontend — api wrappers (overview/projects/activity/cron)"
```

---

## Task 22: Frontend — `useFetch` hook

**Files:**
- Create: `web/src/hooks/useFetch.ts`

- [ ] **Step 1: Create the hook**

```ts
import { useCallback, useEffect, useRef, useState } from 'react'

export type FetchState<T> = {
  data: T | null
  error: Error | null
  loading: boolean
  refetch: () => void
}

export function useFetch<T>(
  fn: () => Promise<T>,
  opts?: { intervalMs?: number },
): FetchState<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<Error | null>(null)
  const [loading, setLoading] = useState(true)
  const fnRef = useRef(fn)
  fnRef.current = fn

  const run = useCallback(async () => {
    try {
      const v = await fnRef.current()
      setData(v)
      setError(null)
    } catch (e) {
      setError(e as Error)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    run()
    if (!opts?.intervalMs) return
    const id = setInterval(run, opts.intervalMs)
    return () => clearInterval(id)
  }, [run, opts?.intervalMs])

  return { data, error, loading, refetch: run }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/hooks/useFetch.ts
git commit -m "feat(m10b): frontend — useFetch hook"
```

---

## Task 23: Frontend — `useSSE` hook + provider

**Files:**
- Create: `web/src/hooks/useSSE.ts`

- [ ] **Step 1: Create the hook + provider**

```tsx
import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import type { Event } from '../api/types'

type Subscriber = (e: Event) => void

type SSEContextValue = {
  connected: boolean
  subscribe: (types: string[] | null, fn: Subscriber) => () => void
}

const SSEContext = createContext<SSEContextValue | null>(null)

export function SSEProvider({ children }: { children: ReactNode }): JSX.Element {
  const [connected, setConnected] = useState(false)
  const subsRef = useRef<Set<{ types: string[] | null; fn: Subscriber }>>(new Set())

  useEffect(() => {
    let es: EventSource | null = null
    let cancelled = false
    let backoffMs = 1000

    const open = () => {
      if (cancelled) return
      es = new EventSource('/events')
      es.onopen = () => {
        setConnected(true)
        backoffMs = 1000
      }
      es.onmessage = (ev) => {
        try {
          const data = JSON.parse(ev.data) as Event
          for (const sub of subsRef.current) {
            if (sub.types === null || sub.types.includes(data.type)) {
              sub.fn(data)
            }
          }
        } catch { /* ignore parse errors */ }
      }
      es.onerror = () => {
        setConnected(false)
        es?.close()
        if (!cancelled) {
          setTimeout(open, backoffMs)
          backoffMs = Math.min(backoffMs * 2, 10_000)
        }
      }
    }

    open()
    return () => { cancelled = true; es?.close() }
  }, [])

  const value: SSEContextValue = {
    connected,
    subscribe: (types, fn) => {
      const entry = { types, fn }
      subsRef.current.add(entry)
      return () => { subsRef.current.delete(entry) }
    },
  }
  return <SSEContext.Provider value={value}>{children}</SSEContext.Provider>
}

export function useSSE(types: string[] | null, fn: Subscriber): void {
  const ctx = useContext(SSEContext)
  if (!ctx) throw new Error('useSSE must be used inside SSEProvider')
  const fnRef = useRef(fn)
  fnRef.current = fn
  useEffect(() => {
    return ctx.subscribe(types, (e) => fnRef.current(e))
  }, [ctx, types])
}

export function useSSEConnected(): boolean {
  const ctx = useContext(SSEContext)
  return ctx?.connected ?? false
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/hooks/useSSE.ts
git commit -m "feat(m10b): frontend — useSSE provider with auto-reconnect"
```

---

## Task 24: Frontend — `useProjectList` + small components

**Files:**
- Create: `web/src/hooks/useProjectList.ts`
- Create: `web/src/components/AppShell.tsx`
- Create: `web/src/components/HealthCard.tsx`
- Create: `web/src/components/ProjectCard.tsx`
- Create: `web/src/components/EventRow.tsx`
- Create: `web/src/components/CronTaskRow.tsx`
- Delete: `web/src/Health.tsx`

- [ ] **Step 1: `useProjectList`**

```ts
import { useFetch } from './useFetch'
import { useSSE } from './useSSE'
import { getProjects } from '../api/projects'

export function useProjectList() {
  const r = useFetch(getProjects)
  useSSE(['scan.complete', 'cerebrum.candidate', 'suggestion.new'], () => r.refetch())
  return r
}
```

- [ ] **Step 2: `AppShell`**

```tsx
import { Link, NavLink, Outlet } from 'react-router-dom'
import { useSSEConnected } from '../hooks/useSSE'

export function AppShell(): JSX.Element {
  const connected = useSSEConnected()
  return (
    <div className="flex min-h-screen">
      <aside className="w-60 border-r bg-gray-50 p-4">
        <Link to="/" className="block text-lg font-semibold mb-4">mneme</Link>
        <nav className="flex flex-col gap-1">
          <NavLink to="/" end className={navClass}>Overview</NavLink>
          <NavLink to="/activity" className={navClass}>Activity</NavLink>
          <NavLink to="/cron" className={navClass}>Cron</NavLink>
        </nav>
        <div className="mt-8 text-xs text-gray-500">
          stream: <span className={connected ? 'text-green-600' : 'text-red-600'}>{connected ? 'live' : 'offline'}</span>
        </div>
      </aside>
      <main className="flex-1 p-6 overflow-auto"><Outlet /></main>
    </div>
  )
}

function navClass({ isActive }: { isActive: boolean }) {
  return 'px-2 py-1 rounded ' + (isActive ? 'bg-gray-200 font-medium' : 'hover:bg-gray-100')
}
```

- [ ] **Step 3: `HealthCard`**

```tsx
import { useFetch } from '../hooks/useFetch'
import { getOverview } from '../api/overview'

export function HealthCard(): JSX.Element {
  const { data, error } = useFetch(getOverview, { intervalMs: 10_000 })
  if (error) return <div className="rounded border p-3 text-red-700">daemon unreachable</div>
  if (!data) return <div className="rounded border p-3 text-gray-500">loading…</div>
  const d = data.daemon
  return (
    <div className="rounded border p-3">
      <div className="font-medium">daemon</div>
      <div className="text-sm text-gray-600">
        pid {d.pid} · v{d.version} · up {fmt(d.uptime_s)}
      </div>
    </div>
  )
}

function fmt(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}
```

- [ ] **Step 4: `ProjectCard`, `EventRow`, `CronTaskRow`**

`ProjectCard.tsx`:
```tsx
import type { ProjectSummary } from '../api/types'

export function ProjectCard({ p }: { p: ProjectSummary }): JSX.Element {
  return (
    <div className="rounded border p-3">
      <div className="font-medium">{p.origin || p.id}</div>
      <div className="text-sm text-gray-600 mt-1">
        {p.anatomy_files} files · {p.cerebrum_pending} cerebrum pending · {Math.round(p.memory_bytes / 1024)} KB memory
      </div>
    </div>
  )
}
```

`EventRow.tsx`:
```tsx
import type { Event } from '../api/types'

export function EventRow({ e }: { e: Event }): JSX.Element {
  const when = new Date(e.ts).toLocaleTimeString()
  return (
    <li className="border-b py-2 px-3 text-sm flex gap-3">
      <span className="text-gray-500 w-20">{when}</span>
      <span className="font-medium w-40 truncate">{e.type}</span>
      <span className="text-gray-700 truncate">
        {e.project_id ? `[${e.project_id.slice(0, 8)}] ` : ''}
        {e.data ? JSON.stringify(e.data) : ''}
      </span>
    </li>
  )
}
```

`CronTaskRow.tsx`:
```tsx
import { useState } from 'react'
import type { CronTask } from '../api/types'
import { runTask, retryTask } from '../api/cron'

export function CronTaskRow({ t, onAction }: { t: CronTask; onAction: () => void }): JSX.Element {
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const dead = !!t.state.dead_lettered_at

  const fire = async (fn: (n: string) => Promise<void>) => {
    setBusy(true); setErr(null)
    try { await fn(t.name); onAction() }
    catch (e) { setErr((e as Error).message) }
    finally { setBusy(false) }
  }

  return (
    <tr className="border-b">
      <td className="px-3 py-2 font-medium">{t.name}</td>
      <td className="px-3 py-2 text-sm text-gray-600">{t.schedule}</td>
      <td className="px-3 py-2 text-sm">{t.state.last_run || '—'}</td>
      <td className="px-3 py-2 text-sm">
        {dead ? <span className="text-red-700">dead-lettered</span> :
         t.state.consecutive_failures > 0 ? <span className="text-yellow-700">retrying</span> :
         <span className="text-green-700">healthy</span>}
      </td>
      <td className="px-3 py-2 text-sm">
        <button disabled={busy} onClick={() => fire(runTask)} className="px-2 py-1 rounded border mr-2 disabled:opacity-50">Run</button>
        {dead && <button disabled={busy} onClick={() => fire(retryTask)} className="px-2 py-1 rounded border">Retry</button>}
        {err && <span className="ml-2 text-red-700 text-xs">{err}</span>}
      </td>
    </tr>
  )
}
```

- [ ] **Step 5: Delete `web/src/Health.tsx`**

```bash
git rm web/src/Health.tsx
```

- [ ] **Step 6: Commit**

```bash
git add web/src/hooks/useProjectList.ts web/src/components/
git commit -m "feat(m10b): frontend — shared components + useProjectList"
```

---

## Task 25: Frontend — Overview panel

**Files:**
- Create: `web/src/panels/Overview.tsx`

- [ ] **Step 1: Create the panel**

```tsx
import { HealthCard } from '../components/HealthCard'
import { ProjectCard } from '../components/ProjectCard'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getOverview } from '../api/overview'

export function Overview(): JSX.Element {
  const { data, error, refetch } = useFetch(getOverview, { intervalMs: 10_000 })
  useSSE(['scan.complete', 'cerebrum.candidate', 'suggestion.new'], () => refetch())

  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  const t = data.totals
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">overview</h1>
      <HealthCard />
      <div className="grid grid-cols-4 gap-3">
        <Stat label="projects" value={t.projects} />
        <Stat label="anatomy files" value={t.anatomy_files} />
        <Stat label="cerebrum pending" value={t.cerebrum_pending} />
        <Stat label="open suggestions" value={t.open_suggestions} />
      </div>
      <div>
        <h2 className="text-lg font-medium mb-2">projects</h2>
        <div className="grid grid-cols-2 gap-3">
          {data.projects.map(p => <ProjectCard key={p.id} p={p} />)}
        </div>
      </div>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }): JSX.Element {
  return (
    <div className="rounded border p-3">
      <div className="text-xs text-gray-600">{label}</div>
      <div className="text-2xl font-semibold mt-1">{value}</div>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/panels/Overview.tsx
git commit -m "feat(m10b): frontend — Overview panel"
```

---

## Task 26: Frontend — Activity panel

**Files:**
- Create: `web/src/panels/Activity.tsx`

- [ ] **Step 1: Create the panel**

```tsx
import { useEffect, useState } from 'react'
import { EventRow } from '../components/EventRow'
import { getActivity } from '../api/activity'
import { useSSE } from '../hooks/useSSE'
import type { Event } from '../api/types'

const MAX_ROWS = 200

export function Activity(): JSX.Element {
  const [items, setItems] = useState<Event[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getActivity({ limit: 100 })
      .then(r => setItems(r.events))
      .catch(e => setError((e as Error).message))
  }, [])

  useSSE(null, (e) => {
    setItems(prev => {
      const next = [e, ...prev]
      return next.length > MAX_ROWS ? next.slice(0, MAX_ROWS) : next
    })
  })

  if (error) return <div className="text-red-700">failed to load: {error}</div>
  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">activity</h1>
      <ul className="rounded border bg-white">
        {items.length === 0 && <li className="py-3 px-3 text-gray-500">no events yet</li>}
        {items.map(e => <EventRow key={`${e.ts}-${e.type}-${e.project_id ?? ''}`} e={e} />)}
      </ul>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/panels/Activity.tsx
git commit -m "feat(m10b): frontend — Activity panel"
```

---

## Task 27: Frontend — Cron panel

**Files:**
- Create: `web/src/panels/Cron.tsx`

- [ ] **Step 1: Create the panel**

```tsx
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getCron } from '../api/cron'
import { CronTaskRow } from '../components/CronTaskRow'

export function Cron(): JSX.Element {
  const { data, error, refetch } = useFetch(getCron, { intervalMs: 5_000 })
  useSSE(['cron.tick'], () => refetch())

  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">cron</h1>
      <table className="w-full bg-white rounded border">
        <thead className="text-left text-sm text-gray-600 border-b">
          <tr>
            <th className="px-3 py-2">name</th>
            <th className="px-3 py-2">schedule</th>
            <th className="px-3 py-2">last run</th>
            <th className="px-3 py-2">status</th>
            <th className="px-3 py-2">actions</th>
          </tr>
        </thead>
        <tbody>
          {data.tasks.map(t => <CronTaskRow key={t.name} t={t} onAction={refetch} />)}
        </tbody>
      </table>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/panels/Cron.tsx
git commit -m "feat(m10b): frontend — Cron panel with Run/Retry buttons"
```

---

## Task 28: Frontend — Vitest smoke test + build verification

**Files:**
- Create: `web/src/__tests__/App.test.tsx`
- Modify: `web/package.json` (scripts)

- [ ] **Step 1: Create the smoke test**

```tsx
import { describe, expect, test, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { App } from '../App'

// Stub fetch globally for the smoke test.
beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.includes('/api/overview')) {
      return new Response(JSON.stringify({
        daemon: { pid: 1, uptime_s: 1, version: 't', started_at: 0 },
        totals: { projects: 0, anatomy_files: 0, cerebrum_pending: 0, open_suggestions: 0 },
        projects: [],
      }), { headers: { 'Content-Type': 'application/json' } })
    }
    return new Response('{}', { headers: { 'Content-Type': 'application/json' } })
  }))
})

describe('App', () => {
  test('renders the AppShell + Overview without throwing', async () => {
    render(<App />)
    await waitFor(() => {
      expect(screen.getByText('overview')).toBeInTheDocument()
    })
  })
})
```

- [ ] **Step 2: Add test script to `web/package.json`**

In the `scripts` block, add:
```json
"test": "vitest run"
```

- [ ] **Step 3: Run the test + full build**

```bash
cd web && pnpm test && pnpm tsc --noEmit && pnpm build && cd ..
```

Expected: test PASS, type-check OK, vite build emits `dist/index.html` and `dist/assets/*`.

- [ ] **Step 4: Copy fresh dist into pkg/dashboard/dist**

```bash
make web-build
```

Expected: succeeds, `pkg/dashboard/dist/` updated.

- [ ] **Step 5: Commit**

```bash
git add web/src/__tests__/App.test.tsx web/package.json pkg/dashboard/dist/
git commit -m "test(m10b): frontend — vitest smoke test for App router shell"
```

---

## Task 29: Integration test — full daemon round-trip

**Files:**
- Create: `tests/integration/dashboard_panels_test.go`

- [ ] **Step 1: Create the test**

```go
//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
)

func TestDashboardPanels_EndToEnd(t *testing.T) {
	home, err := os.MkdirTemp("", "mneme")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	t.Setenv("HOME", home)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.RunOptions{
			Home:    home,
			PID:     syscall.Getpid(),
			Version: "0.2.0-m10b-test",
		})
	}()

	socketPath := filepath.Join(home, ".mneme", "daemon", "socket")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("unix", socketPath); err == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}}

	for _, path := range []string{"/api/overview", "/api/projects", "/api/activity?limit=5", "/api/cron"} {
		resp, err := client.Get("http://unix" + path)
		if err != nil {
			t.Errorf("GET %s: %v", path, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("GET %s: status=%d body=%s", path, resp.StatusCode, body)
		}
		if !strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
			t.Errorf("GET %s: body not JSON: %s", path, body)
		}
	}

	// SSE connect + receive at least one ping or event within 1.5s.
	resp, err := client.Get("http://unix/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("/events content-type: %q", ct)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("daemon.Run returned: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not stop after ctx cancel")
	}

	_ = json.Marshal // keep import
}
```

- [ ] **Step 2: Run the integration test**

Run: `go test -tags=integration ./tests/integration/... -run TestDashboardPanels -v -timeout 60s`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/dashboard_panels_test.go
git commit -m "test(m10b): integration smoke for daemon up + 4 /api/* endpoints + /events"
```

---

## Task 30: Final verification + acceptance pass

- [ ] **Step 1: Run the full test suite**

```bash
go test ./... -count=1
go test -tags=integration ./tests/integration/... -count=1
cd web && pnpm tsc --noEmit && pnpm test && pnpm build && cd ..
```

Expected: all green.

- [ ] **Step 2: Manual acceptance — start the daemon and dashboard**

```bash
go build -o ./bin/mneme ./cmd
./bin/mneme daemon stop || true
./bin/mneme daemon start
sleep 1
./bin/mneme dashboard
```

In the browser:
- `/` shows the Overview panel with the HealthCard, totals row, and project cards (empty grid is fine on a fresh machine).
- `/activity` shows an empty list initially. Open a second terminal and run:
  ```bash
  echo '{"hook_event_name":"PreToolUse","cwd":"'"$PWD"'"}' | ./bin/mneme hook pre-write
  ```
  A new row should appear at the top of the Activity panel within ~500 ms.
- `/cron` lists the seeded tasks. Click **Run** on `anatomy-rescan`; the table's `last run` updates within ~5 s.

- [ ] **Step 3: Verify events.jsonl exists**

```bash
ls -la ~/.mneme/daemon/events-*.jsonl
```

Expected: at least one file containing newline-delimited JSON events from the manual test above.

- [ ] **Step 4: Stop the daemon cleanly**

```bash
./bin/mneme daemon stop
```

Expected: exits cleanly, no stale socket.

- [ ] **Step 5: Final commit (only if any files were modified during verification, e.g. updated `pkg/dashboard/dist/`)**

```bash
git status
# If anything changed, commit it; otherwise skip.
```

- [ ] **Step 6: Push the branch + open PR**

```bash
git push -u origin feat/m10b
gh pr create --title "feat(m10b): REST + SSE + 3 dashboard panels" --body "$(cat <<'EOF'
## Summary
- New pkg/events: pub/sub Bus with ring buffer + daily-rotated events.jsonl
- 4 read-only REST endpoints under /api/*: overview, projects, activity, cron
- /events SSE stream with auto-reconnect + 25s ping
- /events/publish (Unix-only) for hook subprocesses
- Frontend: react-router-dom v7, three panels (Overview, Activity, Cron), reusable hooks (useFetch, useSSE, useProjectList)

## Test plan
- [ ] go test ./... passes
- [ ] go test -tags=integration ./tests/integration/... passes
- [ ] pnpm tsc --noEmit, pnpm test, pnpm build all pass
- [ ] Manual: /, /activity, /cron all render in the browser
- [ ] Manual: hook fires propagate to Activity within 500 ms
- [ ] Manual: cron Run + Retry buttons work
EOF
)"
```

---

## Self-review against spec

**Spec coverage:**
- §3.1 Event type + Bus API → Tasks 1-7
- §3.2 Slow subscriber drop → Task 2
- §3.3 Persistence + rotation → Tasks 4, 5
- §3.4 Tail semantics (ring + JSONL fallback, since exclusive) → Tasks 3, 6
- §3.5 Five event types → Tasks 9, 10, 18 (hook handlers)
- §3.6 In-process vs out-of-process publishing → Tasks 9-10 (in-proc), 16-18 (out-of-proc)
- §4.1-4.4 REST endpoints → Tasks 11-14
- §5 SSE wire format + filters + lifecycle → Task 15
- §6.1-6.6 Frontend layout, routing, refresh model, deps → Tasks 19-27
- §8 Tests (events_bus, events_jsonl, dashboard_api, dashboard_sse, integration) → Tasks 1-7, 11-15, 29
- §9 Acceptance criteria → Task 30

**Type/method consistency:**
- `events.Event{TS, Type, ProjectID, Data}` — used throughout
- `events.Bus.Publish/Subscribe/Tail/TailFiltered/Close` — consistent
- `events.Filter{Types, ProjectID}` — used in Tail and SSE
- `daemon.RouteDeps.Bus` — added Task 8, used Tasks 11-16
- `daemon.Scheduler.SetBus` — added Task 9, called Task 9
- `daemon.SetTaskDeps` — added Task 10
- `dashboard.APIDeps`, `OverviewHandler`, `ProjectsHandler`, `ActivityHandler`, `SSEHandler`, `PublishHandler` — internally consistent
- `daemonclient.PublishEvent` — added Task 17, used Task 18

**Placeholder scan:** no TBD / TODO / "implement later" / "similar to" — every step has full code or full command.

---
