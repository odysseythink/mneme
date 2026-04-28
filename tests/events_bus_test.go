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
