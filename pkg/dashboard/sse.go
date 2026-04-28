package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/events"
)

const ssePingInterval = 25 * time.Second

// SSEHandler streams bus events as text/event-stream.
func SSEHandler(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bus == nil {
			http.Error(w, "bus unavailable", http.StatusServiceUnavailable)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		filter := events.Filter{ProjectID: r.URL.Query().Get("project_id")}
		if t := r.URL.Query().Get("types"); t != "" {
			filter.Types = strings.Split(t, ",")
		}

		ch, unsub := bus.Subscribe()
		defer unsub()

		ping := time.NewTicker(ssePingInterval)
		defer ping.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				if !filter.Match(e) {
					continue
				}
				data, err := json.Marshal(e)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return
				}
				flusher.Flush()
			case <-ping.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

// transportFromCtxFunc is supplied by the daemon to avoid an import cycle.
type transportFromCtxFunc func(ctx context.Context) (isUnix bool)

// PublishHandler accepts events from in-process subprocesses (mneme hook).
// Rejected on TCP. Allowed on Unix socket.
func PublishHandler(bus *events.Bus, isUnix transportFromCtxFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isUnix(r.Context()) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		if bus == nil {
			http.Error(w, "bus unavailable", http.StatusServiceUnavailable)
			return
		}
		var in struct {
			Type      string          `json:"type"`
			ProjectID string          `json:"project_id"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, `{"error":"bad_body"}`, http.StatusBadRequest)
			return
		}
		if in.Type == "" {
			http.Error(w, `{"error":"missing_type"}`, http.StatusBadRequest)
			return
		}
		bus.Publish(events.Event{
			TS:        time.Now().UnixMilli(),
			Type:      in.Type,
			ProjectID: in.ProjectID,
			Data:      in.Data,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
