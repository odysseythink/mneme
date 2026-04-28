package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const TemplateVersion = 1 // bump when bundled templates change shape

func templateVersionPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".mneme", "template-version.txt")
}

// ReadTemplateVersion reads the pinned template version. Returns 0 if the
// file is missing (treat as oldest), and an error if the file is corrupt.
func ReadTemplateVersion(projectRoot string) (int, error) {
	data, err := os.ReadFile(templateVersionPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("template-version.txt corrupt: %v", err)
	}
	return n, nil
}

// WriteTemplateVersion writes the version number to .mneme/template-version.txt.
func WriteTemplateVersion(projectRoot string, version int) error {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return AtomicWrite(templateVersionPath(projectRoot), []byte(fmt.Sprintf("%d\n", version)))
}

// WriteTemplateVersionRaw exists for tests that need to seed corrupt content.
func WriteTemplateVersionRaw(mnemeDir, contents string) error {
	if err := os.MkdirAll(mnemeDir, 0755); err != nil {
		return err
	}
	return AtomicWrite(filepath.Join(mnemeDir, "template-version.txt"), []byte(contents))
}
