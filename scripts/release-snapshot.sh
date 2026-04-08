#!/usr/bin/env bash
# GoReleaser snapshot build (Makefile release-snapshot). Run under Git Bash on Windows.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if ! command -v goreleaser >/dev/null 2>&1; then
	echo "release-snapshot: install https://goreleaser.com/install/ or run the docker one-liner in docs/packaging.md" >&2
	exit 1
fi
exec goreleaser release --snapshot --clean
