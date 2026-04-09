#!/usr/bin/env bash
# Full local bundle: desktop claudia + bifrost-http + qdrant + config (make release-package).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
name="claudia-bundle_${goos}_${goarch}"
OUT="$ROOT/dist/personal/$name"
rm -rf "$OUT"
mkdir -p "$OUT/config"

ext=""
if [[ "$goos" == "windows" ]]; then
	ext=".exe"
fi

DESKTOP_BIN="${1:-}"
if [[ -z "$DESKTOP_BIN" ]]; then
	if [[ -n "$ext" ]]; then
		DESKTOP_BIN="claudia-desktop.exe"
	else
		DESKTOP_BIN="claudia-desktop"
	fi
fi

if [[ ! -f "$ROOT/$DESKTOP_BIN" ]]; then
	echo "package-personal: building $DESKTOP_BIN (CGO + -tags desktop)..."
	bash "$ROOT/scripts/desktop-build.sh" "$DESKTOP_BIN"
fi

BIF="bifrost-http${ext}"
QDR="qdrant${ext}"
if [[ ! -f "$ROOT/bin/$BIF" ]]; then
	echo "package-personal: missing bin/$BIF — run: make install" >&2
	exit 1
fi
if [[ ! -f "$ROOT/bin/$QDR" ]]; then
	echo "package-personal: missing bin/$QDR — run: make install" >&2
	exit 1
fi

cp "$ROOT/$DESKTOP_BIN" "$OUT/claudia${ext}"
cp "$ROOT/bin/$BIF" "$OUT/"
cp "$ROOT/bin/$QDR" "$OUT/"

cp "$ROOT/config/gateway.example.yaml" "$OUT/config/gateway.yaml"
cp "$ROOT/config/tokens.example.yaml" "$OUT/config/tokens.example.yaml"
cp "$ROOT/config/bifrost.config.json" "$OUT/config/bifrost.config.json"
cp "$ROOT/config/routing-policy.yaml" "$OUT/config/routing-policy.yaml"
cp "$ROOT/env.example" "$OUT/env.example"

cat > "$OUT/README.txt" <<'EOF'
Personal bundle (make release-package)

1. Copy env.example to .env in this folder and set keys (see comments inside env.example).
2. First run: start claudia — it opens setup on localhost to create config/tokens.yaml (or copy tokens.example.yaml to tokens.yaml yourself).
3. Run claudia from this folder (same directory as bifrost-http and qdrant):
     — Desktop UI + full stack: double-click claudia (Windows) or ./claudia
     — Headless supervisor: ./claudia --headless
     — Gateway only (no local BiFrost/Qdrant): ./claudia gateway

Sibling binaries (bifrost-http, qdrant) are auto-detected. Config lives in ./config/.

Rebuild: from repo root run  make release-package
EOF

echo "package-personal: wrote $OUT"
