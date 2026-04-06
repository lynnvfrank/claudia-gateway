# Claudia Gateway — toward v0.1 (working notes)

This document is for **Audrey** (and a Cursor agent helping her) to **explore** what “done enough” for **v0.1** means in practice, how the repo behaves **today**, and which directions are **worth investigating** versus **already decided** in the product plan.

**Tone:** everything under *Explorations* is **optional research**, not a commitment. The authoritative roadmap and locked decisions remain in [`docs/claudia-gateway.plan.md`](claudia-gateway.plan.md) and [`docs/overview.md`](overview.md).

---

## Current state (as implemented)

The gateway is a **small Go** service in front of **BiFrost** (OpenAI-compatible HTTP). It exposes **`/`**, **`GET /health`**, **`GET /v1/models`**, **`POST /v1/chat/completions`**, and **`GET /status`** (when **`claudia serve`** supervises children).

**What works today**

- **Virtual model** `Claudia-<semver>` (semver from `config/gateway.yaml`) appears first on **`GET /v1/models`**; concrete upstream ids pass through.
- **Token auth** from YAML (`config/tokens.yaml` by default), with **mtime reload**.
- **Routing policy** (`config/routing-policy.yaml`): for the virtual model only, **rule-based** selection of the **first** upstream model to try (`internal/routing`). Conditions today are thin (e.g. `min_message_chars` on the **last** user message); then optional `ambiguous_default_model`, else **`routing.fallback_chain[0]`** in `config/gateway.yaml`.
- **Fallback chain**: on **429 / 5xx** from the upstream, the gateway walks **`routing.fallback_chain`** starting at the index of the model that was attempted (`internal/chat`).
- **Streaming** (SSE) and non-streaming proxying to BiFrost.
- **`GET /health`**: probes the configured upstream (JSON field **`checks.upstream`**). **Qdrant** is optional via **`claudia serve`**; the **v0.1** gateway does not call Qdrant for chat.

**Default local stack:** **`make up`** or **`go run ./cmd/claudia serve`** with **`./bin/bifrost-http`** after **`make install`**, plus provider env keys for **`config/bifrost.config.json`**.

---

## Friction: keep model ids aligned

Operators need **BiFrost catalog ids** (**`provider/model`**) to line up across:

1. **`config/gateway.yaml`** — `routing.fallback_chain` (failover order).
2. **`config/routing-policy.yaml`** — every `models:` entry and `ambiguous_default_model` should exist on the upstream and (for sensible failover) appear in the fallback chain.

**Continue** (or any client) must point at the gateway base URL and use the virtual model id from **`GET /v1/models`** — documented under **`vscode-continue/`**.

Anything that reduces **drift** (generation, validation, or a single source of truth) is a strong **v0.1 polish** candidate.

---

## In Development

### 1. Moving away from containers (Important)

Making a fast, portable application is important for the v0.1 release as it dictates the framework we are building on top of going forward.

**Default deployment shape:** **Go** **`claudia`** / **`claudia serve`** with **BiFrost** — see [`docs/go-bifrost-migration-plan.md`](go-bifrost-migration-plan.md) for the phased history.

**4c. Vector store without a dedicated Qdrant process**

- **Context:** **v0.2** RAG may use supervised **Qdrant** or another backend. A portable install may want vectors **off by default** until RAG is in use.
- **Idea:** spike **embedded** Qdrant (where license and artifact size allow), another embedded vector backend, or a small local store for early RAG—so operators are not required to run a separate Qdrant container for every setup.
- **Plan alignment:** v0.2 in [`docs/claudia-gateway.plan.md`](claudia-gateway.plan.md) assumes a **swappable vector-store adapter**; if that boundary stays stable, embedded and remote Qdrant can remain interchangeable behind the same interface.

### 2. Portable “first run” / setup wizard

**Idea:** ship or build a **portable application** (desktop or CLI wizard — TBD) that walks a new operator through:

- **Provider credentials** (or env-file generation) for the backends they use.
- **Local LLM server** URL (Ollama, vLLM, llama.cpp-compatible, etc.) and how that maps to BiFrost (or a future proxy).
- **Gateway token** minting or `tokens.yaml` generation.
- Optional: **probe** LiteLLM and the gateway **`/health`** before declaring success.

**Why explore it:** today setup is **document-driven** (`cp` examples, edit multiple files, run **`claudia serve`**). A wizard could cut time-to-first-chat and reduce misconfiguration.

**Open questions:** Electron vs Tauri vs pure CLI; where secrets live; whether the wizard **writes** `gateway.yaml` / `bifrost.config.json` or emits a **bundle**; how upgrades merge user edits.


## Exploration (Ideas we want but can be delayed)

### 1. Setup flow: routing structure from **available** models

**Idea:** instead of hand-authoring `routing-policy.yaml` and fallback chains from scratch, the gateway (or a setup job) would:

1. **Collect a model list** — e.g. call upstream **`GET /v1/models`** / BiFrost **`/api/models`** and/or merge configured static entries.
2. **Order models by “strength”** at **interpreting prompts and producing configuration** — this is intentionally vague: it could mean parameter size, bench scores, operator tiers, latency class, or a curated map file checked into the repo.
3. **Choose the top model as a “router coordinator”** — a single model that runs **once** (or on a schedule) to emit **machine-readable** routing config.
4. Feed that coordinator:
   - a **prompt** focused on routing rules (e.g. when the client specifies a concrete model in the request, honor it; when using the virtual model with **no** explicit tier, **default toward the most capable** option subject to cost/latency constraints);
   - the **full list** of available model ids and any metadata (context length, vision, local vs cloud);
   - a **short specification document** embedded in-repo describing the **router config schema** (could be an extension of today’s YAML or a generated artifact).

**Relationship to current code:** `RoutingPolicy` is **deterministic YAML** + simple predicates — there is **no** LLM-in-the-loop router today. This exploration would be **new behavior**, likely gated behind a setup mode or admin API.

**Risks:** coordinator hallucinates ids; nondeterministic setup; security if the coordinator can be prompted during normal traffic. Mitigations: validate emitted ids against `/v1/models`, dry-run, human review step, separate command.

---

### 2. Per-turn router using a **small** model

**Idea:** at **each** chat completion (when using `Claudia-<semver>`), run a **cheap** model first to **classify** or **select** the upstream model (and optionally parameters), then call the chosen backend for the real completion.

**Contrast with §2:** §2 is closer to **bootstrap / config generation**; §3 is **runtime** routing every turn.

**Why explore it:** YAML rules do not see **semantics** (only things like message length today). A small model could use the **last user turn** (and maybe tool/schema hints) to pick “fast vs strong” or “local vs cloud.”

**Costs:** extra **latency** and **cost** per request; need **timeouts** and a **safe default** if the router fails (fall back to first chain entry or last good choice). Streaming UX needs a clear story (router call must finish before streaming the main model).

---

## Quick reference — key files

| Area | Path |
|------|------|
| Gateway CLI | `cmd/claudia/` |
| HTTP server, health, models, chat | `internal/server/`, `internal/chat/`, `internal/upstream/` |
| Config load / reload | `internal/config/` |
| Routing policy | `internal/routing/` |
| Supervisor (BiFrost / Qdrant) | `internal/supervisor/` |
| Gateway config | `config/gateway.yaml` |
| BiFrost bootstrap | `config/bifrost.config.json` |
| Routing rules | `config/routing-policy.yaml` |
| Product / locked decisions | `docs/claudia-gateway.plan.md` |

---

## Original scratch notes (preserved)

- `gateway.yaml` fallback chain requires manual model entries
- `routing-policy.yaml` requires manual model entries
- BiFrost catalog must expose ids used in YAML
- Continue client configuration is manual (`vscode-continue/`)
