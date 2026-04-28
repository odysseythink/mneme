package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/daemon"
)

func TestRenderLaunchdPlist(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("plist render only on darwin")
	}
	home := "/Users/test"
	bin := "/usr/local/bin/mneme"
	got := daemon.RenderLaunchdPlist(home, bin)
	for _, want := range []string{
		"<string>com.mneme.daemon</string>",
		"<string>" + bin + "</string>",
		"<string>daemon</string>",
		"<string>start</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}
}

func TestRenderSystemdUnit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd render only on linux")
	}
	bin := "/usr/local/bin/mneme"
	got := daemon.RenderSystemdUnit(bin)
	for _, want := range []string{
		"[Service]",
		"Type=simple",
		"ExecStart=" + bin + " daemon start",
		"Restart=on-failure",
		"[Install]",
		"WantedBy=default.target",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("unit missing %q\n%s", want, got)
		}
	}
}

func TestRenderLaunchdPlist_Always(t *testing.T) {
	got := daemon.RenderLaunchdPlist("/h", "/b/mneme")
	if !strings.Contains(got, "/b/mneme") {
		t.Errorf("plist missing binary path: %s", got)
	}
}

func TestInstall_WritesUnit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, "fake-mneme")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MNEME_BIN_PATH", bin)

	if err := daemon.InstallAt(home, bin, daemonInstallSpyDoNothing); err != nil {
		t.Fatalf("InstallAt: %v", err)
	}

	var unitPath string
	switch runtime.GOOS {
	case "darwin":
		unitPath = filepath.Join(home, "Library", "LaunchAgents", "com.mneme.daemon.plist")
	case "linux":
		unitPath = filepath.Join(home, ".config", "systemd", "user", "mneme-daemon.service")
	default:
		t.Skip("install test skipped on " + runtime.GOOS)
	}
	if _, err := os.Stat(unitPath); err != nil {
		t.Errorf("unit not written: %v", err)
	}
}

func daemonInstallSpyDoNothing(args ...string) error { return nil }
