#!/usr/bin/env bash
# GoReleaser snapshot build (Makefile release-snapshot). Run under Git Bash on Windows.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if command -v go >/dev/null 2>&1; then
	_gobin="${GOBIN:-}"
	if [[ -z "${_gobin//[[:space:]]/}" ]]; then
		_gobin="$(go env GOPATH)/bin"
	fi
	_gobin="${_gobin//\\//}"
	export PATH="${_gobin}:$PATH"
	hash -r 2>/dev/null || true
fi
if ! command -v goreleaser >/dev/null 2>&1; then
	echo "release-snapshot: run: make release-install   (or https://goreleaser.com/install/ / docs/packaging.md)" >&2
	exit 1
fi
exec goreleaser release --snapshot --clean
