package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type TurnEdit struct {
	File      string `json:"file"`
	Category  string `json:"category"`
	LineDelta int    `json:"line_delta"`
}

type TurnSummary struct {
	TurnIndex int        `json:"turn_index"`
	Edits     []TurnEdit `json:"edits"`
}

type Session struct {
	Version         int           `json:"_version"`
	SessionID       string        `json:"session_id"`
	ProjectID       string        `json:"project_id"`
	StartedAt       string        `json:"started_at"`
	ClaudeCodeModel string        `json:"claude_code_model"`
	StopCount       int           `json:"stop_count"`
	Reads           []string      `json:"reads,omitempty"`
	TurnEdits       []TurnEdit    `json:"turn_edits,omitempty"`
	TurnSummaries   []TurnSummary `json:"turn_summaries,omitempty"`
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

// ReadSession reads the current _session.json. Returns nil, nil if not found.
func ReadSession(projectRoot string) (*Session, error) {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(GlobalProjectDir(id), "_session.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// AppendTurnEdit atomically appends a TurnEdit to the session's TurnEdits buffer.
// Returns nil silently if no session file exists.
func AppendTurnEdit(projectRoot string, edit TurnEdit) error {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return err
	}
	dir := GlobalProjectDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lp := filepath.Join(dir, "runtime.lock")
	release, err := AcquireLock(lp, 1*time.Second)
	if err != nil {
		return err
	}
	defer release()

	path := filepath.Join(dir, "_session.json")
	s := readSessionNoLock(path)
	if s == nil {
		return nil
	}
	s.TurnEdits = append(s.TurnEdits, edit)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, data)
}

// AggregateTurn moves TurnEdits → TurnSummaries, clears the buffer, and increments StopCount.
func AggregateTurn(projectRoot string) error {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return err
	}
	dir := GlobalProjectDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lp := filepath.Join(dir, "runtime.lock")
	release, err := AcquireLock(lp, 1*time.Second)
	if err != nil {
		return err
	}
	defer release()

	path := filepath.Join(dir, "_session.json")
	s := readSessionNoLock(path)
	if s == nil {
		return nil
	}
	if len(s.TurnEdits) > 0 {
		s.TurnSummaries = append(s.TurnSummaries, TurnSummary{
			TurnIndex: s.StopCount,
			Edits:     s.TurnEdits,
		})
		s.TurnEdits = nil
	}
	s.StopCount++
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, data)
}

func readSessionNoLock(path string) *Session {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}
