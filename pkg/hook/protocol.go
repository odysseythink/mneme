package hook

import (
	"encoding/json"
	"io"
)

type Event struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	Cwd            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`

	PermissionMode string          `json:"permission_mode,omitempty"`

	ToolName       string          `json:"tool_name,omitempty"`
	ToolInput      json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID      string          `json:"tool_use_id,omitempty"`

	ToolResponse   json.RawMessage `json:"tool_response,omitempty"`
	DurationMs     *int            `json:"duration_ms,omitempty"`

	Source         string `json:"source,omitempty"`
	Model          string `json:"model,omitempty"`

	StopHookActive       *bool  `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
}

// ParseEvent parses an Event from a JSON reader.
func ParseEvent(r io.Reader) (*Event, error) {
	var ev Event
	if err := json.NewDecoder(r).Decode(&ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

// FilePathFromToolInput extracts tool_input.file_path from the raw JSON.
func (e *Event) FilePathFromToolInput() (string, bool) {
	if len(e.ToolInput) == 0 {
		return "", false
	}
	var ti struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(e.ToolInput, &ti); err != nil {
		return "", false
	}
	if ti.FilePath == "" {
		return "", false
	}
	return ti.FilePath, true
}

// IsRecursiveStop returns true when stop_hook_active is explicitly true.
func (e *Event) IsRecursiveStop() bool {
	return e.StopHookActive != nil && *e.StopHookActive
}
