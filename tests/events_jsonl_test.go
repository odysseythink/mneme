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

type stepClock struct {
	t time.Time
}

func (c *stepClock) now() time.Time          { return c.t }
func (c *stepClock) advance(d time.Duration) { c.t = c.t.Add(d) }
