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
