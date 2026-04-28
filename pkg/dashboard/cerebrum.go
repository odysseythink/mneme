package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

// resolveProject reads ?project= from r, looks up the project root,
// writes an error response on failure. Returns ("", "") on failure.
func resolveProject(w http.ResponseWriter, r *http.Request) (string, string) {
	projectID := r.URL.Query().Get("project")
	if projectID == "" {
		http.Error(w, `{"error":"missing_project"}`, http.StatusBadRequest)
		return "", ""
	}
	root, err := state.ReadOrigin(projectID)
	if err != nil {
		http.Error(w, `{"error":"project_not_found"}`, http.StatusNotFound)
		return "", ""
	}
	return projectID, root
}

// decodeMutate parses a POST body of {project_id, <idField>: ...}.
// On failure, writes the error and returns (_, _, _, false).
func decodeMutate(w http.ResponseWriter, r *http.Request, idField string) (string, string, string, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return "", "", "", false
	}
	raw := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, `{"error":"bad_body"}`, http.StatusBadRequest)
		return "", "", "", false
	}
	pid := raw["project_id"]
	id := raw[idField]
	if pid == "" || id == "" {
		http.Error(w, `{"error":"missing_field"}`, http.StatusBadRequest)
		return "", "", "", false
	}
	root, err := state.ReadOrigin(pid)
	if err != nil {
		http.Error(w, `{"error":"project_not_found"}`, http.StatusNotFound)
		return "", "", "", false
	}
	return pid, root, id, true
}

func publishEvent(bus *events.Bus, projectID, eventType string, payload map[string]any) {
	if bus == nil {
		return
	}
	data, _ := json.Marshal(payload)
	bus.Publish(events.Event{
		TS:        time.Now().UnixMilli(),
		Type:      eventType,
		ProjectID: projectID,
		Data:      data,
	})
}

// CerebrumHandler returns active rules + pending candidates for the project.
func CerebrumHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		rules, err := state.ReadCerebrum(root)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		pending, err := cerebrum.LoadPending(root)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		if rules == nil {
			rules = []state.CerebrumRule{}
		}
		if pending == nil {
			pending = []cerebrum.Candidate{}
		}
		writeJSON(w, 200, map[string]any{
			"rules":   rules,
			"pending": pending,
		})
	}
}

func CerebrumApproveHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, candID, ok := decodeMutate(w, r, "candidate_id")
		if !ok {
			return
		}
		pending, err := cerebrum.LoadPending(root)
		if err != nil {
			http.Error(w, `{"error":"load_pending"}`, http.StatusInternalServerError)
			return
		}
		var found *cerebrum.Candidate
		for i := range pending {
			if pending[i].ID == candID {
				found = &pending[i]
				break
			}
		}
		if found == nil {
			http.Error(w, `{"error":"candidate_not_found"}`, http.StatusNotFound)
			return
		}
		if err := state.AppendCerebrumRule(root, found.DraftRule); err != nil {
			http.Error(w, `{"error":"append_rule"}`, http.StatusInternalServerError)
			return
		}
		if err := cerebrum.RemovePending(root, map[string]bool{candID: true}); err != nil {
			http.Error(w, `{"error":"remove_pending"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "cerebrum.approved", map[string]any{
			"candidate_id": candID, "rule_pattern": found.DraftRule.Pattern,
		})
		writeJSON(w, 200, map[string]any{"status": "approved"})
	}
}

func CerebrumRejectHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, candID, ok := decodeMutate(w, r, "candidate_id")
		if !ok {
			return
		}
		if err := cerebrum.AppendRejected(root, candID, time.Now().UTC()); err != nil {
			http.Error(w, `{"error":"append_rejected"}`, http.StatusInternalServerError)
			return
		}
		if err := cerebrum.RemovePending(root, map[string]bool{candID: true}); err != nil {
			http.Error(w, `{"error":"remove_pending"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "cerebrum.rejected", map[string]any{"candidate_id": candID})
		writeJSON(w, 200, map[string]any{"status": "rejected"})
	}
}
