package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LedgerTotals struct {
	HookFired             map[string]int `json:"hook_fired"`
	HookErrors            int            `json:"hook_errors"`
	StdinParseFailures    int            `json:"stdin_parse_failures"`
	OutsideProjectSkipped int            `json:"outside_project_skipped"`
	WriteSkipped          int            `json:"write_skipped"`
	AnatomyHits           int            `json:"anatomy_hits"` // M2
	RepeatReads           int            `json:"repeat_reads"` // M2
	ScanCount             int            `json:"scan_count"`   // M2
	EditPatterns          map[string]int `json:"edit_patterns,omitempty"`
	MemoryRowsWritten     int            `json:"memory_rows_written,omitempty"`
}

type Ledger struct {
	Version       int          `json:"_version"`
	ProjectID     string       `json:"project_id"`
	Totals        LedgerTotals `json:"totals"`
	FirstRecorded string       `json:"first_recorded"`
	LastUpdated   string       `json:"last_updated"`
}

const lockTimeout = 5 * time.Second

// IncrementSafe atomically increments a counter in the token-ledger.
// key format: "hook_fired.pre-read", "hook_errors", "stdin_parse_failures",
// "outside_project_skipped", "write_skipped".
func IncrementSafe(projectRoot string, key string) {
	// Acquire lock on local-id first to ensure concurrent calls get the same ID.
	localIDPath := filepath.Join(projectRoot, ".claude-context", ".local-id")
	localIDDir := filepath.Dir(localIDPath)
	if err := os.MkdirAll(localIDDir, 0755); err != nil {
		return
	}
	localIDLock := filepath.Join(localIDDir, ".local-id.lock")
	release, err := AcquireLock(localIDLock, lockTimeout)
	if err != nil {
		return
	}
	defer release()

	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return
	}
	dir := GlobalProjectDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	lp := filepath.Join(dir, "runtime.lock")
	release2, err := AcquireLock(lp, 1*time.Second)
	if err != nil {
		appendErrorLog(filepath.Join(dir, "hook-errors.log"),
			fmt.Sprintf("IncrementSafe lock timeout for key=%s: %v", key, err))
		return
	}
	defer release2()

	ledgerPath := filepath.Join(dir, "token-ledger.json")
	l := readLedgerNoLock(ledgerPath, id)
	l.increment(key)
	l.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	data, _ := json.MarshalIndent(l, "", "  ")
	AtomicWrite(ledgerPath, data)
}

// ReadLedger reads the current ledger for a project (no lock — snapshot read).
func ReadLedger(projectRoot string) (*Ledger, error) {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(GlobalProjectDir(id), "token-ledger.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyLedger(id), nil
		}
		return nil, err
	}
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return emptyLedger(id), nil
	}
	return &l, nil
}

func emptyLedger(projectID string) *Ledger {
	return &Ledger{
		Version:   1,
		ProjectID: projectID,
		Totals: LedgerTotals{
			HookFired:    make(map[string]int),
			EditPatterns: make(map[string]int),
		},
	}
}

func readLedgerNoLock(path, projectID string) *Ledger {
	data, err := os.ReadFile(path)
	if err != nil {
		l := emptyLedger(projectID)
		l.FirstRecorded = time.Now().UTC().Format(time.RFC3339)
		return l
	}
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		l2 := emptyLedger(projectID)
		l2.FirstRecorded = time.Now().UTC().Format(time.RFC3339)
		return l2
	}
	if l.Totals.HookFired == nil {
		l.Totals.HookFired = make(map[string]int)
	}
	if l.Totals.EditPatterns == nil {
		l.Totals.EditPatterns = make(map[string]int)
	}
	return &l
}

func (l *Ledger) increment(key string) {
	if strings.HasPrefix(key, "hook_fired.") {
		event := strings.TrimPrefix(key, "hook_fired.")
		if l.Totals.HookFired == nil {
			l.Totals.HookFired = make(map[string]int)
		}
		l.Totals.HookFired[event]++
		return
	}
	if strings.HasPrefix(key, "edit_pattern.") {
		cat := strings.TrimPrefix(key, "edit_pattern.")
		if l.Totals.EditPatterns == nil {
			l.Totals.EditPatterns = make(map[string]int)
		}
		l.Totals.EditPatterns[cat]++
		return
	}
	switch key {
	case "hook_errors":
		l.Totals.HookErrors++
	case "stdin_parse_failures":
		l.Totals.StdinParseFailures++
	case "outside_project_skipped":
		l.Totals.OutsideProjectSkipped++
	case "write_skipped":
		l.Totals.WriteSkipped++
	case "anatomy_hits":
		l.Totals.AnatomyHits++
	case "repeat_reads":
		l.Totals.RepeatReads++
	case "scan_count":
		l.Totals.ScanCount++
	case "memory_rows_written":
		l.Totals.MemoryRowsWritten++
	}
}

func appendErrorLog(path, msg string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
}
