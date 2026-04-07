#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
bin="${1:?gui-run.sh: missing binary name}"
make_cmd="${2:-make}"
cd "$root"
if [[ ! -f "$bin" ]]; then
  "$make_cmd" gui-build
fi
exec "$root/$bin"
