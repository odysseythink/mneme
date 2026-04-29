# M0 Hook Protocol Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Empirically validate the hook-protocol assumptions (R1-R5) baked into the architecture spec by building a bash-based probe harness, having the user run it against real Claude Code, then producing a written report + permanent test fixtures + (if needed) inline patches to the architecture spec.

**Architecture:** Bash probe scripts in `scripts/m0-probe/` capture data from real Claude Code hook events into `/tmp/claude-context-m0/`. A user-facing runbook in `docs/m0-runbook.md` walks the user through 4 phases of probing. The assistant analyzes captured data, writes the report, selects canonical fixtures, and patches the architecture spec wherever findings contradict assumptions. Probe scripts are then renamed to `scripts/hook-protocol-probe/` for permanent re-verification.

**Tech Stack:** Bash 3.2+ (macOS-compatible), POSIX-friendly tools (sed, awk, shasum), JSON files for hook configs, Markdown for docs.

---

## File Structure

**Phase 1 — assistant produces (probe tooling + runbook):**
- Create: `scripts/m0-probe/echo.sh` (probe: dump stdin to file)
- Create: `scripts/m0-probe/exit-n.sh` (probe: parameterized exit code)
- Create: `scripts/m0-probe/sleep-n.sh` (probe: parameterized sleep)
- Create: `scripts/m0-probe/swap-hook.sh` (helper: swap registered hook in settings.local.json)
- Create: `scripts/m0-probe/setup-sandbox.sh` (creates `/tmp/claude-context-m0-sandbox/` + writes settings.local.json + R5 test files)
- Create: `scripts/m0-probe/cleanup.sh` (removes sandbox + R5 leftovers + verifies env clean)
- Create: `scripts/m0-probe/README.md` (usage notes)
- Create: `docs/m0-runbook.md` (user-facing 4-phase runbook)

**Phase 2 — user executes runbook (no file changes; user provides observations and dump file paths via the conversation).**

**Phase 3 — assistant analyzes + reports:**
- Create: `tests/fixtures/hook-payloads/pre-read.json` (canonical sample, sanitized)
- Create: `tests/fixtures/hook-payloads/pre-write.json`
- Create: `tests/fixtures/hook-payloads/post-write.json`
- Create: `tests/fixtures/hook-payloads/session-start.json`
- Create: `tests/fixtures/hook-payloads/stop.json`
- Create: `docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md`
- Modify (only if findings contradict assumptions): `docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md`

**Phase 4 — release-form rename:**
- Rename: `scripts/m0-probe/` → `scripts/hook-protocol-probe/`
- Modify: `scripts/hook-protocol-probe/README.md` (expand for permanent home)

---

## Phase 1 — Probe Tooling

### Task 1: echo.sh probe

**Files:**
- Create: `scripts/m0-probe/echo.sh`

- [ ] **Step 1: Create scripts/m0-probe/ directory and write echo.sh**

```bash
mkdir -p scripts/m0-probe
```

Write `scripts/m0-probe/echo.sh`:

```bash
#!/usr/bin/env bash
# Probe: dump stdin verbatim to a timestamped file under $OUT_DIR.
# Used to capture real hook payloads (R1).
# Invoked as: echo.sh <event-name>   e.g. echo.sh pre-read
set -euo pipefail
EVENT="${1:-unknown}"
OUT_DIR="${OUT_DIR:-/tmp/claude-context-m0/$EVENT}"
mkdir -p "$OUT_DIR"
ts=$(date +%s)-$$-$RANDOM
cat > "$OUT_DIR/${ts}-stdin.json"
exit 0
```

- [ ] **Step 2: Make executable and smoke-test**

```bash
chmod +x scripts/m0-probe/echo.sh
OUT_DIR=/tmp/m0-test-echo echo '{"hello":"world"}' | scripts/m0-probe/echo.sh test-event
ls /tmp/m0-test-echo/
cat /tmp/m0-test-echo/*-stdin.json
rm -rf /tmp/m0-test-echo
```

Expected: `ls` shows one file matching `*-stdin.json`; `cat` shows `{"hello":"world"}`.

- [ ] **Step 3: Commit**

```bash
git add scripts/m0-probe/echo.sh
git commit -m "feat(m0): add echo probe for capturing hook stdin payloads"
```

---

### Task 2: exit-n.sh probe

**Files:**
- Create: `scripts/m0-probe/exit-n.sh`

- [ ] **Step 1: Write exit-n.sh**

```bash
#!/usr/bin/env bash
# Probe: read stdin (discard), write a marker to stderr, exit with the supplied code.
# Used to pin down R2 (exit code semantics).
# Invoked as: exit-n.sh <code> [stderr-message]
set -euo pipefail
N="${1:-0}"
MSG="${2:-claude-context-m0 probe stderr line}"
cat > /dev/null
echo "$MSG" >&2
exit "$N"
```

- [ ] **Step 2: Make executable and smoke-test**

```bash
chmod +x scripts/m0-probe/exit-n.sh

# Test exit 0 with stderr capture
echo "test" | scripts/m0-probe/exit-n.sh 0 "marker-zero" 2>/tmp/m0-test-stderr
rc=$?
echo "exit=$rc stderr=$(cat /tmp/m0-test-stderr)"
# Expected: exit=0 stderr=marker-zero

# Test exit 2
echo "test" | scripts/m0-probe/exit-n.sh 2 "marker-two" 2>/tmp/m0-test-stderr
rc=$?
echo "exit=$rc stderr=$(cat /tmp/m0-test-stderr)"
# Expected: exit=2 stderr=marker-two

rm -f /tmp/m0-test-stderr
```

- [ ] **Step 3: Commit**

```bash
git add scripts/m0-probe/exit-n.sh
git commit -m "feat(m0): add exit-n probe for testing hook exit code semantics"
```

---

### Task 3: sleep-n.sh probe

**Files:**
- Create: `scripts/m0-probe/sleep-n.sh`

- [ ] **Step 1: Write sleep-n.sh**

```bash
#!/usr/bin/env bash
# Probe: sleep for N milliseconds before exiting 0.
# Used to find the user-perceived latency cliff (R3).
# Invoked as: sleep-n.sh <milliseconds>
set -euo pipefail
N_MS="${1:-100}"
cat > /dev/null
# Cross-platform millisecond sleep via awk (works with macOS bash 3.2 + Linux).
sleep "$(awk "BEGIN { print $N_MS / 1000 }")"
exit 0
```

- [ ] **Step 2: Make executable and smoke-test**

```bash
chmod +x scripts/m0-probe/sleep-n.sh

# Test sleep 200ms — should take ~0.2s
start=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1e9))')
echo "test" | scripts/m0-probe/sleep-n.sh 200
end=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1e9))')
elapsed_ms=$(( (end - start) / 1000000 ))
echo "elapsed=${elapsed_ms}ms (expected ~200)"
# Expected: elapsed_ms in range [180, 350]
```

- [ ] **Step 3: Commit**

```bash
git add scripts/m0-probe/sleep-n.sh
git commit -m "feat(m0): add sleep-n probe for testing hook latency tolerance"
```

---

### Task 4: swap-hook.sh helper

**Files:**
- Create: `scripts/m0-probe/swap-hook.sh`

- [ ] **Step 1: Write swap-hook.sh**

```bash
#!/usr/bin/env bash
# Helper: rewrite sandbox settings.local.json so all 5 hooks point at a different probe.
# Used by Phase 2 (R2 exit codes) and Phase 3 (R3 latency) of the runbook.
# Invoked as: swap-hook.sh <probe-script-name> [arg1] [arg2] ...
#   e.g. swap-hook.sh exit-n.sh 2
#   e.g. swap-hook.sh sleep-n.sh 500
set -euo pipefail

PROBE_NAME="${1:?usage: swap-hook.sh <probe-name> [args...]}"
shift
PROBE_ARGS="$*"

SANDBOX="${SANDBOX:-/tmp/claude-context-m0-sandbox}"
SETTINGS="$SANDBOX/.claude/settings.local.json"
PROBE_DIR="$(cd "$(dirname "$0")" && pwd)"
PROBE_PATH="$PROBE_DIR/$PROBE_NAME"

if [ ! -f "$PROBE_PATH" ]; then
    echo "✗ probe not found: $PROBE_PATH" >&2
    exit 1
fi
if [ ! -f "$SETTINGS" ]; then
    echo "✗ settings not found (run setup-sandbox.sh first): $SETTINGS" >&2
    exit 1
fi

# Replace any "command": "...echo.sh ..." or other probe with the new probe + args.
# The original setup-sandbox.sh writes commands like:
#   "command": "/abs/path/echo.sh pre-read"
# We replace EVERYTHING after the probe directory prefix:
NEW_CMD="$PROBE_PATH $PROBE_ARGS"

# Use a sed character not appearing in the path (prefer | over /)
sed -i.bak -E "s|\"command\": \"$PROBE_DIR/[^\"]+\"|\"command\": \"$NEW_CMD\"|g" "$SETTINGS"
rm -f "$SETTINGS.bak"

echo "✓ swapped all hooks to: $NEW_CMD"
echo "  settings: $SETTINGS"
```

- [ ] **Step 2: Make executable**

```bash
chmod +x scripts/m0-probe/swap-hook.sh
```

(Smoke test deferred to Task 5 — swap-hook.sh requires a sandbox to operate on, which setup-sandbox.sh creates.)

- [ ] **Step 3: Commit**

```bash
git add scripts/m0-probe/swap-hook.sh
git commit -m "feat(m0): add swap-hook helper for rewriting probe registrations"
```

---

### Task 5: setup-sandbox.sh

**Files:**
- Create: `scripts/m0-probe/setup-sandbox.sh`

- [ ] **Step 1: Write setup-sandbox.sh**

```bash
#!/usr/bin/env bash
# Builds /tmp/claude-context-m0-sandbox/ for M0 probing.
# Creates:
#   - 4 realistic source files (Go + Python + README + package.json)
#   - .claude/settings.local.json with all 5 hooks → echo.sh, including _managed_by field for R4
#   - CLAUDE.md with 3 @import lines (one per path form for R5)
#   - 3 R5 test files (one in $HOME with M0 prefix for safe cleanup, two in sandbox)
#   - dump dirs at /tmp/claude-context-m0/{pre-read,pre-write,post-write,session-start,stop}/
#   - baseline hash of ~/.claude/settings.json for cleanup verification
set -euo pipefail

SANDBOX="/tmp/claude-context-m0-sandbox"
DUMP_BASE="/tmp/claude-context-m0"
PROBE_DIR="$(cd "$(dirname "$0")" && pwd)"
TILDE_FILE="$HOME/claude-context-m0-tilde-test.md"
USER_SETTINGS="$HOME/.claude/settings.json"
BASELINE="/tmp/claude-context-m0-baseline-hash"

# Wipe any prior state
rm -rf "$SANDBOX" "$DUMP_BASE"
rm -f "$HOME"/claude-context-m0-* "$BASELINE"

# Create sandbox + dump dirs
mkdir -p "$SANDBOX/.claude"
mkdir -p "$DUMP_BASE"/{pre-read,pre-write,post-write,session-start,stop}

# 4 realistic files
cat > "$SANDBOX/auth.go" <<'EOF'
// Package main provides authentication helpers.
package main

// Authenticate validates a user credential.
func Authenticate(user, pass string) bool {
    return user != "" && pass != ""
}
EOF

cat > "$SANDBOX/utils.py" <<'EOF'
"""Utility functions for data processing."""

def normalize(text):
    """Normalize whitespace in text."""
    return " ".join(text.split())
EOF

cat > "$SANDBOX/README.md" <<'EOF'
# Sandbox Project
This is an M0 probe sandbox for claude-context. See docs/m0-runbook.md for instructions.
EOF

cat > "$SANDBOX/package.json" <<'EOF'
{
  "name": "m0-sandbox",
  "version": "1.0.0",
  "description": "M0 probe sandbox"
}
EOF

# R5 test files (3 forms of @import)
ABS_FILE="$SANDBOX/.claude/m0-test-abs.md"
REL_TARGET="$SANDBOX/.claude/m0-test-rel.md"

echo "passphrase-tilde-abc123" > "$TILDE_FILE"
echo "passphrase-abs-def456" > "$ABS_FILE"
echo "passphrase-rel-ghi789" > "$REL_TARGET"

# CLAUDE.md with 3 import forms
cat > "$SANDBOX/CLAUDE.md" <<EOF
# Sandbox Project Instructions

@~/claude-context-m0-tilde-test.md
@$ABS_FILE
@./.claude/m0-test-rel.md

These imports test R5 (path form support).
EOF

# settings.local.json: all 5 hooks → echo.sh, with _managed_by + _version for R4
cat > "$SANDBOX/.claude/settings.local.json" <<EOF
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Read",
        "hooks": [{
          "type": "command",
          "command": "$PROBE_DIR/echo.sh pre-read",
          "_managed_by": "claude-context",
          "_version": 1
        }]
      },
      {
        "matcher": "Write|Edit",
        "hooks": [{
          "type": "command",
          "command": "$PROBE_DIR/echo.sh pre-write",
          "_managed_by": "claude-context",
          "_version": 1
        }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [{
          "type": "command",
          "command": "$PROBE_DIR/echo.sh post-write",
          "_managed_by": "claude-context",
          "_version": 1
        }]
      }
    ],
    "SessionStart": [{
      "hooks": [{
        "type": "command",
        "command": "$PROBE_DIR/echo.sh session-start",
        "_managed_by": "claude-context",
        "_version": 1
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "$PROBE_DIR/echo.sh stop",
        "_managed_by": "claude-context",
        "_version": 1
      }]
    }]
  }
}
EOF

# Capture baseline hash of user's settings.json (for cleanup verification)
if [ -f "$USER_SETTINGS" ]; then
    shasum -a 256 "$USER_SETTINGS" > "$BASELINE"
else
    echo "no-user-settings" > "$BASELINE"
fi

cat <<EOF

✓ Sandbox at $SANDBOX
✓ Hooks registered in $SANDBOX/.claude/settings.local.json (5 events → echo.sh)
✓ R4: _managed_by + _version fields injected on every hook entry
✓ R5 test files: $TILDE_FILE, $ABS_FILE, $REL_TARGET
✓ Dump dirs at $DUMP_BASE/{pre-read,pre-write,post-write,session-start,stop}
✓ Baseline hash of $USER_SETTINGS captured to $BASELINE

NEXT STEPS:
  1. Open a NEW terminal.
  2. cd $SANDBOX
  3. Run 'claude code' (this fires SessionStart).
  4. Follow Phase 1 of docs/m0-runbook.md (pasted prompts to trigger Read/Write/Edit).
  5. /exit when done (this fires Stop).
  6. Report dump dir contents back via ls $DUMP_BASE/
EOF
```

- [ ] **Step 2: Make executable and smoke-test**

```bash
chmod +x scripts/m0-probe/setup-sandbox.sh

# Run setup
bash scripts/m0-probe/setup-sandbox.sh

# Verify expected structure
test -d /tmp/claude-context-m0-sandbox || { echo "✗ sandbox missing"; exit 1; }
test -f /tmp/claude-context-m0-sandbox/auth.go || { echo "✗ auth.go missing"; exit 1; }
test -f /tmp/claude-context-m0-sandbox/.claude/settings.local.json || { echo "✗ settings missing"; exit 1; }
test -f /tmp/claude-context-m0-sandbox/CLAUDE.md || { echo "✗ CLAUDE.md missing"; exit 1; }
test -f "$HOME/claude-context-m0-tilde-test.md" || { echo "✗ tilde test file missing"; exit 1; }
test -d /tmp/claude-context-m0/pre-read || { echo "✗ dump dir missing"; exit 1; }
test -f /tmp/claude-context-m0-baseline-hash || { echo "✗ baseline hash missing"; exit 1; }
echo "✓ setup-sandbox.sh produced expected structure"

# Verify _managed_by field present (R4 setup)
grep -q '"_managed_by": "claude-context"' /tmp/claude-context-m0-sandbox/.claude/settings.local.json \
    || { echo "✗ _managed_by field missing"; exit 1; }
echo "✓ _managed_by field present"

# Now smoke-test swap-hook.sh (Task 4 deferred this)
bash scripts/m0-probe/swap-hook.sh exit-n.sh 2 "test-marker"
grep -q "exit-n.sh 2" /tmp/claude-context-m0-sandbox/.claude/settings.local.json \
    || { echo "✗ swap-hook didn't update settings"; exit 1; }
echo "✓ swap-hook.sh works on real settings file"
```

- [ ] **Step 3: Manually inspect generated settings.local.json**

```bash
cat /tmp/claude-context-m0-sandbox/.claude/settings.local.json
```

Verify:
- 5 hook event sections present (PreToolUse:Read, PreToolUse:Write|Edit, PostToolUse:Write|Edit, SessionStart, Stop)
- Each hook has `_managed_by: "claude-context"` and `_version: 1`
- All commands point to absolute path of echo.sh under the project's `scripts/m0-probe/`
- Note: setup must be re-run after Task 5 commit since smoke test left exit-n swapped in

- [ ] **Step 4: Re-run setup to restore echo.sh registrations and commit**

```bash
bash scripts/m0-probe/setup-sandbox.sh   # restores echo.sh in all hooks
git add scripts/m0-probe/setup-sandbox.sh
git commit -m "feat(m0): add setup-sandbox script for building probe sandbox"
```

---

### Task 6: cleanup.sh

**Files:**
- Create: `scripts/m0-probe/cleanup.sh`

- [ ] **Step 1: Write cleanup.sh**

```bash
#!/usr/bin/env bash
# Removes M0 sandbox + dump dir + R5 leftovers from $HOME.
# Self-checks:
#   - ~/.claude/settings.json hash matches baseline (must be unchanged)
#   - no stray claude-context-m0 references in ~/.claude/
#   - no leftover $HOME/claude-context-m0-* files
# Exit code 1 with diagnostics if any check fails.
set -euo pipefail

SANDBOX="/tmp/claude-context-m0-sandbox"
DUMP_BASE="/tmp/claude-context-m0"
USER_SETTINGS="$HOME/.claude/settings.json"
BASELINE="/tmp/claude-context-m0-baseline-hash"

problems=0

# Remove sandbox
if [ -d "$SANDBOX" ]; then
    rm -rf "$SANDBOX"
    echo "✓ removed $SANDBOX"
else
    echo "  $SANDBOX already absent"
fi

# Remove dump dir
if [ -d "$DUMP_BASE" ]; then
    rm -rf "$DUMP_BASE"
    echo "✓ removed $DUMP_BASE"
else
    echo "  $DUMP_BASE already absent"
fi

# Remove R5 tilde-test files (in $HOME, M0-prefixed)
HOME_LEFTOVERS=$(ls "$HOME"/claude-context-m0-* 2>/dev/null || true)
if [ -n "$HOME_LEFTOVERS" ]; then
    rm -f "$HOME"/claude-context-m0-*
    echo "✓ removed \$HOME/claude-context-m0-* leftovers"
else
    echo "  no \$HOME/claude-context-m0-* leftovers"
fi

# Verify user settings.json unchanged
if [ -f "$BASELINE" ]; then
    if [ -f "$USER_SETTINGS" ]; then
        CURRENT_HASH=$(shasum -a 256 "$USER_SETTINGS" | awk '{print $1}')
        BASELINE_HASH=$(awk '{print $1}' "$BASELINE")
        if [ "$BASELINE_HASH" = "no-user-settings" ]; then
            echo "✗ $USER_SETTINGS appeared during M0 (was absent at setup)"
            problems=$((problems + 1))
        elif [ "$CURRENT_HASH" = "$BASELINE_HASH" ]; then
            echo "✓ $USER_SETTINGS unchanged from baseline"
        else
            echo "✗ $USER_SETTINGS HAS CHANGED — investigate before discarding baseline"
            echo "  baseline: $BASELINE_HASH"
            echo "  current:  $CURRENT_HASH"
            problems=$((problems + 1))
        fi
    else
        BASELINE_HASH=$(awk '{print $1}' "$BASELINE")
        if [ "$BASELINE_HASH" = "no-user-settings" ]; then
            echo "✓ $USER_SETTINGS still absent (matches baseline)"
        else
            echo "✗ $USER_SETTINGS DISAPPEARED during M0"
            problems=$((problems + 1))
        fi
    fi
    rm -f "$BASELINE"
else
    echo "  no baseline hash to verify against (setup may not have run)"
fi

# Check for stray claude-context-m0 references in ~/.claude/
if [ -d "$HOME/.claude" ]; then
    STRAY=$(grep -r "claude-context-m0" "$HOME/.claude/" 2>/dev/null || true)
    if [ -n "$STRAY" ]; then
        echo "✗ stray claude-context-m0 references in ~/.claude/:"
        echo "$STRAY"
        problems=$((problems + 1))
    else
        echo "✓ no stray claude-context-m0 references in ~/.claude/"
    fi
fi

if [ "$problems" -gt 0 ]; then
    echo
    echo "✗ $problems problem(s) detected — investigate manually."
    exit 1
fi

echo
echo "✓ All clean."
```

- [ ] **Step 2: Make executable and smoke-test**

```bash
chmod +x scripts/m0-probe/cleanup.sh

# Setup → cleanup → verify all gone
bash scripts/m0-probe/setup-sandbox.sh
bash scripts/m0-probe/cleanup.sh

# Should output "All clean." and exit 0
test ! -d /tmp/claude-context-m0-sandbox || { echo "✗ sandbox not removed"; exit 1; }
test ! -d /tmp/claude-context-m0 || { echo "✗ dump dir not removed"; exit 1; }
test ! -f "$HOME/claude-context-m0-tilde-test.md" || { echo "✗ tilde file not removed"; exit 1; }
echo "✓ cleanup.sh works correctly"
```

- [ ] **Step 3: Commit**

```bash
git add scripts/m0-probe/cleanup.sh
git commit -m "feat(m0): add cleanup script with self-checks for M0 environment"
```

---

### Task 7: scripts/m0-probe/README.md

**Files:**
- Create: `scripts/m0-probe/README.md`

- [ ] **Step 1: Write README.md**

```markdown
# M0 Probe Scripts

These bash scripts validate Claude Code's hook protocol assumptions baked into the architecture spec at `docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md`. Designed to be run by a human against a real Claude Code session.

**This directory will be renamed to `scripts/hook-protocol-probe/` after M0 completes (see Phase 4 of the M0 plan).**

## Scripts

| Script | Purpose | Used by |
|---|---|---|
| `echo.sh <event>` | Dump stdin to `/tmp/claude-context-m0/<event>/<ts>-stdin.json`, exit 0 | R1 (schema capture) |
| `exit-n.sh <code> [msg]` | Discard stdin, write msg to stderr, exit with code | R2 (exit code semantics) |
| `sleep-n.sh <ms>` | Discard stdin, sleep N ms, exit 0 | R3 (latency tolerance) |
| `swap-hook.sh <probe> [args]` | Rewrite sandbox `settings.local.json` to register a different probe | Phase 2 + 3 |
| `setup-sandbox.sh` | Build `/tmp/claude-context-m0-sandbox/` + register hooks + capture baseline | Phase 0 (setup) |
| `cleanup.sh` | Remove sandbox + dump dir + R5 leftovers + verify env clean | Phase 4 (cleanup) |

## Usage

Run the runbook at `docs/m0-runbook.md`. It walks through 4 phases:

1. Setup (`setup-sandbox.sh`)
2. Capture (R1 + R4 + R5: real Claude Code session in sandbox)
3. Probe variations (R2 exit codes, R3 latency: `swap-hook.sh` + new sessions)
4. Cleanup (`cleanup.sh`)

## Requirements

- macOS or Linux with bash 3.2+
- POSIX-compatible `sed`, `awk`, `shasum`, `grep`
- A real installed Claude Code CLI (not headless)

## Troubleshooting

- **No payloads in dump dir after Phase 1:** likely Claude Code doesn't honour hooks at the `settings.local.json` level. Try moving the config to `<sandbox>/.claude/settings.json` (drop `.local`).
- **`cleanup.sh` reports `~/.claude/settings.json` modified:** investigate manually before discarding the baseline. M0 should never touch user-level settings.
```

- [ ] **Step 2: Verify content renders correctly**

```bash
cat scripts/m0-probe/README.md
```

Verify all 6 scripts are listed in the table.

- [ ] **Step 3: Commit**

```bash
git add scripts/m0-probe/README.md
git commit -m "docs(m0): add README for probe scripts"
```

---

### Task 8: docs/m0-runbook.md

**Files:**
- Create: `docs/m0-runbook.md`

- [ ] **Step 1: Write the runbook**

```markdown
# M0 Hook Protocol Validation — Runbook

Step-by-step guide for executing the M0 spike against a real Claude Code session. Total user time: 30-60 minutes of focused attention. Do not multitask during R3 (latency grading).

**Before starting:**
- Close any running Claude Code sessions on this machine (avoids interference).
- Clear notification noise (you'll be subjectively grading interactivity).
- Have this file open in one window and a fresh terminal in another.

---

## Phase 0 — Setup (1 minute)

```bash
cd <go-claude-context project root>
bash scripts/m0-probe/setup-sandbox.sh
```

**Expected output:** `✓ Sandbox at /tmp/claude-context-m0-sandbox/` and 5 other ✓ lines, ending with `NEXT STEPS:`.

If anything is missing, stop and investigate.

---

## Phase 1 — R1 + R4 + R5 joint capture (10-15 minutes)

This phase captures stdin payloads (R1), tests `_managed_by` field tolerance (R4), and tests `@import` path forms (R5) — all in a single Claude Code session.

```bash
# In a NEW terminal:
cd /tmp/claude-context-m0-sandbox/
claude code
```

Starting Claude Code triggers `SessionStart`. After Claude is ready, **type these prompts one at a time**, waiting for completion before the next:

| # | Prompt | Triggers | Expected dump location |
|---|---|---|---|
| 1 | `read README.md` | PreToolUse:Read | `/tmp/claude-context-m0/pre-read/` |
| 2 | `read auth.go` | PreToolUse:Read (2nd sample) | same |
| 3 | `create a file hello.txt with content 'hi'` | PreToolUse:Write + PostToolUse:Write | `pre-write/` + `post-write/` |
| 4 | `edit hello.txt to say "hello world"` | PreToolUse:Edit + PostToolUse:Edit | same |
| 5 | `read README.md again` | PreToolUse:Read (3rd sample, same file) | `pre-read/` |
| 6 | `do you see any of these passphrases? passphrase-tilde-abc123, passphrase-abs-def456, passphrase-rel-ghi789` | (R5 test, no hook needed) | — |
| 7 | `/exit` | Stop | `stop/` |

**For prompt #6 (R5):** record which passphrases Claude says it sees. The mapping:
- `passphrase-tilde-abc123` ← imported via `@~/claude-context-m0-tilde-test.md`
- `passphrase-abs-def456` ← imported via `@/abs/path/...m0-test-abs.md`
- `passphrase-rel-ghi789` ← imported via `@./.claude/m0-test-rel.md`

Whichever Claude confirms = which `@import` path forms work.

**For R4:** if Claude Code started without complaining about the `_managed_by` field, AND step 1 produced a file under `pre-read/`, R4 passes.

After `/exit`, list the dumps:

```bash
# Back in your original terminal:
ls -la /tmp/claude-context-m0/
ls -la /tmp/claude-context-m0/pre-read/
ls -la /tmp/claude-context-m0/pre-write/
ls -la /tmp/claude-context-m0/post-write/
ls -la /tmp/claude-context-m0/session-start/
ls -la /tmp/claude-context-m0/stop/
```

**Paste the output of all 6 `ls` commands back to the conversation, plus the R5 passphrase findings, plus any R4 anomalies (Claude Code error messages, hook fire failures).**

---

## Phase 2 — R2 (exit code semantics) (10 minutes)

For each exit code in {0, 1, 2, 127}, swap all hooks to `exit-n.sh <code> "marker-<code>"` and run a brief Claude Code session.

```bash
# In your original terminal (sandbox terminal can stay closed):
bash scripts/m0-probe/swap-hook.sh exit-n.sh 0 "marker-zero"
```

```bash
# In a NEW Claude Code session:
cd /tmp/claude-context-m0-sandbox/
claude code
> read README.md
> /exit
```

**Observe and record:**
1. Did Claude proceed with the Read normally? (yes / no / blocked)
2. Did `marker-zero` appear in your terminal stderr? (yes / no)
3. Did Claude react to `marker-zero` in its next response? (yes / no — look at transcript)
4. Any Claude Code error messages?

**Repeat for codes 1, 2, 127** — each time:
```bash
bash scripts/m0-probe/swap-hook.sh exit-n.sh <CODE> "marker-<CODE>"
# new claude code session, same prompts, same observations
```

**Pay special attention to exit 2** — the architecture spec assumes it might block + feed stderr to Claude. We need definitive evidence.

**Paste a 4-row report (one per exit code) back to the conversation.**

---

## Phase 3 — R3 (latency tolerance) (10 minutes)

For each latency in {50, 100, 200, 500, 1000, 3000} ms, swap to `sleep-n.sh <ms>` and start a session. Read a file. Subjectively grade the interaction.

```bash
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 50
# new Claude Code session, prompt: 'read README.md', grade
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 100
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 200
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 500
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 1000
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 3000
# repeat — watch for Claude Code timing out
```

**Grading scale (1-5):**
- 5 = no perceptible delay
- 4 = slight delay, fine
- 3 = noticeable but tolerable
- 2 = annoying, would not want this every Read
- 1 = broken / unusable

**Record per latency:**
- Subjective grade
- Whether Claude Code appeared to time out (showed an error, killed the hook, or proceeded without waiting)

**Paste 6 rows (one per latency) back to the conversation.**

---

## Phase 4 — Cleanup (1 minute)

```bash
bash scripts/m0-probe/cleanup.sh
```

**Expected output:** all ✓ marks ending with `✓ All clean.`

If you see any ✗, **paste the cleanup output back** before considering M0 done. We may need to investigate manually.

---

## After Cleanup

The conversation will continue with assistant analysis of your captured data, the M0 report, fixture selection, and any required architecture spec patches. You don't need to do anything else.
```

- [ ] **Step 2: Verify the runbook is self-contained**

```bash
cat docs/m0-runbook.md | grep -E "^(##|###|\|)" | head -40
```

Should show all 5 phases (Phase 0, 1, 2, 3, 4) and tables.

- [ ] **Step 3: Commit**

```bash
git add docs/m0-runbook.md
git commit -m "docs(m0): add user-facing runbook for hook protocol validation"
```

---

## Phase 2 — Human-in-the-Loop Execution

### Task 9: User executes the runbook

This task is unusual: it requires the user to execute the runbook in a separate Claude Code session. Plan executor cannot proceed past this task automatically — must wait for user-provided data.

- [ ] **Step 1: Notify user that Phase 1 (tooling) is complete and prompt them to run the runbook**

Message to user (or display in plan executor output):

> **Phase 1 of M0 is complete — all probe scripts and the runbook are in place.**
>
> To proceed, please follow `docs/m0-runbook.md`:
> - Phase 0 (setup, 1 min)
> - Phase 1 (R1 + R4 + R5 capture, 10-15 min)
> - Phase 2 (R2 exit codes, 10 min)
> - Phase 3 (R3 latency, 10 min)
> - Phase 4 (cleanup, 1 min)
>
> Total: 30-60 min of focused attention. Then paste back:
> 1. The 6 `ls` outputs from Phase 1
> 2. The R5 passphrase recall results
> 3. Any R4 anomalies
> 4. The 4-row R2 exit code report
> 5. The 6-row R3 latency report
> 6. The cleanup.sh output (especially if non-clean)

- [ ] **Step 2: Wait for user data**

Plan executor MUST stop here and wait for user input. Do not invent data. Do not proceed to Task 10 until user has explicitly provided observations from all 4 runbook phases.

- [ ] **Step 3: Verify data is sufficient**

When user provides data, verify:
- Phase 1 dump paths are listed (≥1 file per event in pre-read/, pre-write/, post-write/, session-start/, stop/)
- R4 outcome stated (hook fired despite `_managed_by`? yes/no)
- R5 outcome stated (which of 3 passphrases Claude saw)
- 4 exit code observations
- 6 latency grades + timeout notes
- Cleanup output

If any are missing, ask user to complete that part before proceeding. Do not infer.

(No commit for this task — it's a coordination checkpoint, not a code change.)

---

## Phase 3 — Analysis & Report

### Task 10: Analyze R1 (stdin schema)

**Files:**
- Read: `/tmp/claude-context-m0/{pre-read,pre-write,post-write,session-start,stop}/*-stdin.json` (paths user provided)

**NOTE: cleanup.sh deletes `/tmp/claude-context-m0/`. If user already ran cleanup, ask them to re-run setup + Phase 1 only and skip cleanup until analysis is complete.**

- [ ] **Step 1: Read all dumped payloads from Phase 1**

For each of the 5 event dirs, use `Read` tool to load every dump file. Record observations in a working note (this conversation).

- [ ] **Step 2: For each event, list observed top-level fields and their types**

Example output format (per event):
```
pre-read:
  3 samples observed
  Top-level fields (always present): session_id (string), transcript_path (string), tool_name (string), tool_input (object)
  Top-level fields (sometimes present): cwd (string)
  tool_input shape: {file_path: string, limit?: int, offset?: int}
  tool_response: never present (correct — it's PreToolUse)
```

- [ ] **Step 3: Diff against architecture spec §6**

Open `docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md`, locate §6 Hook Protocol input schema. List every field name / type difference.

- [ ] **Step 4: Decide patch action**

For each diff, classify:
- "no patch" — assumption was right or close enough that wording is fine
- "patch §6: rename X → Y / add field Z / remove field W"

Save the analysis as a draft section for the report (in a scratch buffer / current conversation).

(No commit — analysis is intermediate; will be committed as part of Task 16 report.)

---

### Task 11: Analyze R2 (exit code semantics)

- [ ] **Step 1: Take user's 4-row exit code report**

Format observations into a definitive table:
```
exit 0:   blocked? no   stderr-to-terminal? yes   stderr-to-Claude? no/yes
exit 1:   blocked? ...
exit 2:   blocked? ...   stderr-to-Claude? ...   ← KEY DECISION POINT
exit 127: blocked? ...
```

- [ ] **Step 2: Decide the verdict for our output contract**

Based on observations, decide which exit code(s) achieve our intent ("informational stderr, never block"):
- If exit 0 with stderr → Claude transcript: use exit 0 (matches arch spec §6 assumption).
- If exit 0 with stderr → terminal only: stderr is wasted on user, redesign feedback channel.
- If only exit 2 puts stderr → Claude: use exit 2 + accept "blocked" semantics or find workaround.
- If exit 2 hard-blocks even with stderr propagation: exit 2 is forbidden in our hook handlers.

Write a 1-2 sentence verdict for the report.

- [ ] **Step 3: Decide patch action for §6**

Note any §6 wording that needs to change (e.g., "exit ≥1 only used when hook crashed" → adjust if exit 1 has unexpected behavior).

(No commit.)

---

### Task 12: Analyze R3 (latency tolerance)

- [ ] **Step 1: Take user's 6-row latency report**

Format:
```
50ms:   grade=5  timeout=no
100ms:  grade=...
200ms:  grade=...
500ms:  grade=...   ← arch spec assumed 200ms preread budget; check if user is OK at 200, hostile at 500
1000ms: grade=...
3000ms: grade=...   timeout=?  (Claude Code may enforce its own cap)
```

- [ ] **Step 2: Identify the cliff**

The "cliff" is the latency where grade drops from ≥3 to ≤2. This is the upper bound for our soft-timeout budgets.

- [ ] **Step 3: Compare against arch spec §5/§6 budgets**

Architecture spec §5 has:
- preread total: 80ms p99
- prewrite/postwrite total: 200ms
- stop: 500ms

If user grades indicate:
- preread cliff at 100ms → tighten preread budget to ≤80ms (consistent)
- preread cliff at 500ms → can relax preread to 200ms (less aggressive)
- preread cliff at 50ms → MUST tighten to <50ms (very aggressive)

Decide patch action: keep / tighten / relax.

- [ ] **Step 4: Note any timeout from Claude Code itself**

If Claude Code timed out the 3000ms hook: record the timeout cap (e.g., "Claude Code kills hooks after ~2s"). This becomes a hard constraint for arch spec §5/§6.

(No commit.)

---

### Task 13: Analyze R4 (settings.json _managed_by tolerance)

- [ ] **Step 1: Take user's R4 outcome**

The two signals:
1. Did Claude Code start without complaining about `_managed_by` / `_version` extra fields?
2. Did the echo.sh hook fire despite the extra fields? (i.e., dump files appeared in Phase 1)

- [ ] **Step 2: Decide verdict**

- Both yes → R4 PASSES; arch spec §8 boundary scheme stands as designed.
- Started but hooks didn't fire → schema validator silently dropped the entries; need different boundary marker.
- Failed to start → boundary scheme incompatible; need alternative (e.g., separate metadata file alongside settings.json that init reads/writes).

- [ ] **Step 3: Decide patch action**

If R4 fails, write a §8 patch proposal: alternative boundary scheme (e.g., a separate `~/.claude/settings.json.claude-context-managed` JSON file listing which entries we own).

(No commit.)

---

### Task 14: Analyze R5 (@import path forms)

- [ ] **Step 1: Take user's R5 outcome**

Three boolean answers:
- Did Claude see `passphrase-tilde-abc123`? (yes/no → `@~/path` works)
- Did Claude see `passphrase-abs-def456`? (yes/no → `@/abs/path` works)
- Did Claude see `passphrase-rel-ghi789`? (yes/no → `@./rel/path` works)

- [ ] **Step 2: Decide verdict and constrain init's path generation**

Per the architecture spec §8, init writes a single `@~/.claude/claude-context-rules.md` line. If `@~/path` doesn't work, init must generate `@/abs/expanded/path/...` instead.

Write a 1-line verdict: "init uses path form X (Y form failed)".

- [ ] **Step 3: Decide patch action for §8**

If `@~/path` works (the assumed case): no patch.
If only abs path works: patch §8 to mandate absolute paths in `@import` injection.

(No commit.)

---

### Task 15: Select & sanitize canonical fixtures

**Files:**
- Read: `/tmp/claude-context-m0/{pre-read,pre-write,post-write,session-start,stop}/*-stdin.json`
- Create: `tests/fixtures/hook-payloads/pre-read.json`
- Create: `tests/fixtures/hook-payloads/pre-write.json`
- Create: `tests/fixtures/hook-payloads/post-write.json`
- Create: `tests/fixtures/hook-payloads/session-start.json`
- Create: `tests/fixtures/hook-payloads/stop.json`

- [ ] **Step 1: Create the fixtures directory**

```bash
mkdir -p tests/fixtures/hook-payloads
```

- [ ] **Step 2: For each event, pick the canonical sample**

Selection criteria (in order):
1. The sample with the most fields (covers more of the schema)
2. If multiple samples have same field count, pick the one with most-typical content (e.g., `read README.md` over `read auth.go` if README is more likely in real users' projects)
3. For pre-read: prefer the sample from a non-repeated read (cleaner — no priors)

Document the choice in a working note for the report.

- [ ] **Step 3: Sanitize user-specific paths**

For each chosen sample, before writing to fixture:
- Replace `/Users/ranwei/...` and `/home/<user>/...` with `<HOME>` placeholder
- Replace any session_id with `<SESSION_ID>` (uuid placeholder)
- Replace `transcript_path` with `<TRANSCRIPT_PATH>` placeholder
- Leave file_path and tool_input content as-is (they're public-fixture-safe since the sandbox files are sample content)

Use this Python helper (run with python3) for safe sanitization:

```bash
python3 <<'PYEOF'
import json, re, sys
from pathlib import Path

events = ["pre-read", "pre-write", "post-write", "session-start", "stop"]
home = str(Path.home())

for evt in events:
    src_dir = Path(f"/tmp/claude-context-m0/{evt}")
    if not src_dir.exists():
        print(f"⚠ no dump dir for {evt} — skipping (re-run Phase 1?)")
        continue
    samples = sorted(src_dir.glob("*-stdin.json"))
    if not samples:
        print(f"⚠ no samples for {evt}")
        continue
    # Pick the largest sample (most fields)
    chosen = max(samples, key=lambda p: p.stat().st_size)
    print(f"{evt}: chose {chosen.name} (size={chosen.stat().st_size})")
    raw = chosen.read_text()
    data = json.loads(raw)

    # Sanitize known-sensitive fields
    if "session_id" in data:
        data["session_id"] = "<SESSION_ID>"
    if "transcript_path" in data:
        data["transcript_path"] = "<TRANSCRIPT_PATH>"

    # Recursively replace home dir in all string values
    def scrub(o):
        if isinstance(o, dict):
            return {k: scrub(v) for k, v in o.items()}
        elif isinstance(o, list):
            return [scrub(v) for v in o]
        elif isinstance(o, str):
            return o.replace(home, "<HOME>")
        return o

    data = scrub(data)

    out = Path(f"tests/fixtures/hook-payloads/{evt}.json")
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(data, indent=2) + "\n")
    print(f"  → {out}")
PYEOF
```

- [ ] **Step 4: Verify each fixture parses as valid JSON**

```bash
for f in tests/fixtures/hook-payloads/*.json; do
    python3 -c "import json,sys; json.load(open('$f')); print('✓ $f')" || exit 1
done
```

- [ ] **Step 5: Verify no leaked user paths**

```bash
grep -r "$HOME" tests/fixtures/hook-payloads/ && { echo "✗ user path leaked"; exit 1; } || echo "✓ no leaked $HOME paths"
grep -r "ranwei" tests/fixtures/hook-payloads/ && { echo "✗ username leaked"; exit 1; } || echo "✓ no leaked username"
```

- [ ] **Step 6: Commit fixtures**

```bash
git add tests/fixtures/hook-payloads/
git commit -m "feat(m0): add canonical hook payload fixtures captured from real Claude Code"
```

---

### Task 16: Write M0 report

**Files:**
- Create: `docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md`

- [ ] **Step 1: Compose the report from analysis notes (Tasks 10-15)**

Use this exact structure:

```markdown
# M0 Hook Protocol Validation Report

**Date:** YYYY-MM-DD (today)
**Claude Code version:** (ask user to run `claude --version` if not already known)
**OS:** (ask user to run `uname -a` if not already known)
**Tester:** ranwei

---

## Summary

| Risk | Original assumption (arch spec ref) | Reality (evidence) | Action |
|---|---|---|---|
| R1 | (paste from §6) | (one-line summary) | "no patch" / "patch §6: ..." |
| R2 | exit 2 = block + feed stderr (assumed) | (one-line summary) | (verdict) |
| R3 | preread 80ms p99 budget | grades: ... | (revised budget or "keep") |
| R4 | _managed_by tolerated | (yes/no with evidence) | (boundary scheme outcome) |
| R5 | all 3 import forms work | ✓~ ✓abs ✓rel (or whatever) | (init path-form constraint) |

---

## R1: stdin schema findings

(From Task 10 analysis. Per-event sub-section. Fields observed, types, optional/always, diff vs §6.)

## R2: exit code semantics findings

(From Task 11 analysis. 4-row table + 1-paragraph verdict.)

## R3: latency tolerance findings

(From Task 12 analysis. 6-row table, identified cliff, comparison vs §5/§6 budgets, Claude Code own timeout if observed.)

## R4: settings.json schema tolerance

(From Task 13 analysis. Started yes/no, hooks fired yes/no, verdict.)

## R5: @import path forms

(From Task 14 analysis. Three booleans + verdict + init constraint.)

## Architecture spec patches required

Bullet list. Each patch: file path, section, before/after wording, reason. Empty list = no patches needed.

## Fixture selection

5 sub-sections (one per event). For each: source capture file, why chosen as canonical, what was sanitized.
```

Fill every section using the analysis notes from Tasks 10-15.

- [ ] **Step 2: Self-review for placeholders**

Search the report for "TBD", "TODO", "(paste from", "(one-line summary)", "(verdict)" — any remaining placeholder template text means a section wasn't filled in.

```bash
grep -E "(TBD|TODO|\(paste|\(one-line|\(verdict|\(if|\(yes/no\)|\(or whatever\))" docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md
```

Expected: no matches. If matches found, fill them in.

- [ ] **Step 3: Commit report**

```bash
git add docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md
git commit -m "docs(m0): add validation report with R1-R5 findings"
```

---

### Task 17: Patch architecture spec (only if needed)

**Files:**
- Modify: `docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md` (only sections that contradict findings)

- [ ] **Step 1: Review the "Architecture spec patches required" section of the M0 report**

If empty, skip to Step 4 (no patches needed).

- [ ] **Step 2: Apply each patch with Edit tool**

For each entry in the patches list:
1. Use `Read` to locate the exact pre-patch wording
2. Use `Edit` to replace with post-patch wording
3. Verify the edit succeeded by re-reading the section

- [ ] **Step 3: Verify the architecture spec is internally consistent after patches**

```bash
# Quick consistency checks
grep -n "preread.*ms" docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md
grep -n "exit code" docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md
grep -n "_managed_by" docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md
grep -n "@import\|@~/" docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md
```

Manually verify the numbers / verdicts match the M0 report.

- [ ] **Step 4: Commit patches (if any)**

```bash
# Only if changes were made:
git diff --quiet docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md || {
    git add docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md
    git commit -m "docs(arch): patch hook architecture spec per M0 validation findings"
}
```

If nothing was patched, skip this commit (the report itself is enough).

---

## Phase 4 — Release-Form Rename

### Task 18: Rename scripts/m0-probe/ → scripts/hook-protocol-probe/

**Files:**
- Rename: `scripts/m0-probe/` → `scripts/hook-protocol-probe/`
- Modify: `scripts/hook-protocol-probe/README.md` (expand prominence of "not product code" notice)

- [ ] **Step 1: git mv the directory**

```bash
git mv scripts/m0-probe scripts/hook-protocol-probe
```

- [ ] **Step 2: Update the README to its permanent home form**

Use `Edit` to change `scripts/hook-protocol-probe/README.md`. Replace the first paragraph with:

```markdown
# Hook Protocol Probe Scripts

> **THIS IS NOT PRODUCT CODE.** These bash scripts validate Claude Code's hook protocol assumptions baked into `claude-context`. They were originally created during the M0 spike (see `docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-design.md`) and are kept here for **re-verification when Claude Code's hook protocol changes**.

To re-validate against a new Claude Code version: follow the runbook at `docs/m0-runbook.md` and diff your findings against the M0 report at `docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md`. Patch the architecture spec if the protocol has shifted.
```

- [ ] **Step 3: Verify nothing else references the old path**

```bash
grep -r "scripts/m0-probe" docs/ scripts/ tests/ pkg/ cmd/ 2>/dev/null && {
    echo "✗ stale references to scripts/m0-probe found — update them";
    exit 1
} || echo "✓ no stale references"
```

If any matches in `docs/m0-runbook.md` (the runbook still says `scripts/m0-probe/`), use Edit to update each occurrence to `scripts/hook-protocol-probe/`.

- [ ] **Step 4: Re-test the renamed scripts work**

```bash
bash scripts/hook-protocol-probe/setup-sandbox.sh
test -d /tmp/claude-context-m0-sandbox || { echo "✗ setup failed after rename"; exit 1; }
bash scripts/hook-protocol-probe/cleanup.sh
test ! -d /tmp/claude-context-m0-sandbox || { echo "✗ cleanup failed after rename"; exit 1; }
echo "✓ renamed scripts still work"
```

- [ ] **Step 5: Commit the rename + README update**

```bash
git add scripts/hook-protocol-probe/ docs/m0-runbook.md
git commit -m "refactor(m0): rename m0-probe to hook-protocol-probe for permanent re-use"
```

---

### Task 19: Final acceptance verification

- [ ] **Step 1: Verify all 5 acceptance criteria from the M0 spec**

```bash
# AC1: report exists with definitive answers
test -f docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md \
    || { echo "✗ AC1: report missing"; exit 1; }
grep -E "(TBD|TODO|needs more research)" docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md \
    && { echo "✗ AC1: report has placeholders"; exit 1; }
echo "✓ AC1: report exists, no placeholders"

# AC2: 5 fixtures exist and parse
for evt in pre-read pre-write post-write session-start stop; do
    f="tests/fixtures/hook-payloads/$evt.json"
    test -f "$f" || { echo "✗ AC2: $f missing"; exit 1; }
    python3 -c "import json; json.load(open('$f'))" || { echo "✗ AC2: $f invalid"; exit 1; }
done
echo "✓ AC2: all 5 fixtures present and valid"

# AC3: scripts/hook-protocol-probe/ exists with all 6 scripts + README
for s in echo.sh exit-n.sh sleep-n.sh swap-hook.sh setup-sandbox.sh cleanup.sh README.md; do
    test -f "scripts/hook-protocol-probe/$s" || { echo "✗ AC3: $s missing"; exit 1; }
done
echo "✓ AC3: all 7 files in scripts/hook-protocol-probe/"

# AC4: arch spec consistent with report (no obvious contradictions)
# This is a manual judgment call; we already enforced it in Task 17.
echo "✓ AC4: arch spec patches applied per Task 17 (verify manually)"

# AC5: cleanup self-checks
test ! -d /tmp/claude-context-m0-sandbox || { echo "✗ AC5: sandbox not cleaned"; exit 1; }
test ! -d /tmp/claude-context-m0 || { echo "✗ AC5: dump dir not cleaned"; exit 1; }
test -z "$(ls $HOME/claude-context-m0-* 2>/dev/null)" || { echo "✗ AC5: $HOME leftovers"; exit 1; }
echo "✓ AC5: environment fully cleaned"
```

- [ ] **Step 2: Print final summary**

```bash
echo
echo "=== M0 COMPLETE ==="
echo "Report:    docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md"
echo "Fixtures:  tests/fixtures/hook-payloads/"
echo "Probes:    scripts/hook-protocol-probe/"
echo
git log --oneline | head -10
```

- [ ] **Step 3: No commit needed for verification**

Verification doesn't change files. M0 is done.

---

## Self-Review Notes

**Spec coverage check:** Every section of the M0 spec maps to at least one task:
- Goals → Tasks 10-17 (analysis + report + patches)
- Deliverables Phase 1 → Tasks 1-8
- Deliverables Phase 2 → Tasks 10-17
- Deliverables Phase 3 → Task 18
- Probe scripts (echo/exit-n/sleep-n/swap-hook/setup/cleanup) → Tasks 1-6
- Runbook → Task 8
- Report format → Task 16
- Acceptance criteria → Task 19

**Placeholder scan:** All bash code, all paths, all commit messages are concrete. No "TBD" / "TODO". The R5 file at `~/claude-context-m0-tilde-test.md` and the dump dir at `/tmp/claude-context-m0/` are referenced consistently across tasks.

**Type / signature consistency:**
- `echo.sh` is invoked as `echo.sh <event>` everywhere
- `exit-n.sh` is invoked as `exit-n.sh <code> [msg]` everywhere
- `sleep-n.sh` is invoked as `sleep-n.sh <ms>` everywhere
- `swap-hook.sh` is invoked as `swap-hook.sh <probe-name> [args...]` everywhere
- Path constants `/tmp/claude-context-m0-sandbox/`, `/tmp/claude-context-m0/`, `$HOME/claude-context-m0-*` used consistently
