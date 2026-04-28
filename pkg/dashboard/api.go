package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// APIDeps is the cross-cutting set of dependencies the API handlers need.
// Filled in by daemon at mount time.
type APIDeps struct {
	Home      string
	PID       int
	Version   string
	StartedAt int64
}

// ProjectSummary is the per-project shape used by Overview + Projects.
type ProjectSummary struct {
	ID              string `json:"id"`
	Origin          string `json:"origin"`
	AnatomyFiles    int    `json:"anatomy_files"`
	CerebrumPending int    `json:"cerebrum_pending"`
	MemoryBytes     int64  `json:"memory_bytes"`
	LastActivityTS  int64  `json:"last_activity_ts"`
}

// OverviewResponse mirrors the Go-to-JSON shape promised by the spec.
type OverviewResponse struct {
	Daemon struct {
		PID       int    `json:"pid"`
		UptimeS   int64  `json:"uptime_s"`
		Version   string `json:"version"`
		StartedAt int64  `json:"started_at"`
	} `json:"daemon"`
	Totals struct {
		Projects        int `json:"projects"`
		AnatomyFiles    int `json:"anatomy_files"`
		CerebrumPending int `json:"cerebrum_pending"`
		OpenSuggestions int `json:"open_suggestions"`
	} `json:"totals"`
	Projects []ProjectSummary `json:"projects"`
}

// EnumerateProjects walks ~/.mneme/projects/<id>/ and assembles summaries.
func EnumerateProjects(home string) ([]ProjectSummary, error) {
	root := filepath.Join(home, ".mneme", "projects")
	out := []ProjectSummary{}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ps := ProjectSummary{ID: e.Name()}
		dir := filepath.Join(root, e.Name())
		if data, err := os.ReadFile(filepath.Join(dir, "origin")); err == nil {
			ps.Origin = strings.TrimSpace(string(data))
		}
		if data, err := os.ReadFile(filepath.Join(dir, "anatomy.md")); err == nil {
			ps.AnatomyFiles = strings.Count(string(data), "\n## ")
		}
		if data, err := os.ReadFile(filepath.Join(dir, "cerebrum-pending.json")); err == nil {
			ps.CerebrumPending = strings.Count(string(data), "{")
		}
		if info, err := os.Stat(filepath.Join(dir, "memory.md")); err == nil {
			ps.MemoryBytes = info.Size()
		}
		out = append(out, ps)
	}
	return out, nil
}

// OverviewHandler serves /api/overview.
func OverviewHandler(deps APIDeps, openSuggestionsCount func() int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projects, err := EnumerateProjects(deps.Home)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var resp OverviewResponse
		resp.Daemon.PID = deps.PID
		resp.Daemon.Version = deps.Version
		resp.Daemon.StartedAt = deps.StartedAt
		if deps.StartedAt > 0 {
			resp.Daemon.UptimeS = time.Now().Unix() - deps.StartedAt
		}
		resp.Projects = projects
		resp.Totals.Projects = len(projects)
		for _, p := range projects {
			resp.Totals.AnatomyFiles += p.AnatomyFiles
			resp.Totals.CerebrumPending += p.CerebrumPending
		}
		if openSuggestionsCount != nil {
			resp.Totals.OpenSuggestions = openSuggestionsCount()
		}
		writeJSON(w, 200, resp)
	}
}

// ProjectsHandler serves /api/projects.
func ProjectsHandler(deps APIDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projects, err := EnumerateProjects(deps.Home)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, 200, map[string]interface{}{"projects": projects})
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
