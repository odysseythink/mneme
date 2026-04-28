package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ranwei/mneme/pkg/dashboard"
	"github.com/ranwei/mneme/pkg/events"
)

// RouteDeps wires handler dependencies.
type RouteDeps struct {
	Log       Logger
	Home      string
	PID       int
	Version   string
	StartedAt int64 // unix seconds
	Scheduler *Scheduler
	Token     string // M10a: dev_token handler
	DevMode   bool   // M10a: enables /dev-token
	Bus       *events.Bus // M10b
}

// NewMux builds the handler tree.
func NewMux(deps RouteDeps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", deps.health)
	mux.HandleFunc("/status", deps.status)
	mux.HandleFunc("/scan", deps.scan)
	mux.HandleFunc("/update", deps.update)
	mux.HandleFunc("/restore", deps.restore)
	mux.HandleFunc("/cron/list", deps.cronList)
	mux.HandleFunc("/cron/run", deps.cronRun)
	mux.HandleFunc("/cron/retry", deps.cronRetry)
	// M10a: dashboard static + dev-token (must come last so specific paths win)
	mux.Handle("/dev-token", dashboard.DevTokenHandler(deps.Token, deps.DevMode))
	dashboard.Mount(mux, dashboard.Deps{})
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, code int, errCode, message string) {
	writeJSON(w, code, map[string]string{"error": message, "code": errCode})
}

func (d *RouteDeps) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method", "GET only")
		return
	}
	now := time.Now().Unix()
	uptime := now - d.StartedAt
	if d.StartedAt == 0 {
		uptime = 0
	}
	writeJSON(w, 200, map[string]interface{}{
		"pid":        d.PID,
		"uptime_s":   uptime,
		"version":    d.Version,
		"started_at": d.StartedAt,
	})
}

func (d *RouteDeps) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method", "GET only")
		return
	}
	taskCount := 0
	if d.Home != "" {
		if m, err := LoadManifest(d.Home); err == nil {
			taskCount = len(m.Tasks)
		}
	}
	writeJSON(w, 200, map[string]interface{}{
		"pid":            d.PID,
		"version":        d.Version,
		"manifest_tasks": taskCount,
	})
}

func (d *RouteDeps) scan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method", "POST only")
		return
	}
	if d.Scheduler == nil {
		writeError(w, http.StatusNotImplemented, "no_scheduler", "scheduler not configured")
		return
	}
	var body struct {
		ProjectID string `json:"project_id"`
		Force     bool   `json:"force"`
		Check     bool   `json:"check"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := d.Scheduler.RunOnce(r.Context(), "anatomy-rescan"); err != nil {
		writeError(w, http.StatusInternalServerError, "scan_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "scheduled"})
}

func (d *RouteDeps) update(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method", "POST only")
		return
	}
	writeError(w, http.StatusNotImplemented, "m7_required",
		"update requires M7 multi-project sync; not yet implemented")
}

func (d *RouteDeps) restore(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method", "POST only")
		return
	}
	writeError(w, http.StatusNotImplemented, "m7_required",
		"restore requires M7 backup logic; not yet implemented")
}

func (d *RouteDeps) cronList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method", "GET only")
		return
	}
	if d.Home == "" {
		writeError(w, http.StatusInternalServerError, "no_home", "Home not configured")
		return
	}
	m, err := LoadOrSeedManifest(d.Home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "manifest_load", err.Error())
		return
	}
	st, _ := LoadCronState(d.Home)

	type taskOut struct {
		Name     string    `json:"name"`
		Schedule string    `json:"schedule"`
		Enabled  bool      `json:"enabled"`
		State    TaskState `json:"state"`
	}
	out := make([]taskOut, 0, len(m.Tasks))
	for _, t := range m.Tasks {
		out = append(out, taskOut{
			Name: t.Name, Schedule: t.Schedule, Enabled: t.Enabled, State: st.Tasks[t.Name],
		})
	}
	writeJSON(w, 200, map[string]interface{}{"tasks": out})
}

func (d *RouteDeps) cronRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method", "POST only")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", err.Error())
		return
	}
	if LookupTask(body.Name) == nil {
		writeError(w, http.StatusBadRequest, "unknown_task", fmt.Sprintf("unknown task: %s", body.Name))
		return
	}
	if d.Scheduler == nil {
		writeError(w, http.StatusNotImplemented, "no_scheduler", "scheduler not configured")
		return
	}
	if err := d.Scheduler.RunOnce(r.Context(), body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "task_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "scheduled"})
}

func (d *RouteDeps) cronRetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method", "POST only")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", err.Error())
		return
	}
	if d.Scheduler == nil {
		writeError(w, http.StatusNotImplemented, "no_scheduler", "scheduler not configured")
		return
	}
	if err := d.Scheduler.ClearDeadLetter(body.Name); err != nil {
		writeError(w, http.StatusBadRequest, "clear_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "cleared"})
}
