#!/usr/bin/env bash
# Idempotent: create .env and config/tokens.yaml from examples if absent.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

if [[ ! -f .env ]]; then
	cp env.example .env
	echo "configure: created .env from env.example — edit keys (see README)."
else
	echo "configure: .env already exists — left unchanged."
fi

if [[ ! -f config/tokens.yaml ]]; then
	cp config/tokens.example.yaml config/tokens.yaml
	echo "configure: created config/tokens.yaml from tokens.example.yaml — set gateway tokens."
else
	echo "configure: config/tokens.yaml already exists — left unchanged."
fi

echo "configure: edit config/gateway.yaml and config/bifrost.config.json as needed (see README)."
