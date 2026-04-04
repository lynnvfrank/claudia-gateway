# Claudia Gateway

**v0.1** — OpenAI-compatible **Go** gateway in front of **BiFrost**: virtual model `Claudia-<semver>`, YAML **tokens** and **routing policy** with mtime reload, **sequential fallback** on 429/5xx, and `GET /health`. Optional **Qdrant** via `claudia serve` targets **v0.2** RAG; the gateway does not call it in v0.1.

## Installation

**Prerequisites**

- **Go** 1.22 or later ([Go section](docs/installation.md#go))
- **Node.js** 20 or later ([Node.js section](docs/installation.md#nodejs-20-or-later))
- **GNU Make** ([Make section](docs/installation.md#make))
- **BiFrost** at git ref `58076d50df0d48d47ad917da3f604cf787ec7708` (`maximhq/bifrost`, pinned in repo-root `deps.lock`)
- **Qdrant** at `v1.14.1` (pinned in repo-root `deps.lock`)

**Install BiFrost and Qdrant**

1. Clone the repository and `cd` into it.
2. Fetch BiFrost and download the pinned Qdrant binary:

   ```bash
   make bootstrap-deps   # → .deps/bifrost, ./bin/bifrost-http, ./bin/qdrant
   ```

Further explanation in [docs/installation.md](docs/installation.md).

## Configuration

| Purpose | File | Role |
| ------- | ---- | ---- |
| Process environment | `.env`<br/>(copied from [env.example](env.example)) | Optional local environment variable file to set Claudia<->BiFrost key and API keys. Not committed. |
| Gateway client auth | `config/tokens.yaml`<br/>(copied from [config/tokens.example.yaml](config/tokens.example.yaml)) | Tokens for you and other users to use to authenticate with Claudia Gateway. Not committed. |
| Gateway listen + upstream | `config/gateway.yaml` | Claudia Gateway's primary configuration file to connect the Client<->Claudia<->BiFrost  |
| BiFrost bootstrap | `config/bifrost.config.json` | BiFrost HTTP config; provider secrets pulled from environment variables set in in `.env` or the shell. |
| Virtual model mapping | `config/routing-policy.yaml` | Rules that define how the virtual `Claudia-<semver>` model routes prompts/turns to underlying models |

**Set up files**

1. Process environment - `.env`
   - Copy: `cp env.example .env`  
   - Edit: set `CLAUDIA_UPSTREAM_API_KEY` to match `upstream.api_key_env` in `config/gateway.yaml` (BiFrost often accepts any non-empty placeholder unless governance keys are enabled). Set `GROQ_API_KEY`, `GEMINI_API_KEY`, or other keys that `config/bifrost.config.json` references. Do not commit `.env`.

1. Gateway client auth - `config/tokens.yaml`  
   - Copy: `cp config/tokens.example.yaml config/tokens.yaml`  
   - Edit: set at least one gateway token and a strong secret; clients send it as `Authorization: Bearer <token>`. Adjust `tenant_id` if you use multiple tenants.

3. Gateway listen + upstream - `config/gateway.yaml`  
   - The repo includes a starter `config/gateway.yaml`. To build your own from the documented template: `cp config/gateway.example.yaml config/gateway.yaml`  
   - Edit: `listen_host` / `listen_port`, `upstream.base_url`, `routing.fallback_chain` (use BiFrost model ids: `provider/model`), and `paths.tokens` / `paths.routing_policy` if you move those files.

4. BiFrost bootstrap - `config/bifrost.config.json`  
   - No separate example file in-tree: adjust provider blocks and `env.*` names so they match variables you define in `.env` or the environment. Add or remove providers to match the models you list in `routing.fallback_chain`.

5. Virtual model mapping - `config/routing-policy.yaml`  
   - Committed default; edit rules for your virtual model and upstream behavior. If you use a copy under another name, point `paths.routing_policy` in `config/gateway.yaml` at that file.

The `claudia` binary resolves `gateway.yaml` via `-config`, `CLAUDIA_GATEWAY_CONFIG`, or default `./config/gateway.yaml` (working directory). It loads `.env` from the working directory when present. Keep `routing.fallback_chain` aligned with model ids BiFrost actually exposes (`provider/model`).

Full reference (env vars, reload semantics, field tables): [docs/configuration.md](docs/configuration.md).

## Execution

Run Claudia Gateway, BiFrost and Qdrant with the [Supervisor](docs/supervisor.md).

```bash
make claudia-serve-local
curl -sS http://127.0.0.1:3000/health
```

## Common commands and shortcuts

Run `make help` from the repo root to see the [make](Makefile) tasks.

| Goal | Command |
|------|---------|
| Build and download Bootstrap BiFrost + Qdrant | `make bootstrap-deps` |
| Build BiFrost from `$HOME/src/bifrost` (or `BIFROST_SRC`) → `bin/bifrost-http` | `make bifrost-from-src` |
| Download pinned Qdrant binary → `./bin/qdrant` | `make qdrant-from-release` |
| Build `claudia` | `make claudia-build` |
| Run Claudia Gateway | `make claudia-run` |
| Run Gateway + BiFrost | `make claudia-serve` |
| Run Gateway + BiFrost + Qdrant | `make claudia-serve-stack` |
| Build `claudia-gui` | `make claudia-gui-build` |
| Run GUI (builds if missing) | `make claudia-gui-run` |
| GoReleaser snapshot → `dist/` | `make release-snapshot` |

**Automated checks:** `./scripts/smoke-go-gateway.sh` (`gofmt`, `go vet`, `go test ./... -race`).

Module path `github.com/lynn/claudia-gateway`; change `go.mod` if your fork uses another import path.

## Documentation

- **Index:** [docs/README.md](docs/README.md)
- **Overview / ports:** [docs/overview.md](docs/overview.md), [docs/network.md](docs/network.md)
- **Installation:** [docs/installation.md](docs/installation.md)
- **Configuration:** [docs/configuration.md](docs/configuration.md)
- **Supervisor (`claudia serve`):** [docs/supervisor.md](docs/supervisor.md)
- **Packaging / releases:** [docs/packaging.md](docs/packaging.md)
- **GUI:** [docs/gui-testing.md](docs/gui-testing.md)
- **End-to-end operator path:** [docs/e2e-operator-path.md](docs/e2e-operator-path.md)
- **Continue samples:** [vscode-continue/README.md](vscode-continue/README.md)
- **Security:** [SECURITY.md](SECURITY.md)
- **Product / requirements (normative):** [docs/claudia-gateway.plan.md](docs/claudia-gateway.plan.md)

## Development roadmap

| Version | Where to read |
|---------|----------------|
| **v0.1** | [Working notes](docs/version-v0.1.md); [Go + BiFrost migration plan](docs/go-bifrost-migration-plan.md) |
| **v0.2+** | [Release roadmap](docs/claudia-gateway.plan.md#release-roadmap) in [docs/claudia-gateway.plan.md](docs/claudia-gateway.plan.md) |

The plan file still describes the original LiteLLM + Compose product shape for requirements; the in-tree implementation is **Go + BiFrost** as documented above and in `docs/`.

## License

Private / unspecified — add a `LICENSE` if you publish.
