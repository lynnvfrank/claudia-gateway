#!/usr/bin/env bash
# Stop background supervisor started by scripts/claudia-start.sh (run/claudia.pid).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f run/claudia.pid ]]; then
	echo "claudia-stop: no run/claudia.pid — nothing to stop"
	exit 0
fi

pid="$(cat run/claudia.pid)"
if kill -0 "$pid" 2>/dev/null; then
	kill "$pid" 2>/dev/null || true
	echo "claudia-stop: sent SIGTERM to pid $pid (supervisor; child processes may exit with it)"
else
	echo "claudia-stop: stale pid file (process $pid not running)"
fi
rm -f run/claudia.pid
