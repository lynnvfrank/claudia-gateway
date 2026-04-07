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

**Creates (only if absent):** **`.env`** from **`env.example`**, **`config/tokens.yaml`** from **`config/tokens.example.yaml`**. Does not overwrite existing files. Afterward edit values to match your providers and **`config/gateway.yaml`**.

| Purpose | File | Role |
| ------- | ---- | ---- |
| Process environment | `.env`<br/>(copied from [env.example](env.example)) | Optional local environment variable file to set Claudia<->BiFrost key and API keys. Not committed. |
| Gateway client auth | `config/tokens.yaml`<br/>(copied from [config/tokens.example.yaml](config/tokens.example.yaml)) | Tokens for you and other users to use to authenticate with Claudia Gateway. Not committed. |
| Gateway listen + upstream | `config/gateway.yaml` | Claudia Gateway's primary configuration file to connect the Client<->Claudia<->BiFrost  |
| BiFrost bootstrap | `config/bifrost.config.json` | BiFrost HTTP config; provider secrets pulled from environment variables set in `.env` or the shell. |
| Virtual model mapping | `config/routing-policy.yaml` | Rules that define how the virtual `Claudia-<semver>` model routes prompts/turns to underlying models |

**Manual follow-up**

1. **`.env`:** set `CLAUDIA_UPSTREAM_API_KEY` to match `upstream.api_key_env` in `config/gateway.yaml`. Set `GROQ_API_KEY`, `GEMINI_API_KEY`, or other keys that `config/bifrost.config.json` references.
2. **`config/tokens.yaml`:** at least one gateway token; clients use `Authorization: Bearer <token>`.
3. **`config/gateway.yaml`** — starter in repo; adjust `listen_host` / `listen_port`, `upstream.base_url`, `routing.fallback_chain`, paths if needed.
4. **`config/bifrost.config.json`** — align provider blocks and `env.*` with your **`.env`**.
5. **`config/routing-policy.yaml`** — committed default; edit or point `gateway.yaml` at another file.

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

## Graphic User Interface (gui)

| Target | What it does |
| ------ | ------------ |
| **`make gui-install`** | Installs native dependencies for the Fyne GUI (**Ubuntu/Debian** incl. 14.04+ via **`apt-get`**, **macOS** Xcode CLT via **`xcode-select --install`**, **Windows 11** **`winget`** MSYS2 + printed **`pacman`** steps). May prompt for **`sudo`** or admin. |
| **`make gui-build`** | **`CGO_ENABLED=1 go build`** from **`gui/`** into **`./claudia-gui`** (or **`./claudia-gui.exe`** on Windows); requires a Fyne-capable toolchain (see [docs/gui-testing.md](docs/gui-testing.md)). |
| **`make gui-run`** | Runs **`gui-build`** if the binary is missing, then runs **`./claudia-gui`** (or **`./claudia-gui.exe`** on Windows). |
| **`make vet-gui`** | **`go vet -C gui ./...`** with **CGO** enabled — same expectations as **`test-gui`**. |
| **`make test-gui`** | **`go test -C gui ./...`** with **CGO** — same native libraries as **`make gui-build`**. |

**`make precommit`** runs **`vet-gui`** and **`test-gui`** unless **`SKIP_GUI=1`**; details are in **Testing and Linting** below.

## Testing and Linting

| Target | What it does |
| ------ | ------------ |
| **`make fmt`** | **`gofmt -w`** on **`cmd`**, **`internal`**, and **`gui`**. |
| **`make fmt-check`** | Fails if **`gofmt`** would change any file (same check as CI). Used by **`precommit`**. |
| **`make vet-gateway`** | **`go vet ./...`** on the main module (no **`gui`**). Used by **`precommit`**. |
| **`make vet-gui`** | **`go vet -C gui ./...`** with **CGO** enabled — same toolchain expectations as **`test-gui`**. Used by **`precommit`** unless **`SKIP_GUI=1`**. |
| **`make test-gateway`** | **`go test ./...`** on the main module with **`-race`** on Unix; does not run **`gui`** tests. |
| **`make test-gui`** | **`go test -C gui ./...`** — requires **CGO** and the same native libraries as **`make gui-build`**. |
| **`make precommit`** | Runs **`fmt-check`**, **`vet-gateway`**, **`test-gateway`**, and **`vet-gui`** + **`test-gui`** unless **`SKIP_GUI=1`** (no Fyne/CGO, e.g. Windows without a GUI toolchain). On Windows/Git Bash, **`./scripts/precommit-smoke.sh`** runs **`precommit`** with **`SKIP_GUI=1`** by default; set **`FULL_GUI=1`** to include GUI checks. |

## Repo Management and Packaging

### Clean up built binaries

Remove **built gateway/GUI binaries** and release scratch output.

```bash
make clean
```

**Deletes** **`./claudia`**, **`./claudia-gui`**, Windows **`.exe`** variants, and **`dist/`**. Does **not** remove **`bin/bifrost-http`**, **`bin/qdrant`**, **`.deps/`**, **`run/`**, or **`logs/`**.

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
- **Makefile plan:** [makefile.plan.md](makefile.plan.md)
- **GUI:** [docs/gui-testing.md](docs/gui-testing.md)
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
