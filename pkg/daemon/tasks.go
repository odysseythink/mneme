package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/consolidator"
	"github.com/ranwei/mneme/pkg/events"
	"github.com/ranwei/mneme/pkg/scanner"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/suggestions"
	"github.com/ranwei/mneme/pkg/waste"
)

// TaskFunc is the signature of a registered cron task.
type TaskFunc func(ctx context.Context, log Logger) error

var registry = map[string]TaskFunc{
	"anatomy-rescan":      runAnatomyRescan,
	"consolidate-memory":  runConsolidateMemory,
	"gc-stale-projects":   runGCStaleProjects,
	"prune-backups":       runPruneBackups,
	"weekly-waste-report": runWeeklyWasteReport,
	"suggestions-refresh": runSuggestionsRefresh,
}

// taskBus is the optional event bus for tasks to publish to.
// nil means events are dropped silently (e.g., in tests that don't need them).
var taskBus *events.Bus

// SetTaskBus is called by daemon.Run to wire the shared bus.
func SetTaskBus(b *events.Bus) { taskBus = b }

// LookupTask returns the registered task for a name, or nil if unknown.
func LookupTask(name string) TaskFunc {
	return registry[name]
}

func runAnatomyRescan(ctx context.Context, log Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectsDir := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug("task", "anatomy-rescan: no projects dir; skipping")
			return nil
		}
		return err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		originPath := filepath.Join(projectsDir, e.Name(), "origin")
		data, err := os.ReadFile(originPath)
		if err != nil {
			log.Debug("task", fmt.Sprintf("anatomy-rescan: skip %s: no origin file", e.Name()))
			continue
		}
		root := string(data)
		for len(root) > 0 && (root[len(root)-1] == '\n' || root[len(root)-1] == ' ' || root[len(root)-1] == '\r' || root[len(root)-1] == '\t') {
			root = root[:len(root)-1]
		}
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			log.Debug("task", fmt.Sprintf("anatomy-rescan: skip %s: root missing (%s)", e.Name(), root))
			continue
		}

		lockPath := filepath.Join(root, ".mneme", ".lock")
		release, lockErr := state.AcquireLock(lockPath, 30*time.Second)
		if lockErr != nil {
			log.Warn("task", fmt.Sprintf("anatomy-rescan: skip %s: %v", root, lockErr))
			continue
		}

		if err := scanProjectIncremental(root); err != nil {
			log.Warn("task", fmt.Sprintf("anatomy-rescan: %s: %v", root, err))
		} else {
			log.Info("task", fmt.Sprintf("anatomy-rescan: ok %s", root))
			if taskBus != nil {
				payload, _ := json.Marshal(map[string]interface{}{
					"files_changed": -1, // unknown — scanner does not surface count today
				})
				taskBus.Publish(events.Event{
					TS:        time.Now().UnixMilli(),
					Type:      "scan.complete",
					ProjectID: e.Name(), // directory name is the project_id
					Data:      payload,
				})
			}
		}
		release()
	}
	return nil
}

func scanProjectIncremental(root string) error {
	paths, err := scanner.Walk(root)
	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}
	since, tsErr := state.ReadAnatomyGeneratedTime(root)
	existing, _ := state.ReadAnatomy(root)

	var entries []scanner.FileEntry
	if tsErr == nil && len(existing) > 0 {
		entries, err = scanner.ScanProjectIncremental(root, paths, since, existing)
	} else {
		entries, err = scanner.ExtractAll(root, paths)
	}
	if err != nil {
		return err
	}
	out := make([]state.AnatomyEntry, len(entries))
	for i, e := range entries {
		out[i] = state.AnatomyEntry{Path: e.Path, Description: e.Description, EstTokens: e.EstTokens, Language: e.Language}
	}
	return state.WriteAnatomy(root, out)
}

func runConsolidateMemory(ctx context.Context, log Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rows, err := state.ReadMemory(home)
	if err != nil {
		return err
	}
	if len(rows) <= 50 {
		log.Debug("task", fmt.Sprintf("consolidate-memory: %d rows; below threshold (50)", len(rows)))
		return nil
	}
	folded, err := consolidator.Consolidate(home)
	if err != nil {
		return err
	}
	log.Info("task", fmt.Sprintf("consolidate-memory: folded %d rows", folded))
	return nil
}

func runGCStaleProjects(ctx context.Context, log Logger) error {
	n, err := state.GarbageCollectProjects()
	if err != nil {
		return err
	}
	if n > 0 {
		log.Info("task", fmt.Sprintf("gc-stale-projects: removed %d stale directories", n))
	} else {
		log.Debug("task", "gc-stale-projects: no stale directories found")
	}
	return nil
}

func runPruneBackups(ctx context.Context, log Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return PruneBackupsAt(home, time.Now, 14, 10)
}

// PruneBackupsAt is the testable variant.
// Per-project: keep entries newer than retentionDays AND the minKeep most-recent overall.
func PruneBackupsAt(home string, clock func() time.Time, retentionDays, minKeep int) error {
	if clock == nil {
		clock = time.Now
	}
	root := filepath.Join(home, ".mneme", "backups")
	projects, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := clock().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		projectDir := filepath.Join(root, p.Name())
		snaps, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}
		type snap struct {
			name  string
			mtime time.Time
		}
		all := make([]snap, 0, len(snaps))
		for _, s := range snaps {
			if !s.IsDir() {
				continue
			}
			info, err := s.Info()
			if err != nil {
				continue
			}
			all = append(all, snap{name: s.Name(), mtime: info.ModTime()})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].mtime.After(all[j].mtime) })

		keep := 0
		for _, s := range all {
			if keep < minKeep || s.mtime.After(cutoff) {
				keep++
				continue
			}
			_ = os.RemoveAll(filepath.Join(projectDir, s.name))
		}
	}
	return nil
}

func runWeeklyWasteReport(ctx context.Context, log Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectsDir := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		originBytes, err := os.ReadFile(filepath.Join(projectsDir, e.Name(), "origin"))
		if err != nil {
			continue
		}
		root := strings.TrimSpace(string(originBytes))
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			continue
		}
		out, err := waste.GenerateReport(waste.ReportInput{
			ProjectRoot: root,
			HomeDir:     home,
			Now:         time.Now().UTC(),
			Threshold:   0.15,
		})
		if err != nil {
			log.Warn("task", fmt.Sprintf("weekly-waste-report: %s: %v", root, err))
			continue
		}
		if _, err := waste.WriteReport(root, out); err != nil {
			log.Warn("task", fmt.Sprintf("weekly-waste-report: %s: %v", root, err))
		}
	}
	return nil
}

func runSuggestionsRefresh(ctx context.Context, log Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectsDir := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		originBytes, err := os.ReadFile(filepath.Join(projectsDir, e.Name(), "origin"))
		root := strings.TrimSpace(string(originBytes))
		if err != nil || root == "" {
			root = filepath.Join(projectsDir, e.Name())
		}
		if _, err := os.Stat(root); err != nil {
			continue
		}
		got, err := suggestions.Refresh(root, time.Now().UTC())
		if err == nil && taskBus != nil && len(got) > 0 {
			payload, _ := json.Marshal(map[string]interface{}{"count": len(got)})
			taskBus.Publish(events.Event{
				TS:        time.Now().UnixMilli(),
				Type:      "suggestion.new",
				ProjectID: e.Name(),
				Data:      payload,
			})
		}
	}
	return nil
}
