package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteOrigin records the absolute path of a project's working tree to
// ~/.mneme/projects/<projectID>/origin so that multi-project commands
// (update, restore) can locate every initialized project.
func WriteOrigin(projectID, projectRoot string) error {
	dir := GlobalProjectDir(projectID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "origin")
	return AtomicWrite(path, []byte(projectRoot+"\n"))
}

// ReadOrigin returns the recorded project root for the given ID.
// Returns an error if the origin file is missing or empty.
func ReadOrigin(projectID string) (string, error) {
	path := filepath.Join(GlobalProjectDir(projectID), "origin")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(data))
	if root == "" {
		return "", fmt.Errorf("origin file is empty: %s", path)
	}
	return root, nil
}

// ListOrigins enumerates every initialized project.
// Returns map[projectID]projectRoot. Missing or unreadable origin files
// are skipped silently.
func ListOrigins() (map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	projectsDir := filepath.Join(home, ".mneme", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	result := make(map[string]string, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root, err := ReadOrigin(e.Name())
		if err != nil {
			continue
		}
		result[e.Name()] = root
	}
	return result, nil
}
