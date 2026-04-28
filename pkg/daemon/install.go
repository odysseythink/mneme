package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

// Loader runs an external command (launchctl, systemctl). Tests inject a spy.
type Loader func(args ...string) error

// Install detects the OS and writes the appropriate unit + loads it.
func Install(home string) error {
	bin, err := mnemeBinPath()
	if err != nil {
		return err
	}
	return InstallAt(home, bin, defaultLoader)
}

// Uninstall reverses Install.
func Uninstall(home string) error {
	return UninstallAt(home, defaultLoader)
}

func InstallAt(home, bin string, loader Loader) error {
	switch runtime.GOOS {
	case "darwin":
		return installDarwin(home, bin, loader)
	case "linux":
		return installLinux(home, bin, loader)
	default:
		return fmt.Errorf("install not supported on %s; start manually with `mneme daemon start`", runtime.GOOS)
	}
}

func UninstallAt(home string, loader Loader) error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallDarwin(home, loader)
	case "linux":
		return uninstallLinux(home, loader)
	default:
		return fmt.Errorf("uninstall not supported on %s", runtime.GOOS)
	}
}

func installDarwin(home, bin string, loader Loader) error {
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.mneme.daemon.plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(plistPath, []byte(RenderLaunchdPlist(home, bin)), 0o644); err != nil {
		return err
	}
	uid := strconv.Itoa(os.Getuid())
	if err := loader("launchctl", "bootstrap", "gui/"+uid, plistPath); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w", err)
	}
	return loader("launchctl", "enable", "gui/"+uid+"/com.mneme.daemon")
}

func uninstallDarwin(home string, loader Loader) error {
	uid := strconv.Itoa(os.Getuid())
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.mneme.daemon.plist")
	_ = loader("launchctl", "bootout", "gui/"+uid+"/com.mneme.daemon")
	return os.Remove(plistPath)
}

func installLinux(home, bin string, loader Loader) error {
	unitPath := filepath.Join(home, ".config", "systemd", "user", "mneme-daemon.service")
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, []byte(RenderSystemdUnit(bin)), 0o644); err != nil {
		return err
	}
	if err := loader("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl reload: %w", err)
	}
	return loader("systemctl", "--user", "enable", "--now", "mneme-daemon.service")
}

func uninstallLinux(home string, loader Loader) error {
	unitPath := filepath.Join(home, ".config", "systemd", "user", "mneme-daemon.service")
	_ = loader("systemctl", "--user", "disable", "--now", "mneme-daemon.service")
	if err := os.Remove(unitPath); err != nil {
		return err
	}
	return loader("systemctl", "--user", "daemon-reload")
}

// RenderLaunchdPlist produces the macOS launchd unit content.
func RenderLaunchdPlist(home, bin string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mneme.daemon</string>
    <key>ProgramArguments</key>
    <array>
      <string>` + bin + `</string>
      <string>daemon</string>
      <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>` + filepath.Join(home, ".mneme", "daemon", "logs", "launchd.log") + `</string>
    <key>StandardErrorPath</key>
    <string>` + filepath.Join(home, ".mneme", "daemon", "logs", "launchd.log") + `</string>
</dict>
</plist>
`
}

// RenderSystemdUnit produces the Linux systemd user unit content.
func RenderSystemdUnit(bin string) string {
	return `[Unit]
Description=mneme daemon (cron + HTTP control plane)
After=default.target

[Service]
Type=simple
ExecStart=` + bin + ` daemon start
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`
}

func defaultLoader(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("loader: empty args")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func mnemeBinPath() (string, error) {
	if v := os.Getenv("MNEME_BIN_PATH"); v != "" {
		return v, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}
