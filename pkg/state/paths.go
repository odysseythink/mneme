package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// FindGitRoot runs git rev-parse to find the repository root from startDir.
func FindGitRoot(startDir string) (string, bool) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = startDir
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// FindProjectRoot returns the project root by trying git root first, then walking up
// looking for a .mneme/ marker directory.
func FindProjectRoot(startDir string) (string, bool) {
	if root, ok := FindGitRoot(startDir); ok {
		return root, true
	}
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".mneme")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// GlobalProjectDir returns ~/.mneme/projects/<projectID>/.
func GlobalProjectDir(projectID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mneme", "projects", projectID)
}

// ReadOrCreateLocalID reads the per-project UUID from
// <projectRoot>/.mneme/.local-id, creating it on first call.
func ReadOrCreateLocalID(projectRoot string) (string, error) {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, ".local-id")
	data, err := os.ReadFile(path)
	if err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id, nil
		}
	}
	id := uuid.New().String()
	if err := AtomicWrite(path, []byte(id+"\n")); err != nil {
		return "", err
	}
	return id, nil
}
