package tests

import (
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/classifier"
)

func TestClassifyEdit(t *testing.T) {
	cases := []struct {
		toolName string
		filePath string
		oldStr   string
		newStr   string
		want     string
	}{
		{"Edit", "go.mod", "module foo\n\ngo 1.21\n", "module foo\n\ngo 1.22\n", "dependency"},
		{"Write", "package.json", "", `{"name":"x"}`, "dependency"},
		{"Edit", "Cargo.toml", "[package]\n", "[package]\nversion=\"2\"\n", "dependency"},
		{"Edit", "config.yaml", "port: 8080\n", "port: 9090\n", "config"},
		{"Edit", ".env", "KEY=old\n", "KEY=new\n", "config"},
		{"Edit", "settings.toml", "x=1\n", "x=2\n", "config"},
		{"Edit", "README.md", "# old\n", "# new\n", "docs"},
		{"Write", "CHANGELOG.rst", "", "## v1\n", "docs"},
		{"Edit", "pkg/foo_test.go", "func TestA(t *testing.T) {}", "func TestA(t *testing.T) { t.Log(1) }", "test"},
		{"Edit", "tests/state_test.go", "old", "new", "test"},
		{"Write", "cmd/hook_stop.go", "", "package main\n", "new_file"},
		{"Write", "pkg/state/memory.go", "", "package state\n", "new_file"},
		{"Edit", "pkg/foo.go", "  x := 1\n", "x := 1\n", "style"},
		{"Edit", "pkg/auth.go", strings.Repeat("line\n", 5), strings.Repeat("line\n", 4) + "fix\n", "bugfix"},
		{"Edit", "pkg/server.go", strings.Repeat("a\n", 10), strings.Repeat("b\n", 10), "refactor"},
		{"Edit", "pkg/api.go", "func A() {}\n", strings.Repeat("line\n", 30), "feature"},
		{"Edit", "pkg/old.go", strings.Repeat("line\n", 20), "line\n", "delete_content"},
		{"Edit", "pkg/weird.go", "", "", "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.filePath+"_"+tc.want, func(t *testing.T) {
			got := classifier.ClassifyEdit(tc.toolName, tc.filePath, tc.oldStr, tc.newStr)
			if got != tc.want {
				t.Errorf("ClassifyEdit(%q, %q, ...) = %q, want %q", tc.toolName, tc.filePath, got, tc.want)
			}
		})
	}
}
