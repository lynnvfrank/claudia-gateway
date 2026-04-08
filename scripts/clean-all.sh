#!/usr/bin/env bash
# Remove third-party bootstrap artifacts, packaging scratch dirs, optional package caches, run state, and logs (see Makefile clean-all).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

rm -rf bin
rm -rf packaging/qdrant-bundles
rm -rf packages
rm -rf node_modules

rm -rf .deps
rm -rf run logs

echo "clean-all: removed bin/, packaging/qdrant-bundles/, packages/, node_modules/, .deps/, run/, logs/"
