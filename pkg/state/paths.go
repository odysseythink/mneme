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

// GarbageCollectProjects removes project directories under ~/.mneme/projects/
// whose origin file is missing, empty, or points to a non-existent directory.
// Returns the number of directories removed.
func GarbageCollectProjects() (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}
	root := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		originPath := filepath.Join(root, e.Name(), "origin")
		data, err := os.ReadFile(originPath)
		if err != nil {
			// no origin file → stale
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
			removed++
			continue
		}
		origin := strings.TrimSpace(string(data))
		if origin == "" {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
			removed++
			continue
		}
		if _, err := os.Stat(origin); err != nil {
			// origin path doesn't exist → stale
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
			removed++
			continue
		}
	}
	return removed, nil
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
