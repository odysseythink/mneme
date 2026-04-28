package dashboard

import "net/http"

func DesignQCHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		// Validate project=<id> exists; reuse the helper for parity with
		// other panels even though the stub does not read project state.
		if _, root := resolveProject(w, r); root == "" {
			return
		}
		writeJSON(w, 200, map[string]any{
			"available": false,
			"reason":    "M11 not yet implemented",
			"captures":  []any{},
		})
	}
}
