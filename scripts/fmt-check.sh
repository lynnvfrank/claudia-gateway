#!/usr/bin/env bash
# Fail if gofmt would change cmd/, internal/, or gui/ (parity with CI Format steps).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
bad="$(gofmt -l cmd internal gui || true)"
if [[ -n "$bad" ]]; then
	echo 'gofmt: run "make fmt" to fix formatting in:' >&2
	echo "$bad" >&2
	exit 1
fi
