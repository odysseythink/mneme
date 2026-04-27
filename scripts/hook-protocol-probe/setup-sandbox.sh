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
