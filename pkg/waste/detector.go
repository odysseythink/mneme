package waste

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const (
	largeEntryTokenThreshold  = 500
	memoryBloatThreshold      = 50
	cerebrumStalenessDays     = 14
	anatomyMissRateMinReads   = 10
	anatomyHitRateThreshold   = 0.8
)

// WastePattern describes a single detected (or not) inefficiency.
type WastePattern struct {
	Name     string `json:"name"`
	Detected bool   `json:"detected"`
	Severity string `json:"severity"` // "warn" or "info"
	Details  string `json:"details,omitempty"`
}

// Detect runs all 5 waste checks and returns one WastePattern per check.
// projectRoot is the repo root (containing .mneme/).
// homeDir is the user's home directory (containing .claude/).
func Detect(projectRoot, homeDir string) ([]WastePattern, error) {
	l, err := state.ReadLedger(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	anatomy, _ := state.ReadAnatomy(projectRoot)
	memRows, _ := state.ReadMemory(homeDir)

	return []WastePattern{
		checkRepeatedReads(l),
		checkLargeReadsWithAnatomy(l, anatomy),
		checkMemoryBloat(memRows),
		checkCerebrumStaleness(projectRoot),
		checkAnatomyMissRate(l),
	}, nil
}

func checkRepeatedReads(l *state.Ledger) WastePattern {
	detected := l.Totals.RepeatReads > 0
	details := ""
	if detected {
		details = fmt.Sprintf("%d repeated file read(s) recorded — same files read multiple times in a session; check pre-read hook anatomy coverage", l.Totals.RepeatReads)
	}
	return WastePattern{Name: "repeated_reads", Detected: detected, Severity: "warn", Details: details}
}

func checkLargeReadsWithAnatomy(l *state.Ledger, anatomy map[string]state.AnatomyEntry) WastePattern {
	largeCount := 0
	for _, e := range anatomy {
		if e.EstTokens > largeEntryTokenThreshold {
			largeCount++
		}
	}
	detected := l.Totals.AnatomyHits > 0 && largeCount > 0
	details := ""
	if detected {
		details = fmt.Sprintf("%d large file(s) (>%d tok) have anatomy descriptions; anatomy consulted %d time(s) — ensure Claude trusts anatomy to avoid full reads", largeCount, largeEntryTokenThreshold, l.Totals.AnatomyHits)
	}
	return WastePattern{Name: "large_reads_with_anatomy", Detected: detected, Severity: "info", Details: details}
}

func checkMemoryBloat(rows []state.MemoryRow) WastePattern {
	detected := len(rows) > memoryBloatThreshold
	details := ""
	if detected {
		details = fmt.Sprintf("memory has %d session rows (threshold: %d) — run: mneme memory consolidate (M6)", len(rows), memoryBloatThreshold)
	}
	return WastePattern{Name: "memory_bloat", Detected: detected, Severity: "warn", Details: details}
}

func checkCerebrumStaleness(projectRoot string) WastePattern {
	path := filepath.Join(projectRoot, ".mneme", "cerebrum.md")
	info, err := os.Stat(path)
	if err != nil {
		return WastePattern{Name: "cerebrum_staleness", Detected: false, Severity: "warn"}
	}
	days := int(time.Since(info.ModTime()).Hours() / 24)
	detected := days > cerebrumStalenessDays
	details := ""
	if detected {
		details = fmt.Sprintf("cerebrum.md last updated %d day(s) ago (threshold: %d) — review rules with: mneme cerebrum list", days, cerebrumStalenessDays)
	}
	return WastePattern{Name: "cerebrum_staleness", Detected: detected, Severity: "warn", Details: details}
}

func checkAnatomyMissRate(l *state.Ledger) WastePattern {
	total := l.Totals.HookFired["pre-read"]
	if total < anatomyMissRateMinReads {
		return WastePattern{Name: "anatomy_miss_rate", Detected: false, Severity: "warn"}
	}
	hitRate := float64(l.Totals.AnatomyHits) / float64(total)
	detected := hitRate < anatomyHitRateThreshold
	details := ""
	if detected {
		details = fmt.Sprintf("anatomy hit rate %.0f%% (miss rate %.0f%%, threshold: ≥80%%) — run: mneme scan to add missing files", hitRate*100, (1-hitRate)*100)
	}
	return WastePattern{Name: "anatomy_miss_rate", Detected: detected, Severity: "warn", Details: details}
}
