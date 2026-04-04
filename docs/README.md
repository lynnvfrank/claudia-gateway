# Claudia Gateway — operator documentation (v0.1)

| Document | Description |
|----------|-------------|
| [overview.md](overview.md) | What the gateway and BiFrost do; v0.1 vs later versions |
| [network.md](network.md) | Local process layout, ports, traffic flow |
| [installation.md](installation.md) | Toolchains, `make bootstrap-deps`, BiFrost/Qdrant binaries, `claudia` build |
| [configuration.md](configuration.md) | Gateway config files, env vars, reload semantics |
| [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md) | Phased plan history: Go, BiFrost, packaging, GUI |
| [supervisor.md](supervisor.md) | `claudia serve`: BiFrost subprocess + Go gateway |
| [packaging.md](packaging.md) | GoReleaser releases, artifacts, **`claudia -version`** |
| [gui-testing.md](gui-testing.md) | Fyne **`claudia-gui`**, manual checklist, build deps |
| [e2e-operator-path.md](e2e-operator-path.md) | Install → first **`curl`** chat |
| [../SECURITY.md](../SECURITY.md) | Tokens, logging redaction, local attack surface |

Normative product requirements: [claudia-gateway.plan.md](claudia-gateway.plan.md).
