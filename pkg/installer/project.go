package installer

import (
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/state"
)

const gitignoreContent = "_session.json\n*.bak.*\n"

// ScaffoldProject creates <projectRoot>/.mneme/ with .gitignore and .local-id.
// Returns the project UUID. Idempotent.
func ScaffoldProject(projectRoot string) (string, error) {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	gi := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gi); os.IsNotExist(err) {
		if err := os.WriteFile(gi, []byte(gitignoreContent), 0644); err != nil {
			return "", err
		}
	}

	return state.ReadOrCreateLocalID(projectRoot)
}
