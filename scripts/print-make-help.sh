#!/usr/bin/env bash
# Printed by `make help` so Windows/PowerShell/cmd do not mangle quotes or `echo`/printf handling.
set -euo pipefail
echo "Claudia (Go) - README order (primary flow: make up = install -> configure -> build -> background stack)"
echo
echo "  make up                 install + configure + claudia-build + claudia-start (UP_STACK=0 for no Qdrant)"
echo
echo "  make install            verify toolchain + bootstrap BiFrost/Qdrant from deps.lock (idempotent)"
echo "  make configure          seed .env + config/tokens.yaml from examples if missing"
echo
echo "  make claudia-build      go build -o claudia ./cmd/claudia (headless; no CGO)"
echo "  make claudia-run        go run ./cmd/claudia"
echo "  make claudia-serve      foreground supervisor + BiFrost + Qdrant"
echo "  make claudia-start      background: ./claudia serve (logs/claudia.log, run/claudia.pid)"
echo "  make logs               tail logs/claudia.log"
echo "  make claudia-status     PID file + HTTP probes (gateway / BiFrost / Qdrant)"
echo "  make claudia-stop       stop background supervisor from run/claudia.pid"
echo
echo "  make desktop-install    native deps for WebView + CGO (Debian/Ubuntu, macOS CLT, Windows hints)"
echo "  make desktop-build      go build -tags desktop -> ./claudia-desktop[.exe] (CGO required)"
echo "  make desktop-run        desktop-build if missing, then claudia-desktop desktop (same flags as claudia-serve)"
echo "  make vet-desktop        go vet -tags desktop ./cmd/claudia (CGO)"
echo
echo "  make fmt                gofmt -w cmd internal"
echo "  make fmt-check          fail if gofmt would change files"
echo "  make vet-gateway        go vet ./..."
echo "  make test-gateway       go test ./... (with -race on Unix)"
echo "  make precommit          fmt-check, vet, test; vet-desktop unless SKIP_DESKTOP=1"
echo
echo "  make bash               interactive bash (-il); Windows: Git for Windows bash"
echo
echo "  make clean              remove claudia[.exe], claudia-desktop[.exe], dist/"
echo "  make clean-all          remove clean + bin/*third-party + .deps + run + logs (needs CONFIRM=1)"
echo "  make clean-data         remove data/bifrost + data/qdrant (fresh BiFrost/Qdrant; needs CONFIRM=1)"
echo "  make release-snapshot   goreleaser snapshot -> dist/"
