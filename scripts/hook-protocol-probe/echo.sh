#!/usr/bin/env bash
# Probe: dump stdin verbatim to a timestamped file under $OUT_DIR.
# Used to capture real hook payloads (R1).
# Invoked as: echo.sh <event-name>   e.g. echo.sh pre-read
set -euo pipefail
EVENT="${1:-unknown}"
OUT_DIR="${OUT_DIR:-/tmp/mneme-m0/$EVENT}"
mkdir -p "$OUT_DIR"
ts=$(date +%s)-$$-$RANDOM
cat > "$OUT_DIR/${ts}-stdin.json"
exit 0
