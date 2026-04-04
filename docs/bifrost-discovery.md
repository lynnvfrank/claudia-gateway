# BiFrost discovery (Phase 0 archive)

This document records **Phase 0** of [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md): running **Maxim BiFrost** as the OpenAI-compatible upstream for Claudia. The gateway implementation is **Go** only; use **`claudia`** or **`claudia serve`** with a local **`bifrost-http`** binary (see [supervisor.md](supervisor.md)).

**Operator verification (2026-04-03).** BiFrost was exercised with the repo configuration; **VS Code** (OpenAI-compatible client → **Claudia** at the gateway base URL) was used for real chat.

---

## How we run BiFrost today

| Item | Value |
|------|--------|
| **Binary** | **`bifrost-http`** — e.g. **`make bootstrap-deps`** or **`make bifrost-from-src`** → **`./bin/bifrost-http`**; pins in **`deps.lock`**, or **`claudia serve`** supervises it |
| **Listen** | Default **`127.0.0.1:8080`** (`claudia serve` flags: **`-bifrost-bind`**, **`-bifrost-port`**) |
| **Health** | `GET http://127.0.0.1:8080/health` |
| **Bootstrap config** | Repo **`config/bifrost.config.json`** — copied into BiFrost data dir as **`config.json`** when using **`claudia serve`** |

**Minimal bring-up**

```bash
export CLAUDIA_UPSTREAM_API_KEY=bifrost-local-dummy
export GROQ_API_KEY=...   # per bifrost.config.json
make bootstrap-deps       # once — versions from deps.lock; or: make bifrost-from-src
make claudia-serve-local
```

---

## Minimal BiFrost configuration

1. **`config/bifrost.config.json`** — providers and **`env.VAR_NAME`** key references (no raw secrets in JSON).
2. **Environment** — **`GROQ_API_KEY`**, **`GEMINI_API_KEY`**, etc., in the shell (or **`.env`**) when starting **`claudia`** / **`claudia serve`**.
3. **Optional:** `./scripts/list-bifrost-models.sh` or `curl` **`/api/models?unfiltered=true&limit=500`** on BiFrost.

---

## Gateway configuration — Claudia → BiFrost

Gateway YAML uses **`upstream.*`** for the OpenAI-compatible hop (BiFrost or any compatible proxy). Legacy **`litellm`** / **`health.litellm_url`** keys are still accepted when the corresponding **`upstream`** / **`health.upstream_url`** fields are omitted.

| Field | Role |
|-------|------|
| **`upstream.base_url`** | Upstream root. Local default **`http://127.0.0.1:8080`**. **`claudia serve`** overrides this to match the supervised BiFrost. |
| **`upstream.api_key_env`** | Env var for **`Authorization: Bearer`** on upstream **`/v1/*`**. Default **`CLAUDIA_UPSTREAM_API_KEY`**. |
| **`routing.fallback_chain`** | Ordered BiFrost model ids as **`provider/model`**. |
| **`paths.tokens`** / **`paths.routing_policy`** | Gateway auth and routing policy. |

---

## Compatibility notes (BiFrost behind Claudia)

| Area | Behavior |
|------|----------|
| **Chat** | `POST /v1/chat/completions`; streaming SSE pass-through |
| **Model list** | Gateway prefers **`GET /api/models?unfiltered=true&limit=500`**, maps to **`provider/name`**, prepends **`Claudia-<semver>`** |
| **Health** | **`GET {base_url}/health`** — JSON **`checks.upstream`** reflects upstream probe |
| **Fallback** | **429** / selected **5xx** walk **`routing.fallback_chain`** |

---

## References

- [configuration.md](configuration.md)
- [supervisor.md](supervisor.md)
- [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md)
- Upstream: [BiFrost docs](https://docs.getbifrost.ai/)
