package events

import "encoding/json"

// Event is the canonical envelope for every dashboard event.
type Event struct {
	TS        int64           `json:"ts"` // unix milliseconds
	Type      string          `json:"type"`
	ProjectID string          `json:"project_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// Filter selects which events Tail/Subscribe-stream returns.
// Empty Types means "all types"; empty ProjectID means "all projects".
type Filter struct {
	Types     []string
	ProjectID string
}

func (f Filter) Match(e Event) bool {
	if f.ProjectID != "" && e.ProjectID != f.ProjectID {
		return false
	}
	if len(f.Types) > 0 {
		hit := false
		for _, t := range f.Types {
			if e.Type == t {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}
