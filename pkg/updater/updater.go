package updater

import (
	"fmt"

	"github.com/ranwei/mneme/pkg/installer"
	"github.com/ranwei/mneme/pkg/state"
)

type SyncOptions struct {
	DryRun    bool
	ProjectID string
	Verbose   bool
}

type SyncResult struct {
	Updated     int
	Skipped     int
	Failed      int
	WouldUpdate int
	Failures    []SyncFailure
}

type SyncFailure struct {
	ProjectID string
	Path      string
	Err       error
}

func SyncAll(opts SyncOptions) (*SyncResult, error) {
	origins, err := state.ListOrigins()
	if err != nil {
		return nil, fmt.Errorf("list origins: %w", err)
	}
	res := &SyncResult{}
	for id, root := range origins {
		if opts.ProjectID != "" && opts.ProjectID != id {
			continue
		}
		pinned, err := state.ReadTemplateVersion(root)
		if err != nil {
			res.Failed++
			res.Failures = append(res.Failures, SyncFailure{ProjectID: id, Path: root, Err: err})
			continue
		}
		if pinned >= state.TemplateVersion {
			res.Skipped++
			if opts.Verbose {
				fmt.Printf("  skip   %s (up-to-date, v%d)\n", root, pinned)
			}
			continue
		}
		if opts.DryRun {
			res.WouldUpdate++
			fmt.Printf("  would update %s (v%d → v%d)\n", root, pinned, state.TemplateVersion)
			continue
		}
		if err := syncOne(id, root); err != nil {
			res.Failed++
			res.Failures = append(res.Failures, SyncFailure{ProjectID: id, Path: root, Err: err})
			continue
		}
		res.Updated++
		fmt.Printf("  updated %s (v%d → v%d)\n", root, pinned, state.TemplateVersion)
	}
	return res, nil
}

func syncOne(projectID, projectRoot string) error {
	files := []string{
		fmt.Sprintf("%s/.mneme/mneme.md", projectRoot),
		fmt.Sprintf("%s/.mneme/identity.md", projectRoot),
	}
	if _, err := installer.WriteBackup(installer.BackupOp{
		ProjectID: projectID,
		Operation: "update",
		Files:     files,
		MnemeVer:  fmt.Sprintf("templates v%d", state.TemplateVersion),
	}); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	if err := installer.WriteMnemeMD(projectRoot); err != nil {
		return fmt.Errorf("write mneme.md: %w", err)
	}
	if err := installer.WriteIdentity(projectRoot); err != nil {
		return fmt.Errorf("write identity.md: %w", err)
	}
	if err := state.WriteTemplateVersion(projectRoot, state.TemplateVersion); err != nil {
		return fmt.Errorf("pin template version: %w", err)
	}
	return nil
}
