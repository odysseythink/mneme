package tests

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func TestAPI_Token_TotalsAndHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	state.IncrementSafe(root, "hook_fired.pre-write")
	state.IncrementSafe(root, "scan_count")

	if err := state.AppendLedgerHistory(root, state.LedgerSnapshot{
		TS: time.Now().UTC().Format(time.RFC3339), SessionID: "s1",
	}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/token?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Totals  map[string]any   `json:"totals"`
		History []map[string]any `json:"history"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Totals == nil {
		t.Errorf("totals missing")
	}
	if len(body.History) != 1 {
		t.Errorf("history: got %d, want 1", len(body.History))
	}
}
