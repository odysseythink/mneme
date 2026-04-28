package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

func TestAppendLedgerHistory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	root := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(filepath.Join(root, ".mneme"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadOrCreateLocalID(root); err != nil {
		t.Fatal(err)
	}

	snap := state.LedgerSnapshot{
		TS:        "2026-04-28T10:00:00Z",
		SessionID: "s1",
		Totals:    state.LedgerTotals{HookFired: map[string]int{"pre-read": 5}},
	}
	if err := state.AppendLedgerHistory(root, snap); err != nil {
		t.Fatalf("append: %v", err)
	}

	id, _ := state.ReadOrCreateLocalID(root)
	histPath := filepath.Join(state.GlobalProjectDir(id), "ledger-history.jsonl")
	data, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `"session_id":"s1"`) {
		t.Errorf("missing session_id: %s", data)
	}

	var got state.LedgerSnapshot
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Totals.HookFired["pre-read"] != 5 {
		t.Errorf("HookFired[pre-read]: got %d, want 5", got.Totals.HookFired["pre-read"])
	}
}

func TestReadLedgerHistorySince(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	old := state.LedgerSnapshot{TS: "2026-01-01T00:00:00Z", SessionID: "old"}
	newSnap := state.LedgerSnapshot{TS: "2026-04-25T00:00:00Z", SessionID: "new"}
	state.AppendLedgerHistory(root, old)
	state.AppendLedgerHistory(root, newSnap)

	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	snaps, err := state.ReadLedgerHistory(root, since)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(snaps) != 1 || snaps[0].SessionID != "new" {
		t.Errorf("got %d snaps, want 1 with session_id=new: %+v", len(snaps), snaps)
	}
}

func TestLedgerHistoryRollingTrim(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := filepath.Join(tmp, "proj")
	os.MkdirAll(filepath.Join(root, ".mneme"), 0755)
	state.ReadOrCreateLocalID(root)

	pad := strings.Repeat("x", 1024)
	for i := 0; i < 1500; i++ {
		snap := state.LedgerSnapshot{
			TS:        time.Now().UTC().Add(-time.Duration(1500-i) * time.Hour).Format(time.RFC3339),
			SessionID: "s",
			Totals:    state.LedgerTotals{HookFired: map[string]int{pad: i}},
		}
		state.AppendLedgerHistory(root, snap)
	}

	id, _ := state.ReadOrCreateLocalID(root)
	histPath := filepath.Join(state.GlobalProjectDir(id), "ledger-history.jsonl")
	info, _ := os.Stat(histPath)
	if info.Size() > 2*1024*1024 {
		t.Errorf("file size after trim: %d bytes (want < 2 MiB)", info.Size())
	}
}
