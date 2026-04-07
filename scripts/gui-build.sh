#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
# shellcheck source=scripts/msys2-gcc-path.sh
source "$root/scripts/msys2-gcc-path.sh"
# make invokes non-interactive bash — ~/.bashrc is not loaded; put MSYS2 gcc on PATH like gui-install does.
msys2_prepend_gcc_path || true
bin="${1:?gui-build.sh: missing binary name (e.g. claudia-gui or claudia-gui.exe)}"
cd "$root/gui"
if ! CGO_ENABLED=1 go build -o "$root/$bin" .; then
  echo "" >&2
  echo "gui-build: Fyne needs CGO and platform development libraries." >&2
  echo "  Run:  make gui-install" >&2
  echo "  Doc:  docs/gui-testing.md" >&2
  exit 1
fi
echo "Built $root/$bin — run:  make gui-run   (or:  ./$bin)"
