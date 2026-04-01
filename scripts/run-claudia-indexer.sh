#!/usr/bin/env bash
# Start the file-indexer container on the same Docker network as the Claudia gateway.
#
# Resolves the Compose-created network (e.g. claudia-gateway_claudianet) by:
#   docker compose config  — validate the compose file
#   docker compose ps -q claudia — running gateway container
#   docker inspect — read NetworkSettings.Networks for the claudianet attachment
#
# Usage:
#   ./scripts/run-claudia-indexer.sh              # docker run with defaults
#   ./scripts/run-claudia-indexer.sh --print-network   # only print network name
#
# Environment:
#   CLAUDIA_COMPOSE_FILE  — default: repo-root docker-compose.yml
#   COMPOSE_PROJECT_NAME  — optional; passed to docker compose -p
#   INDEXER_IMAGE         — default: claudia-file-indexer:latest (build separately)
#   INDEXER_WORKDIR       — host path mounted as /workspace; default: $PWD
#
set -euo pipefail

print_network_only=0
if [[ "${1:-}" == "--print-network" ]]; then
  print_network_only=1
  shift
fi

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
COMPOSE_FILE="${CLAUDIA_COMPOSE_FILE:-$REPO_ROOT/docker-compose.yml}"
INDEXER_IMAGE="${INDEXER_IMAGE:-claudia-file-indexer:latest}"
INDEXER_WORKDIR="${INDEXER_WORKDIR:-$PWD}"

compose_cmd=(docker compose -f "$COMPOSE_FILE")
if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
  compose_cmd=(docker compose -p "$COMPOSE_PROJECT_NAME" -f "$COMPOSE_FILE")
fi

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "error: compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi

# Validate compose definition (fail fast on YAML/schema issues).
"${compose_cmd[@]}" config >/dev/null

cid=$("${compose_cmd[@]}" ps -q claudia 2>/dev/null | head -n1 | tr -d '\r')
if [[ -z "$cid" ]]; then
  echo "error: claudia service is not running." >&2
  echo "Start the stack from the compose project directory, e.g.:" >&2
  echo "  ${compose_cmd[*]} up -d" >&2
  exit 1
fi

# Collect non-empty network names from the running claudia container.
nets=()
while IFS= read -r line; do
  line=${line//$'\r'/}
  [[ -n "$line" ]] && nets+=("$line")
done < <(docker inspect -f '{{range $k, $_ := .NetworkSettings.Networks}}{{$k}}{{"\n"}}{{end}}' "$cid")

claudia_net=""
for n in "${nets[@]}"; do
  case "$n" in
    *_claudianet | claudianet)
      claudia_net=$n
      break
      ;;
  esac
done

# If the compose network key were renamed, fall back to the only non-host network.
if [[ -z "$claudia_net" && ${#nets[@]} -eq 1 ]]; then
  claudia_net="${nets[0]}"
fi

if [[ -z "$claudia_net" ]]; then
  echo "error: could not find claudianet among container networks: ${nets[*]-<none>}" >&2
  exit 1
fi

if [[ "$print_network_only" -eq 1 ]]; then
  printf '%s\n' "$claudia_net"
  exit 0
fi

suffix=$(date +%s)-$$
name="claudia-indexer-${suffix}"

echo "Compose file:  $COMPOSE_FILE"
echo "Claudia net:   $claudia_net"
echo "Indexer image: $INDEXER_IMAGE"
echo "Mount:         $INDEXER_WORKDIR -> /workspace"
echo "Container:     $name"
echo

exec docker run --rm -it \
  --name "$name" \
  --network "$claudia_net" \
  -v "$INDEXER_WORKDIR:/workspace" \
  -w /workspace \
  "$INDEXER_IMAGE" \
  "$@"
