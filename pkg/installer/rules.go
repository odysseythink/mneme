package installer

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/ranwei/claude-context/pkg/state"
)

//go:embed templates/rules.md
var rulesTemplate string

const beginMarker = "<!-- claude-context-managed BEGIN -->"
const endMarker = "<!-- claude-context-managed END -->"

// WriteRules writes the embedded rules template to rulesPath (overwrites).
func WriteRules(rulesPath string) error {
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0755); err != nil {
		return err
	}
	return state.AtomicWrite(rulesPath, []byte(rulesTemplate))
}

// InjectCLAUDEMD appends the 3-line boundary block to claudeMDPath.
// Idempotent: skips if the BEGIN marker is already present.
func InjectCLAUDEMD(claudeMDPath, rulesPath string) error {
	data, _ := os.ReadFile(claudeMDPath) // file may not exist yet; treat as empty
	if strings.Contains(string(data), beginMarker) {
		return nil // already present
	}

	block := "\n" + beginMarker + "\n@" + rulesPath + "\n" + endMarker + "\n"
	newData := append(data, []byte(block)...)

	if err := os.MkdirAll(filepath.Dir(claudeMDPath), 0755); err != nil {
		return err
	}
	return state.AtomicWrite(claudeMDPath, newData)
}

// RemoveCLAUDEMDBlock removes the BEGIN/END block from claudeMDPath.
func RemoveCLAUDEMDBlock(claudeMDPath string) error {
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return nil // file doesn't exist, nothing to do
	}
	content := string(data)
	start := strings.Index(content, beginMarker)
	end := strings.Index(content, endMarker)
	if start < 0 || end < 0 {
		return nil // markers not found
	}
	end += len(endMarker)
	// Also remove the leading newline before BEGIN if present
	prefix := content[:start]
	suffix := content[end:]
	prefix = strings.TrimRight(prefix, "\n") + "\n"
	result := prefix + strings.TrimLeft(suffix, "\n")
	if strings.TrimSpace(result) == "" {
		result = ""
	}
	return state.AtomicWrite(claudeMDPath, []byte(result))
}
