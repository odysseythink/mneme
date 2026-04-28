package tests

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/designqc"
)

func TestReport_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &designqc.Report{
		Version:    1,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Framework:  "vite",
		BaseURL:    "http://localhost:5173",
		Captures: []designqc.Capture{
			{Route: "/", File: "route-root.jpg", Width: 1440, Height: 2400, CapturedAtMS: 1714290000000},
		},
	}
	if err := designqc.WriteReport(dir, want); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	got, err := designqc.ReadReport(dir)
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}
	if got.Version != 1 || got.Framework != "vite" || len(got.Captures) != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.Captures[0].Route != "/" || got.Captures[0].File != "route-root.jpg" {
		t.Errorf("capture mismatch: %+v", got.Captures[0])
	}
}

func TestReport_MissingFile(t *testing.T) {
	_, err := designqc.ReadReport(filepath.Join(t.TempDir(), "no-such"))
	if err == nil {
		t.Error("ReadReport on missing dir should error")
	}
}

func TestReport_UnknownVersion(t *testing.T) {
	dir := t.TempDir()
	r := &designqc.Report{Version: 99, Framework: "future"}
	if err := designqc.WriteReport(dir, r); err != nil {
		t.Fatal(err)
	}
	_, err := designqc.ReadReport(dir)
	if err == nil {
		t.Error("ReadReport with unknown version should error")
	}
}
