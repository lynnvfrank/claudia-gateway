# Claudia Gateway

**v0.1** — TypeScript gateway in front of **LiteLLM**: one OpenAI-compatible URL, virtual model **`Claudia-<semver>`**, YAML **tokens** and **routing policy** with mtime reload, **sequential fallback** on 429/5xx, and **`GET /health`** (LiteLLM probe only). **Qdrant** is included in Compose for **v0.2** RAG readiness; the gateway does not use it in v0.1.

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
- Product / roadmap: [docs/claudia-gateway.plan.md](docs/claudia-gateway.plan.md)
- Continue samples: [vscode-continue/README.md](vscode-continue/README.md)

## Development

```bash
npm install
npm run dev
```

Set `config/gateway.yaml` `litellm.base_url` to your running LiteLLM and export **`LITELLM_MASTER_KEY`**.

## License

Private / unspecified — add a `LICENSE` if you publish.
