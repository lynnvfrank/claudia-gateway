Claudia release archive — what you have

This zip/tar contains:
  • claudia (or claudia.exe) — headless build (no native desktop window)
  • qdrant (or qdrant.exe) — same folder as claudia; optional for local vector store
  • config/ — starter YAML (gateway, routing, bifrost, tokens example, provider allowlist)
  • env.example — copy to .env and add provider API keys

BiFrost (bifrost-http) is not in this archive. Point gateway.yaml upstream.base_url at a running BiFrost, or install BiFrost separately and run e.g.:
  claudia serve -bifrost-bin /path/to/bifrost-http [-qdrant-bin ./qdrant]

Quick start:
  1. Copy env.example to .env (same directory you run claudia from) and set keys.
  2. Edit config/gateway.yaml and config/bifrost.config.json for your upstream.
  3. Run ./claudia gateway  (gateway only, remote BiFrost)
     or ./claudia serve -bifrost-bin …  (supervisor; add -qdrant-bin ./qdrant if you use local Qdrant)
  4. Create config/tokens.yaml via the setup UI when prompted, or copy config/tokens.example.yaml.

Full detail: PACKAGING.md in this archive (and the project repository for deeper docs).

Maintainers / full local stack (BiFrost + desktop UI): build from a git checkout with make release-package (not included here).
