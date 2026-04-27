package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// FileEntry holds the result of scanning a single file.
type FileEntry struct {
	Path        string // project-relative path
	Description string // ≤100 chars
	EstTokens   int    // len(fileBytes) / 4
	Language    string // "go", "python", "js", "markdown", "config", "unknown"
}

// ExtractAll reads each path under projectRoot and dispatches to the appropriate extractor.
// Returns (nil, nil) on empty paths list.
func ExtractAll(projectRoot string, paths []string) ([]FileEntry, error) {
	entries := make([]FileEntry, 0, len(paths))
	for _, rel := range paths {
		abs := filepath.Join(projectRoot, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		e := dispatch(rel, data)
		e.Path = rel
		e.EstTokens = len(data) / 4
		entries = append(entries, e)
	}
	return entries, nil
}

// ScanProject combines Walk + ExtractAll.
func ScanProject(projectRoot string) ([]FileEntry, error) {
	paths, err := Walk(projectRoot)
	if err != nil {
		return nil, err
	}
	return ExtractAll(projectRoot, paths)
}

func dispatch(rel string, data []byte) FileEntry {
	base := strings.ToLower(filepath.Base(rel))
	ext := strings.ToLower(filepath.Ext(rel))

	if desc := extractKnown(base); desc != "" {
		return FileEntry{Description: desc, Language: "config"}
	}

	switch ext {
	case ".go":
		return FileEntry{Description: extractGo(data), Language: "go"}
	case ".py":
		return FileEntry{Description: extractPy(data), Language: "python"}
	case ".js", ".ts", ".jsx", ".tsx":
		return FileEntry{Description: extractJS(data), Language: "js"}
	case ".md", ".markdown":
		return FileEntry{Description: extractMD(data), Language: "markdown"}
	default:
		return FileEntry{Description: extractFallback(data), Language: "unknown"}
	}
}

func extractFallback(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") ||
			strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") {
			continue
		}
		return truncate(t, 100)
	}
	return "(no description)"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// Stubs replaced by Tasks 3-6.
func extractGo(data []byte) string  { return extractFallback(data) }
func extractPy(data []byte) string  { return extractFallback(data) }
func extractJS(data []byte) string  { return extractFallback(data) }
func extractMD(data []byte) string  { return extractFallback(data) }
