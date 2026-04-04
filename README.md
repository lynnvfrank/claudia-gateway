# Claudia Gateway

**v0.1** — OpenAI-compatible gateway in front of **BiFrost** (default in Compose) or **LiteLLM**: virtual model **`Claudia-<semver>`**, YAML **tokens** and **routing policy** with mtime reload, **sequential fallback** on 429/5xx, and **`GET /health`**. The **primary implementation** is the **Go** binary (**`go build -o claudia ./cmd/claudia`**) using **`config/gateway.yaml`**, **`tokens.yaml`**, and **`routing-policy.yaml`** (see [configuration.md](docs/configuration.md#go-gateway-binary)). **Qdrant** is in Compose for **v0.2** RAG; the gateway does not call it in v0.1. Packaging and GUI phases: [`docs/go-bifrost-migration-plan.md`](docs/go-bifrost-migration-plan.md).

**TypeScript gateway sunset:** The **Node** server under **`src/`** remains only for the **Docker Compose `claudia` image** (`Dockerfile`). **New deployments** should use **Go** (`claudia` / `claudia serve`). Migration: [docs/operator-migration-to-go.md](docs/operator-migration-to-go.md). Security notes: [SECURITY.md](SECURITY.md).

## Version roadmap

| Version | Where to read |
|---------|----------------|
| **v0.1** | [Working notes & explorations](docs/version-v0.1.md); [Go + BiFrost migration plan](docs/go-bifrost-migration-plan.md) (phased delivery, discovery → packaging) |
| **v0.2**, **v0.3**, **v0.4**, **v0.5**, **v0.7**, **v0.8** | [Release roadmap](docs/claudia-gateway.plan.md#release-roadmap) and full spec in [`docs/claudia-gateway.plan.md`](docs/claudia-gateway.plan.md). |

## Quick start

```bash
cp env.example .env
# Edit .env: CLAUDIA_UPSTREAM_API_KEY (BiFrost placeholder ok), provider keys for BiFrost/LiteLLM
cp config/tokens.example.yaml config/tokens.yaml   # required; edit token / tenant_id
docker compose up -d --build
curl -sS http://localhost:3000/health
```

## Documentation

- Operator docs index: [docs/README.md](docs/README.md)
- **v0.1** (toward release): [docs/version-v0.1.md](docs/version-v0.1.md)
- **Go rewrite + BiFrost + packaging + GUI:** [docs/go-bifrost-migration-plan.md](docs/go-bifrost-migration-plan.md); releases: [docs/packaging.md](docs/packaging.md); desktop shell: [docs/gui-testing.md](docs/gui-testing.md)
- **Other versions** — scope and requirements: [docs/claudia-gateway.plan.md](docs/claudia-gateway.plan.md) (see [Release roadmap](docs/claudia-gateway.plan.md#release-roadmap))
- Continue samples: [vscode-continue/README.md](vscode-continue/README.md)

## Development

```bash
npm install
npm run dev
```

Set `config/gateway.yaml` `litellm.base_url` (e.g. **BiFrost** `http://localhost:8080` on the host) and export **`CLAUDIA_UPSTREAM_API_KEY`** (or **`LITELLM_MASTER_KEY`** if `api_key_env` still points there).

**Shortcuts** (repo root):

| Goal | Make | npm |
|------|------|-----|
| Build Go `claudia` | `make claudia-build` | `npm run go:build` |
| Build Fyne **`claudia-gui`** (Phase 5) | `make claudia-gui-build` | `npm run gui:build` |
| Run Go gateway | `make claudia-run` | `npm run go:run` |
| Go gateway + BiFrost **process** (Phase 3) | `make claudia-serve` | `npm run go:serve` |
| Build BiFrost from **`$HOME/src/bifrost`** → **`bin/bifrost-http`** | `make bifrost-from-src` | `npm run bifrost:from-src` |
| Same, using local **`bifrost-http`** | `make claudia-serve-local` | `npm run go:serve:local` |
| Local **Qdrant** binary (pinned release) | `make qdrant-from-release` | `npm run qdrant:from-release` |
| **Qdrant** + **`bifrost-http`** + gateway | `make claudia-serve-stack` | `npm run go:serve:stack` |
| Run BiFrost (Compose, foreground) | `make bifrost` | `npm run bifrost` |
| Run BiFrost (background) | `make bifrost-d` | `npm run bifrost:d` |
| Stop BiFrost | `make bifrost-down` | `npm run bifrost:down` |

`make help` lists the same targets. For Go, set **`CLAUDIA_UPSTREAM_API_KEY`** and **`config/tokens.yaml`** as for the Node server; point **`litellm.base_url`** at **`http://127.0.0.1:8080`** when BiFrost runs via Compose on the host. For **BiFrost as a subprocess** with the Go gateway, use **`claudia serve`** — see [docs/supervisor.md](docs/supervisor.md).

### Go gateway (v0.1 parity)

The **`claudia`** binary uses **`config/gateway.yaml`** (via **`-config`** or **`CLAUDIA_GATEWAY_CONFIG`** or default **`./config/gateway.yaml`**), the same **`tokens.yaml`** / **`routing-policy.yaml`** paths, **`LOG_LEVEL`**, and the upstream key named by **`litellm.api_key_env`**. Routes: **`/`**, **`GET /health`**, **`GET /v1/models`**, **`POST /v1/chat/completions`** (virtual model, routing, fallback, BiFrost **`/api/models`** catalog — same as TypeScript). Module path **`github.com/lynn/claudia-gateway`**; change **`go.mod`** if your clone uses another import path.

```bash
go build -o claudia ./cmd/claudia
export CLAUDIA_UPSTREAM_API_KEY=bifrost-local-dummy
./claudia
# Optional: ./claudia -config /path/to/gateway.yaml -listen :3001

# One command: BiFrost child process + gateway (install bifrost-http on PATH as `bifrost`, or use -bifrost-bin)
./claudia serve
# From a local BiFrost clone: make bifrost-from-src && make claudia-serve-local
```

**Compose image:** The **`claudia`** service still builds **Node/TypeScript** from **`Dockerfile`**; switching that image to Go is a follow-up. Host-side or release-binary deploys should use **Go** only; same YAML for both. See [operator-migration-to-go.md](docs/operator-migration-to-go.md).

Automated checks: **`./scripts/smoke-go-gateway.sh`** (`gofmt`, `go vet`, **`go test ./... -race`**).

## License

Private / unspecified — add a `LICENSE` if you publish.
