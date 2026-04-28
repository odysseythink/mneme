package cerebrum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const pendingLockTimeout = 5 * time.Second

type Trigger struct {
	Phrase    string `json:"phrase"`
	UserMsg   string `json:"user_msg"`
	PriorAsst string `json:"prior_asst"`
	Turn      int    `json:"turn"`
}

type Candidate struct {
	ID         string             `json:"id"`
	Trigger    Trigger            `json:"trigger"`
	DraftRule  state.CerebrumRule `json:"draft_rule"`
	Confidence float64            `json:"confidence"`
	QueuedAt   string             `json:"queued_at"`
	HitCount   int                `json:"hit_count"`
}

type rejectedRecord struct {
	ID         string `json:"id"`
	RejectedAt string `json:"rejected_at"`
}

// NewCandidateID returns a stable 16-char hex hash of phrase + priorAsst.
func NewCandidateID(phrase, priorAsst string) string {
	h := sha256.Sum256([]byte("v1|" + phrase + "|" + priorAsst))
	return hex.EncodeToString(h[:8])
}

func pendingPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "cerebrum-pending.json"), nil
}

func rejectedPath(projectRoot string) (string, error) {
	id, err := state.ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(state.GlobalProjectDir(id), "cerebrum-rejected.json"), nil
}

func LoadPending(projectRoot string) ([]Candidate, error) {
	p, err := pendingPath(projectRoot)
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
	var cs []Candidate
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, err
	}
	return cs, nil
}

func SavePending(projectRoot string, cs []Candidate) error {
	p, err := pendingPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, pendingLockTimeout)
	if err != nil {
		return err
	}
	defer release()
	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

func AppendPending(projectRoot string, c Candidate) error {
	p, err := pendingPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, pendingLockTimeout)
	if err != nil {
		return err
	}
	defer release()

	var existing []Candidate
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, &existing)
	}
	for i := range existing {
		if existing[i].ID == c.ID {
			existing[i].HitCount += c.HitCount
			data, _ := json.MarshalIndent(existing, "", "  ")
			return state.AtomicWrite(p, data)
		}
	}
	existing = append(existing, c)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

func AppendRejected(projectRoot string, id string, rejectedAt time.Time) error {
	p, err := rejectedPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	lock := p + ".lock"
	release, err := state.AcquireLock(lock, pendingLockTimeout)
	if err != nil {
		return err
	}
	defer release()

	var rec []rejectedRecord
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, &rec)
	}
	rec = append(rec, rejectedRecord{ID: id, RejectedAt: rejectedAt.UTC().Format(time.RFC3339)})
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(p, data)
}

func LoadRejected(projectRoot string, ttlDays int, now time.Time) (map[string]bool, error) {
	p, err := rejectedPath(projectRoot)
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
	var rec []rejectedRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rec))
	cutoff := now.Add(-time.Duration(ttlDays) * 24 * time.Hour)
	for _, r := range rec {
		ts, err := time.Parse(time.RFC3339, r.RejectedAt)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			out[r.ID] = true
		}
	}
	return out, nil
}

func RemovePending(projectRoot string, remove map[string]bool) error {
	cs, err := LoadPending(projectRoot)
	if err != nil {
		return err
	}
	kept := cs[:0]
	for _, c := range cs {
		if !remove[c.ID] {
			kept = append(kept, c)
		}
	}
	return SavePending(projectRoot, kept)
}
