#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
bin="${1:?desktop-run.sh: missing binary name (e.g. claudia-desktop.exe)}"
make_cmd="${2:-make}"
shift 2 || true
cd "$root"
if [[ ! -f "$bin" ]]; then
  "$make_cmd" desktop-build
fi
exec "$root/$bin" desktop "$@"
