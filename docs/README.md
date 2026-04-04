# Claudia Gateway — operator documentation (v0.1)

| Document | Description |
|----------|-------------|
| [overview.md](overview.md) | What the gateway, LiteLLM, and Qdrant do; v0.1 vs later versions |
| [network.md](network.md) | Compose topology, DNS, published ports, traffic flow |
| [installation.md](installation.md) | Prerequisites, `.env`, first `docker compose up`, health checks |
| [docker-commands.md](docker-commands.md) | Common `docker compose` / `docker` commands |
| [configuration.md](configuration.md) | All gateway config files, env vars, reload semantics |
| [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md) | Phased plan: Go rewrite, BiFrost, cross-platform packaging, GUI |
| [supervisor.md](supervisor.md) | `claudia serve`: BiFrost subprocess + Go gateway (Phase 3) |
| [packaging.md](packaging.md) | GoReleaser releases, artifacts, **`claudia -version`** (Phase 4) |
| [gui-testing.md](gui-testing.md) | Fyne **`claudia-gui`**, manual checklist, build deps (Phase 5) |
| [operator-migration-to-go.md](operator-migration-to-go.md) | Moving from Docker/TypeScript to the Go binary (Phase 6) |
| [e2e-operator-path.md](e2e-operator-path.md) | Documented path: install → first **`curl`** chat |
| [../SECURITY.md](../SECURITY.md) | Tokens, logging redaction, local attack surface (Phase 6) |

Normative product requirements: [claudia-gateway.plan.md](claudia-gateway.plan.md).

External LiteLLM references:

- [LiteLLM documentation hub](https://docs.litellm.ai/docs/)
- [LiteLLM Proxy — deploy / Docker](https://docs.litellm.ai/docs/proxy/deploy)
