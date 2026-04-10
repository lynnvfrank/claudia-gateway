#!/usr/bin/env bash
# Idempotent: create .env from example if absent. Does not create tokens.yaml
# (first-run bootstrap in the UI writes config/tokens.yaml; see docs/version-v0.1.md §5).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

if [[ ! -f .env ]]; then
	cp env.example .env
	echo "configure: created .env from env.example — edit keys (see README)."
else
	echo "configure: .env already exists — left unchanged."
fi

if [[ -f config/tokens.yaml ]]; then
	echo "configure: config/tokens.yaml exists — left unchanged."
else
	echo "configure: no config/tokens.yaml — run make claudia-serve or make up, then open /ui/setup (or copy config/tokens.example.yaml to config/tokens.yaml)."
fi

echo "configure: edit config/gateway.yaml and config/bifrost.config.json as needed (see README)."
