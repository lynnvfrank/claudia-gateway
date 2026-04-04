#!/usr/bin/env bash
# Print BiFrost’s full model catalog (management API). Same source the gateway uses
# when the upstream is BiFrost (before falling back to GET /v1/models).
#
# Usage:
#   ./scripts/list-bifrost-models.sh
#   BIFROST_URL=http://localhost:8080 ./scripts/list-bifrost-models.sh

set -euo pipefail
BASE="${BIFROST_URL:-http://127.0.0.1:8080}"
BASE="${BASE%/}"
curl -sS "${BASE}/api/models?unfiltered=true&limit=500"
