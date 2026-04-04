#!/usr/bin/env bash
# Clone BiFrost at the ref in deps.lock, build bifrost-http, and download the pinned Qdrant binary.
# Requires: git, curl, tar, make, Node.js 20+ (BiFrost UI build), Go toolchain (BiFrost backend).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=deps-lock.sh
source "$REPO_ROOT/scripts/deps-lock.sh"

DEPS_DIR="${DEPS_DIR:-$REPO_ROOT/.deps}"
BIFROST_DIR="${BIFROST_DIR:-$DEPS_DIR/bifrost}"

QDRANT_RELEASE="$(deps_lock_get QDRANT_RELEASE)"
BIFROST_GIT_URL="$(deps_lock_get BIFROST_GIT_URL)"
BIFROST_GIT_REF="$(deps_lock_get BIFROST_GIT_REF)"

mkdir -p "$DEPS_DIR" "$REPO_ROOT/bin"

echo "==> BiFrost @ $BIFROST_GIT_REF from $BIFROST_GIT_URL -> $BIFROST_DIR"
if [[ ! -d "$BIFROST_DIR/.git" ]]; then
	git clone "$BIFROST_GIT_URL" "$BIFROST_DIR"
else
	echo "    (existing clone; fetching)"
	git -C "$BIFROST_DIR" remote set-url origin "$BIFROST_GIT_URL" 2>/dev/null || true
fi
git -C "$BIFROST_DIR" fetch origin
if ! git -C "$BIFROST_DIR" rev-parse -q --verify "${BIFROST_GIT_REF}^{commit}" >/dev/null 2>&1; then
	git -C "$BIFROST_DIR" fetch origin "$BIFROST_GIT_REF"
fi
git -C "$BIFROST_DIR" checkout -q "$BIFROST_GIT_REF"

command -v node >/dev/null 2>&1 || {
	echo "bootstrap-deps: install Node.js 20+ and ensure it is on PATH (BiFrost UI build)." >&2
	exit 1
}
node_major="$(node -p "parseInt(process.versions.node.split('.')[0],10)" 2>/dev/null || echo 0)"
if [[ "$node_major" -lt 20 ]]; then
	echo "bootstrap-deps: BiFrost needs Node.js >= 20; found $(node -v 2>/dev/null)." >&2
	exit 1
fi

# BiFrost's default `make build` sets GOWORK=off and compiles against published
# modules (e.g. framework v1.2.x). The clone's transports code can drift ahead
# (e.g. DefaultClientConfig uses fields only present in the local framework tree).
# setup-workspace + LOCAL=1 builds with repo-root go.work so local modules match.
echo "==> Go workspace + build in BiFrost (may run npm ci in ui/)"
make -C "$BIFROST_DIR" setup-workspace
make -C "$BIFROST_DIR" build LOCAL=1
cp -f "$BIFROST_DIR/tmp/bifrost-http" "$REPO_ROOT/bin/bifrost-http"
chmod +x "$REPO_ROOT/bin/bifrost-http"
echo "    installed $REPO_ROOT/bin/bifrost-http"

echo "==> Qdrant $QDRANT_RELEASE -> bin/qdrant"
bash "$REPO_ROOT/scripts/fetch-qdrant-local.sh"

echo ""
echo "Done. Binaries: $REPO_ROOT/bin/bifrost-http  $REPO_ROOT/bin/qdrant"
echo "Optional: export BIFROST_SRC=$BIFROST_DIR   # for make bifrost-from-src after upstream changes"
