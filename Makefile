# Shortcuts from repo root.
BIFROST_SRC ?= $(HOME)/src/bifrost

.PHONY: help bootstrap-deps claudia-build claudia-gui-help claudia-gui-build claudia-gui-run claudia-run claudia-serve claudia-serve-local claudia-serve-stack release-snapshot bifrost-node-check bifrost-from-src qdrant-from-release

help:
	@echo "Claudia (Go) — config/gateway.yaml, tokens.yaml, routing-policy.yaml"
	@echo "  make bootstrap-deps     clone BiFrost + build bifrost-http + Qdrant (pins in deps.lock)"
	@echo "  make claudia-build       go build -o claudia ./cmd/claudia"
	@echo "  make claudia-gui-help    print apt install line for Fyne on Debian/Ubuntu"
	@echo "  make claudia-gui-build   Fyne GUI → ./claudia-gui (CGO + OS deps; see docs/gui-testing.md)"
	@echo "  make claudia-gui-run      run ./claudia-gui (builds first if missing)"
	@echo "  make claudia-run         go run ./cmd/claudia (uses CLAUDIA_GATEWAY_CONFIG or ./config/gateway.yaml)"
	@echo "  make claudia-serve       go run ./cmd/claudia serve (BiFrost subprocess + gateway; see docs/supervisor.md)"
	@echo "  make bifrost-from-src    build BiFrost from BIFROST_SRC (Node 20+ on PATH; not snap node 10) → ./bin/bifrost-http"
	@echo "  make claudia-serve-local claudia-serve with -bifrost-bin ./bin/bifrost-http"
	@echo "  make claudia-serve-stack  serve + ./bin/qdrant + ./bin/bifrost-http (run qdrant-from-release / bifrost-from-src first)"
	@echo "  make qdrant-from-release  download pinned Qdrant binary → ./bin/qdrant (Linux/macOS; versions in deps.lock)"
	@echo "  make release-snapshot   GoReleaser snapshot → dist/ (needs goreleaser on PATH; see docs/packaging.md)"

bootstrap-deps:
	bash scripts/bootstrap-deps.sh

claudia-build:
	go build -o claudia ./cmd/claudia

# Nested module gui/ (Fyne). Requires CGO and platform libraries — see docs/gui-testing.md.
claudia-gui-help:
	@echo "Debian/Ubuntu — install OpenGL + X11 dev packages for Fyne, then re-run make claudia-gui-build:"
	@echo "  sudo apt-get install -y gcc pkg-config libgl1-mesa-dev libx11-dev libxrandr-dev libxinerama-dev libxcursor-dev libxi-dev libxxf86vm-dev"

claudia-gui-build:
	@cd gui && CGO_ENABLED=1 go build -o ../claudia-gui . || { \
		echo "" >&2; \
		echo "claudia-gui-build: Fyne needs CGO plus OpenGL and X11 headers on Linux." >&2; \
		echo "  Run:  make claudia-gui-help" >&2; \
		echo "  Doc:  docs/gui-testing.md" >&2; \
		exit 1; \
	}
	@echo "Built ./claudia-gui — open a graphical session and run:  ./claudia-gui   (or:  make claudia-gui-run)"

claudia-gui-run:
	@test -f claudia-gui || $(MAKE) claudia-gui-build
	./claudia-gui

claudia-run:
	go run ./cmd/claudia

claudia-serve:
	go run ./cmd/claudia serve -bifrost-bin ./bin/bifrost-http

claudia-serve-stack:
	go run ./cmd/claudia serve -qdrant-bin ./bin/qdrant -bifrost-bin ./bin/bifrost-http

qdrant-from-release:
	bash scripts/fetch-qdrant-local.sh

release-snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { echo "release-snapshot: install https://goreleaser.com/install/ or run the docker one-liner in docs/packaging.md" >&2; exit 1; }
	goreleaser release --snapshot --clean

# BiFrost `make build` runs `npm ci` in ui/ (Next 15). Snap's `node` is often v10 and breaks with:
#   npm ERR! Cannot read property '@base-ui/react' of undefined
bifrost-node-check:
	@command -v node >/dev/null 2>&1 || { echo "bifrost-from-src: install Node.js 20+ (https://nodejs.org/) and ensure it is on PATH before npm." >&2; exit 1; }
	@node_major=$$(node -p "parseInt(process.versions.node.split('.')[0],10)" 2>/dev/null || echo 0); \
	if [ "$$node_major" -lt 20 ]; then \
		echo "bifrost-from-src: BiFrost UI needs Node.js >= 20. On PATH now: $$(command -v node) $$(node -v 2>/dev/null)." >&2; \
		echo "If you use snap, it often installs ancient Node — use nvm, fnm, volta, or a distro/nodejs.org build and put it first in PATH." >&2; \
		exit 1; \
	fi

bifrost-from-src: bifrost-node-check
	@test -d "$(BIFROST_SRC)" || { echo "bifrost-from-src: set BIFROST_SRC or clone BiFrost to $(BIFROST_SRC)" >&2; exit 1; }
	mkdir -p bin
	$(MAKE) -C "$(BIFROST_SRC)" setup-workspace
	$(MAKE) -C "$(BIFROST_SRC)" build LOCAL=1
	cp -f "$(BIFROST_SRC)/tmp/bifrost-http" bin/bifrost-http
	chmod +x bin/bifrost-http
	@echo "Installed $$(pwd)/bin/bifrost-http"
