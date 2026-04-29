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
