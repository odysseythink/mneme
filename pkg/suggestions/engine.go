package suggestions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const refreshLockTimeout = 5 * time.Second

type Suggestion struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Target      string `json:"target"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	GeneratedAt string `json:"generated_at"`
}

func suggestionsPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "suggestions.json"), nil
}

// Refresh runs every generator and writes the unfiltered result.
func Refresh(projectRoot string, now time.Time) ([]Suggestion, error) {
	var all []Suggestion
	all = append(all, GenStaleAnatomy(projectRoot, now)...)
	all = append(all, GenStaleRule(projectRoot, now)...)
	all = append(all, GenUnreadFile(projectRoot, now)...)
	all = append(all, GenCoRead(projectRoot, now)...)

	p, err := suggestionsPath(projectRoot)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return nil, err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, refreshLockTimeout)
	if err != nil {
		return nil, err
	}
	defer release()

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := state.AtomicWrite(p, data); err != nil {
		return nil, err
	}
	return all, nil
}

// List reads suggestions.json and filters by dismissed TTL.
func List(projectRoot string, ttlDays int, now time.Time) ([]Suggestion, error) {
	p, err := suggestionsPath(projectRoot)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var all []Suggestion
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	suppressed, err := LoadDismissed(projectRoot, ttlDays, now)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, s := range all {
		if suppressed[s.ID] {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// HumanLine renders one suggestion as a single-line summary.
func HumanLine(s Suggestion) string {
	return fmt.Sprintf("[%s] %s — %s", s.ID[:8], s.Type, s.Title)
}
