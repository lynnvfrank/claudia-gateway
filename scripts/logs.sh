#!/usr/bin/env bash
# make logs — tail logs/claudia.log. Usage: scripts/logs.sh [lines]
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
n="${1:-80}"
if [[ -f logs/claudia.log ]]; then
	tail -n "$n" logs/claudia.log
else
	echo "logs: no logs/claudia.log — run make claudia-start or make up first"
fi
