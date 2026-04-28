package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/suggestions"
)

func SuggestionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projectID, root := resolveProject(w, r)
		if root == "" {
			return
		}
		path := filepath.Join(state.GlobalProjectDir(projectID), "suggestions.json")
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			http.Error(w, `{"error":"read"}`, http.StatusInternalServerError)
			return
		}
		var all []suggestions.Suggestion
		if len(data) > 0 {
			_ = json.Unmarshal(data, &all)
		}
		dismissed, _ := suggestions.LoadDismissed(root, 30, time.Now().UTC())
		out := make([]suggestions.Suggestion, 0, len(all))
		for _, s := range all {
			if !dismissed[s.ID] {
				out = append(out, s)
			}
		}
		writeJSON(w, 200, map[string]any{"suggestions": out})
	}
}

func SuggestionsDismissHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, root, sid, ok := decodeMutate(w, r, "suggestion_id")
		if !ok {
			return
		}
		if err := suggestions.AppendDismissed(root, sid, time.Now().UTC()); err != nil {
			http.Error(w, `{"error":"append_dismissed"}`, http.StatusInternalServerError)
			return
		}
		publishEvent(bus, pid, "suggestion.dismissed", map[string]any{"suggestion_id": sid})
		writeJSON(w, 200, map[string]any{"status": "dismissed"})
	}
}
