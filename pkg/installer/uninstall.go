package installer

import "os"

// UninstallOpts configures the uninstall operation.
type UninstallOpts struct {
	SettingsPath string
	CLAUDEMDPath string
	RulesPath    string
}

// Uninstall removes all claude-context managed entries.
// Does not delete the project .claude-context/ directory (user data).
func Uninstall(opts UninstallOpts) error {
	if err := UninstallHooks(opts.SettingsPath); err != nil {
		return err
	}
	if err := RemoveCLAUDEMDBlock(opts.CLAUDEMDPath); err != nil {
		return err
	}
	// Best-effort delete of rules file; not an error if missing
	os.Remove(opts.RulesPath)
	return nil
}
