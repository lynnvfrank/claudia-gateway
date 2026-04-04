# BiFrost discovery — TypeScript Claudia Gateway (Phase 0)

This document records **Phase 0** of [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md): running **Maxim BiFrost** as the OpenAI-compatible upstream for the **existing** Node/Fastify gateway, without a Go rewrite. It is the artifact the migration plan calls **`docs/bifrost-discovery.md`**.

**Operator verification (2026-04-03).** BiFrost was started with the repo configuration; **VS Code** (OpenAI-compatible client → **Claudia** at the gateway base URL, not direct-to-BiFrost) was used for real chat. Completions and model discovery behaved comparably to a **LiteLLM** upstream: same client integration surface, **fewer moving parts** when LiteLLM/Postgres are omitted, and **faster cold start** than bringing up the full LiteLLM stack.

---

## BiFrost version and how we run it

| Item | Value |
|------|--------|
| **Image** | `maximhq/bifrost:latest` (see `docker-compose.yml` service `bifrost`) |
| **Recommendation** | Pin by **image digest** in production so discovery stays reproducible; `:latest` is fine for local dev. |
| **Listen** | `APP_HOST=0.0.0.0`, `APP_PORT=8080` (Compose env). Published as **host `8080`**. |
| **Health** | `GET http://<bifrost-host>:8080/health` — used by Compose `healthcheck` (`wget` to `/health`). |
| **Bootstrap config file** | Host `./config/bifrost.config.json` → container `/app/data/config.json` (read-only mount). |

**Minimal Compose bring-up (BiFrost + Claudia only).** From the repo root, with `.env` supplying provider keys and a gateway token file present:

```bash
docker compose up -d --build bifrost claudia
```

Omit `litellm` and `postgres` when you do not need LiteLLM; the gateway’s default `config/gateway.yaml` already points at `http://bifrost:8080`.

---

## Minimal BiFrost configuration (real completion + model listing)

1. **`config/bifrost.config.json`** — defines providers and key references (no raw secrets in JSON):

   - Keys use **`env.VAR_NAME`** (e.g. `env.GROQ_API_KEY`, `env.GEMINI_API_KEY`).
   - **Do not** set `"models": ["*"]` expecting a wildcard; BiFrost treats `*` as a literal model id. Omit `models` or use `[]` on a key to allow the provider catalog (see [configuration.md](configuration.md)).

2. **Environment** (Compose passes these into the `bifrost` service):

   - **`GROQ_API_KEY`**, **`GEMINI_API_KEY`** (and any other providers you add to the JSON).

3. **Direct smoke (optional).** Against BiFrost on the host:

   ```bash
   ./scripts/list-bifrost-models.sh
   # or: curl -sS "http://127.0.0.1:8080/api/models?unfiltered=true&limit=500"
   ```

Provider secrets live **only in BiFrost** (env + config file); the gateway does not read `GROQ_API_KEY` / `GEMINI_API_KEY`.

---

## Gateway configuration — Claudia → BiFrost

Claudia uses **`litellm.*` and `health.*` names** in YAML for historical reasons; values aim at **any** OpenAI-compatible upstream, including BiFrost.

### `config/gateway.yaml` (subset that matters for BiFrost)

| Field | Role |
|-------|------|
| **`litellm.base_url`** | Upstream root, **no** trailing slash required. In Compose: **`http://bifrost:8080`**. On host dev (gateway on host, BiFrost on host): **`http://localhost:8080`**. |
| **`litellm.api_key_env`** | Process env var whose value is sent as **`Authorization: Bearer <value>`** on upstream `/v1/*` and on the catalog/health probes as implemented today. Default: **`CLAUDIA_UPSTREAM_API_KEY`**. |
| **`health.timeout_ms`** | Used for upstream **health** probe and for **`GET /v1/models`** aggregation timeout (default **5000** ms). |
| **`health.chat_timeout_ms`** | Per-attempt timeout for **`POST /v1/chat/completions`** to upstream (default **300000** ms). |
| **`health.litellm_url`** | Optional. If omitted, defaults to **`{litellm.base_url}/health`**. |
| **`gateway.semver`** | Builds virtual model id **`Claudia-<semver>`** (e.g. `Claudia-0.1.0`). |
| **`routing.fallback_chain`** | Ordered **BiFrost model ids** as **`provider/model`** (must match catalog ids). |
| **`paths.tokens`** / **`paths.routing_policy`** | Gateway auth and routing policy (unchanged from LiteLLM mode). |

### Environment

| Variable | BiFrost notes |
|----------|----------------|
| **`CLAUDIA_UPSTREAM_API_KEY`** | Must be **non-empty** if `api_key_env` points here. For BiFrost **without** governance virtual keys, a **placeholder** is enough (e.g. Compose default `bifrost-local-dummy`). |
| **`CLAUDIA_GATEWAY_CONFIG`** | Optional path to `gateway.yaml` (Compose sets `/app/config/gateway.yaml`). |

### `config/tokens.yaml`

Clients (VS Code / Continue) use **`Authorization: Bearer <gateway token>`** to hit **`/v1/chat/completions`** and **`/v1/models`**. That is **independent** of the upstream Bearer sent to BiFrost.

---

## Compatibility matrix — BiFrost vs LiteLLM (via Claudia)

Observations below assume **Claudia** is in front; they describe what the gateway implements today (`src/chat.ts`, `src/litellm.ts`, `src/server.ts`).

| Area | BiFrost (observed / by design) | LiteLLM | Notes |
|------|-------------------------------|---------|--------|
| **Chat** | `POST /v1/chat/completions` with JSON body; **`stream: true`** returns SSE-style stream | Same shape | Gateway forwards **`Authorization: Bearer`**, **`Content-Type: application/json`**; streaming **pass-through** of upstream body with `Content-Type`, `Cache-Control`, `Connection`, optional `x-request-id`. |
| **Non-streaming** | JSON response body passed through | Same | Operator verified end-to-end via **VS Code → gateway**. |
| **Streaming** | Same pass-through path | Same | Same code path; treat as **supported** unless you hit a provider-specific edge case (document any new failure with request id + model). |
| **Model list** | Gateway prefers **`GET /api/models?unfiltered=true&limit=500`**, maps `{ provider, name }` → id **`provider/name`**, prepends virtual **`Claudia-<semver>`** | If catalog route missing or unusable, falls back to **`GET /v1/models`** | BiFrost’s raw **`GET /v1/models`** may expose entries like `groq/*`; the catalog call avoids that for clients that need concrete ids. |
| **Upstream auth** | Bearer sent; often **ignored** unless governance keys enabled | **`LITELLM_MASTER_KEY`** must match when proxy enforces key | See [configuration.md](configuration.md). |
| **Health** | **`GET {base_url}/health`** — gateway sends Bearer if key set; BiFrost **`/health`** is typically fine either way | LiteLLM may **require** Bearer on `/health` | Claudia **`GET /health`**: **200** `{ "status": "ok", "checks": { "litellm": { "ok": true, "status": … } } }` if upstream OK; **503** `degraded` if probe fails. Field name remains **`litellm`** in JSON for compatibility. |
| **Errors** | Upstream JSON/text surfaced on non-retryable failures; **429 / 5xx** on virtual model may trigger **fallback chain** | Similar | Retry set: **429, 500, 502, 503, 504** (`src/chat.ts`). |
| **Timeouts** | `health.timeout_ms`, `health.chat_timeout_ms` | Same config keys | AbortController **abort** on upstream fetch. |
| **Dependencies** | BiFrost container + provider env keys | Often Postgres + LiteLLM image + `litellm_config.yaml` | BiFrost-only path drops DB + second proxy for local dev. |

---

## Go migration implications (for later phases)

When porting to Go, preserve at least:

1. **Routes:** `GET /health`, `GET /v1/models`, `POST /v1/chat/completions` (and gateway token behavior on `/v1/*`).
2. **BiFrost model listing:** Prefer **`GET /api/models?unfiltered=true&limit=500`** with the same **provider/name → `provider/name`** mapping before falling back to **`GET /v1/models`**.
3. **Headers to upstream:** `Authorization: Bearer`, `Content-Type: application/json` on chat; Bearer on catalog and health probes as today.
4. **Streaming:** Raw byte pass-through of upstream SSE (no buffering of full completion in the common case).
5. **Virtual model + fallback:** Port `routing-policy.yaml` evaluation and **`chatWithVirtualModelFallback`** semantics (Phase 2 scope; this doc only lists dependencies).
6. **Naming debt:** YAML fields **`litellm.*`** / **`checks.litellm`** are really **“upstream OpenAI proxy”**; a Go config could rename for clarity while accepting the same env concepts.

---

## Definition of done (Phase 0)

Use this checklist when closing Phase 0 in [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md).

- [x] BiFrost **version/image** and **run method** recorded (this doc + Compose).
- [x] **Minimal BiFrost config** for chat + models documented (`bifrost.config.json` + env).
- [x] **Gateway YAML/env** for BiFrost upstream documented (`litellm.base_url` → BiFrost).
- [x] **Compatibility matrix** (streaming, auth, health, models, timeouts, deps vs LiteLLM).
- [x] **Go migration** notes (endpoints, catalog route, headers, streaming).
- [x] **Operator smoke:** VS Code through **Claudia** (not only direct-to-BiFrost) **succeeded** for real usage (2026-04-03).
- [ ] **Optional `curl` receipts:** add pasted outputs if you want CI-independent evidence (status lines only; no secrets).

### Optional `curl` examples (gateway on `localhost:3000`)

Replace `GATEWAY_TOKEN` with a token from `config/tokens.yaml`.

```bash
curl -sS http://127.0.0.1:3000/health | python3 -m json.tool
curl -sS -H "Authorization: Bearer GATEWAY_TOKEN" http://127.0.0.1:3000/v1/models | python3 -m json.tool
curl -sS http://127.0.0.1:3000/v1/chat/completions \
  -H "Authorization: Bearer GATEWAY_TOKEN" -H "Content-Type: application/json" \
  -d '{"model":"Claudia-0.1.0","messages":[{"role":"user","content":"Say hi in one word."}],"stream":false}'
```

---

## References

- [configuration.md](configuration.md) — full gateway + BiFrost YAML/env reference.
- [docker-compose.yml](../docker-compose.yml) — `bifrost` and `claudia` services.
- [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md) — phased migration.
- Upstream docs: [BiFrost](https://docs.getbifrost.ai/) (gateway setup, API).
