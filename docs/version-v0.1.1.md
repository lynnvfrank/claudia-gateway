# Version 0.1.1 — implementation plan (agent brief)

**Product:** Claudia Gateway — OpenAI-compatible LLM gateway with **BiFrost** upstream.

**Audience:** Implementing agents should treat this doc as the **source of truth** for *what* v0.1.1 delivers, *constraints*, *order of work*, and *how to verify*.

---

## 1. Goals (what v0.1.1 implements)

| # | Goal | User-visible outcome |
|---|------|----------------------|
| G1 | **Local (Ollama) models in free-tier filtering** | When free-tier filtering is on, **Ollama** models from the live catalog can appear in merged model listing and in **generated** routing/fallback (same intersection rules as other providers). |
| G2 | **Smaller upstream `tools` payloads** | For virtual Claudia (and any path where the transformer runs), the gateway may **replace** the client `tools` array with a **subset** chosen by a **router model**, reducing TPM pressure from huge tool lists (e.g. VS Code Continue **agent** vs **plan** modes still send many tools; upstream sees fewer). |
| G3 | **Configurable router model + admin visibility** | Operators set **`router_models`** in config (first entry used today); desktop admin shows **resolved router model**, last use, and errors if missing from catalog. |
| G4 | **Transformer pipeline on chat completions** | `POST /v1/chat/completions` runs an ordered (and eventually parallel) **transformer** stage before the request is proxied upstream. |
| G5 | **413-aware behavior with cooldowns** | For **virtual** routing, upstream **413** (especially TPM-style) drives **fallback / cooldown** using **metrics** once available. **Direct** model requests: **no** fallback on 413 — surface the error. |
| G6 | **Observability path** | **Time-boxed BiFrost plugin spike** first to expose metrics (413 counts, timestamps, etc.). Full gateway-side persistence and advanced routing **follow** spike results; **fallback** is gateway-internal metrics if the spike is abandoned. |

**Explicitly out of scope for v0.1.1**

- **Message deduplication** (e.g. duplicate file bodies in context): error-prone; defer (possible future summarizer).
- **RAG transformer behavior beyond a stub** — any real vector/RAG work is **v0.2+**; v0.1.1 may reserve hooks only.

---

## 2. Dependencies and sequencing (what must happen first)

1. **BiFrost plugin spike (G6)** — Blocks **complete** G5 cooldown logic that depends on **plugin-reported** 413 history. Until the spike lands (or is rejected), implement:
   - **Tool slimming (G2)**, **Ollama patterns (G1)**, **router config + UI (G3)**, **transformer framework + tool transformer (G4)** without requiring plugin metrics.
   - **413 handling:** implement **direct vs virtual** behavior and **config placeholders** for cooldown clocks; wire **full** cooldown to metrics when the spike exposes an API.

2. **Router model available in config** — Tool slimming **requires** a working router model id and a call path (same BiFrost base + key as chat, per decision).

3. **Continue / client** — Tool slimming assumes tools arrive in the JSON **`tools`** array with **unique `name`** fields. Per-request **threshold override** requires agreeing on **request field or header** (TBD in code + `docs/configuration.md`).

---

## 3. Technical contracts (implement exactly)

### 3.1 Tool slimming (router output)

- **Input to router:** User prompt + context as needed (and optionally merged tool list vs MCP later); **source of truth** for definitions is the request **`tools`** array.
- **Router output (JSON):** array of `{ "name": "<string>", "confidence": <number> }` where `name` matches a tool in the incoming list.
- **Selection:** Keep tools where `confidence >= threshold`.
- **Threshold:** Default from **`gateway.yaml`** (or equivalent); **optional per-request override** (Continue model config) — mechanism **TBD**.
- **Failure** (timeout, non-200, invalid JSON, missing names): **do not slim** — pass **all** client `tools` upstream.
- **No** extra chat messages and **no** new routing-policy rules for this step — only mutate **`tools`** on the outbound body.

### 3.2 Ollama + free tier

- Use **`patterns:`** in `config/provider-free-tier.yaml` with existing **`path.Match`** semantics (`internal/providerfreetier/spec.go`). Example: `ollama/*` under `patterns:` (one segment after `ollama/` per `*`; add more patterns if ids are deeper).
- **Policy:** **allow-all-local** — any Ollama model returned by the upstream catalog that matches the pattern is eligible; **no** extra security/ops gate in v0.1.1 beyond normal config hygiene.
- **Ship:** default or documented pattern + short note in operator docs so generate-routing and `GET /v1/models` behave as expected.

### 3.3 413 behavior

- **Virtual Claudia** (policy + fallback chain): 413 should contribute to **skipping / cooling down** a model and moving on — **not** blind immediate retry of the same payload on the same model for TPM cases. Exact timers: **configurable**; clock mode **configurable**: **last successful request** vs **last request** (per upstream model id — specify in implementation). If retry after short wait still 413, use **longer** backoff.
- **Direct** request with a **concrete** upstream model id: **413 is expected** for that model — **do not** fall back or change model; return the error to the client.
- **Classification:** Prefer parsing response body where needed to distinguish **TPM/quota 413** vs **payload-too-large** semantics (TBD per provider).

### 3.4 Router model config

- **`router_models: []`** in YAML; **only the first** entry is used in v0.1.1; list reserved for future multi-router use.
- **Invocation:** same BiFrost **base URL** and **API key** as normal chat proxy.
- **Admin panel:** show resolved id, last-used time, catalog-missing errors.

### 3.5 Transformers

- **Pipeline:** runs on `/v1/chat/completions`; order **configurable**; **parallel execution** acceptable in the future for independent transformers (tool slimming + RAG stub).
- **Tool slimming transformer:** implements section 3.1.
- **RAG transformer:** **stub only** until v0.2 (no real Qdrant/query behavior required for v0.1.1 ship).
- **MCP mock:** clarify in implementation — **tests** at minimum; optional **dev flag** if useful.

---

## 4. Where work likely lands (codebase map)

Agents should confirm paths with the repo; starting points:

| Area | Likely locations |
|------|-------------------|
| Free-tier / patterns | `internal/providerfreetier/`, `config/provider-free-tier.yaml`, `internal/server/ui_routing_generate.go`, routing gen |
| Chat proxy + retries | `internal/chat/chat.go` (e.g. `retryStatuses`), `internal/server/` handlers for `/v1/chat/completions` |
| Config load / validation | `internal/config/config.go`, `config/gateway.yaml` schema docs |
| Transformers | New package e.g. `internal/transform/` or under `internal/server/` — **single** hook before upstream marshal |
| Router calls | Shared HTTP client pattern with existing BiFrost proxy |
| Desktop admin | Webview / admin UI code paths that already serve routing generate |

---

## 5. Verification (agents must satisfy)

- **Unit / integration tests** for: `tools` rewriting (threshold, override, router failure → all tools), policy `Match` / `Filter` with `ollama/*`, and chat proxy behavior for **413** on virtual vs direct (where testable without live Groq).
- **Config:** Example snippets in `docs/configuration.md` (or existing config doc) for `router_models`, tool threshold, and cooldown clock mode when implemented.
- **Manual:** Operator can add `patterns: ["ollama/*"]`, regenerate routing, and see Ollama ids in fallback; Continue chat with large `tools` shows reduced upstream size in logs (if logged) or smaller TPM failures.

---

## 6. Open items (resolve during implementation; do not block G1–G4)

| Item | Notes |
|------|--------|
| Per-request threshold | Field name on JSON body vs header; document for Continue model config. |
| 413 timer values | Defaults in YAML; optional tie-in to plugin metrics after spike. |
| TPM vs payload 413 | Parser rules per upstream error format. |
| Plugin API shape | Defined by spike; gateway subscribes or polls. |
| Persistence (plan B) | If spike fails: SQLite vs JSON vs external metrics store. |

---

## 7. Quick decision table (frozen for v0.1.1)

| Topic | Decision |
|-------|-----------|
| Tool definitions | Client **`tools`** array only for slimming; no synthetic messages. |
| Router output | `{ name, confidence }[]`; threshold in config + optional per-request override. |
| Router failure | Pass **all** tools. |
| Ollama | **`patterns:`** e.g. `ollama/*`; **allow-all-local** matching catalog. |
| Dedup | **Out of scope.** |
| Direct model 413 | **No fallback.** |
| Observability | **BiFrost plugin spike first**; gateway metrics as fallback. |
| RAG transformer | **Stub** until v0.2. |
