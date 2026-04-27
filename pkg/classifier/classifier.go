package classifier

import (
	"path/filepath"
	"strings"
)

// ClassifyEdit returns one of 13 category strings for a file edit.
// Rules are applied top-to-bottom; first match wins.
func ClassifyEdit(toolName, filePath, oldStr, newStr string) string {
	base := filepath.Base(filePath)

	if isDependencyFile(base) {
		return "dependency"
	}
	if isConfigFile(filePath) {
		return "config"
	}
	if isDocsFile(filePath) {
		return "docs"
	}
	if isTestFile(filePath) {
		return "test"
	}
	if toolName == "Write" {
		return "new_file"
	}
	if isTypesFile(base) {
		return "types"
	}
	if oldStr != "" && newStr != "" {
		if strings.TrimSpace(oldStr) == strings.TrimSpace(newStr) {
			return "style"
		}
		if isOnlyImportChanges(oldStr, newStr) {
			return "import"
		}
	}

	linesAdded := countLines(newStr)
	linesRemoved := countLines(oldStr)

	if linesRemoved == 0 && linesAdded == 0 {
		return "unknown"
	}
	if linesRemoved > 0 && linesAdded < linesRemoved/5 {
		return "delete_content"
	}

	netDelta := linesAdded - linesRemoved
	if netDelta < 0 {
		netDelta = -netDelta
	}

	if linesRemoved > 0 {
		ratio := float64(linesAdded) / float64(linesRemoved)
		if ratio >= 0.8 && ratio <= 1.2 {
			// For refactor, only match if it's a larger change
			if linesRemoved >= 10 {
				return "refactor"
			}
		}
	}

	if netDelta < 10 && linesAdded < 20 {
		return "bugfix"
	}

	if linesAdded > linesRemoved {
		return "feature"
	}
	return "unknown"
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(s), "\n"))
}

func isDependencyFile(base string) bool {
	switch base {
	case "go.mod", "go.sum", "package.json", "package-lock.json", "yarn.lock",
		"requirements.txt", "Pipfile", "Pipfile.lock", "Cargo.toml", "Cargo.lock":
		return true
	}
	return false
}

func isConfigFile(filePath string) bool {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".yaml", ".yml", ".toml", ".env", ".ini", ".cfg":
		return true
	}
	if strings.ToLower(filepath.Ext(filePath)) == ".json" {
		return true
	}
	return false
}

func isDocsFile(filePath string) bool {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".md", ".txt", ".rst", ".adoc":
		return true
	}
	return false
}

func isTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	if strings.HasSuffix(base, "_test.go") {
		return true
	}
	if strings.HasPrefix(base, "test_") {
		return true
	}
	if strings.Contains(filePath, "/tests/") || strings.Contains(filePath, "/test/") {
		return true
	}
	if strings.Contains(base, ".spec.") || strings.Contains(base, "_spec.") {
		return true
	}
	return false
}

func isTypesFile(base string) bool {
	noExt := strings.TrimSuffix(base, filepath.Ext(base))
	for _, kw := range []string{"types", "type", "interfaces", "interface", "models", "model", "schema"} {
		if noExt == kw {
			return true
		}
		if strings.HasSuffix(noExt, "_"+kw) || strings.HasPrefix(noExt, kw+"_") {
			return true
		}
	}
	return false
}

func isOnlyImportChanges(oldStr, newStr string) bool {
	return allLinesMatch(oldStr) && allLinesMatch(newStr)
}

func allLinesMatch(s string) bool {
	prefixes := []string{"import ", "from ", `require("`, `require "`, "use "}
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == ")" || line == "(" || line == `"` {
			continue
		}
		matched := false
		for _, p := range prefixes {
			if strings.HasPrefix(line, p) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
