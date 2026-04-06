# Claudia Gateway — see makefile.plan.md and README.md
#
# clean:      removes ./claudia[.exe], ./claudia-gui[.exe], dist/ only.
# clean-all:  also removes bin/bifrost-http*, bin/qdrant*, .deps/, run/, logs/ (requires CONFIRM=1).

ifeq ($(OS),Windows_NT)
  # Same bash as install/*.sh (Git for Windows). MSYS2-only: set GITBASH, e.g.
  #   set "GITBASH=C:\msys64\usr\bin\bash.exe"
  ifeq ($(origin GITBASH),undefined)
    GITBASH := "$(ProgramW6432)/Git/bin/bash.exe"
  endif
  RACE_GATEWAY :=
  BIFROST_BIN := bin/bifrost-http.exe
  QDRANT_BIN := bin/qdrant.exe
else
  ifeq ($(origin GITBASH),undefined)
    GITBASH := bash
  endif
  RACE_GATEWAY := -race
  BIFROST_BIN := bin/bifrost-http
  QDRANT_BIN := bin/qdrant
endif

SKIP_GUI ?=
ifeq ($(SKIP_GUI),1)
  _GUI_PRECOMMIT_TARGETS :=
else
  _GUI_PRECOMMIT_TARGETS := vet-gui test-gui
endif

# UP_STACK=0 starts background supervisor without Qdrant; default is full stack.
ifeq ($(UP_STACK),0)
  _BG_FLAGS :=
else
  _BG_FLAGS := --stack
endif

.PHONY: help up install configure clean clean-all fmt fmt-check logs \
	bash \
	claudia-build claudia-gui-help claudia-gui-build claudia-gui-run \
	claudia-run claudia-serve claudia-start claudia-stop claudia-status \
	release-snapshot \
	vet-gateway vet-gui test-gateway test-gui precommit

# One bash process (same as install/*.sh) so Win32 Make does not run cmd `echo`/printf per line (quotes + CreateProcess failures).
help:
	@$(GITBASH) scripts/print-make-help.sh

# --- Full stack onboarding (§A.7 makefile.plan.md) ---
up: install configure claudia-build claudia-start

bash:
	$(GITBASH) -il

install:
	$(GITBASH) scripts/install.sh

configure:
	$(GITBASH) scripts/configure.sh

claudia-start:
	$(GITBASH) scripts/claudia-start.sh $(_BG_FLAGS)

claudia-stop:
	$(GITBASH) scripts/claudia-stop.sh

claudia-status:
	$(GITBASH) scripts/claudia-status.sh

logs:
	$(GITBASH) scripts/logs.sh

clean:
	$(GITBASH) scripts/clean.sh

clean-all:
	@test "$(CONFIRM)" = "1" || { echo "clean-all: removes .deps/, bin/bifrost-http*, bin/qdrant*, run/, logs/ — re-run with CONFIRM=1" >&2; exit 1; }
	$(MAKE) clean
	$(GITBASH) scripts/clean-all.sh

# --- CI / pre-commit (.github/workflows/go.yml test + gui jobs) ---
fmt:
	gofmt -w cmd internal gui

fmt-check:
	$(GITBASH) scripts/fmt-check.sh

vet-gateway:
	go vet ./...

test-gateway:
	go test ./... $(RACE_GATEWAY) -count=1

vet-gui: export CGO_ENABLED := 1
vet-gui:
	go vet -C gui ./...

test-gui: export CGO_ENABLED := 1
test-gui:
	go test -C gui ./... -count=1

precommit: fmt-check vet-gateway test-gateway $(_GUI_PRECOMMIT_TARGETS)

claudia-build:
	go build -o claudia ./cmd/claudia

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
	go run ./cmd/claudia serve -qdrant-bin $(QDRANT_BIN) -bifrost-bin $(BIFROST_BIN)

release-snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { echo "release-snapshot: install https://goreleaser.com/install/ or run the docker one-liner in docs/packaging.md" >&2; exit 1; }
	goreleaser release --snapshot --clean
