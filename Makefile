# Claudia Gateway — see makefile.plan.md and README.md
#
# clean:      removes ./claudia[.exe], claudia-desktop[.exe], dist/ only.
# clean-all:  also removes bin/, packaging/qdrant-bundles/, packages/, node_modules/, .deps/, run/, logs/ (requires CONFIRM=1; runs clean first).
# clean-data: removes data/bifrost/, data/qdrant/ — fresh BiFrost + Qdrant state (requires CONFIRM=1).

ifeq ($(OS),Windows_NT)
  # Same bash as scripts/*.sh (Git for Windows). MSYS2-only: set GITBASH, e.g.
  #   set "GITBASH=C:\msys64\usr\bin\bash.exe"
  ifeq ($(origin GITBASH),undefined)
    # Per-machine install first; then per-user (winget / default installer path).
    _GIT_BASH := $(wildcard $(ProgramW6432)/Git/bin/bash.exe)
    ifeq ($(_GIT_BASH),)
      _GIT_BASH := $(wildcard $(LOCALAPPDATA)/Programs/Git/bin/bash.exe)
    endif
    ifneq ($(_GIT_BASH),)
      GITBASH := "$(firstword $(_GIT_BASH))"
    else
      GITBASH := "$(ProgramW6432)/Git/bin/bash.exe"
    endif
  endif
  RACE_GATEWAY :=
  BIFROST_BIN := bin/bifrost-http.exe
  QDRANT_BIN := bin/qdrant.exe
  DESKTOP_BIN := claudia-desktop.exe
else
  ifeq ($(origin GITBASH),undefined)
    GITBASH := bash
  endif
  RACE_GATEWAY := -race
  BIFROST_BIN := bin/bifrost-http
  QDRANT_BIN := bin/qdrant
  DESKTOP_BIN := claudia-desktop
endif

SKIP_DESKTOP ?=
ifeq ($(SKIP_DESKTOP),1)
  _DESKTOP_PRECOMMIT_TARGETS :=
else
  _DESKTOP_PRECOMMIT_TARGETS := vet-desktop
endif

# UP_STACK=0 starts background supervisor without Qdrant; default is full stack.
ifeq ($(UP_STACK),0)
  _BG_FLAGS :=
else
  _BG_FLAGS := --stack
endif

.PHONY: help up install configure clean clean-all clean-data fmt fmt-check logs \
	bash \
	claudia-build desktop-install desktop-build desktop-run \
	claudia-run claudia-serve claudia-start claudia-stop claudia-status \
	catalog-write-free catalog-write-available \
	release-install release-snapshot release-package \
	vet-gateway vet-desktop test-gateway precommit

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

# claudia-start: pass --stack (Qdrant + bifrost) unless UP_STACK=0 (BiFrost only, same as make up).
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
	$(GITBASH) scripts/clean-all-confirm.sh $(CONFIRM)
	$(MAKE) clean
	$(GITBASH) scripts/clean-all.sh

clean-data:
	$(GITBASH) scripts/clean-data.sh $(CONFIRM)

# --- CI / pre-commit (fmt-check, vet-gateway, test-gateway; vet-desktop unless SKIP_DESKTOP=1) ---
fmt:
	gofmt -w cmd internal

fmt-check:
	$(GITBASH) scripts/fmt-check.sh

vet-gateway:
	go vet ./...

vet-desktop: export CGO_ENABLED := 1
vet-desktop:
	go vet -tags desktop ./cmd/claudia

test-gateway:
	go test ./... $(RACE_GATEWAY) -count=1

precommit: fmt-check vet-gateway test-gateway $(_DESKTOP_PRECOMMIT_TARGETS)

claudia-build:
	go build -o claudia ./cmd/claudia

# Fetch Groq rate-limits + Gemini pricing pages and write BiFrost-style model ids (requires network).
# Optional: INTERSECT=path to JSON or YAML (OpenAI-style data[].id, e.g. catalog-available.snapshot.yaml).
# Override OUT=path for snapshot file (default config/free-tier-catalog.snapshot.yaml).
catalog-write-free:
	go run ./cmd/free-tier-catalog \
		-out "$(if $(OUT),$(OUT),config/free-tier-catalog.snapshot.yaml)" \
		$(if $(INTERSECT),-intersect $(INTERSECT),)

# GET BiFrost /v1/models and write YAML (running BiFrost; env BIFROST_BASE_URL, CLAUDIA_UPSTREAM_API_KEY).
# Override OUT=path (default config/catalog-available.snapshot.yaml).
catalog-write-available:
	go run ./cmd/catalog-write-available \
		-out "$(if $(OUT),$(OUT),config/catalog-available.snapshot.yaml)"

desktop-install:
	$(GITBASH) scripts/desktop-install.sh

desktop-build:
	$(GITBASH) scripts/desktop-build.sh $(DESKTOP_BIN)

desktop-run:
	$(GITBASH) scripts/desktop-run.sh $(DESKTOP_BIN) "$(MAKE)" desktop -qdrant-bin $(QDRANT_BIN) -bifrost-bin $(BIFROST_BIN)

claudia-run:
	go run ./cmd/claudia

# Foreground supervisor: same bin paths as claudia-start --stack (requires make install).
claudia-serve:
	go run ./cmd/claudia serve -qdrant-bin $(QDRANT_BIN) -bifrost-bin $(BIFROST_BIN)

release-install:
	$(GITBASH) scripts/release-install.sh

release-snapshot:
	$(GITBASH) scripts/release-snapshot.sh

# Desktop claudia + bifrost-http + qdrant + config → dist/personal/ (needs make install; CGO for desktop build).
release-package:
	$(GITBASH) scripts/release-package.sh "$(DESKTOP_BIN)"
