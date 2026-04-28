package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/consolidator"
	"github.com/ranwei/mneme/pkg/scanner"
	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

// TaskFunc is the signature of a registered cron task.
type TaskFunc func(ctx context.Context, log Logger) error

var registry = map[string]TaskFunc{
	"anatomy-rescan":      runAnatomyRescan,
	"consolidate-memory":  runConsolidateMemory,
	"prune-backups":       runPruneBackups,
	"weekly-waste-report": runWeeklyWasteReport,
}

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
