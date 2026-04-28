# M4a: Cerebrum Rules Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a project-local rules engine that warns Claude before it writes code violating known conventions.

**Architecture:** Three new files — `pkg/state/cerebrum.go` (rule I/O), `cmd/hook_prewrite.go` (full PreToolUse handler), `cmd/cmd_cerebrum.go` (CLI). `ExtractAddedLines` lives in `pkg/hook/protocol.go` so it can be unit-tested. The prewrite hook reads cerebrum.md (unlocked, stale OK), diffs old vs new content to find added lines, matches rules against them, and exits 1 with a block-format warning if any match.

**Tech Stack:** Go 1.21+, `pkg/state` (AtomicWrite + AcquireLock), `pkg/hook` (Event parsing), standard library only.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `pkg/state/cerebrum.go` | Create | `CerebrumRule` type, `ReadCerebrum`, `AppendCerebrumRule`, `WriteCerebrum` |
| `pkg/hook/protocol.go` | Modify | Add `ExtractAddedLines() map[int]string` method on `*Event` |
| `cmd/hook_prewrite.go` | Create | Full `runPreWrite` handler |
| `cmd/cmd_cerebrum.go` | Create | `dispatchCerebrum` + add/list/remove subcommands |
| `cmd/cmd_hook.go` | Modify | Remove `runPreWrite` stub (lines 45–54) |
| `cmd/main.go` | Modify | Add `cerebrum` dispatch case |
| `cmd/usage.go` | Modify | Add `cerebrum` help, version → `0.1.0-m4a` |
| `tests/cerebrum_test.go` | Create | Unit tests for cerebrum state + ExtractAddedLines |
| `tests/integration/hook_chain_test.go` | Modify | 3 new prewrite integration tests |

---

### Task 1: pkg/state/cerebrum.go

**Files:**
- Create: `pkg/state/cerebrum.go`
- Create: `tests/cerebrum_test.go`

- [ ] **Step 1: Write failing tests**

Create `tests/cerebrum_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/state"
)

func TestReadCerebrumEmpty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)
	rules, err := state.ReadCerebrum(dir)
	if err != nil {
		t.Fatalf("ReadCerebrum on missing file: %v", err)
	}
	if rules != nil {
		t.Errorf("expected nil rules, got %v", rules)
	}
}

func TestAppendAndReadRules(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	r1 := state.CerebrumRule{Comment: "no var", Pattern: `\bvar\s+\w+\s*=`, Message: "prefer :="}
	r2 := state.CerebrumRule{Pattern: `fmt\.Println\(`, Message: "use structured logger"}

	if err := state.AppendCerebrumRule(dir, r1); err != nil {
		t.Fatalf("AppendCerebrumRule r1: %v", err)
	}
	if err := state.AppendCerebrumRule(dir, r2); err != nil {
		t.Fatalf("AppendCerebrumRule r2: %v", err)
	}

	rules, err := state.ReadCerebrum(dir)
	if err != nil {
		t.Fatalf("ReadCerebrum: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Pattern != `\bvar\s+\w+\s*=` {
		t.Errorf("rule[0].Pattern = %q", rules[0].Pattern)
	}
	if rules[0].Message != "prefer :=" {
		t.Errorf("rule[0].Message = %q", rules[0].Message)
	}
	if rules[0].Comment != "no var" {
		t.Errorf("rule[0].Comment = %q", rules[0].Comment)
	}
	if rules[1].Pattern != `fmt\.Println\(` {
		t.Errorf("rule[1].Pattern = %q", rules[1].Pattern)
	}
}

func TestWriteCerebrumRoundtrip(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	rules := []state.CerebrumRule{
		{Comment: "a", Pattern: "pat1", Message: "msg1"},
		{Comment: "b", Pattern: "pat2", Message: "msg2"},
		{Comment: "c", Pattern: "pat3", Message: "msg3"},
	}
	if err := state.WriteCerebrum(dir, rules); err != nil {
		t.Fatalf("WriteCerebrum: %v", err)
	}
	got, err := state.ReadCerebrum(dir)
	if err != nil {
		t.Fatalf("ReadCerebrum: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(got))
	}
	for i, r := range rules {
		if got[i].Pattern != r.Pattern {
			t.Errorf("rule[%d].Pattern = %q, want %q", i, got[i].Pattern, r.Pattern)
		}
		if got[i].Message != r.Message {
			t.Errorf("rule[%d].Message = %q, want %q", i, got[i].Message, r.Message)
		}
	}
}

func TestHeaderWrittenOnce(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".mneme"), 0755)

	r := state.CerebrumRule{Pattern: "x", Message: "y"}
	state.AppendCerebrumRule(dir, r)
	state.AppendCerebrumRule(dir, r)

	data, _ := os.ReadFile(filepath.Join(dir, ".mneme", "cerebrum.md"))
	count := strings.Count(string(data), "<!-- mneme cerebrum v1 -->")
	if count != 1 {
		t.Errorf("header appears %d times, want 1", count)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v ./tests -run "TestReadCerebrum|TestAppend|TestWrite|TestHeader"
```

Expected: FAIL — `state.CerebrumRule undefined`

- [ ] **Step 3: Create pkg/state/cerebrum.go**

```go
package state

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CerebrumRule struct {
	Comment string // # header line content (without leading "# "); empty if absent
	Pattern string // raw Go regexp string
	Message string // warning text shown to Claude
}

const cerebrumHeader = "<!-- mneme cerebrum v1 -->"

// ReadCerebrum reads .mneme/cerebrum.md.
// Returns nil, nil if the file does not exist.
// Unlocked — stale reads are acceptable in hook context.
func ReadCerebrum(projectRoot string) ([]CerebrumRule, error) {
	path := filepath.Join(projectRoot, ".mneme", "cerebrum.md")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseCerebrum(string(data)), nil
}

// AppendCerebrumRule appends one rule to cerebrum.md, creating it with the v1 header if absent.
// Uses flock-X with 50ms timeout.
func AppendCerebrumRule(projectRoot string, rule CerebrumRule) error {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "cerebrum.lock")
	release, err := AcquireLock(lockPath, 50*time.Millisecond)
	if err != nil {
		return err
	}
	defer release()

	path := filepath.Join(dir, "cerebrum.md")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var content string
	if len(existing) == 0 {
		content = cerebrumHeader + "\n\n" + formatRule(rule)
	} else {
		content = string(existing) + "\n" + formatRule(rule)
	}
	return AtomicWrite(path, []byte(content))
}

// WriteCerebrum overwrites cerebrum.md with the given slice.
// Used by cerebrum remove. Writes the v1 header.
// Uses flock-X with 50ms timeout.
func WriteCerebrum(projectRoot string, rules []CerebrumRule) error {
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "cerebrum.lock")
	release, err := AcquireLock(lockPath, 50*time.Millisecond)
	if err != nil {
		return err
	}
	defer release()

	var sb strings.Builder
	sb.WriteString(cerebrumHeader + "\n")
	for _, r := range rules {
		sb.WriteString("\n")
		sb.WriteString(formatRule(r))
	}
	return AtomicWrite(filepath.Join(dir, "cerebrum.md"), []byte(sb.String()))
}

func formatRule(r CerebrumRule) string {
	var sb strings.Builder
	if r.Comment != "" {
		sb.WriteString("# " + r.Comment + "\n")
	}
	sb.WriteString("pattern: " + r.Pattern + "\n")
	sb.WriteString("warning: " + r.Message + "\n")
	return sb.String()
}

func parseCerebrum(content string) []CerebrumRule {
	var rules []CerebrumRule
	var cur CerebrumRule
	inRule := false

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "<!--") || line == "" {
			if inRule && cur.Pattern != "" && cur.Message != "" {
				rules = append(rules, cur)
				cur = CerebrumRule{}
				inRule = false
			}
			continue
		}
		if strings.HasPrefix(line, "# ") {
			if inRule && cur.Pattern != "" && cur.Message != "" {
				rules = append(rules, cur)
				cur = CerebrumRule{}
			}
			cur.Comment = strings.TrimPrefix(line, "# ")
			inRule = true
			continue
		}
		if strings.HasPrefix(line, "pattern: ") {
			cur.Pattern = strings.TrimPrefix(line, "pattern: ")
			inRule = true
			continue
		}
		if strings.HasPrefix(line, "warning: ") {
			cur.Message = strings.TrimPrefix(line, "warning: ")
			inRule = true
			continue
		}
	}
	if inRule && cur.Pattern != "" && cur.Message != "" {
		rules = append(rules, cur)
	}
	return rules
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -v ./tests -run "TestReadCerebrum|TestAppend|TestWrite|TestHeader"
```

Expected: PASS (4 tests)

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add pkg/state/cerebrum.go tests/cerebrum_test.go
git commit -m "feat(m4a): add cerebrum state — CerebrumRule, ReadCerebrum, AppendCerebrumRule, WriteCerebrum"
```

---

### Task 2: ExtractAddedLines in pkg/hook/protocol.go

**Files:**
- Modify: `pkg/hook/protocol.go`
- Modify: `tests/cerebrum_test.go`

- [ ] **Step 1: Add failing tests to tests/cerebrum_test.go**

Append to `tests/cerebrum_test.go`:

```go
func TestExtractAddedLinesEdit(t *testing.T) {
	// Edit: old has 2 lines, new has 3 lines; 2 lines are added
	payload := `{
		"session_id": "s1",
		"transcript_path": "/tmp/t",
		"cwd": "/tmp",
		"hook_event_name": "PreToolUse",
		"tool_name": "Edit",
		"tool_input": {
			"file_path": "/tmp/foo.go",
			"old_string": "line1\nline2",
			"new_string": "line1\nline2\nnewline3\nnewline4"
		}
	}`
	ev, err := hook.ParseEvent(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	added := ev.ExtractAddedLines()
	if len(added) != 2 {
		t.Fatalf("expected 2 added lines, got %d: %v", len(added), added)
	}
	// newline3 is at position 3, newline4 at position 4
	if added[3] != "newline3" {
		t.Errorf("line 3 = %q, want %q", added[3], "newline3")
	}
	if added[4] != "newline4" {
		t.Errorf("line 4 = %q, want %q", added[4], "newline4")
	}
}

func TestExtractAddedLinesWrite(t *testing.T) {
	// Write: all lines are "added"
	payload := `{
		"session_id": "s1",
		"transcript_path": "/tmp/t",
		"cwd": "/tmp",
		"hook_event_name": "PreToolUse",
		"tool_name": "Write",
		"tool_input": {
			"file_path": "/tmp/new.go",
			"content": "package main\n\nfunc main() {}"
		}
	}`
	ev, _ := hook.ParseEvent(strings.NewReader(payload))
	added := ev.ExtractAddedLines()
	if len(added) != 3 {
		t.Fatalf("expected 3 added lines, got %d: %v", len(added), added)
	}
}

func TestExtractAddedLinesNoChange(t *testing.T) {
	// Edit: new == old → no added lines
	payload := `{
		"session_id": "s1",
		"transcript_path": "/tmp/t",
		"cwd": "/tmp",
		"hook_event_name": "PreToolUse",
		"tool_name": "Edit",
		"tool_input": {
			"file_path": "/tmp/foo.go",
			"old_string": "same\ncontent",
			"new_string": "same\ncontent"
		}
	}`
	ev, _ := hook.ParseEvent(strings.NewReader(payload))
	added := ev.ExtractAddedLines()
	if len(added) != 0 {
		t.Errorf("expected 0 added lines, got %d: %v", len(added), added)
	}
}
```

Add `"github.com/ranwei/mneme/pkg/hook"` to the import block in `tests/cerebrum_test.go`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v ./tests -run "TestExtractAddedLines"
```

Expected: FAIL — `ev.ExtractAddedLines undefined`

- [ ] **Step 3: Add ExtractAddedLines to pkg/hook/protocol.go**

Add `"strings"` to the import block, then append this method to the file:

```go
// ExtractAddedLines returns lines present in new content but absent in old content,
// keyed by 1-based line number within the new content.
// For Write (no old content): all non-empty lines are returned.
// For MultiEdit: new content is the concatenation of all new_string values.
// Returns nil if the tool is not Write/Edit/MultiEdit or if nothing was added.
func (e *Event) ExtractAddedLines() map[int]string {
	var oldStr, newStr string

	switch e.ToolName {
	case "Write":
		var ti struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(e.ToolInput, &ti); err != nil {
			return nil
		}
		newStr = ti.Content
	case "Edit":
		var ti struct {
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if err := json.Unmarshal(e.ToolInput, &ti); err != nil {
			return nil
		}
		oldStr = ti.OldString
		newStr = ti.NewString
	case "MultiEdit":
		var ti struct {
			Edits []struct {
				OldString string `json:"old_string"`
				NewString string `json:"new_string"`
			} `json:"edits"`
		}
		if err := json.Unmarshal(e.ToolInput, &ti); err != nil {
			return nil
		}
		var oldParts, newParts []string
		for _, ed := range ti.Edits {
			oldParts = append(oldParts, ed.OldString)
			newParts = append(newParts, ed.NewString)
		}
		oldStr = strings.Join(oldParts, "\n")
		newStr = strings.Join(newParts, "\n")
	default:
		return nil
	}

	oldSet := make(map[string]bool)
	for _, line := range strings.Split(oldStr, "\n") {
		oldSet[line] = true
	}

	result := make(map[int]string)
	for i, line := range strings.Split(newStr, "\n") {
		if !oldSet[line] {
			result[i+1] = line
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -v ./tests -run "TestExtractAddedLines"
```

Expected: PASS (3 tests)

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add pkg/hook/protocol.go tests/cerebrum_test.go
git commit -m "feat(m4a): add ExtractAddedLines to hook.Event"
```

---

### Task 3: cmd/hook_prewrite.go — full runPreWrite

**Files:**
- Create: `cmd/hook_prewrite.go`
- Modify: `cmd/cmd_hook.go` (remove stub)

- [ ] **Step 1: Create cmd/hook_prewrite.go**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ranwei/mneme/pkg/hook"
	"github.com/ranwei/mneme/pkg/state"
)

func runPreWrite(stdin io.Reader) {
	defer recoverAndLog("pre-write")
	ev := parseOrExit(stdin, "pre-write")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.pre-write")

	rules, err := state.ReadCerebrum(root)
	if err != nil || len(rules) == 0 {
		exitHook("pre-write", root)
	}

	added := ev.ExtractAddedLines()
	if len(added) == 0 {
		exitHook("pre-write", root)
	}

	type matchResult struct {
		msg  string
		line int
	}
	var matches []matchResult

	for _, rule := range rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			appendGlobalLog(fmt.Sprintf("pre-write: invalid regex %q: %v", rule.Pattern, err))
			continue
		}
		// Sort line numbers for deterministic output
		lineNums := make([]int, 0, len(added))
		for ln := range added {
			lineNums = append(lineNums, ln)
		}
		sort.Ints(lineNums)
		for _, ln := range lineNums {
			if re.MatchString(added[ln]) {
				matches = append(matches, matchResult{rule.Message, ln})
				break // one match per rule
			}
		}
	}

	if len(matches) == 0 {
		exitHook("pre-write", root)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "⚠️ %d cerebrum rule(s) matched:\n", len(matches))
	for _, m := range matches {
		fmt.Fprintf(&sb, "  • %s (line %d)", m.msg, m.line)
	}
	hook.WriteStderr(sb.String())
	os.Exit(1)
}
```

- [ ] **Step 2: Remove runPreWrite stub from cmd/cmd_hook.go**

Delete the `runPreWrite` function (lines 45–54 in the current file):

```go
func runPreWrite(stdin io.Reader) {
	defer recoverAndLog("pre-write")
	ev := parseOrExit(stdin, "pre-write")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.pre-write")
	exitHook("pre-write", root)
}
```

The `case "pre-write": runPreWrite(os.Stdin)` dispatch line stays — it now calls the function defined in `hook_prewrite.go`.

- [ ] **Step 3: Build to verify it compiles**

```bash
go build -o ./bin/mneme ./cmd
```

Expected: no errors.

- [ ] **Step 4: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/hook_prewrite.go cmd/cmd_hook.go
git commit -m "feat(m4a): full runPreWrite — cerebrum regex matching with added-lines diff"
```

---

### Task 4: cmd/cmd_cerebrum.go — CLI

**Files:**
- Create: `cmd/cmd_cerebrum.go`
- Modify: `cmd/main.go`
- Modify: `cmd/usage.go`

- [ ] **Step 1: Create cmd/cmd_cerebrum.go**

```go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/ranwei/mneme/pkg/state"
)

func dispatchCerebrum(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mneme cerebrum <add|list|remove>")
		os.Exit(2)
	}
	switch args[0] {
	case "add":
		cerebrumAdd(args[1:])
	case "list":
		cerebrumList()
	case "remove":
		cerebrumRemove(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown cerebrum subcommand: %q\n", args[0])
		os.Exit(2)
	}
}

func cerebrumAdd(args []string) {
	fs := flag.NewFlagSet("cerebrum add", flag.ContinueOnError)
	pattern := fs.String("pattern", "", "regex pattern to match")
	message := fs.String("message", "", "warning message shown to Claude")
	comment := fs.String("comment", "", "human-readable label (optional)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	scanner := bufio.NewScanner(os.Stdin)
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	if *comment == "" && isTTY {
		fmt.Fprint(os.Stderr, "Comment (optional, press Enter to skip): ")
		scanner.Scan()
		*comment = strings.TrimSpace(scanner.Text())
	}
	if *pattern == "" {
		if !isTTY {
			fmt.Fprintln(os.Stderr, "✗ --pattern required in non-TTY mode")
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, "Pattern (regex): ")
		scanner.Scan()
		*pattern = strings.TrimSpace(scanner.Text())
	}
	if *message == "" {
		if !isTTY {
			fmt.Fprintln(os.Stderr, "✗ --message required in non-TTY mode")
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, "Warning message: ")
		scanner.Scan()
		*message = strings.TrimSpace(scanner.Text())
	}

	if *pattern == "" || *message == "" {
		fmt.Fprintln(os.Stderr, "✗ pattern and message are required")
		os.Exit(1)
	}
	if _, err := regexp.Compile(*pattern); err != nil {
		fmt.Fprintf(os.Stderr, "✗ invalid regex: %v\n", err)
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	root, _ := state.FindProjectRoot(cwd)
	rule := state.CerebrumRule{Comment: *comment, Pattern: *pattern, Message: *message}
	if err := state.AppendCerebrumRule(root, rule); err != nil {
		fmt.Fprintln(os.Stderr, "✗ write error:", err)
		os.Exit(1)
	}

	rules, _ := state.ReadCerebrum(root)
	fmt.Fprintf(os.Stderr, "✓ Rule added (%d rules total in .mneme/cerebrum.md)\n", len(rules))
}

func cerebrumList() {
	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}

	rules, err := state.ReadCerebrum(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ read error:", err)
		os.Exit(1)
	}
	if len(rules) == 0 {
		fmt.Println("No cerebrum rules. Run: mneme cerebrum add")
		return
	}

	fmt.Printf("Cerebrum rules (%d):\n", len(rules))
	for i, r := range rules {
		fmt.Printf("  %d. %s\n", i+1, r.Message)
		fmt.Printf("     pattern: %s\n", r.Pattern)
	}
}

func cerebrumRemove(args []string) {
	fs := flag.NewFlagSet("cerebrum remove", flag.ContinueOnError)
	yes := fs.BoolVar(new(bool), "yes", false, "skip confirmation")
	_ = yes
	var skipConfirm bool
	fs.BoolVar(&skipConfirm, "yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: mneme cerebrum remove <N> [--yes]")
		os.Exit(2)
	}

	n, err := strconv.Atoi(fs.Arg(0))
	if err != nil || n < 1 {
		fmt.Fprintln(os.Stderr, "✗ N must be a positive integer")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	root, ok := state.FindProjectRoot(cwd)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ not inside an initialized project (run: mneme init)")
		os.Exit(1)
	}

	rules, err := state.ReadCerebrum(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ read error:", err)
		os.Exit(1)
	}
	if n > len(rules) {
		fmt.Fprintf(os.Stderr, "✗ no rule %d (have %d rules)\n", n, len(rules))
		os.Exit(1)
	}

	target := rules[n-1]
	if !skipConfirm {
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))
		if !isTTY {
			fmt.Fprintln(os.Stderr, "✗ confirmation required: re-run with --yes to skip")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Remove rule %d: %q? [y/N]: ", n, target.Message)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		reply := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if reply != "y" && reply != "yes" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			os.Exit(0)
		}
	}

	newRules := append(rules[:n-1:n-1], rules[n:]...)
	if err := state.WriteCerebrum(root, newRules); err != nil {
		fmt.Fprintln(os.Stderr, "✗ write error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Removed. %d rules remaining.\n", len(newRules))
}
```

- [ ] **Step 2: Add cerebrum dispatch to cmd/main.go**

In the `switch os.Args[1]` block, add before `default`:

```go
case "cerebrum":
    dispatchCerebrum(os.Args[2:])
```

- [ ] **Step 3: Update cmd/usage.go**

Replace the full file:

```go
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-m4a"

func printTopUsage() {
	fmt.Fprintln(os.Stderr, `mneme — Claude Code context management

Usage:
  mneme                   start MCP server (for claude mcp add)
  mneme hook <event>      handle a Claude Code hook event
  mneme init [flags]      install hooks and scaffolding
  mneme stats             show ledger counters for current project
  mneme cerebrum <cmd>    manage project coding rules
  mneme version           print version

cerebrum commands:
  cerebrum add [--pattern P] [--message M] [--comment C]
  cerebrum list
  cerebrum remove <N> [--yes]

init flags:
  --yes         skip y/N confirmation
  --dry-run     show plan without writing
  --print       print final file contents to stdout
  --project     write to <project>/.claude/settings.json
  --local       write to <project>/.claude/settings.local.json
  --no-scan     skip anatomy scan on init
  --uninstall   remove all mneme managed entries`)
}
```

- [ ] **Step 4: Build to verify it compiles**

```bash
go build -o ./bin/mneme ./cmd
```

Expected: no errors.

- [ ] **Step 5: Smoke-test the CLI**

In a git repo with `mneme init` already run:

```bash
./bin/mneme cerebrum list
# Expected: "No cerebrum rules. Run: mneme cerebrum add"

./bin/mneme cerebrum add --pattern '\bvar\s+\w+\s*=' --message 'prefer :='
# Expected: "✓ Rule added (1 rules total in .mneme/cerebrum.md)"

./bin/mneme cerebrum list
# Expected:
# Cerebrum rules (1):
#   1. prefer :=
#      pattern: \bvar\s+\w+\s*=

./bin/mneme cerebrum add --pattern 'INVALID[' --message 'bad regex'
# Expected: "✗ invalid regex: ..."  exit 1
```

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/cmd_cerebrum.go cmd/main.go cmd/usage.go
git commit -m "feat(m4a): cerebrum CLI — add, list, remove subcommands"
```

---

### Task 5: Integration tests + benchmark

**Files:**
- Modify: `tests/integration/hook_chain_test.go`

- [ ] **Step 1: Append 3 integration tests to tests/integration/hook_chain_test.go**

```go
func TestPreWriteMatchesCerebrumRule(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	// Add a rule via the binary
	addCmd := exec.Command(binaryPath, "cerebrum", "add",
		"--pattern", `fmt\.Println\(`,
		"--message", "use structured logger",
	)
	addCmd.Dir = project
	addCmd.Env = env
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("cerebrum add: %v\n%s", err, out)
	}

	// Fire pre-write with content that matches the rule
	payload := map[string]interface{}{
		"session_id":      "sess-cere",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input": map[string]interface{}{
			"file_path":  filepath.Join(project, "main.go"),
			"old_string": "func main() {}",
			"new_string": "func main() {\n\tfmt.Println(\"hello\")\n}",
		},
	}
	b, _ := json.Marshal(payload)
	cmd := exec.Command(binaryPath, "hook", "pre-write")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(b))
	out, err := cmd.CombinedOutput()
	// Should exit 1 (informational warning)
	if err == nil {
		t.Errorf("expected exit 1 (warning), got exit 0\noutput: %s", out)
	}
	if !strings.Contains(string(out), "cerebrum rule") {
		t.Errorf("output missing cerebrum warning: %s", out)
	}
	if !strings.Contains(string(out), "use structured logger") {
		t.Errorf("output missing rule message: %s", out)
	}
}

func TestPreWriteNoMatchExitsZero(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)

	// Add a rule
	exec.Command(binaryPath, "cerebrum", "add",
		"--pattern", `fmt\.Println\(`,
		"--message", "use structured logger",
	).Run()

	// Fire pre-write with content that does NOT match
	payload := map[string]interface{}{
		"session_id":      "sess-nomatch",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input": map[string]interface{}{
			"file_path":  filepath.Join(project, "main.go"),
			"old_string": "func main() {}",
			"new_string": "func main() {\n\tlog.Println(\"hello\")\n}",
		},
	}
	b, _ := json.Marshal(payload)
	cmd := exec.Command(binaryPath, "hook", "pre-write")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(b))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected exit 0 (no match), got error: %v\noutput: %s", err, out)
	}
}

func TestPreWriteNoCerebrumExitsZero(t *testing.T) {
	project, home := setupInitializedProject(t)
	env := append(os.Environ(), "HOME="+home)
	// No cerebrum.md — hook should exit 0 silently

	payload := map[string]interface{}{
		"session_id":      "sess-nocerebrum",
		"transcript_path": "/tmp/t",
		"cwd":             project,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": filepath.Join(project, "newfile.go"),
			"content":   "package main\n",
		},
	}
	b, _ := json.Marshal(payload)
	cmd := exec.Command(binaryPath, "hook", "pre-write")
	cmd.Dir = project
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(b))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected exit 0 (no cerebrum), got error: %v\noutput: %s", err, out)
	}
}
```

- [ ] **Step 2: Run the new integration tests**

```bash
go test -v ./tests/integration -run "TestPreWrite"
```

Expected: PASS (3 tests)

- [ ] **Step 3: Run full integration suite**

```bash
go test -v ./tests/integration
```

Expected: all pass.

- [ ] **Step 4: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add tests/integration/hook_chain_test.go
git commit -m "test(m4a): integration tests for prewrite cerebrum matching"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `CerebrumRule` type | Task 1 |
| `ReadCerebrum`, `AppendCerebrumRule`, `WriteCerebrum` | Task 1 |
| cerebrum.md format with v1 header | Task 1 |
| `ExtractAddedLines` (added lines diff) | Task 2 |
| Full `runPreWrite` handler | Task 3 |
| Cerebrum rule matching with block-format warning | Task 3 |
| Invalid regex skip + log | Task 3 |
| `cerebrum add` — flags + interactive fallback | Task 4 |
| `cerebrum add` regex validation | Task 4 |
| `cerebrum list` | Task 4 |
| `cerebrum remove` — y/N confirmation, --yes flag | Task 4 |
| `cerebrum remove` non-TTY exits 1 with error | Task 4 |
| `cerebrum remove` out-of-range exits 1 | Task 4 |
| Version → `0.1.0-m4a` | Task 4 |
| `TestReadCerebrumEmpty` | Task 1 |
| `TestAppendAndReadRules` | Task 1 |
| `TestWriteCerebrumRoundtrip` | Task 1 |
| `TestHeaderWrittenOnce` | Task 1 |
| `TestExtractAddedLinesEdit/Write/NoChange` | Task 2 |
| `TestPreWriteMatchesCerebrumRule` | Task 5 |
| `TestPreWriteNoMatchExitsZero` | Task 5 |
| `TestPreWriteNoCerebrumExitsZero` | Task 5 |

All spec requirements covered. No placeholders. Types consistent across tasks (`CerebrumRule.Pattern`/`.Message`/`.Comment` used identically in Task 1, 3, 4).
