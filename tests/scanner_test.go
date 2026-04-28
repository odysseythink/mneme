package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/scanner"
	"github.com/ranwei/mneme/pkg/state"
)

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@t.com")
	run("git", "config", "user.name", "T")
	return dir
}

func TestWalkFilesGitRepo(t *testing.T) {
	wd, _ := os.Getwd()
	root, ok := state.FindGitRoot(wd)
	if !ok {
		t.Skip("not running inside a git repo")
	}
	paths, err := scanner.Walk(root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(paths) == 0 {
		t.Error("expected at least one file from Walk")
	}
	for _, p := range paths {
		if len(p) == 0 {
			t.Error("Walk returned empty path")
		}
		if p[0] == '/' {
			t.Errorf("Walk returned absolute path: %s", p)
		}
	}
}

func TestExtractKnown(t *testing.T) {
	dir := makeGitRepo(t)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.21\n"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	entries, err := scanner.ExtractAll(dir, []string{"go.mod"})
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Language != "config" {
		t.Errorf("Language = %q, want config", entries[0].Language)
	}
	if entries[0].Description != "Go module definition" {
		t.Errorf("Description = %q, want %q", entries[0].Description, "Go module definition")
	}
}

func TestExtractFallback(t *testing.T) {
	dir := makeGitRepo(t)
	content := "# comment\nclass Foo\n  def bar\n  end\nend\n"
	os.WriteFile(filepath.Join(dir, "foo.rb"), []byte(content), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	entries, err := scanner.ExtractAll(dir, []string{"foo.rb"})
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Language != "unknown" {
		t.Errorf("Language = %q, want unknown", entries[0].Language)
	}
	if entries[0].Description != "class Foo" {
		t.Errorf("Description = %q, want class Foo", entries[0].Description)
	}
}

func TestTokenEstimate(t *testing.T) {
	dir := makeGitRepo(t)
	os.WriteFile(filepath.Join(dir, "tok.rb"), []byte("hello world"), 0644) // 11 bytes → 2 tokens
	exec.Command("git", "-C", dir, "add", ".").Run()

	entries, err := scanner.ExtractAll(dir, []string{"tok.rb"})
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	if entries[0].EstTokens != 2 {
		t.Errorf("EstTokens = %d, want 2", entries[0].EstTokens)
	}
}

func TestExtractGo(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "comment_before_func",
			content: "package main\n\n// main is the entry point.\nfunc main() {}\n",
			want:    "main is the entry point.",
		},
		{
			name:    "multi_comment_uses_first_line",
			content: "package main\n\n// Package foo provides utilities.\n// See README for details.\nfunc Foo() {}\n",
			want:    "Package foo provides utilities.",
		},
		{
			name:    "no_comment_uses_decl",
			content: "package main\n\nfunc doWork() {}\n",
			want:    "func doWork() {}",
		},
		{
			name:    "type_declaration",
			content: "package main\n\n// Config holds settings.\ntype Config struct{}\n",
			want:    "Config holds settings.",
		},
		{
			name:    "truncates_at_100",
			content: "package main\n\n// " + strings.Repeat("a", 110) + "\nfunc f() {}\n",
			want:    strings.Repeat("a", 100),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := makeGitRepo(t)
			os.WriteFile(filepath.Join(dir, "x.go"), []byte(tc.content), 0644)
			exec.Command("git", "-C", dir, "add", ".").Run()

			entries, _ := scanner.ExtractAll(dir, []string{"x.go"})
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].Description != tc.want {
				t.Errorf("Description = %q, want %q", entries[0].Description, tc.want)
			}
			if entries[0].Language != "go" {
				t.Errorf("Language = %q, want go", entries[0].Language)
			}
		})
	}
}

func TestExtractPy(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "def_with_docstring",
			content: "def greet(name):\n    \"\"\"Greet the user by name.\"\"\"\n    pass\n",
			want:    "Greet the user by name.",
		},
		{
			name:    "class_with_docstring",
			content: "class Parser:\n    \"\"\"Parses config files.\"\"\"\n    pass\n",
			want:    "Parses config files.",
		},
		{
			name:    "def_no_docstring",
			content: "def compute():\n    return 42\n",
			want:    "def compute():",
		},
		{
			name:    "multiline_docstring_uses_first",
			content: "def run():\n    \"\"\"Run the job.\n    Extra detail here.\n    \"\"\"\n    pass\n",
			want:    "Run the job.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := makeGitRepo(t)
			os.WriteFile(filepath.Join(dir, "x.py"), []byte(tc.content), 0644)
			exec.Command("git", "-C", dir, "add", ".").Run()

			entries, _ := scanner.ExtractAll(dir, []string{"x.py"})
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].Description != tc.want {
				t.Errorf("Description = %q, want %q", entries[0].Description, tc.want)
			}
			if entries[0].Language != "python" {
				t.Errorf("Language = %q, want python", entries[0].Language)
			}
		})
	}
}

func TestExtractJS(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "export_function",
			content: "export function greet(name) {\n  return `Hello ${name}`;\n}\n",
			want:    "export function greet(name) {",
		},
		{
			name:    "class_declaration",
			content: "class Parser {\n  constructor() {}\n}\n",
			want:    "class Parser {",
		},
		{
			name: "jsdoc_description_tag",
			content: "/**\n * @description Handles user auth.\n */\nexport function login() {}\n",
			want: "Handles user auth.",
		},
		{
			name: "jsdoc_first_content_line",
			content: "/**\n * Validates input data.\n * @param x - value\n */\nexport function validate(x) {}\n",
			want: "Validates input data.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := makeGitRepo(t)
			os.WriteFile(filepath.Join(dir, "x.ts"), []byte(tc.content), 0644)
			exec.Command("git", "-C", dir, "add", ".").Run()

			entries, _ := scanner.ExtractAll(dir, []string{"x.ts"})
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].Description != tc.want {
				t.Errorf("Description = %q, want %q", entries[0].Description, tc.want)
			}
			if entries[0].Language != "js" {
				t.Errorf("Language = %q, want js", entries[0].Language)
			}
		})
	}
}

func TestExtractMD(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "h1_heading",
			content: "# My Project\n\nSome text.\n",
			want:    "My Project",
		},
		{
			name:    "h2_heading",
			content: "## Configuration\n\nDetails here.\n",
			want:    "Configuration",
		},
		{
			name:    "no_heading_uses_first_line",
			content: "This is a plain note.\n",
			want:    "This is a plain note.",
		},
		{
			name:    "heading_strips_hashes",
			content: "### Deep Section\n",
			want:    "Deep Section",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := makeGitRepo(t)
			os.WriteFile(filepath.Join(dir, "x.md"), []byte(tc.content), 0644)
			exec.Command("git", "-C", dir, "add", ".").Run()

			entries, _ := scanner.ExtractAll(dir, []string{"x.md"})
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].Description != tc.want {
				t.Errorf("Description = %q, want %q", entries[0].Description, tc.want)
			}
			if entries[0].Language != "markdown" {
				t.Errorf("Language = %q, want markdown", entries[0].Language)
			}
		})
	}
}

func TestScanProjectSmall(t *testing.T) {
	dir := makeGitRepo(t)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.21\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\n// main is the entry point.\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Project\n\nA small test.\n"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	entries, err := scanner.ScanProject(dir)
	if err != nil {
		t.Fatalf("ScanProject: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), entries)
	}

	byPath := make(map[string]scanner.FileEntry)
	for _, e := range entries {
		byPath[e.Path] = e
	}

	if byPath["go.mod"].Language != "config" {
		t.Errorf("go.mod Language = %q, want config", byPath["go.mod"].Language)
	}
	if byPath["go.mod"].Description != "Go module definition" {
		t.Errorf("go.mod Description = %q", byPath["go.mod"].Description)
	}
	if byPath["main.go"].Language != "go" {
		t.Errorf("main.go Language = %q, want go", byPath["main.go"].Language)
	}
	if byPath["main.go"].Description != "main is the entry point." {
		t.Errorf("main.go Description = %q", byPath["main.go"].Description)
	}
	if byPath["README.md"].Language != "markdown" {
		t.Errorf("README.md Language = %q, want markdown", byPath["README.md"].Language)
	}
	if byPath["README.md"].Description != "Test Project" {
		t.Errorf("README.md Description = %q, want Test Project", byPath["README.md"].Description)
	}
	for _, e := range entries {
		if e.EstTokens == 0 {
			t.Errorf("%s: EstTokens = 0, want > 0", e.Path)
		}
	}
}
