#!/usr/bin/env bash
# First successful chat completion (non-streaming) against a running Go gateway.
# Usage: e2e-first-chat-curl.sh [BASE_URL] TOKEN MODEL
# Example:
#   ./scripts/e2e-first-chat-curl.sh http://127.0.0.1:3000 sk-local Claudia-0.1.0
set -euo pipefail

BASE_URL="${1:-http://127.0.0.1:3000}"
TOKEN="${2:-}"
MODEL="${3:-}"

if [[ -z "$TOKEN" || -z "$MODEL" ]]; then
	echo "usage: $0 [BASE_URL] TOKEN MODEL" >&2
	echo "  TOKEN: gateway bearer from config/tokens.yaml" >&2
	echo "  MODEL: virtual model (e.g. Claudia-0.1.0) or upstream model id" >&2
	exit 2
fi

curl -fsS "$BASE_URL/health" >/dev/null
curl -fsS "$BASE_URL/v1/chat/completions" \
	-H "Authorization: Bearer ${TOKEN}" \
	-H "Content-Type: application/json" \
	-d "{\"model\":\"${MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hello in one word.\"}],\"max_tokens\":16}"
echo
