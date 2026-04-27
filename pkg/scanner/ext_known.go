package scanner

import "strings"

var knownFiles = map[string]string{
	"go.mod":              "Go module definition",
	"go.sum":              "Go dependency checksums",
	"package.json":        "Node.js package descriptor",
	"package-lock.json":   "Node.js lockfile",
	"yarn.lock":           "Yarn lockfile",
	"cargo.toml":          "Rust crate manifest",
	"cargo.lock":          "Rust dependency lockfile",
	"makefile":            "Build rules",
	"dockerfile":          "Container image definition",
	"docker-compose.yml":  "Multi-container Docker config",
	".gitignore":          "Git ignore rules",
	"tsconfig.json":       "TypeScript compiler config",
	"pyproject.toml":      "Python project config",
	"requirements.txt":    "Python dependencies",
	"claude.md":           "Claude Code project instructions",
}

// extractKnown returns a fixed description for well-known filenames,
// or "" if the filename is not in the known table (case-insensitive match).
func extractKnown(baseLower string) string {
	return knownFiles[strings.ToLower(baseLower)]
}
