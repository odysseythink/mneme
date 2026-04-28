package tests

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func TestAPI_Anatomy_GroupsByDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	if err := state.WriteAnatomy(root, []state.AnatomyEntry{
		{Path: "pkg/a/x.go", Description: "x", EstTokens: 100, Language: "go"},
		{Path: "pkg/a/y.go", Description: "y", EstTokens: 50, Language: "go"},
		{Path: "pkg/b/z.go", Description: "z", EstTokens: 75, Language: "go"},
	}); err != nil {
		t.Fatal(err)
	}

	bus, _ := events.NewBus(home, &stubLogger{})
	defer bus.Close()
	mux := daemon.NewMux(daemon.RouteDeps{Log: &stubLogger{}, Home: home, Bus: bus})

	req := httptest.NewRequest("GET", "/api/anatomy?project=p1", nil)
	req = req.WithContext(daemon.WithTransport(req.Context(), daemon.TransportUnix))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Directories []struct {
			Path  string                   `json:"path"`
			Files []map[string]interface{} `json:"files"`
		} `json:"directories"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Directories) != 2 {
		t.Errorf("directories: got %d, want 2", len(body.Directories))
	}
	for _, d := range body.Directories {
		if d.Path == "pkg/a" && len(d.Files) != 2 {
			t.Errorf("pkg/a: got %d files, want 2", len(d.Files))
		}
		if d.Path == "pkg/b" && len(d.Files) != 1 {
			t.Errorf("pkg/b: got %d files, want 1", len(d.Files))
		}
	}
}
