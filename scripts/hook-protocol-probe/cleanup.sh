#!/usr/bin/env bash
# Removes M0 sandbox + dump dir + R5 leftovers from $HOME.
# Self-checks:
#   - ~/.claude/settings.json hash matches baseline (must be unchanged)
#   - no stray mneme-m0 references in ~/.claude/
#   - no leftover $HOME/mneme-m0-* files
# Exit code 1 with diagnostics if any check fails.
set -euo pipefail

SANDBOX="/tmp/mneme-m0-sandbox"
DUMP_BASE="/tmp/mneme-m0"
USER_SETTINGS="$HOME/.claude/settings.json"
BASELINE="/tmp/mneme-m0-baseline-hash"

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
HOME_LEFTOVERS=$(ls "$HOME"/mneme-m0-* 2>/dev/null || true)
if [ -n "$HOME_LEFTOVERS" ]; then
    rm -f "$HOME"/mneme-m0-*
    echo "✓ removed \$HOME/mneme-m0-* leftovers"
else
    echo "  no \$HOME/mneme-m0-* leftovers"
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

# Check for stray mneme-m0 references in ~/.claude/
if [ -d "$HOME/.claude" ]; then
    STRAY=$(grep -r "mneme-m0" "$HOME/.claude/" 2>/dev/null || true)
    if [ -n "$STRAY" ]; then
        echo "✗ stray mneme-m0 references in ~/.claude/:"
        echo "$STRAY"
        problems=$((problems + 1))
    else
        echo "✓ no stray mneme-m0 references in ~/.claude/"
    fi
fi

if [ "$problems" -gt 0 ]; then
    echo
    echo "✗ $problems problem(s) detected — investigate manually."
    exit 1
fi

echo
echo "✓ All clean."
