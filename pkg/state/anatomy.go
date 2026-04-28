package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AnatomyEntry is the state package's representation of a scanned file.
type AnatomyEntry struct {
	Path        string
	Description string
	EstTokens   int
	Language    string
}

// WriteAnatomy groups entries by directory, sorts lexicographically,
// renders anatomy.md, and writes it atomically to
// <projectRoot>/.claude-context/anatomy.md.
func WriteAnatomy(projectRoot string, entries []AnatomyEntry) error {
	id, err := ReadOrCreateLocalID(projectRoot)
	if err != nil {
		return err
	}

	// Group entries by directory.
	dirFiles := make(map[string][]AnatomyEntry)
	for _, e := range entries {
		dir := filepath.Dir(e.Path)
		dirFiles[dir] = append(dirFiles[dir], e)
	}

	// Sort directory names.
	dirs := make([]string, 0, len(dirFiles))
	for d := range dirFiles {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var sb strings.Builder
	sb.WriteString("<!-- claude-context anatomy v1 -->\n")
	fmt.Fprintf(&sb, "<!-- generated: %s | files: %d | project: %s -->\n",
		time.Now().UTC().Format(time.RFC3339), len(entries), id)

	for _, dir := range dirs {
		sb.WriteString("\n")
		if dir == "." {
			sb.WriteString("## ./\n\n")
		} else {
			fmt.Fprintf(&sb, "## %s/\n\n", dir)
		}

		files := dirFiles[dir]
		sort.Slice(files, func(i, j int) bool {
			return filepath.Base(files[i].Path) < filepath.Base(files[j].Path)
		})
		for _, e := range files {
			fmt.Fprintf(&sb, "- `%s` — %s (%s, ~%d tok)\n",
				filepath.Base(e.Path), e.Description, e.Language, e.EstTokens)
		}
	}

	path := filepath.Join(projectRoot, ".claude-context", "anatomy.md")
	if err := AtomicWrite(path, []byte(sb.String())); err != nil {
		return err
	}
	IncrementSafe(projectRoot, "scan_count")
	return nil
}

// ReadAnatomy parses anatomy.md into a map keyed by project-relative path.
// Returns an empty map (not an error) if anatomy.md does not exist.
func ReadAnatomy(projectRoot string) (map[string]AnatomyEntry, error) {
	path := filepath.Join(projectRoot, ".claude-context", "anatomy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]AnatomyEntry), nil
		}
		return nil, err
	}

	result := make(map[string]AnatomyEntry)
	currentDir := "."

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			dir := strings.TrimPrefix(line, "## ")
			dir = strings.TrimSuffix(dir, "/")
			if dir == "." || dir == "./" {
				currentDir = "."
			} else {
				currentDir = dir
			}
			continue
		}
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		rest := strings.TrimPrefix(line, "- `")
		backtick := strings.Index(rest, "`")
		if backtick < 0 {
			continue
		}
		filename := rest[:backtick]
		after := strings.TrimPrefix(rest[backtick+1:], " — ")

		// Parse "description (lang, ~N tok)"
		parenStart := strings.LastIndex(after, " (")
		var desc, lang string
		var estTok int
		if parenStart >= 0 {
			desc = after[:parenStart]
			meta := strings.TrimSuffix(after[parenStart+2:], ")")
			parts := strings.SplitN(meta, ", ~", 2)
			if len(parts) == 2 {
				lang = parts[0]
				n, _ := strconv.Atoi(strings.TrimSuffix(parts[1], " tok"))
				estTok = n
			}
		} else {
			desc = after
		}

		var relPath string
		if currentDir == "." {
			relPath = filename
		} else {
			relPath = currentDir + "/" + filename
		}
		result[relPath] = AnatomyEntry{
			Path:        relPath,
			Description: desc,
			EstTokens:   estTok,
			Language:    lang,
		}
	}
	return result, nil
}

// ReadAnatomyGeneratedTime parses the <!-- generated: RFC3339 --> timestamp
// from anatomy.md. Returns an error if the file is missing or the timestamp
// cannot be parsed.
func ReadAnatomyGeneratedTime(projectRoot string) (time.Time, error) {
	path := filepath.Join(projectRoot, ".claude-context", "anatomy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	for _, line := range strings.SplitN(string(data), "\n", 5) {
		if !strings.HasPrefix(line, "<!-- generated: ") {
			continue
		}
		rest := strings.TrimPrefix(line, "<!-- generated: ")
		if i := strings.Index(rest, " "); i > 0 {
			rest = rest[:i]
		}
		return time.Parse(time.RFC3339, rest)
	}
	return time.Time{}, fmt.Errorf("anatomy.md has no generated timestamp")
}
