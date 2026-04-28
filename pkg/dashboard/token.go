package dashboard

import (
	"net/http"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const tokenHistoryCap = 12

func TokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		ledger, err := state.ReadLedger(root)
		if err != nil {
			http.Error(w, `{"error":"read_ledger"}`, http.StatusInternalServerError)
			return
		}
		hist, _ := state.ReadLedgerHistory(root, time.Time{})
		if len(hist) > tokenHistoryCap {
			hist = hist[len(hist)-tokenHistoryCap:]
		}
		out := map[string]any{
			"totals":         ledger.Totals,
			"first_recorded": ledger.FirstRecorded,
			"last_updated":   ledger.LastUpdated,
			"history":        hist,
		}
		writeJSON(w, 200, out)
	}
}
