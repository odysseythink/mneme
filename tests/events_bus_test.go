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
