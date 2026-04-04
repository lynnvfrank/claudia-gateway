# Configuration reference

The gateway reads **YAML files** and **environment variables**. **`gateway.yaml`**, **`tokens.yaml`**, and **`routing-policy.yaml`** are reloaded when their file **modification time** changes (checked on incoming traffic).

## Go gateway binary

The **`claudia`** program (**`go build -o claudia ./cmd/claudia`**) reads:

- **Config path:** **`CLAUDIA_GATEWAY_CONFIG`**, or **`-config /path/to/gateway.yaml`**, or default **`./config/gateway.yaml`** (relative to the process working directory).
- **Listen address:** from **`gateway.listen_host`** and **`gateway.listen_port`**, unless overridden with **`-listen`** (e.g. **`:3001`** or **`host:port`**).
- **Log level:** **`gateway.log_level`** unless **`LOG_LEVEL`** is set (**`debug`**, **`info`**, **`warn`**, **`error`**); Go uses **`log/slog`** text logs on stdout.
- **Upstream:** **`upstream.base_url`**, **`upstream.api_key_env`**, **`health.*`**, **`routing.fallback_chain`**, **`paths.*`** — see tables below. The upstream is your OpenAI-compatible proxy (typically BiFrost). Omitted **`upstream`** fields fall back to deprecated **`litellm.*`** / **`health.litellm_url`** when present.
- **`.env`:** At startup, **`claudia`** loads an optional **`.env`** in the **process working directory** (via **`github.com/joho/godotenv`**). Missing file is normal when the environment is injected by your shell or service manager.

**`GET /health`** returns JSON including **`checks.upstream`** (upstream probe). **`GET /v1/models`** prepends the virtual **`Claudia-<semver>`** model, then merges BiFrost’s catalog when available. **`POST /v1/chat/completions`** validates the gateway Bearer token, applies routing for the virtual model, and walks the fallback chain on 429/selected 5xx.

To run **BiFrost as a local process** supervised by the same binary, use **`claudia serve`** — see [supervisor.md](supervisor.md).

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| **`CLAUDIA_UPSTREAM_API_KEY`** | Yes for typical BiFrost setups | Bearer token the gateway sends to its OpenAI-compatible upstream (`upstream.base_url`). Name is configurable via **`upstream.api_key_env`** in **`gateway.yaml`** (default **`CLAUDIA_UPSTREAM_API_KEY`**). For BiFrost without governance virtual keys, any non-empty placeholder works. |
| **`LOG_LEVEL`** | No | Log level for **`log/slog`**: `debug`, `info`, `warn`, `error`. Overrides **`gateway.log_level`** when set. |
| **`CLAUDIA_GATEWAY_CONFIG`** | No | Path to **`gateway.yaml`**. Default **`./config/gateway.yaml`** on the host. |

Provider keys (**`GROQ_API_KEY`**, **`GEMINI_API_KEY`**, **`OPENAI_API_KEY`**, etc.) are **not** read by the gateway; **BiFrost** (**`config/bifrost.config.json`**) consumes them.

**Model listing (BiFrost):** `GET /v1/models` on BiFrost alone may return entries like `groq/*`. The gateway first calls BiFrost’s **`GET /api/models?unfiltered=true&limit=500`**, maps each **`{ provider, name }`** to an OpenAI-style id **`provider/name`**, then prepends the virtual **`Claudia-<semver>`** model. If that route is missing, the gateway uses **`GET /v1/models`** only. See **`scripts/list-bifrost-models.sh`**.

## `config/gateway.yaml`

| Field | Description |
|-------|-------------|
| **`gateway.semver`** | Semantic version string used to build the virtual model id **`Claudia-<semver>`**. |
| **`gateway.listen_port` / `listen_host`** | HTTP bind address. |
| **`gateway.log_level`** | Suggested log level (use **`LOG_LEVEL`** env for a simple override). |
| **`upstream.base_url`** | OpenAI-compatible upstream root (no trailing slash required), e.g. **`http://127.0.0.1:8080`** for local BiFrost. |
| **`upstream.api_key_env`** | Name of the process env var holding the upstream Bearer token (e.g. **`CLAUDIA_UPSTREAM_API_KEY`**). |
| **`health.upstream_url`** | Optional explicit URL for **`GET /health`** upstream probe; default **`{upstream.base_url}/health`**. The probe sends **`Authorization: Bearer`** + that token when set (BiFrost’s **`/health`** is typically unauthenticated). Deprecated alias: **`health.litellm_url`**. |
| **`health.timeout_ms`** | Timeout for the upstream health request and for **`GET /v1/models`** upstream list (default **5000**). |
| **`health.chat_timeout_ms`** | Timeout for each upstream **`POST /v1/chat/completions`** attempt (default **300000**). |
| **`paths.tokens`** | Path to **`tokens.yaml`** (relative to **`gateway.yaml`**’s directory unless absolute). |
| **`paths.routing_policy`** | Path to **`routing-policy.yaml`**. |
| **`routing.fallback_chain`** | Ordered upstream **model ids** for **`Claudia-<semver>`** requests (BiFrost: **`provider/model`**). On **429** / selected **5xx**, the gateway tries the next entry. |

Reload: change file and **save** (mtime update). On reload, if token or policy **paths** change, those stores are re-opened.

## `config/tokens.yaml`

```yaml
tokens:
  - label: optional-human-name
    token: "secret-bearer-value"
    tenant_id: "tenant-slug"
```

- **`token`** — must match the client’s `Authorization: Bearer` value exactly.
- **`tenant_id`** — carried in logs today; **v0.2+** RAG scopes by tenant.

## `config/routing-policy.yaml`

| Field | Description |
|-------|-------------|
| **`ambiguous_default_model`** | Upstream model id used when **no rule** matches (**#29**). |
| **`rules`** | Ordered list. Each rule may set **`when.min_message_chars`** (compared to the **last user** message length). First match wins; **`models[0]`** is the **initial** upstream model. Every id should appear in **`routing.fallback_chain`**. |

## `config/bifrost.config.json`

BiFrost bootstrap file. Provider keys use **`env.VAR`** for secrets.

**Per-key `models`:** In BiFrost, an **empty** or **omitted** `models` list means the key may be used for **any** model for that provider (minus **`blacklisted_models`** if set). **`"models": ["*"]` is not a wildcard** — it is treated as the literal model name `*`, so chat requests for real model ids will fail with *no keys found that support model*. Use no `models` field (or `[]`) when you want full catalog access without enumerating models.

## Logging semantics (v0.1)

- **INFO**: each HTTP response (method, path, status, duration, redacted `Authorization` prefix).
- **INFO**: upstream chat probe summary (status, model, stream flag).
- **DEBUG**: routing rule match, config path resolution, reload events, upstream relay details.
