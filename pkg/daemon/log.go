package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

// Logger is the daemon log interface used by scheduler/server/handlers.
type Logger interface {
	Debug(component, msg string)
	Info(component, msg string)
	Warn(component, msg string)
	Error(component, msg string)
	Rotate() error
	Close() error
}

type level int

const (
	levelDebug level = iota
	levelInfo
	levelWarn
	levelError
)

func parseLevel(s string) level {
	switch strings.ToLower(s) {
	case "debug":
		return levelDebug
	case "info", "":
		return levelInfo
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	}
	return levelInfo
}

func (l level) name() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR"}[l]
}

type fileLogger struct {
	home  string
	min   level
	clock func() time.Time

	mu      sync.Mutex
	current *os.File
	curDate string
}

// NewFileLogger creates a new daily-rotated logger.
func NewFileLogger(home, levelName string, clock func() time.Time) (Logger, error) {
	if clock == nil {
		clock = time.Now
	}
	if err := os.MkdirAll(state.DaemonLogsDir(home), 0o700); err != nil {
		return nil, err
	}
	lg := &fileLogger{home: home, min: parseLevel(levelName), clock: clock}
	if err := lg.openForToday(); err != nil {
		return nil, err
	}
	return lg, nil
}

func (l *fileLogger) openForTodayLocked() error {
	date := l.clock().Format("20060102")
	if l.current != nil && l.curDate == date {
		return nil
	}
	if l.current != nil {
		_ = l.current.Close()
		l.current = nil
	}
	path := state.DaemonLogPath(l.home, date)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	l.current = f
	l.curDate = date
	return nil
}

func (l *fileLogger) openForToday() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.openForTodayLocked()
}

func (l *fileLogger) write(lv level, component, msg string) {
	if lv < l.min {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.openForTodayLocked(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon-log: %v\n", err)
		return
	}
	line := fmt.Sprintf("%s %s %s %s\n",
		l.clock().UTC().Format(time.RFC3339),
		lv.name(),
		component,
		msg,
	)
	_, _ = l.current.WriteString(line)
}

func (l *fileLogger) Debug(c, m string) { l.write(levelDebug, c, m) }
func (l *fileLogger) Info(c, m string)  { l.write(levelInfo, c, m) }
func (l *fileLogger) Warn(c, m string)  { l.write(levelWarn, c, m) }
func (l *fileLogger) Error(c, m string) { l.write(levelError, c, m) }

func (l *fileLogger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current != nil {
		_ = l.current.Close()
		l.current = nil
	}
	l.curDate = ""
	return l.pruneOldLocked(14)
}

func (l *fileLogger) pruneOldLocked(retentionDays int) error {
	dir := state.DaemonLogsDir(l.home)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := l.clock().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "daemon-") || !strings.HasSuffix(e.Name(), ".log") {
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
	return nil
}

func (l *fileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current != nil {
		err := l.current.Close()
		l.current = nil
		return err
	}
	return nil
}
