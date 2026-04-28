package daemon

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/state"
)

const cerebrumQueueCapacity = 100

type CerebrumLearnRequest struct {
	ProjectID      string `json:"project_id"`
	TranscriptPath string `json:"transcript_path"`
	SessionID      string `json:"session_id"`
}

type CerebrumLearnDeps struct {
	LearningEnabled    bool
	RejectTTLDays      int
	Now                func() time.Time
	ResolveProjectRoot func(projectID string) (string, error)
}

type cerebrumLearnHandler struct {
	deps  CerebrumLearnDeps
	queue chan CerebrumLearnRequest
	stop  chan struct{}

	mu     sync.Mutex
	inProg map[string]bool
}

func NewCerebrumLearnHandler(deps CerebrumLearnDeps) http.Handler {
	if deps.ResolveProjectRoot == nil {
		deps.ResolveProjectRoot = defaultResolveProjectRoot
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	h := &cerebrumLearnHandler{
		deps:   deps,
		queue:  make(chan CerebrumLearnRequest, cerebrumQueueCapacity),
		stop:   make(chan struct{}),
		inProg: map[string]bool{},
	}
	go h.workerLoop()
	return h
}

func (h *cerebrumLearnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.deps.LearningEnabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"disabled":true}`))
		return
	}
	var req CerebrumLearnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.ProjectID == "" || req.TranscriptPath == "" || req.SessionID == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}
	select {
	case h.queue <- req:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"queued":true}`))
	default:
		http.Error(w, "queue full", http.StatusServiceUnavailable)
	}
}

func (h *cerebrumLearnHandler) workerLoop() {
	for {
		select {
		case <-h.stop:
			return
		case req := <-h.queue:
			h.processOne(req)
		}
	}
}

func (h *cerebrumLearnHandler) processOne(req CerebrumLearnRequest) {
	h.mu.Lock()
	if h.inProg[req.ProjectID] {
		h.mu.Unlock()
		return
	}
	h.inProg[req.ProjectID] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.inProg, req.ProjectID)
		h.mu.Unlock()
	}()

	projectRoot, err := h.deps.ResolveProjectRoot(req.ProjectID)
	if err != nil {
		return
	}
	defer func() { _ = recover() }()
	cerebrum.Learn(req.TranscriptPath, projectRoot, req.SessionID, h.deps.Now())
}

func defaultResolveProjectRoot(projectID string) (string, error) {
	dir := state.GlobalProjectDir(projectID)
	if data, err := os.ReadFile(filepath.Join(dir, "origin")); err == nil {
		root := string(data)
		for len(root) > 0 && (root[len(root)-1] == '\n' || root[len(root)-1] == '\r' || root[len(root)-1] == ' ') {
			root = root[:len(root)-1]
		}
		if root != "" {
			return root, nil
		}
	}
	return dir, nil
}
