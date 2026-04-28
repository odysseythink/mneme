package events

import (
	"encoding/json"
	"fmt"
	"os"
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
