package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

type jsonlWriter struct {
	home  string
	clock func() time.Time
	log   Logger

	mu      sync.Mutex
	current *os.File
	curDate string
}

func newJSONLWriter(home string, clock func() time.Time, log Logger) *jsonlWriter {
	return &jsonlWriter{home: home, clock: clock, log: log}
}

func (w *jsonlWriter) append(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		w.log.Error("events", fmt.Sprintf("marshal event: %v", err))
		return
	}
	data = append(data, '\n')
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.openTodayLocked(); err != nil {
		w.log.Error("events", fmt.Sprintf("open events file: %v", err))
		return
	}
	if _, err := w.current.Write(data); err != nil {
		w.log.Error("events", fmt.Sprintf("write event: %v", err))
	}
}

const retentionDays = 14

func (w *jsonlWriter) pruneOldLocked() {
	dir := state.DaemonDir(w.home)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := w.clock().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() ||
			!strings.HasPrefix(e.Name(), "events-") ||
			!strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func (w *jsonlWriter) openTodayLocked() error {
	date := w.clock().UTC().Format("20060102")
	if w.current != nil && w.curDate == date {
		return nil
	}
	if w.current != nil {
		_ = w.current.Close()
		w.current = nil
	}
	if err := os.MkdirAll(state.DaemonDir(w.home), 0o700); err != nil {
		return err
	}
	path := state.DaemonEventsPath(w.home, date)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	w.current = f
	w.curDate = date
	w.pruneOldLocked()
	return nil
}

func (w *jsonlWriter) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		err := w.current.Close()
		w.current = nil
		return err
	}
	return nil
}

func (w *jsonlWriter) tail(limit int, since int64) []Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	dir := state.DaemonDir(w.home)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Collect events files newest-first by name (date-sorted lexicographically).
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() ||
			!strings.HasPrefix(e.Name(), "events-") ||
			!strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		names = append(names, e.Name())
	}
	// Sort descending so newest dates come first.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] > names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}

	out := make([]Event, 0, limit)
	for _, n := range names {
		// Read the whole file (small per day at expected rates).
		data, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		// Parse newest-line-first by scanning end-to-start.
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
			if lines[i] == "" {
				continue
			}
			var e Event
			if err := json.Unmarshal([]byte(lines[i]), &e); err != nil {
				continue
			}
			if e.TS > since {
				out = append(out, e)
			}
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}
