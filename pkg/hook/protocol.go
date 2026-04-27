package hook

import (
	"encoding/json"
	"io"
	"strings"
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

// ExtractAddedLines extracts newly added lines from tool operations.
// For Edit, it computes the diff between old_string and new_string.
// For Write, it uses the file content as new lines (old is empty).
// For MultiEdit, it concatenates all edits into old and new strings.
// Returns a map of 1-indexed line numbers to line content.
// Returns nil if no new lines were added or on error.
func (e *Event) ExtractAddedLines() map[int]string {
	var oldStr, newStr string

	switch e.ToolName {
	case "Write":
		var ti struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(e.ToolInput, &ti); err != nil {
			return nil
		}
		newStr = ti.Content
	case "Edit":
		var ti struct {
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if err := json.Unmarshal(e.ToolInput, &ti); err != nil {
			return nil
		}
		oldStr = ti.OldString
		newStr = ti.NewString
	case "MultiEdit":
		var ti struct {
			Edits []struct {
				OldString string `json:"old_string"`
				NewString string `json:"new_string"`
			} `json:"edits"`
		}
		if err := json.Unmarshal(e.ToolInput, &ti); err != nil {
			return nil
		}
		var oldParts, newParts []string
		for _, ed := range ti.Edits {
			oldParts = append(oldParts, ed.OldString)
			newParts = append(newParts, ed.NewString)
		}
		oldStr = strings.Join(oldParts, "\n")
		newStr = strings.Join(newParts, "\n")
	default:
		return nil
	}

	oldSet := make(map[string]bool)
	for _, line := range strings.Split(oldStr, "\n") {
		oldSet[line] = true
	}

	result := make(map[int]string)
	for i, line := range strings.Split(newStr, "\n") {
		if !oldSet[line] {
			result[i+1] = line
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
