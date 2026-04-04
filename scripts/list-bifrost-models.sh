#!/usr/bin/env bash
# Print the upstream OpenAI-compatible model list (GET /v1/models). Same route the
# gateway uses when merging upstream models into GET /v1/models.
#
# Usage:
#   ./scripts/list-bifrost-models.sh
#   BIFROST_URL=http://localhost:8080 ./scripts/list-bifrost-models.sh

set -euo pipefail
BASE="${BIFROST_URL:-http://127.0.0.1:8080}"
BASE="${BASE%/}"
curl -sS "${BASE}/v1/models"
