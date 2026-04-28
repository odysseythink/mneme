package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

// BackupOp describes a destructive operation about to occur.
type BackupOp struct {
	ProjectID string   `json:"project_id"`
	Operation string   `json:"operation"`
	Files     []string `json:"files"`
	MnemeVer  string   `json:"mneme_version"`
}

// Backup is the result of WriteBackup.
type Backup struct {
	Timestamp string
	Path      string
	Files     []string
}

func WriteBackup(op BackupOp) (*Backup, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	dir := filepath.Join(home, ".mneme", "backups", op.ProjectID, ts)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var copied []string
	for _, src := range op.Files {
		base := filepath.Base(src)
		dst := filepath.Join(dir, base)
		if err := copyFile(src, dst); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("copy %s: %v", src, err)
		}
		copied = append(copied, base)
	}

	manifest := struct {
		BackupOp
		CopiedFiles []string `json:"copied_files"`
		CreatedAt   string   `json:"created_at"`
	}{
		BackupOp:    op,
		CopiedFiles: copied,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := state.AtomicWrite(filepath.Join(dir, "manifest.json"), data); err != nil {
		return nil, err
	}

	return &Backup{Timestamp: ts, Path: dir, Files: copied}, nil
}

func ListBackups(projectID string) ([]Backup, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, ".mneme", "backups", projectID)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Backup
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, Backup{
			Timestamp: e.Name(),
			Path:      filepath.Join(root, e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out, nil
}

func RestoreBackup(timestamp, projectID string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".mneme", "backups", projectID, timestamp)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("backup not found: %s", dir)
	}
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Files       []string `json:"files"`
		CopiedFiles []string `json:"copied_files"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return err
	}
	srcByBase := make(map[string]string, len(manifest.Files))
	for _, src := range manifest.Files {
		srcByBase[filepath.Base(src)] = src
	}
	for _, base := range manifest.CopiedFiles {
		dst, ok := srcByBase[base]
		if !ok {
			continue
		}
		if err := copyFile(filepath.Join(dir, base), dst); err != nil {
			return fmt.Errorf("restore %s: %v", base, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
