package tests

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
	"github.com/ranwei/mneme/pkg/waste"
)

func TestGenerateAndRenderReport(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	now := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	state.AppendLedgerHistory(root, state.LedgerSnapshot{
		TS:        now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		SessionID: "old",
		Totals:    state.LedgerTotals{HookFired: map[string]int{"pre-read": 5}},
	})
	state.AppendLedgerHistory(root, state.LedgerSnapshot{
		TS:        now.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
		SessionID: "new",
		Totals:    state.LedgerTotals{HookFired: map[string]int{"pre-read": 12}},
	})

	out, err := waste.GenerateReport(waste.ReportInput{
		ProjectRoot: root,
		HomeDir:     tmp,
		Now:         now,
		Threshold:   0.15,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out.Date != "2026-04-28" {
		t.Errorf("Date: got %q, want 2026-04-28", out.Date)
	}
	md := waste.RenderMarkdown(out)
	if !strings.Contains(md, "# Mneme Waste Report — 2026-04-28") {
		t.Errorf("missing header: %s", md)
	}
	if !strings.Contains(md, "pre-read") {
		t.Errorf("expected pre-read in deltas section: %s", md)
	}
}

func TestWriteReport(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)

	out := waste.ReportOutput{Date: "2026-04-28"}
	path, err := waste.WriteReport(root, out)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	want := filepath.Join(root, ".mneme", "reports", "waste-2026-04-28.md")
	if path != want {
		t.Errorf("path: got %q, want %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file missing: %v", err)
	}
}

func TestReportWasteDryRun(t *testing.T) {
	bin := buildMnemeBinary(t) // build before HOME swap so go can find modules
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	cmd := exec.Command(bin, "report", "waste", "--dry-run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v\nstdout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Mneme Waste Report") {
		t.Errorf("expected report header, got: %s", out.String())
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".mneme", "reports", "waste-*.md"))
	if len(matches) > 0 {
		t.Errorf("--dry-run wrote a file: %v", matches)
	}
}

func TestReportWasteWritesFile(t *testing.T) {
	bin := buildMnemeBinary(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	cmd := exec.Command(bin, "report", "waste")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	cmd.Stdout = io.Discard
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".mneme", "reports", "waste-*.md"))
	if len(matches) == 0 {
		t.Errorf("expected at least one waste-*.md file")
	}
}
