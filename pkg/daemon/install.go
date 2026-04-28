package daemon

import "errors"

// Install installs the daemon as a launchd plist (macOS) or systemd user unit (Linux).
// Replaced in Task 15.
func Install(home string) error {
	return errors.New("install not yet implemented (Task 15)")
}

// Uninstall removes the unit installed by Install. Replaced in Task 15.
func Uninstall(home string) error {
	return errors.New("uninstall not yet implemented (Task 15)")
}
