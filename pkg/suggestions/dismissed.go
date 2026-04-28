package suggestions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const dismissedLockTimeout = 5 * time.Second

type dismissedRecord struct {
	ID          string `json:"id"`
	DismissedAt string `json:"dismissed_at"`
}

func dismissedPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "suggestions-dismissed.json"), nil
}

func AppendDismissed(projectRoot, id string, at time.Time) error {
	p, err := dismissedPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, dismissedLockTimeout)
	if err != nil {
		return err
	}
	defer release()

	var rec []dismissedRecord
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, &rec)
	}
	rec = append(rec, dismissedRecord{ID: id, DismissedAt: at.UTC().Format(time.RFC3339)})
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

func LoadDismissed(projectRoot string, ttlDays int, now time.Time) (map[string]bool, error) {
	p, err := dismissedPath(projectRoot)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var rec []dismissedRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rec))
	cutoff := now.Add(-time.Duration(ttlDays) * 24 * time.Hour)
	for _, r := range rec {
		ts, err := time.Parse(time.RFC3339, r.DismissedAt)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			out[r.ID] = true
		}
	}
	return out, nil
}
