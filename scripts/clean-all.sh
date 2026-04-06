#!/usr/bin/env bash
# Remove third-party bootstrap artifacts, run state, and logs (see Makefile clean-all).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

rm -f bin/bifrost-http bin/bifrost-http.exe bin/qdrant bin/qdrant.exe
rmdir bin 2>/dev/null || true

rm -rf .deps
rm -rf run logs

echo "clean-all: removed .deps/, bin/bifrost-http*, bin/qdrant*, run/, logs/"
