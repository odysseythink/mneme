package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/daemon"
	"github.com/ranwei/mneme/pkg/state"
)

func TestCerebrumLearnHandlerEnqueues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	id, _ := state.ReadOrCreateLocalID(root)
	os.MkdirAll(state.GlobalProjectDir(id), 0755)
	os.WriteFile(filepath.Join(state.GlobalProjectDir(id), "origin"), []byte(root+"\n"), 0644)

	transcript := filepath.Join(tmp, "english.jsonl")
	src, _ := os.ReadFile("golden/m9/transcripts/english.jsonl")
	os.WriteFile(transcript, src, 0644)

	h := daemon.NewCerebrumLearnHandler(daemon.CerebrumLearnDeps{
		LearningEnabled: true,
		RejectTTLDays:   90,
		Now:             func() time.Time { return time.Now().UTC() },
	})

	body, _ := json.Marshal(map[string]string{
		"project_id":      id,
		"transcript_path": transcript,
		"session_id":      "s1",
	})
	req := httptest.NewRequest("POST", "/cerebrum/learn", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202; body=%s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cs, _ := cerebrum.LoadPending(root)
		if len(cs) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("no candidates queued within timeout")
}

func TestCerebrumLearnHandlerDisabled(t *testing.T) {
	h := daemon.NewCerebrumLearnHandler(daemon.CerebrumLearnDeps{LearningEnabled: false})
	req := httptest.NewRequest("POST", "/cerebrum/learn", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var out struct {
		Disabled bool `json:"disabled"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.Disabled {
		t.Errorf("expected disabled=true")
	}
}
