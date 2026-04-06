#!/usr/bin/env bash
# Idempotent: verify toolchain, then install-bootstrap.sh (BiFrost + Qdrant from deps.lock).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"
# shellcheck source=scripts/compiler-detect.sh
source "$REPO_ROOT/scripts/compiler-detect.sh"

echo "==> install: toolchain"
missing=0
need() {
	if command -v "$1" >/dev/null 2>&1; then
		echo "    OK  $1 → $(command -v "$1")"
	else
		echo "    MISSING  $1" >&2
		missing=1
	fi
}

need go
need git
need make
if command -v go >/dev/null 2>&1; then
	echo "    go version: $(go version)"
fi
if command -v node >/dev/null 2>&1; then
	ver="$(node -v 2>/dev/null || true)"
	major="$(node -p "parseInt(process.versions.node.split('.')[0],10)" 2>/dev/null || echo 0)"
	if [ "$major" -lt 20 ]; then
		echo "    WARN  Node.js should be >= 20 for BiFrost UI (found $ver)" >&2
	else
		echo "    OK  node $ver"
	fi
else
	echo "    MISSING  node (required for BiFrost UI build during bootstrap)" >&2
	missing=1
fi

# BiFrost's bifrost-http binary is built with CGO; Go needs a C toolchain (gcc or clang on PATH).
if has_cc; then
	echo "    OK  C compiler → $(cc_on_path)"
else
	if [[ "${SKIP_AUTO_GCC:-}" == "1" ]]; then
		echo "    (no gcc/clang — sourcing scripts/install-gcc.sh; SKIP_AUTO_GCC=1 skips auto-install)" >&2
	else
		echo "    (no gcc/clang — sourcing scripts/install-gcc.sh)"
	fi
	# shellcheck source=scripts/install-gcc.sh
	if source "$REPO_ROOT/scripts/install-gcc.sh"; then
		if has_cc; then
			echo "    OK  C compiler → $(cc_on_path)"
		else
			echo "    MISSING  gcc or clang after auto-install — open a new shell or see docs/installation.md#c-compiler-cgo" >&2
			missing=1
		fi
	else
		echo "    MISSING  gcc or clang (auto-install failed or SKIP_AUTO_GCC=1 — see docs/installation.md#c-compiler-cgo)" >&2
		missing=1
	fi
fi

if [ "$missing" -ne 0 ]; then
	echo "" >&2
	echo "install: install missing tools, then re-run: make install" >&2
	exit 1
fi

echo "==> install: BiFrost + Qdrant (deps.lock)"
bash "$REPO_ROOT/scripts/install-bootstrap.sh"

echo "==> install: artifacts"
found=0
for f in bin/bifrost-http bin/bifrost-http.exe bin/qdrant bin/qdrant.exe; do
	if [ -f "$f" ]; then
		echo "    OK  $f"
		found=1
	fi
done
if [ "$found" -eq 0 ]; then
	echo "    WARN  no bifrost-http or qdrant under bin/ — check bootstrap output above" >&2
fi

echo ""
echo "install: done. Next:"
echo "    make configure   # seed .env and config/tokens.yaml if missing"
