#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
# shellcheck source=scripts/msys2-gcc-path.sh
source "$root/scripts/msys2-gcc-path.sh"
msys2_prepend_gcc_path || true
bin="${1:?desktop-build.sh: missing output binary name (e.g. claudia-desktop or claudia-desktop.exe)}"
cd "$root"
export CGO_ENABLED=1
if ! go build -tags desktop -o "$root/$bin" ./cmd/claudia; then
  echo "" >&2
  echo "desktop-build: needs CGO and native WebView deps (WebKitGTK on Linux, WebView2 on Windows)." >&2
  echo "  Run:  make desktop-install" >&2
  exit 1
fi
echo "Built $root/$bin — run:  ./$bin (supervisor+UI) or ./$bin --headless   (same flags as claudia serve)"
