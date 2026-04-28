package tests

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
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
}
