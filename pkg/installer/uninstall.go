package installer

import (
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/state"
)

// UninstallOpts configures the uninstall operation.
type UninstallOpts struct {
	SettingsPath string
	CLAUDEMDPath string
	RulesPath    string
	ProjectRoot  string // optional; when set, the per-project origin pointer is also removed
}

// Uninstall removes all mneme managed entries.
// Does not delete the project .mneme/ directory (user data).
//
// When ProjectRoot is set, the multi-project registry pointer at
// ~/.mneme/projects/<id>/origin is also removed (best-effort) so that
// `mneme update` and similar multi-project commands stop iterating
// over this project.
func Uninstall(opts UninstallOpts) error {
	if err := UninstallHooks(opts.SettingsPath); err != nil {
		return err
	}
	if err := RemoveCLAUDEMDBlock(opts.CLAUDEMDPath); err != nil {
		return err
	}
	// Best-effort delete of rules file; not an error if missing
	os.Remove(opts.RulesPath)

	if opts.ProjectRoot != "" {
		if id, err := state.ReadOrCreateLocalID(opts.ProjectRoot); err == nil {
			os.Remove(filepath.Join(state.GlobalProjectDir(id), "origin"))
		}
	}
	return nil
}
