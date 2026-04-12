#!/usr/bin/env bash
# Optional wrapper: make precommit (fmt-check, vet, test; desktop vet/test unless SKIP_DESKTOP=1).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
os="$(uname -s 2>/dev/null || true)"
case "$os" in
MINGW* | MSYS* | CYGWIN*)
	if [[ "${FULL_DESKTOP:-}" == 1 ]]; then
		make precommit
	else
		make precommit SKIP_DESKTOP=1
	fi
	;;
*)
	make precommit
	;;
esac
echo "precommit OK"
