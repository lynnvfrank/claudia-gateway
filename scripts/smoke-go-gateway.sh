#!/usr/bin/env bash
# Go gateway CI smoke: gofmt, vet, and tests (routing, server integration, 429 fallback, models order).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
bad="$(gofmt -l cmd internal gui || true)"
if [[ -n "$bad" ]]; then
  echo "gofmt needed on:" >&2
  echo "$bad" >&2
  exit 1
fi
go vet ./...
go test ./... -race -count=1
echo "go gateway smoke OK"
