package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ledgerHistoryMaxBytes = 1 << 20 // 1 MiB

type LedgerSnapshot struct {
	TS        string       `json:"ts"`
	SessionID string       `json:"session_id"`
	Totals    LedgerTotals `json:"totals"`
}

// AppendLedgerHistory appends a snapshot line to ledger-history.jsonl.
// If the file grows beyond 1 MiB after the append, drops the oldest 25%.
func AppendLedgerHistory(projectRoot string, snap LedgerSnapshot) error {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return err
	}
	dir := GlobalProjectDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "ledger-history.lock")
	release, err := AcquireLock(lockPath, lockTimeout)
	if err != nil {
		return err
	}
	defer release()

	path := filepath.Join(dir, "ledger-history.jsonl")
	line, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if info, err := os.Stat(path); err == nil && info.Size() > ledgerHistoryMaxBytes {
		return trimLedgerHistory(path)
	}
	return nil
}

// ReadLedgerHistory returns snapshots with TS >= since.
func ReadLedgerHistory(projectRoot string, since time.Time) ([]LedgerSnapshot, error) {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(GlobalProjectDir(id), "ledger-history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []LedgerSnapshot
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var snap LedgerSnapshot
		if err := json.Unmarshal([]byte(line), &snap); err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, snap.TS)
		if err != nil {
			continue
		}
		if ts.Before(since) {
			continue
		}
		out = append(out, snap)
	}
	return out, sc.Err()
}

func trimLedgerHistory(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 8 {
		return nil
	}
	keep := lines[len(lines)/4:]
	out := strings.Join(keep, "\n") + "\n"
	return AtomicWrite(path, []byte(out))
}
