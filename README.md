# Claudia Gateway

**v0.1** — TypeScript gateway in front of **LiteLLM**: one OpenAI-compatible URL, virtual model **`Claudia-<semver>`**, YAML **tokens** and **routing policy** with mtime reload, **sequential fallback** on 429/5xx, and **`GET /health`** (LiteLLM probe only). **Qdrant** is included in Compose for **v0.2** RAG readiness; the gateway does not use it in v0.1.

## Version roadmap

| Version | Where to read |
|---------|----------------|
| **v0.1** | [Working notes & explorations](docs/version-v0.1.md) — current focus, friction, and optional spikes. |
| **v0.2**, **v0.3**, **v0.4**, **v0.5**, **v0.7**, **v0.8** | [Release roadmap](docs/claudia-gateway.plan.md#release-roadmap) and full spec in [`docs/claudia-gateway.plan.md`](docs/claudia-gateway.plan.md). |

## Quick start

```bash
cp env.example .env
# Edit .env: LITELLM_MASTER_KEY and provider keys
cp config/tokens.example.yaml config/tokens.yaml   # required; edit token / tenant_id
docker compose up -d --build
curl -sS http://localhost:3000/health
```

## Documentation

- Operator docs index: [docs/README.md](docs/README.md)
- **v0.1** (toward release): [docs/version-v0.1.md](docs/version-v0.1.md)
- **Other versions** — scope and requirements: [docs/claudia-gateway.plan.md](docs/claudia-gateway.plan.md) (see [Release roadmap](docs/claudia-gateway.plan.md#release-roadmap))
- Continue samples: [vscode-continue/README.md](vscode-continue/README.md)

## Development

```bash
npm install
npm run dev
```

Set `config/gateway.yaml` `litellm.base_url` to your running LiteLLM and export **`LITELLM_MASTER_KEY`**.

## License

Private / unspecified — add a `LICENSE` if you publish.
