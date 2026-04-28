package installer

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/ranwei/mneme/pkg/state"
)

//go:embed templates/mneme.md.tmpl
var mnemeMDTemplate string

const userSectionBegin = "<!-- mneme:user-section BEGIN -->"
const userSectionEnd = "<!-- mneme:user-section END -->"

// WriteMnemeMD writes mneme.md, preserving any existing user section
// between the user-section fences.
func WriteMnemeMD(projectRoot string) error {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "mneme.md")

	contents := mnemeMDTemplate
	if existing, err := os.ReadFile(path); err == nil {
		if user := extractUserSection(string(existing)); user != "" {
			contents = replaceUserSection(contents, user)
		}
	}
	return state.AtomicWrite(path, []byte(contents))
}

// extractUserSection returns the text between (but not including) the fence
// markers. Returns "" if either fence is absent.
func extractUserSection(s string) string {
	start := strings.Index(s, userSectionBegin)
	if start < 0 {
		return ""
	}
	start += len(userSectionBegin)
	end := strings.Index(s[start:], userSectionEnd)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

// replaceUserSection swaps the user section in template with replacement.
// If template has no fences, returns template unchanged.
func replaceUserSection(template, replacement string) string {
	start := strings.Index(template, userSectionBegin)
	if start < 0 {
		return template
	}
	start += len(userSectionBegin)
	end := strings.Index(template[start:], userSectionEnd)
	if end < 0 {
		return template
	}
	return template[:start] + replacement + template[start+end:]
}
