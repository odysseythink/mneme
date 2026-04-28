package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/designqc"
	"github.com/ranwei/mneme/pkg/state"
)

type fakeCapturer struct {
	failOn map[string]bool
}

func (f *fakeCapturer) Capture(opts designqc.CaptureOptions) ([]byte, int, int, error) {
	if f.failOn != nil {
		for path := range f.failOn {
			if path != "" && path != "/" && len(opts.URL) >= len(path) && opts.URL[len(opts.URL)-len(path):] == path {
				return nil, 0, 0, errors.New("simulated failure")
			}
		}
	}
	return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, 1440, 800, nil
}
func (f *fakeCapturer) Close() {}

func TestRunner_WipesPreviousCaptures(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	dir := filepath.Join(home, ".mneme", "designqc", "p1", "captures")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "stale.jpg"), []byte("old"), 0o600)

	report, err := designqc.RunWithCapturer(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		RouteOverride: []string{"/", "/about"},
		BaseURL:       "http://localhost:5173",
		Framework:     "vite",
	}, &fakeCapturer{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Captures) != 2 {
		t.Errorf("got %d captures, want 2", len(report.Captures))
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.jpg")); err == nil {
		t.Error("stale capture not wiped")
	}
}

func TestRunner_PartialFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	report, err := designqc.RunWithCapturer(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		RouteOverride: []string{"/", "/broken"},
		BaseURL:       "http://localhost:5173",
		Framework:     "vite",
	}, &fakeCapturer{failOn: map[string]bool{"/broken": true}})
	if err != nil {
		t.Fatalf("Run (partial): %v", err)
	}
	if len(report.Captures) != 2 {
		t.Fatalf("got %d captures, want 2", len(report.Captures))
	}
	got := map[string]string{}
	for _, c := range report.Captures {
		got[c.Route] = c.Error
	}
	if got["/"] != "" {
		t.Errorf("/ should succeed, got error %q", got["/"])
	}
	if got["/broken"] == "" {
		t.Errorf("/broken should have error")
	}
}

func TestRunner_AllFail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := seedProject(t, home, "p1")

	_, err := designqc.RunWithCapturer(context.Background(), designqc.RunOptions{
		ProjectRoot:   root,
		ProjectID:     "p1",
		HomeDir:       home,
		RouteOverride: []string{"/x"},
		BaseURL:       "http://localhost:5173",
		Framework:     "vite",
	}, &fakeCapturer{failOn: map[string]bool{"/x": true}})
	if err == nil {
		t.Error("expected error when all routes fail")
	}
	if _, statErr := state.ReadOrigin("p1"); statErr != nil {
		// project still seeded; sanity check.
	}
}
