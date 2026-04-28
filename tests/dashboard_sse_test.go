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
