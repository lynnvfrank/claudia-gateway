# Claudia Gateway

**v0.1** — OpenAI-compatible **Go** gateway in front of **BiFrost**: virtual model **`Claudia-<semver>`**, YAML **tokens** and **routing policy** with mtime reload, **sequential fallback** on 429/5xx, and **`GET /health`**. Config: **`config/gateway.yaml`**, **`tokens.yaml`**, **`routing-policy.yaml`** ([configuration.md](docs/configuration.md)). **Qdrant** is optional via **`claudia serve`** for **v0.2** RAG; the gateway does not call it in v0.1.

Run **`claudia`** against a BiFrost you start yourself, or use **`claudia serve`** to supervise BiFrost (and optionally Qdrant) — [docs/supervisor.md](docs/supervisor.md). Security: [SECURITY.md](SECURITY.md).

## Version roadmap

| Version | Where to read |
|---------|----------------|
| **v0.1** | [Working notes](docs/version-v0.1.md); [Go + BiFrost plan](docs/go-bifrost-migration-plan.md) |
| **v0.2+** | [Release roadmap](docs/claudia-gateway.plan.md#release-roadmap) in [`docs/claudia-gateway.plan.md`](docs/claudia-gateway.plan.md) |

## Quick start (local Go + BiFrost)

```bash
cp env.example .env   # optional; edit keys
cp config/tokens.example.yaml config/tokens.yaml   # required; edit token / tenant_id
export CLAUDIA_UPSTREAM_API_KEY=bifrost-local-dummy
# Build BiFrost → ./bin/bifrost-http (see docs/supervisor.md), then:
make claudia-serve-local
curl -sS http://127.0.0.1:3000/health
```

Or run only the gateway if BiFrost is already on **`litellm.base_url`** in **`config/gateway.yaml`**:

```bash
make claudia-build && ./claudia
```

## Documentation

- Index: [docs/README.md](docs/README.md)
- Config: [docs/configuration.md](docs/configuration.md)
- Supervisor: [docs/supervisor.md](docs/supervisor.md)
- Releases: [docs/packaging.md](docs/packaging.md)
- GUI: [docs/gui-testing.md](docs/gui-testing.md)
- Continue samples: [vscode-continue/README.md](vscode-continue/README.md)

## Development

**Shortcuts** (repo root):

| Goal | Command |
|------|---------|
| Build **`claudia`** | `make claudia-build` |
| Build Fyne **`claudia-gui`** | `make claudia-gui-build` |
| Run gateway only | `make claudia-run` |
| Gateway + BiFrost subprocess | `make claudia-serve` |
| BiFrost from **`$HOME/src/bifrost`** → **`bin/bifrost-http`** | `make bifrost-from-src` |
| Serve with local **`./bin/bifrost-http`** | `make claudia-serve-local` |
| Download Qdrant binary | `make qdrant-from-release` |
| Qdrant + **`bifrost-http`** + gateway | `make claudia-serve-stack` |

`make help` lists the same targets.

The **`claudia`** binary uses **`config/gateway.yaml`** (**`-config`**, **`CLAUDIA_GATEWAY_CONFIG`**, or default **`./config/gateway.yaml`**), **`tokens.yaml`**, **`routing-policy.yaml`**, **`LOG_LEVEL`**, and the upstream key named by **`litellm.api_key_env`**. Routes: **`/`**, **`GET /health`**, **`GET /v1/models`**, **`POST /v1/chat/completions`**, **`GET /status`** (supervisor mode). Module path **`github.com/lynn/claudia-gateway`**; change **`go.mod`** if your clone uses another import path.

Automated checks: **`./scripts/smoke-go-gateway.sh`** (`gofmt`, `go vet`, **`go test ./... -race`**).

## License

Private / unspecified — add a `LICENSE` if you publish.
