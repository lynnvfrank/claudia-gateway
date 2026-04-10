#!/usr/bin/env bash
# make desktop-run — ensure claudia-desktop exists, then exec with remaining args (e.g. desktop -qdrant-bin …).
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
bin="${1:?desktop-run.sh: missing binary name (e.g. claudia-desktop.exe)}"
make_cmd="${2:-make}"
shift 2 || true
cd "$root"
if [[ ! -f "$bin" ]]; then
  "$make_cmd" desktop-build
fi
exec "$root/$bin" "$@"
