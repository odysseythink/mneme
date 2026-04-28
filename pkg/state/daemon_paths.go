package state

import "path/filepath"

// DaemonDir returns ~/.mneme/daemon for the given home directory.
func DaemonDir(homeDir string) string {
	return filepath.Join(homeDir, ".mneme", "daemon")
}

func DaemonPidPath(homeDir string) string       { return filepath.Join(DaemonDir(homeDir), "pid") }
func DaemonSocketPath(homeDir string) string    { return filepath.Join(DaemonDir(homeDir), "socket") }
func DaemonTokenPath(homeDir string) string     { return filepath.Join(DaemonDir(homeDir), "token") }
func DaemonManifestPath(homeDir string) string  { return filepath.Join(DaemonDir(homeDir), "cron-manifest.json") }
func DaemonStatePath(homeDir string) string     { return filepath.Join(DaemonDir(homeDir), "cron-state.json") }
func DaemonHeartbeatPath(homeDir string) string { return filepath.Join(DaemonDir(homeDir), "heartbeat.json") }
func DaemonLogsDir(homeDir string) string       { return filepath.Join(DaemonDir(homeDir), "logs") }

// DaemonLogPath returns the path for a daily-rotated log file.
// dateYYYYMMDD must be formatted as "20260428".
func DaemonLogPath(homeDir, dateYYYYMMDD string) string {
	return filepath.Join(DaemonLogsDir(homeDir), "daemon-"+dateYYYYMMDD+".log")
}
