# Claudia Gateway — operator documentation

Current release line: **v0.2.x** (RAG, ingest, indexer, operator UI). Summary of shipped work: [**releases-v0.2.x.md**](releases-v0.2.x.md).

| Document | Description |
|----------|-------------|
| [releases-v0.2.x.md](releases-v0.2.x.md) | What shipped in **v0.2.0**, **v0.2.1**, and **v0.2.2** |
| [overview.md](overview.md) | What the gateway and BiFrost do; stack capabilities |
| [network.md](network.md) | Local process layout, ports, traffic flow |
| [installation.md](installation.md) | Toolchains, `make claudia-install` / `make install`, BiFrost/Qdrant binaries, `claudia` build |
| [configuration.md](configuration.md) | Gateway config files, env vars, reload semantics |
| [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md) | Phased plan history: Go, BiFrost, packaging, GUI |
| [supervisor.md](supervisor.md) | `claudia serve`: BiFrost subprocess + Go gateway |
| [packaging.md](packaging.md) | GoReleaser releases, artifacts, **`claudia -version`** |
| [gui-testing.md](gui-testing.md) | Desktop webview (`-tags desktop`), manual checklist, build deps |
| [e2e-operator-path.md](e2e-operator-path.md) | Install → first **`curl`** chat |
| [../SECURITY.md](../SECURITY.md) | Tokens, logging redaction, local attack surface |

Normative product requirements: [claudia-gateway.plan.md](claudia-gateway.plan.md).
