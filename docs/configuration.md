# Configuration reference

The gateway reads **YAML files** and **environment variables**. **`gateway.yaml`**, **`tokens.yaml`**, and **`routing-policy.yaml`** are reloaded when their file **modification time** changes (checked on incoming traffic).

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| **`LITELLM_MASTER_KEY`** | Yes (Compose) | Bearer token the gateway sends to LiteLLM. Name is configurable via `litellm.api_key_env` in `gateway.yaml` (default `LITELLM_MASTER_KEY`). |
| **`LOG_LEVEL`** | No | Pino level: `debug`, `info`, `warn`, `error`. Overrides typical default when set. |
| **`CLAUDIA_GATEWAY_CONFIG`** | No | Absolute path to `gateway.yaml` inside the container. Default: `/app/config/gateway.yaml` in Docker; on the host, default is `./config/gateway.yaml` relative to `process.cwd()`. |

Provider keys (**`GROQ_API_KEY`**, **`OPENAI_API_KEY`**, etc.) are **not** read by the gateway; they are consumed by **LiteLLM** per `config/litellm_config.yaml`.

## `config/gateway.yaml`

| Field | Description |
|-------|-------------|
| **`gateway.semver`** | Semantic version string used to build the virtual model id **`Claudia-<semver>`**. |
| **`gateway.listen_port` / `listen_host`** | HTTP bind address inside the container. |
| **`gateway.log_level`** | Suggested log level (use `LOG_LEVEL` env for a simple override). |
| **`litellm.base_url`** | LiteLLM root URL (no trailing slash required), e.g. `http://litellm:4000`. |
| **`litellm.api_key_env`** | Name of the process env var holding the LiteLLM master key. |
| **`health.litellm_url`** | Optional explicit URL for **`GET /health`** LiteLLM probe; default `{litellm.base_url}/health`. The probe sends **`Authorization: Bearer` + `LITELLM_MASTER_KEY`** so it matches proxies that require a key on `/health`. |
| **`health.timeout_ms`** | Timeout for the LiteLLM health request and for **`GET /v1/models`** upstream list (default **5000**). |
| **`health.chat_timeout_ms`** | Timeout for each upstream **`POST /v1/chat/completions`** attempt (default **300000**). |
| **`paths.tokens`** | Path to **`tokens.yaml`** (relative to `gateway.yaml`’s directory unless absolute). |
| **`paths.routing_policy`** | Path to **`routing-policy.yaml`**. |
| **`routing.fallback_chain`** | Ordered LiteLLM **model names** for **`Claudia-<semver>`** requests. On **429** / selected **5xx**, the gateway tries the next entry. |

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
| **`ambiguous_default_model`** | LiteLLM model name used when **no rule** matches (**#29**). |
| **`rules`** | Ordered list. Each rule may set **`when.min_message_chars`** (compared to the **last user** message length). First match wins; **`models[0]`** is the **initial** upstream model. Every id should appear in **`routing.fallback_chain`**. |

## `config/litellm_config.yaml`

LiteLLM proxy configuration (models, env-backed keys). Not parsed by Claudia; keep **`model_name`** values aligned with **`routing.fallback_chain`**.

## Docker bind mounts

See **`docker-compose.yml`** comments: host **`./config/*.yaml`** files are mounted read-only into **`/app/config/`** in the `claudia` service.

## Logging semantics (v0.1)

- **INFO**: each HTTP response (method, path, status, duration, redacted `Authorization` prefix).
- **INFO**: upstream chat probe summary (status, model, stream flag).
- **DEBUG**: routing rule match, config path resolution, reload events, LiteLLM relay details.
