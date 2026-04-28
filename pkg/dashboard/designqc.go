package dashboard

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ranwei/mneme/pkg/designqc"
)

// DesignQCHandler reads ~/.mneme/designqc/<project-id>/report.json. When the
// file is missing or the schema version is unknown, returns a structured
// "not available" envelope rather than a 5xx error.
func DesignQCHandler(homeDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		projectID, root := resolveProject(w, r)
		if root == "" {
			return
		}
		runDir := filepath.Join(homeDir, ".mneme", "designqc", projectID)
		report, err := designqc.ReadReport(runDir)
		if err != nil {
			reason := "no captures yet — run mneme designqc"
			if report == nil && err.Error() != "" && len(err.Error()) > 0 {
				// Distinguish "missing file" from "unsupported version".
				if filepath.IsAbs(err.Error()) || filepath.Ext(err.Error()) != "" {
					// Path-like error; keep default reason.
				} else {
					reason = err.Error()
				}
			}
			writeJSON(w, 200, map[string]any{
				"available": false,
				"reason":    reason,
				"captures":  []any{},
			})
			return
		}
		writeJSON(w, 200, map[string]any{
			"available": true,
			"report":    report,
		})
	}
}

// captureFileRegex enforces the safe filename shape; rejects traversal.
var captureFileRegex = regexp.MustCompile(`^route-[a-z0-9-]+\.jpg$`)

// CapturesBytesHandler serves a single capture file's bytes.
// Path: /api/designqc/captures/<project-id>/<file>
func CapturesBytesHandler(homeDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		const prefix = "/api/designqc/captures/"
		path := r.URL.Path

		// Explicitly reject paths with .. or . components.
		// The HTTP mux will redirect these, but we also check here for defense in depth.
		if hasPathTraversal(path) {
			http.NotFound(w, r)
			return
		}

		if len(path) <= len(prefix) {
			http.NotFound(w, r)
			return
		}
		rest := path[len(prefix):]
		slashIdx := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				slashIdx = i
				break
			}
		}
		if slashIdx < 0 || slashIdx == len(rest)-1 {
			http.NotFound(w, r)
			return
		}
		projectID := rest[:slashIdx]
		fileNameWithPossiblePath := rest[slashIdx+1:]

		// Reject any filename that contains a path separator or is not the expected format.
		// This catches both "../etc/passwd" and URL-encoded variants like "..%2Fetc%2Fpasswd".
		fileName := filepath.Base(fileNameWithPossiblePath)
		if fileName != fileNameWithPossiblePath {
			// filepath.Base stripped a directory component; reject.
			http.NotFound(w, r)
			return
		}

		if !captureFileRegex.MatchString(fileName) {
			http.NotFound(w, r)
			return
		}
		full := filepath.Join(homeDir, ".mneme", "designqc", projectID, "captures", fileName)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, full)
	}
}

func hasPathTraversal(path string) bool {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == ".." || part == "." {
			return true
		}
	}
	return false
}
