package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	Version         int      `json:"_version"`
	SessionID       string   `json:"session_id"`
	ProjectID       string   `json:"project_id"`
	StartedAt       string   `json:"started_at"`
	ClaudeCodeModel string   `json:"claude_code_model"`
	StopCount       int      `json:"stop_count"`
	Reads           []string `json:"reads,omitempty"`
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

// AppendSessionRead appends relPath to the current session's Reads list.
// Returns (true, nil) if relPath was already in Reads.
// Returns (false, nil) if relPath was newly appended.
// If the session file does not exist, returns (false, nil) without error.
func AppendSessionRead(projectRoot, relPath string) (alreadyRead bool, err error) {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return false, err
	}

	globalDir := GlobalProjectDir(id)
	lockPath := filepath.Join(globalDir, "runtime.lock")
	release, err := AcquireLock(lockPath, 50*time.Millisecond)
	if err != nil {
		return false, err
	}
	defer release()

	sessionPath := filepath.Join(globalDir, "_session.json")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return false, err
	}

	// Check if relPath already in Reads
	for _, r := range s.Reads {
		if r == relPath {
			return true, nil
		}
	}

	// Append and write back
	s.Reads = append(s.Reads, relPath)
	updated, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return false, err
	}

	return false, AtomicWrite(sessionPath, updated)
}
