#!/usr/bin/env bash
# Used by Makefile clean-all on Windows (POSIX test in the Makefile runs under cmd.exe and fails).
set -euo pipefail
if [[ "${1:-}" != "1" ]]; then
	echo "clean-all: removes .deps/, bin/bifrost-http*, bin/qdrant*, run/, logs/ — re-run with CONFIRM=1" >&2
	exit 1
fi
