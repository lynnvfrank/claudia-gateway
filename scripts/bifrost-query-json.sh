#!/usr/bin/env bash
# Query a running BiFrost HTTP instance and print JSON with indentation.
#
# Defaults match config/gateway.yaml (upstream.base_url) and
# upstream.api_key_env (CLAUDIA_UPSTREAM_API_KEY).
#
# Usage:
#   ./scripts/bifrost-query-json.sh [command-or-path]
#
# Commands (shorthand):
#   models      GET /v1/models  (default)
#   api-models  GET /api/models?unfiltered=true&limit=500
#   health      GET /health
#
# Any other argument is used as a path/query (leading slash optional), e.g.:
#   ./scripts/bifrost-query-json.sh '/api/models?limit=10'
#
# Environment:
#   BIFROST_BASE_URL            Upstream root (default http://127.0.0.1:8080)
#   CLAUDIA_UPSTREAM_API_KEY    Optional Bearer token for protected routes
#   BIFROST_NO_AUTH=1           Do not send Authorization (e.g. for /health)
#
# Requires: curl. For formatting: jq, else python3/python json.tool.
#
# Windows: run from Git Bash, MSYS, or WSL (same as other repo scripts).
set -euo pipefail

BASE_URL="${BIFROST_BASE_URL:-http://127.0.0.1:8080}"
BASE_URL="${BASE_URL%/}"
TOKEN="${CLAUDIA_UPSTREAM_API_KEY:-}"

pretty_json() {
	if command -v jq >/dev/null 2>&1; then
		jq .
	elif command -v python3 >/dev/null 2>&1; then
		python3 -m json.tool
	elif command -v python >/dev/null 2>&1; then
		python -m json.tool
	else
		echo "bifrost-query-json: need jq or python for pretty-print; raw output follows:" >&2
		cat
	fi
}

usage() {
	echo "usage: $0 [models|api-models|health|PATH]" >&2
	echo "  env BIFROST_BASE_URL (default http://127.0.0.1:8080)" >&2
	echo "  env CLAUDIA_UPSTREAM_API_KEY for Bearer auth" >&2
	exit 2
}

raw="${1:-models}"
case "$raw" in
-h | --help | help) usage ;;
models) path="/v1/models" ;;
api-models) path="/api/models?unfiltered=true&limit=500" ;;
health) path="/health" ;;
*)
	if [[ "$raw" == http://* || "$raw" == https://* ]]; then
		echo "bifrost-query-json: pass a path only, not a full URL (use BIFROST_BASE_URL)." >&2
		exit 2
	fi
	path="$raw"
	[[ "$path" == /* ]] || path="/${path}"
	;;
esac

url="${BASE_URL}${path}"

curl_args=(-sS)
if [[ -z "${BIFROST_NO_AUTH:-}" && -n "$TOKEN" ]]; then
	curl_args+=(-H "Authorization: Bearer ${TOKEN}")
fi

# Print URL on stderr so stdout stays pure JSON (pipe-friendly).
echo "GET ${url}" >&2
if ! body="$(curl "${curl_args[@]}" "$url")"; then
	exit 1
fi

printf '%s\n' "$body" | pretty_json
