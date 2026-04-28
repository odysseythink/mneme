package events

import "encoding/json"

// Event is the canonical envelope for every dashboard event.
type Event struct {
	TS        int64           `json:"ts"` // unix milliseconds
	Type      string          `json:"type"`
	ProjectID string          `json:"project_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}
