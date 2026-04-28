package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranwei/claude-context/pkg/state"
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

func extractPy(data []byte) string {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "def ") && !strings.HasPrefix(t, "class ") {
			continue
		}
		// Check next non-empty line for a triple-quoted docstring.
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" {
				continue
			}
			for _, q := range []string{`"""`, `'''`} {
				if strings.HasPrefix(next, q) {
					doc := strings.TrimPrefix(next, q)
					doc = strings.TrimSpace(doc)
					if doc != "" {
						// Inline docstring: `"""text"""` or `"""text`
						doc = strings.TrimSuffix(doc, q)
						doc = strings.TrimSpace(doc)
						return truncate(doc, 100)
					}
					// Docstring opens on next line
					if j+1 < len(lines) {
						return truncate(strings.TrimSpace(lines[j+1]), 100)
					}
				}
			}
			break
		}
		return truncate(t, 100)
	}
	return extractFallback(data)
}

func extractJS(data []byte) string {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "export ") &&
			!strings.HasPrefix(t, "function ") &&
			!strings.HasPrefix(t, "class ") {
			continue
		}

		// Look for a JSDoc block ending immediately before this line.
		if i > 0 && strings.TrimSpace(lines[i-1]) == "*/" {
			// Walk back to find "/**"
			for k := i - 2; k >= 0; k-- {
				inner := strings.TrimSpace(lines[k])
				if strings.HasPrefix(inner, "/**") {
					// Search the block for @description or first * content line.
					for m := k + 1; m <= i-2; m++ {
						c := strings.TrimSpace(lines[m])
						if strings.Contains(c, "@description ") {
							idx := strings.Index(c, "@description ")
							return truncate(strings.TrimSpace(c[idx+len("@description "):]), 100)
						}
					}
					for m := k + 1; m <= i-2; m++ {
						c := strings.TrimSpace(lines[m])
						if strings.HasPrefix(c, "* ") && !strings.HasPrefix(c, "*/") {
							desc := strings.TrimPrefix(c, "* ")
							if desc != "" {
								return truncate(desc, 100)
							}
						}
					}
					break
				}
			}
		}

		return truncate(t, 100)
	}
	return extractFallback(data)
}

func extractMD(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			heading := strings.TrimLeft(t, "#")
			heading = strings.TrimSpace(heading)
			return truncate(heading, 100)
		}
		// First non-empty, non-heading line
		return truncate(t, 100)
	}
	return "(no description)"
}

// ScanProjectIncremental re-extracts only files whose mtime is strictly after
// `since`. Files not modified since `since` are returned from `existing`
// unchanged. New files (not in existing) are always extracted.
func ScanProjectIncremental(projectRoot string, paths []string, since time.Time, existing map[string]state.AnatomyEntry) ([]FileEntry, error) {
	var entries []FileEntry
	for _, rel := range paths {
		abs := filepath.Join(projectRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		cached, inCache := existing[rel]
		if inCache && !info.ModTime().After(since) {
			entries = append(entries, FileEntry{
				Path:        cached.Path,
				Description: cached.Description,
				EstTokens:   cached.EstTokens,
				Language:    cached.Language,
			})
			continue
		}
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
