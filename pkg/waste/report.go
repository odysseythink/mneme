package waste

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

type ReportInput struct {
	ProjectRoot string
	HomeDir     string
	Now         time.Time
	Threshold   float64
}

type ReportOutput struct {
	Date               string
	Patterns           []WastePattern
	Deltas             map[string]int
	BreachedThresholds []string
}

func GenerateReport(in ReportInput) (ReportOutput, error) {
	patterns, err := Detect(in.ProjectRoot, in.HomeDir)
	if err != nil {
		return ReportOutput{}, err
	}

	deltas, err := computeWeeklyDeltas(in.ProjectRoot, in.Now)
	if err != nil {
		return ReportOutput{}, err
	}

	var breached []string
	for _, p := range patterns {
		if p.Detected {
			breached = append(breached, p.Name)
		}
	}

	return ReportOutput{
		Date:               in.Now.UTC().Format("2006-01-02"),
		Patterns:           patterns,
		Deltas:             deltas,
		BreachedThresholds: breached,
	}, nil
}

func computeWeeklyDeltas(root string, now time.Time) (map[string]int, error) {
	since := now.Add(-14 * 24 * time.Hour)
	snaps, err := state.ReadLedgerHistory(root, since)
	if err != nil {
		return nil, err
	}
	weekAgo := now.Add(-7 * 24 * time.Hour)

	var thisWeek, prevWeek state.LedgerTotals
	thisWeek.HookFired = map[string]int{}
	prevWeek.HookFired = map[string]int{}

	for _, s := range snaps {
		ts, err := time.Parse(time.RFC3339, s.TS)
		if err != nil {
			continue
		}
		dst := &prevWeek
		if ts.After(weekAgo) {
			dst = &thisWeek
		}
		for k, v := range s.Totals.HookFired {
			if v > dst.HookFired[k] {
				dst.HookFired[k] = v
			}
		}
		if s.Totals.AnatomyHits > dst.AnatomyHits {
			dst.AnatomyHits = s.Totals.AnatomyHits
		}
		if s.Totals.RepeatReads > dst.RepeatReads {
			dst.RepeatReads = s.Totals.RepeatReads
		}
	}

	deltas := map[string]int{}
	for k, v := range thisWeek.HookFired {
		deltas[k] = v - prevWeek.HookFired[k]
	}
	deltas["anatomy_hits"] = thisWeek.AnatomyHits - prevWeek.AnatomyHits
	deltas["repeat_reads"] = thisWeek.RepeatReads - prevWeek.RepeatReads
	return deltas, nil
}

func RenderMarkdown(out ReportOutput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Mneme Waste Report — %s\n\n", out.Date)

	sb.WriteString("## Detected patterns\n\n")
	if len(out.Patterns) == 0 {
		sb.WriteString("_no patterns evaluated_\n\n")
	} else {
		for _, p := range out.Patterns {
			marker := "·"
			if p.Detected {
				marker = "⚠"
			}
			fmt.Fprintf(&sb, "- %s **%s** [%s]", marker, p.Name, p.Severity)
			if p.Details != "" {
				fmt.Fprintf(&sb, " — %s", p.Details)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Week-over-week deltas\n\n")
	if len(out.Deltas) == 0 {
		sb.WriteString("_no history available_\n\n")
	} else {
		keys := make([]string, 0, len(out.Deltas))
		for k := range out.Deltas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "- %s: %+d\n", k, out.Deltas[k])
		}
		sb.WriteString("\n")
	}

	if len(out.BreachedThresholds) > 0 {
		sb.WriteString("## Breached thresholds\n\n")
		for _, n := range out.BreachedThresholds {
			fmt.Fprintf(&sb, "- %s\n", n)
		}
	}
	return sb.String()
}

func WriteReport(projectRoot string, out ReportOutput) (string, error) {
	dir := filepath.Join(projectRoot, ".mneme", "reports")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "waste-"+out.Date+".md")
	return path, state.AtomicWrite(path, []byte(RenderMarkdown(out)))
}
