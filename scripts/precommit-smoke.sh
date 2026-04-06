#!/usr/bin/env bash
# Optional wrapper: same checks as CI test+gui via make precommit.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
os="$(uname -s 2>/dev/null || true)"
case "$os" in
MINGW* | MSYS* | CYGWIN*)
	if [[ "${FULL_GUI:-}" == 1 ]]; then
		make precommit
	else
		make precommit SKIP_GUI=1
	fi
	;;
*)
	make precommit
	;;
esac
echo "precommit OK"
