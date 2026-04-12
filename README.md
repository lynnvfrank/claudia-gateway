# Claudia Gateway

**v0.1** — OpenAI-compatible **Go** gateway in front of **BiFrost**: virtual model `Claudia-<semver>`, YAML **tokens** and **routing policy** with mtime reload, **sequential fallback** on 429/5xx, and `GET /health`. Optional **Qdrant** via `claudia serve` targets **v0.2** RAG; the gateway does not call it in v0.1.

## Quick start

You need **GNU Make**.

On Windows:

```powershell
pwsh -ExecutionPolicy Bypass -File scripts/install-make.ps1
```

On Ubuntu and OSX:

```bash
bash scripts/install-make.sh
```

### Install and Start (all-in-one)

One-shot onboarding: get dependencies, seed config if needed, build the gateway, and start the supervisor in the background.

```bash
make up
```

## Installing Dependencies and Building the Service

### Install

Install all the dependencies and build the dependent projects.

```bash
make install
```

**`make install`** runs **`make claudia-install`** (toolchain check, BiFrost, Qdrant) and then **`make desktop-install`** (native WebView/CGO deps). For a **headless** machine or CI-style setup where you only need **`./bin/bifrost-http`** and **`./bin/qdrant`**, use **`make claudia-install`** only.

**Dependencies**

- **Go (1.22+)** — builds the gateway and BiFrost’s Go code.
- **Node.js (20+)** — BiFrost’s UI is built with npm during install; it is not shipped inside the **`claudia`** binary.
- **Git** — BiFrost is vendored from a tracked revision, not embedded in the clone.
- **GNU Make** — single entrypoint for install and build targets from the repo root.
- **gcc or clang** — BiFrost’s HTTP server is built with **CGO**; the Go toolchain must invoke a C compiler or the build fails early.
- **bash, curl, tar** (and **unzip** on Windows) — reliable way to download and unpack release artifacts the same way on every platform.

Full reference: **[docs/installation.md](docs/installation.md)**.

### Configuration

Create local config files from the shipped examples when they are missing.

```bash
make configure
```

**Creates (only if absent):** **`config/gateway.yaml`** from [config/gateway.example.yaml](config/gateway.example.yaml). Copy [env.example](env.example) to **`.env`** yourself when you want local env vars. It does **not** create **`config/tokens.yaml`**: on first run, **`claudia serve`** (or **`claudia gateway`**) enters **bootstrap** on localhost and **`/ui/setup`** creates **`tokens.yaml`**; then **restart** for the full stack. You can still copy [config/tokens.example.yaml](config/tokens.example.yaml) to **`config/tokens.yaml`** manually if you prefer.

| Purpose | File | Role |
| ------- | ---- | ---- |
| Process environment | `.env`<br/>(copied from [env.example](env.example)) | Optional local environment variable file to set Claudia<->BiFrost key and API keys. Not committed. |
| Gateway client auth | `config/tokens.yaml`<br/>(from [config/tokens.example.yaml](config/tokens.example.yaml) or **setup UI**) | Tokens for clients (`Authorization: Bearer …`) and admin UI login. Not committed. |
| Gateway listen + upstream | `config/gateway.yaml` | Claudia Gateway's primary configuration file to connect the Client<->Claudia<->BiFrost  |
| BiFrost bootstrap | `config/bifrost.config.json` | BiFrost HTTP config; provider secrets pulled from environment variables set in `.env` or the shell. |
| Virtual model mapping | `config/routing-policy.yaml` | Rules that define how the virtual `Claudia-<semver>` model routes prompts/turns to underlying models |
| Free-tier allowlist (optional) | `config/provider-free-tier.yaml` | When `routing.filter_free_tier_models` is true, restricts merged model listing and UI routing generation to ids in this file |

**Manual follow-up**

1. **`.env`:** set `CLAUDIA_UPSTREAM_API_KEY` to match `upstream.api_key_env` in `config/gateway.yaml`. Set `GROQ_API_KEY`, `GEMINI_API_KEY`, or other keys that `config/bifrost.config.json` references.
2. **`config/tokens.yaml`:** create via first-run **`/ui/setup`** (see [docs/version-v0.1.md](docs/version-v0.1.md)) or copy from **`tokens.example.yaml`**; clients use `Authorization: Bearer <token>`.
3. **`config/gateway.yaml`** — run **`make configure`** once (or copy **`gateway.example.yaml`**) if missing; adjust `listen_host` / `listen_port`, `upstream.base_url`, `routing.fallback_chain`, paths if needed. Not committed.
4. **`config/bifrost.config.json`** — align provider blocks and `env.*` with your **`.env`**.
5. **`config/routing-policy.yaml`** — committed default; edit or point `gateway.yaml` at another file.
6. **`config/provider-free-tier.yaml`** — optional operator allowlist; shipped default; tune `models` / `patterns` and set `routing.filter_free_tier_models` in `gateway.yaml` if you want catalog filtering. To refresh a **reference** list from [Groq](https://console.groq.com/docs/rate-limits) + [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing) docs (network required), run **`make catalog-free`** (optional **`INTERSECT=`** path to JSON or YAML `data[].id`, e.g. **`config/catalog-available.snapshot.yaml`**). To snapshot models from a running BiFrost, run **`make catalog-available`**. See [docs/configuration.md](docs/configuration.md).

Full reference: [docs/configuration.md](docs/configuration.md).

### Build the Gateway

The install process builds and downloads **BiFrost** and **Qdrant**. This builds the gateway.

```bash
make claudia-build
```

## Managing the Service

With the gateway and BiFrost built and Qdrant installed the components of the service are in place.

### Start the Service

Claudia runs Gateway, BiFrost, and Qdrant.

Run the **services in the foreground**:

```bash
make claudia-serve
```

Run the **service in the background**:

Run 
```bash
make claudia-start
```

Further reference: [docs/supervisor.md](docs/supervisor.md).

### View the logs of the background service

Follow the supervisor log in the terminal.

```bash
make logs
```

**Tails** **`logs/claudia.log`** (typically **`tail -f`**). Useful after **`make up`** or **`make claudia-start`**.

### Check Service

Check the service (PID file) checks the health of the gateway, bifrost, and qdrant.

```bash
make claudia-status
```

### Stop the Service

```bash
make claudia-stop
```

## Desktop (native webview shell)

Operator UI is served by the gateway at **`/ui/login`** and **`/ui/panel`** (browser or embedded webview). A separate **`claudia-desktop`** binary (**`-tags desktop`**, **CGO**) runs **`claudia desktop`** — same flags as **`claudia serve`** — and opens a native window to the panel.

| Target | What it does |
| ------ | ------------ |
| **`make desktop-install`** | Installs native deps for **WebView** + **CGO** (Debian/Ubuntu **WebKitGTK**, macOS **CLT**, Windows hints + MSYS2 **`gcc`**). |
| **`make desktop-build`** | **`CGO_ENABLED=1 go build -tags desktop`** → **`./claudia-desktop`** (or **`.exe`** on Windows). |
| **`make desktop-run`** | **`desktop-build`** if missing, then **`claudia-desktop desktop`** with the same **Qdrant/BiFrost** flags as **`make claudia-serve`**. |
| **`make vet-desktop`** | **`go vet -tags desktop ./cmd/claudia`** with **CGO** (same toolchain as **`desktop-build`**). |

**`make vet`** includes **`vet-desktop`** unless **`SKIP_DESKTOP=1`**; see **Testing and Linting** below.

## Testing and Linting

| Target | What it does |
| ------ | ------------ |
| **`make fmt`** | **`gofmt -w`** on **`cmd`** and **`internal`**. |
| **`make fmt-check`** | Fails if **`gofmt`** would change any file (same check as CI). Used by **`precommit`**. |
| **`make vet`** | **`vet-module`** (**`go vet ./...`**) plus **`vet-desktop`** unless **`SKIP_DESKTOP=1`**. Used by **`precommit`**. |
| **`make vet-module`** | **`go vet ./...`** on the main module. |
| **`make vet-desktop`** | **`go vet -tags desktop ./cmd/claudia`** with **CGO**. |
| **`make test`** | Runs **`test-internal`**, **`test-catalog-free`**, **`test-catalog-available`**, **`test-claudia`**, and **`test-desktop`** unless **`SKIP_DESKTOP=1`**. **`-race`** on Unix (same as before). |
| **`make test-internal`** / **`test-claudia`** / **`test-desktop`** / **`test-catalog-free`** / **`test-catalog-available`** | **`go test`** on that subtree; use **`test-desktop`** for the desktop-tagged **`claudia`** binary (CGO). |
| **`make precommit`** | Runs **`fmt-check`**, **`vet`**, and **`test`**. On Windows/Git Bash, **`./scripts/precommit-smoke.sh`** uses **`SKIP_DESKTOP=1`** by default; set **`FULL_DESKTOP=1`** to include desktop vet/test. |

## Repo Management and Packaging

### Clean up built binaries

Remove **built gateway/GUI binaries** and release scratch output.

```bash
make clean
```

**Deletes** **`./claudia`**, **`./claudia-desktop`**, Windows **`.exe`** variants, and **`dist/`**. Does **not** remove **`bin/bifrost-http`**, **`bin/qdrant`**, **`.deps/`**, **`run/`**, or **`logs/`**.

### Clean up everything

Deep clean: everything **`make clean`** removes **plus** third-party binaries and working directories.

```bash
make clean-all CONFIRM=1
```

**Requires** **`CONFIRM=1`**. **Also removes** **`bin/bifrost-http`**, **`bin/qdrant`**, **`.deps/`**, **`run/`**, **`logs/`** (after running **`make clean`**). Use when you want a fresh **`make install`**.

### `make release-snapshot`

```bash
make release-snapshot
```

Build snapshot artifacts with **GoReleaser**.

**Requires** **`goreleaser`** on **`PATH`**. **Writes** snapshot outputs under **`dist/`** (see [docs/packaging.md](docs/packaging.md)).


## Documentation

- **Index:** [docs/README.md](docs/README.md)
- **Overview / ports:** [docs/overview.md](docs/overview.md), [docs/network.md](docs/network.md)
- **Installation:** [docs/installation.md](docs/installation.md)
- **Configuration:** [docs/configuration.md](docs/configuration.md)
- **Supervisor:** [docs/supervisor.md](docs/supervisor.md)
- **Packaging / releases:** [docs/packaging.md](docs/packaging.md)
- **Makefile plan:** [docs/makefile-plan.md](docs/makefile-plan.md)
- **Admin UI / desktop shell:** [docs/ui-tool.plan.md](docs/ui-tool.plan.md); legacy Fyne checklist removed (use **`/ui/*`** + **`make desktop-build`**).
- **End-to-end operator path:** [docs/e2e-operator-path.md](docs/e2e-operator-path.md)
- **Continue samples:** [vscode-continue/README.md](vscode-continue/README.md)
- **Security:** [SECURITY.md](SECURITY.md)
- **Product / requirements (normative):** [docs/claudia-gateway.plan.md](docs/claudia-gateway.plan.md)

## Development roadmap

| Version | Where to read |
|---------|---------------|
| **v0.1** | [Working notes](docs/version-v0.1.md); [Go + BiFrost migration plan](docs/go-bifrost-migration-plan.md) |
| **v0.2+** | [Release roadmap](docs/claudia-gateway.plan.md#release-roadmap) in [docs/claudia-gateway.plan.md](docs/claudia-gateway.plan.md) |

The plan file still describes the original LiteLLM + Compose product shape for requirements; the in-tree implementation is **Go + BiFrost** as documented above and in `docs/`.

## License

Private / unspecified — add a `LICENSE` if you publish.
