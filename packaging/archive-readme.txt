Claudia release archive — quick start

1. Copy env.example to .env next to claudia (or claudia.exe) and set API keys.
2. First run: start claudia — it listens on localhost and opens setup to create config/tokens.yaml (or copy tokens.example.yaml to tokens.yaml yourself).
3. BiFrost (bifrost-http) is not in this zip (license/CGO). Options:
   — Full local stack with UI: from a dev checkout run  make release-package  (bundles bifrost-http + qdrant + desktop claudia + this config layout).
   — Or install BiFrost separately, then run:
       claudia serve -bifrost-bin /path/to/bifrost-http
     qdrant is included next to claudia; the supervisor auto-detects it when the binary sits in the same folder.
4. Gateway-only (remote BiFrost):  claudia gateway
5. With a desktop build (-tags desktop), no arguments start the supervisor + web UI; use --headless for no window.

See PACKAGING.md and docs/supervisor.md.
