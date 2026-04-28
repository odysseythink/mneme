package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// BuglogEntry records a previously fixed bug for re-introduction detection.
type BuglogEntry struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	Source      string `json:"source"`      // "auto" | "manual"
	File        string `json:"file"`
	Description string `json:"description"`
	BadCode     string `json:"bad_code"`
}

type buglogFile struct {
	Version int           `json:"version"`
	Entries []BuglogEntry `json:"entries"`
}

func buglogPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".mneme", "buglog.json")
}

// ReadBuglog reads buglog.json; returns nil, nil if absent or corrupt.
// Unlocked read — stale is acceptable in hook context.
func ReadBuglog(projectRoot string) ([]BuglogEntry, error) {
	data, err := os.ReadFile(buglogPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f buglogFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, nil // corrupt → treat as empty
	}
	return f.Entries, nil
}

// AppendBuglogEntry appends one entry to buglog.json, creating the file if absent.
// Auto-sets ID and CreatedAt if empty. Uses flock-X with 50ms timeout.
func AppendBuglogEntry(projectRoot string, e BuglogEntry) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.CreatedAt == "" {
		e.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	lockPath := filepath.Join(projectRoot, ".mneme", "buglog.lock")
	release, err := AcquireLock(lockPath, 50*time.Millisecond)
	if err != nil {
		return err
	}
	defer release()

	entries, _ := ReadBuglog(projectRoot)
	entries = append(entries, e)
	return writeBuglogNoLock(projectRoot, entries)
}

// WriteBuglog overwrites buglog.json with the given slice.
// Passing nil writes an empty entries array. Uses flock-X with 50ms timeout.
func WriteBuglog(projectRoot string, entries []BuglogEntry) error {
	lockPath := filepath.Join(projectRoot, ".mneme", "buglog.lock")
	release, err := AcquireLock(lockPath, 50*time.Millisecond)
	if err != nil {
		return err
	}
	defer release()
	return writeBuglogNoLock(projectRoot, entries)
}

func writeBuglogNoLock(projectRoot string, entries []BuglogEntry) error {
	if entries == nil {
		entries = []BuglogEntry{}
	}
	f := buglogFile{Version: 1, Entries: entries}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	path := buglogPath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return AtomicWrite(path, data)
}
