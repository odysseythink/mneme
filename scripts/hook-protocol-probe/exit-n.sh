#!/usr/bin/env bash
# Probe: read stdin (discard), write a marker to stderr, exit with the supplied code.
# Used to pin down R2 (exit code semantics).
# Invoked as: exit-n.sh <code> [stderr-message]
set -euo pipefail
N="${1:-0}"
MSG="${2:-mneme-m0 probe stderr line}"
cat > /dev/null
echo "$MSG" >&2
exit "$N"
