package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	Version         int    `json:"_version"`
	SessionID       string `json:"session_id"`
	ProjectID       string `json:"project_id"`
	StartedAt       string `json:"started_at"`
	ClaudeCodeModel string `json:"claude_code_model"`
	StopCount       int    `json:"stop_count"`
}

// UpsertSession writes (or overwrites) _session.json in the global project dir.
func UpsertSession(projectRoot string, s Session) error {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return err
	}
	s.Version = 1
	s.ProjectID = id
	if s.StartedAt == "" {
		s.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}

	dir := GlobalProjectDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(filepath.Join(dir, "_session.json"), data)
}
