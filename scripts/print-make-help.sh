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
echo "  make claudia-build      go build -o claudia ./cmd/claudia"
echo "  make claudia-serve      foreground supervisor + BiFrost + Qdrant"
echo "  make claudia-start      background: ./claudia serve (logs/claudia.log, run/claudia.pid)"
echo "  make logs               tail logs/claudia.log"
echo "  make claudia-status     PID file + HTTP probes (gateway / BiFrost / Qdrant)"
echo "  make claudia-stop       stop background supervisor from run/claudia.pid"
echo
echo "  make claudia-gui-help   Debian/Ubuntu packages for Fyne"
echo "  make claudia-gui-build  build GUI (CGO)"
echo "  make claudia-gui-run    run GUI (CGO)"
echo "  make vet-gui            go vet -C gui (CGO)"
echo "  make test-gui           go test -C gui (CGO)"
echo
echo "  make fmt                gofmt -w cmd internal gui"
echo "  make fmt-check          fail if gofmt would change files"
echo "  make vet-gateway        go vet ./..."
echo "  make test-gateway       go test ./... (with -race on Unix)"
echo "  make precommit          fmt-check, vet, test (gateway + gui; SKIP_GUI=1 if no Fyne/CGO)"
echo
echo "  make bash               interactive bash (-il); Windows: Git for Windows bash"
echo
echo "  make clean              remove claudia[.exe], claudia-gui[.exe], dist/"
echo "  make clean-all          remove clean + bin/*third-party + .deps + run + logs (needs CONFIRM=1)"
echo "  make release-snapshot   goreleaser snapshot -> dist/"
