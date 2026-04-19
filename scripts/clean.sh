#!/usr/bin/env bash
# Remove local build artifacts only (see Makefile clean).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
rm -f claudia claudia.exe claudia-desktop claudia-desktop.exe claudia-index claudia-index.exe
rm -rf dist
echo "clean: removed claudia[.exe], claudia-desktop[.exe], claudia-index[.exe], dist/"
