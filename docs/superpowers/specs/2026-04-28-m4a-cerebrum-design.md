# M4a: Cerebrum Rules Engine — Design Spec

## Overview

M4a is the first of three M4 sub-milestones. It adds a project-local rules engine ("cerebrum") that warns Claude before it writes code that violates known project conventions.

Three deliverables:
- `pkg/state/cerebrum.go` — rule I/O and state API
- `cmd/hook_prewrite.go` — full PreToolUse handler (replaces M1 stub)
- `cmd/cmd_cerebrum.go` — `cerebrum {add,list,remove}` CLI

M4b (buglog) and M4c (MCP tools) follow separately.

---

## 1. Architecture & Data Flow

```
PreToolUse (Write|Edit|MultiEdit) fires
  → cmd/hook_prewrite.go: runPreWrite(stdin)
  → state.ReadCerebrum(projectRoot) → []CerebrumRule  (unlocked, stale OK)
  → extractAddedLines(ev) → map[lineNo]string  (diff old vs new)
  → for each rule: regexp match against added lines
  → if matches:
      stderr block: "⚡ claude-context: ⚠️ N cerebrum rule(s) matched:\n  • msg (line N)"
      exit 1  (informational — content delivered to Claude)
  → if no matches: exit 0

cerebrum add [--pattern P] [--message M] [--comment C]
  → cmd/cmd_cerebrum.go
  → prompts interactively for any missing flags
  → validates regex compiles; exits 1 if invalid
  → state.AppendCerebrumRule(projectRoot, rule)

cerebrum list
  → state.ReadCerebrum(projectRoot) → numbered output

cerebrum remove <N>
  → state.ReadCerebrum → remove by 1-based index → state.WriteCerebrum
  → confirms via y/N (skippable with --yes)
```

### File map

**New files:**

| File | Responsibility |
|---|---|
| `pkg/state/cerebrum.go` | `CerebrumRule` type, `ReadCerebrum`, `AppendCerebrumRule`, `WriteCerebrum` |
| `cmd/hook_prewrite.go` | Full `runPreWrite` + `extractAddedLines` |
| `cmd/cmd_cerebrum.go` | `dispatchCerebrum` + subcommands |

**Modified files:**

| File | Change |
|---|---|
| `cmd/cmd_hook.go` | Remove `runPreWrite` stub |
| `cmd/main.go` | Add `cerebrum` dispatch case |
| `cmd/usage.go` | Add `cerebrum` help, version → `0.1.0-m4a` |
| `tests/integration/hook_chain_test.go` | 3 new prewrite integration tests |

---

## 2. cerebrum.md Format & State API

**Location:** `<project>/.claude-context/cerebrum.md`

**Format:**

```markdown
<!-- claude-context cerebrum v1 -->

# prefer := over var declarations
pattern: \bvar\s+\w+\s*=
warning: prefer := for short variable declarations

# no fmt.Println in production
pattern: fmt\.Println\(
warning: use a structured logger instead of fmt.Println

# no hardcoded credentials
pattern: (?i)(password|secret|api_key)\s*[:=]\s*["'][^"']{4,}
warning: never hardcode credentials; use environment variables
```

Rules are separated by blank lines. Each rule block contains:
- An optional `# comment` header (human label, ignored by parser)
- A `pattern:` line — Go `regexp` syntax, validated on `add`
- A `warning:` line — shown to Claude verbatim

**State API:**

```go
type CerebrumRule struct {
    Comment string  // the # line, without leading #; empty if omitted
    Pattern string  // raw regex string
    Message string  // warning text shown to Claude
}

// ReadCerebrum reads cerebrum.md; returns nil, nil if file does not exist.
// Unlocked read — stale is acceptable in hook context.
func ReadCerebrum(projectRoot string) ([]CerebrumRule, error)

// AppendCerebrumRule appends one rule to cerebrum.md.
// Creates the file with the v1 header if absent.
// Uses AtomicWrite (flock-X, 50ms timeout).
func AppendCerebrumRule(projectRoot string, rule CerebrumRule) error

// WriteCerebrum overwrites cerebrum.md with the given slice.
// Used by cerebrum remove. Preserves v1 header.
// Uses AtomicWrite (flock-X, 50ms timeout).
func WriteCerebrum(projectRoot string, rules []CerebrumRule) error
```

Header `<!-- claude-context cerebrum v1 -->` is written once on first `AppendCerebrumRule`. Subsequent calls check for its presence before writing.

---

## 3. `cerebrum` CLI

```
claude-context cerebrum add [--pattern P] [--message M] [--comment C]
claude-context cerebrum list
claude-context cerebrum remove <N> [--yes]
```

### `cerebrum add`

Flags take priority; missing flags prompt interactively:

```
$ claude-context cerebrum add
Comment (optional, press Enter to skip): prefer := over var
Pattern (regex): \bvar\s+\w+\s*=
Warning message: prefer := for short variable declarations
✓ Rule added (3 rules total in .claude-context/cerebrum.md)
```

If `--pattern` and `--message` are both provided, no prompts. Validates regex compiles — exits 1 with an error message if invalid.

### `cerebrum list`

```
Cerebrum rules (3):
  1. prefer := for short variable declarations
     pattern: \bvar\s+\w+\s*=
  2. use a structured logger instead of fmt.Println
     pattern: fmt\.Println\(
  3. never hardcode credentials
     pattern: (?i)(password|secret|api_key)\s*[:=]\s*["'][^"']{4,}
```

Exits 0 with "No cerebrum rules. Run: claude-context cerebrum add" if file is empty or absent.

### `cerebrum remove <N>`

```
$ claude-context cerebrum remove 2
Remove rule 2: "use a structured logger instead of fmt.Println"? [y/N]: y
✓ Removed. 2 rules remaining.
```

- Non-TTY stdin without `--yes`: exits 1 with "confirmation required: re-run with --yes to skip" (refuses to run non-interactively without explicit consent).
- Out-of-range index: exits 1 with "no rule N (have M rules)".

---

## 4. prewrite hook handler

`cmd/hook_prewrite.go` — full implementation:

```go
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

    added := extractAddedLines(ev)
    if len(added) == 0 {
        exitHook("pre-write", root)
    }

    type match struct {
        msg  string
        line int
    }
    var matches []match
    for _, rule := range rules {
        re, err := regexp.Compile(rule.Pattern)
        if err != nil {
            appendGlobalLog(fmt.Sprintf("pre-write: invalid regex %q: %v", rule.Pattern, err))
            continue
        }
        for lineNo, line := range added {
            if re.MatchString(line) {
                matches = append(matches, match{rule.Message, lineNo})
                break // one match per rule is enough
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

### `extractAddedLines`

Returns a `map[int]string` (1-based line number in `new_string` → line content) for lines present in `new_string` but absent in `old_string`. Line numbers reflect position in the new content so Claude can locate the violation.

```go
// extractAddedLines returns lines added by this edit (present in new, absent in old).
// Line numbers are 1-based positions within the new content.
// For Write (no old content): all lines are "added".
// For MultiEdit: union across all edits.
func extractAddedLines(ev *hook.Event) map[int]string
```

**Algorithm:** build a `set` of old lines (via `map[string]bool`), then enumerate new lines; if a line is not in the old set, it's "added". Line numbers are 1-based positions in new content.

**Tool dispatch:**
- `Write`: `old = ""`, `new = tool_input.content`
- `Edit`: `old = tool_input.old_string`, `new = tool_input.new_string`
- `MultiEdit`: iterate `tool_input.edits[]`, concatenate all `new_string` values into one block, union all added lines; line numbers are 1-based within the concatenated block
- Anything else: return empty map (exit 0 silently)

---

## 5. Error Handling & Safety

| Error | Behavior |
|---|---|
| `cerebrum.md` missing | `ReadCerebrum` returns nil, nil → exit 0 (no rules = no warnings) |
| `cerebrum.md` corrupt | Return nil, nil (same as missing; log parse error internally) |
| Invalid regex in a rule | Skip that rule, log to `hook-errors.log`, continue with remaining rules |
| Lock timeout on `AppendCerebrumRule` | Return error to CLI; print to stderr; exit 1 |
| `cerebrum remove` out-of-range | Print error to stderr; exit 1 |
| Hook panic | `recoverAndLog` catches it; exit 0 (never blocks Claude) |

Warn-only invariant: the prewrite hook **never blocks** a write — it always exits 0 or 1. Exit 1 is informational only (Claude sees the message but proceeds).

---

## 6. Testing Strategy

**Unit tests** in `tests/cerebrum_test.go`:

- `TestReadCerebrumEmpty` — missing file → nil rules, nil error
- `TestAppendAndReadRules` — append 2 rules, read back, verify count and content
- `TestWriteCerebrumRoundtrip` — write 3 rules, read back, verify order preserved
- `TestHeaderWrittenOnce` — two appends → header appears exactly once in file
- `TestExtractAddedLinesEdit` — Edit payload with 3 new lines → returns correct set
- `TestExtractAddedLinesWrite` — Write payload → all lines returned
- `TestExtractAddedLinesNoChange` — new == old → empty map

**Integration tests** in `tests/integration/hook_chain_test.go`:

- `TestPreWriteMatchesCerebrumRule` — init project, add rule via CLI, fire pre-write with matching content → assert exit 1, stderr contains rule warning
- `TestPreWriteNoMatchExitsZero` — fire pre-write with non-matching content → assert exit 0
- `TestPreWriteNoCerebrumExitsZero` — no cerebrum.md → assert exit 0

---

## 7. Out of Scope (M4a)

- Buglog auto-upsert from post-write (M4b)
- Buglog hints in pre-write (M4b)
- `find_similar_bugs` MCP tool (M4c)
- `describe_codebase` / `get_project_rules` MCP tools (M4c)
- Severity levels on rules (warn vs block) — all rules are warn-only
- Auto-learning rules from edits — rules are always user-defined
