package tests

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
)

func TestAPI_DesignQC_Stub(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedProject(t, home, "p1")

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/designqc?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Available bool             `json:"available"`
		Reason    string           `json:"reason"`
		Captures  []map[string]any `json:"captures"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Available {
		t.Errorf("available should be false until M11")
	}
	if body.Captures == nil {
		t.Errorf("captures should be [] not nil")
	}
}
