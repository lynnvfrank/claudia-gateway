#!/usr/bin/env bash
# make claudia-start — run ./claudia serve in background; logs → logs/claudia.log, pid → run/claudia.pid
# Usage: scripts/claudia-start.sh [--stack]   (--stack adds -qdrant-bin when qdrant binary exists)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
mkdir -p logs run

STACK=0
[[ "${1:-}" == "--stack" ]] && STACK=1

if [[ -f run/claudia.pid ]]; then
	pid="$(cat run/claudia.pid)"
	if kill -0 "$pid" 2>/dev/null; then
		echo "claudia-start: already running (pid $pid). Stop with: make claudia-stop"
		exit 1
	fi
	rm -f run/claudia.pid
fi

if [[ -f claudia.exe ]]; then
	BIN=./claudia.exe
elif [[ -f claudia ]]; then
	BIN=./claudia
else
	echo "claudia-start: no ./claudia binary — run: make claudia-build" >&2
	exit 1
fi

BF=bin/bifrost-http
[[ -f bin/bifrost-http.exe ]] && BF=bin/bifrost-http.exe
if [[ ! -f "$BF" ]]; then
	echo "claudia-start: missing $BF — run: make install" >&2
	exit 1
fi

ARGS=(serve -bifrost-bin "$BF")
if [[ "$STACK" -eq 1 ]]; then
	QT=bin/qdrant
	[[ -f bin/qdrant.exe ]] && QT=bin/qdrant.exe
	if [[ -f "$QT" ]]; then
		ARGS+=(-qdrant-bin "$QT")
	else
		echo "claudia-start: --stack requested but no $QT — run: make install" >&2
		exit 1
	fi
fi

nohup "$BIN" "${ARGS[@]}" >>logs/claudia.log 2>&1 &
echo $! >run/claudia.pid
echo "claudia-start: pid $(cat run/claudia.pid)  log: logs/claudia.log"
echo "  Gateway   http://127.0.0.1:3000/health"
echo "  BiFrost   http://127.0.0.1:8080"
[[ "$STACK" -eq 1 ]] && echo "  Qdrant    http://127.0.0.1:6333"
