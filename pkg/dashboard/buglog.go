package dashboard

import (
	"net/http"

	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
)

func BugLogHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		entries, err := state.ReadBuglog(root)
		if err != nil {
			http.Error(w, `{"error":"read_buglog"}`, http.StatusInternalServerError)
			return
		}
		if entries == nil {
			entries = []state.BuglogEntry{}
		}
		writeJSON(w, 200, map[string]any{"entries": entries})
	}
}

func BugLogDeleteHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, entryID, ok := decodeMutate(w, r, "entry_id")
		if !ok {
			return
		}
		entries, err := state.ReadBuglog(root)
		if err != nil {
			http.Error(w, `{"error":"read_buglog"}`, http.StatusInternalServerError)
			return
		}
		var kept []state.BuglogEntry
		found := false
		for _, e := range entries {
			if e.ID == entryID {
				found = true
				continue
			}
			kept = append(kept, e)
		}
		if !found {
			http.Error(w, `{"error":"entry_not_found"}`, http.StatusNotFound)
			return
		}
		if err := state.WriteBuglog(root, kept); err != nil {
			http.Error(w, `{"error":"write_buglog"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "buglog.deleted", map[string]any{"entry_id": entryID})
		writeJSON(w, 200, map[string]any{"status": "deleted"})
	}
}
