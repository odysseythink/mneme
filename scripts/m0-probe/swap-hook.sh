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
