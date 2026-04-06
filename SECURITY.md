# Security notes — Claudia Gateway (Go)

This document summarizes **operator-relevant** security properties of the **Go** `claudia` binary. It is not a formal audit.

## Secrets and logging

- **Gateway tokens** (`Authorization: Bearer …` from clients) are **not** written to logs in full. HTTP access logs record a **redacted** form (`Bearer abcd…` for long tokens, `Bearer ***` for short values). See `internal/server/server.go` (`redactAuth`, `loggingMiddleware`).
- **Upstream API keys** (environment variable named by `upstream.api_key_env`, usually `CLAUDIA_UPSTREAM_API_KEY`) are **not** logged when set. Debug logs may include **upstream base URL** and paths (no key material).
- **`tokens.yaml`** reload logs include **path** and **token count**, not individual token strings (`internal/tokens/tokens.go`).
- **Chat request logs** include `clientModel`, `stream`, and `tenant` (from the validated gateway token), not message bodies.

Operators should still treat logs as sensitive (tenant IDs, model names, URLs) and restrict log storage and retention accordingly.

## Local attack surface

- The gateway listens on **`gateway.listen_host`:`listen_port`** from YAML (default **`0.0.0.0:3000`**). Anything that can reach that port can call **`/v1/*`** with a valid gateway token or probe **`/health`**. Use host firewall rules, bind to loopback, or a reverse proxy with TLS where appropriate.
- **`GET /status`** (no auth, like **`/health`**) returns JSON including **effective listen address**, **upstream base URL**, **upstream probe result**, and when **`claudia serve`** was used, **BiFrost/Qdrant listen hints**. Do not expose the gateway port to untrusted networks if that metadata is sensitive.
- **TLS termination** is **not** built into `claudia`. Use a sidecar or front proxy for HTTPS in production-style deployments.
- **Config files** (`gateway.yaml`, `tokens.yaml`, `routing-policy.yaml`) hold **gateway tokens** and routing rules. Use filesystem permissions (e.g. `chmod 600`) and avoid committing real `tokens.yaml` to version control.

## `claudia serve` (supervisor)

- Spawns **BiFrost** and optionally **Qdrant** as child processes. The effective user can **execute** whatever **`-bifrost-bin`** / **`-qdrant-bin`** point to; use trusted binaries and paths.
- Environment is inherited from the parent process (`MergeEnv`); avoid passing untrusted env into the supervisor process.
- BiFrost **config** is copied into the data directory; keep **`bifrost.config.json`** permissions tight on disk.

## Reporting

If you find a security issue in this repository, contact the maintainer privately (add contact method when the project is public).
