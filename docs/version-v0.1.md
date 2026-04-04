# Claudia Gateway — toward v0.1 (working notes)

This document is for **Audrey** (and a Cursor agent helping her) to **explore** what “done enough” for **v0.1** means in practice, how the repo behaves **today**, and which directions are **worth investigating** versus **already decided** in the product plan.

**Tone:** everything under *Explorations* is **optional research**, not a commitment. The authoritative roadmap and locked decisions remain in [`docs/claudia-gateway.plan.md`](claudia-gateway.plan.md) and [`docs/overview.md`](overview.md).

---

## Current state (as implemented)

The gateway is a **small TypeScript (Fastify) service** that sits **in front of LiteLLM over HTTP** only (no in-process LiteLLM SDK). It exposes an **OpenAI-compatible** surface for chat completions and models listing.

**What works today**

- **Virtual model** `Claudia-<semver>` (semver from `config/gateway.yaml`) appears first on **`GET /v1/models`**; other ids are passed through to LiteLLM unchanged.
- **Token auth** from YAML (`config/tokens.yaml` by default), with **mtime reload**.
- **Routing policy** (`config/routing-policy.yaml`): for the virtual model only, **rule-based** selection of the **first** LiteLLM model to try (see `src/routing.ts`). Conditions today are thin (e.g. `min_message_chars` on the **last** user message); then optional `ambiguous_default_model`, else **`routing.fallback_chain[0]`** in `config/gateway.yaml`.
- **Fallback chain**: on **429 / 5xx** from LiteLLM, the gateway walks **`routing.fallback_chain`** starting at the index of the model that was attempted (`src/chat.ts`).
- **Streaming** (SSE) and non-streaming proxying to LiteLLM.
- **`GET /health`**: probes **LiteLLM** only in v0.1 (Qdrant is in Compose for **v0.2** readiness but the gateway does not use it yet).

**Default stack** (`docker-compose.yml`): **`claudia`** + **`litellm`** (official image) + **`postgres`** (for LiteLLM UI / virtual keys) + **`qdrant`** (unused by gateway in v0.1).

---

## Friction: three places, same model ids

Operators currently need to keep **LiteLLM aliases** aligned in **three** YAML surfaces:

1. **`config/litellm_config.yaml`** — `model_list` / `model_name` entries (what LiteLLM exposes on `/v1/models`).
2. **`config/gateway.yaml`** — `routing.fallback_chain` (order of failover).
3. **`config/routing-policy.yaml`** — every `models:` entry and `ambiguous_default_model` must reference ids that exist in LiteLLM **and** (for sensible failover) appear in the fallback chain.

The original one-liner notes still apply:

- `gateway.yaml` fallback chain requires **manual** model entries.
- `litellm_config.yaml` requires **manual** model entries.
- `routing-policy.yaml` requires **manual** model entries.
- **Continue** (or any client) must point at the gateway base URL and use the virtual model id from **`GET /v1/models`** — also a manual wiring step, documented under `vscode-continue/`.

Anything that reduces **drift** between these (generation, validation, or a single source of truth) is a strong **v0.1 polish** candidate even if it is not spelled out in the long-form plan yet.

---

## In Development

### 1. Moving away from containers (Important)

Making a fast, portable application is important for the v0.1 release as it dictates the framework we are building on top of going forward.

The **plan** currently **locks** Docker Compose + official LiteLLM image as the default deployment shape. The following are **architecture experiments** that would **change** that story; treat them as **spikes** with explicit trade-off notes if pursued.

**Tracked migration work:** a **phased plan** (discovery: BiFrost + today’s TypeScript gateway → Go rewrite with BiFrost managing keys/connections → cross-platform packaging → GUI) lives in [`docs/go-bifrost-migration-plan.md`](go-bifrost-migration-plan.md). Phases are implemented when someone asks for the next phase; completed work is recorded there.

**4a. Removing Docker**

- **Goal:** single binary or `npm start` on the host, fewer moving parts for developers.
- **Reality check:** LiteLLM today is still a **separate service** in the reference architecture; “no Docker” might mean **operators install LiteLLM themselves** (pip/uv) or **replace** LiteLLM (see 4b). The migration plan above targets **native packages** and a **supervised** BiFrost + gateway stack instead.

**4b. Replacing LiteLLM with [BiFrost](https://docs.getbifrost.ai/)** ([`maximhq/bifrost`](https://github.com/maximhq/bifrost) on GitHub) *(or similar)*

- **Hypothesis:** BiFrost (or another gateway) could unify providers with a smaller footprint or different operational model.
- **Work:** map **feature parity** — model aliases, env-based keys, `/v1/chat/completions`, `/v1/models`, streaming, embeddings path for **v0.2** RAG, virtual keys / Postgres if those matter to your operators. **Phase 0** of [`go-bifrost-migration-plan.md`](go-bifrost-migration-plan.md) is the discovery pass (BiFrost installed and exercised behind the existing gateway); later phases port to Go and package both apps.
- **Code impact:** `src/chat.ts`, `src/litellm.ts`, config loading, health checks, and all docs that say “LiteLLM” would need a **provider abstraction** or a clean fork.

**4c. Vector store without a dedicated Qdrant service**

- **Context:** Compose includes **Qdrant** today for **v0.2** RAG readiness; the **v0.1** gateway does not use it. A portable or single-package install may want Qdrant **off by default** until RAG is in use.
- **Idea:** spike **embedded** Qdrant (where license and artifact size allow), another embedded vector backend, or a small local store for early RAG—so operators are not required to run a separate Qdrant container for every setup.
- **Plan alignment:** v0.2 in [`docs/claudia-gateway.plan.md`](claudia-gateway.plan.md) assumes a **swappable vector-store adapter**; if that boundary stays stable, embedded and remote Qdrant can remain interchangeable behind the same interface.

### 2. Portable “first run” / setup wizard

**Idea:** ship or build a **portable application** (desktop or CLI wizard — TBD) that walks a new operator through:

- **Provider credentials** (or env-file generation) for the backends they use.
- **Local LLM server** URL (Ollama, vLLM, llama.cpp-compatible, etc.) and how that maps to LiteLLM (or a future proxy).
- **Gateway token** minting or `tokens.yaml` generation.
- Optional: **probe** LiteLLM and the gateway **`/health`** before declaring success.

**Why explore it:** today setup is **document-driven** (`cp` examples, edit multiple files, `docker compose up`). A wizard could cut time-to-first-chat and reduce misconfiguration.

**Open questions:** Electron vs Tauri vs pure CLI; where secrets live; whether the wizard **writes** `litellm_config.yaml` / `gateway.yaml` or emits a **bundle**; how upgrades merge user edits.


## Exploration (Ideas we want but can be delayed)

### 1. Setup flow: routing structure from **available** models

**Idea:** instead of hand-authoring `routing-policy.yaml` and fallback chains from scratch, the gateway (or a setup job) would:

1. **Collect a model list** — e.g. call upstream **`GET /v1/models`** (LiteLLM today) and/or merge configured static entries.
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
| Gateway entry / HTTP | `src/server.ts`, `src/index.ts` |
| Virtual model + fallback | `src/chat.ts`, `src/routing.ts` |
| LiteLLM client / health | `src/litellm.ts` |
| Config loading | `src/config.ts` |
| Compose stack | `docker-compose.yml` |
| Gateway config | `config/gateway.yaml` |
| LiteLLM models | `config/litellm_config.yaml` |
| Routing rules | `config/routing-policy.yaml` |
| Product / locked decisions | `docs/claudia-gateway.plan.md` |

---

## Original scratch notes (preserved)

- `gateway.yaml` fallback chain requires the manual model entry
- `litellm_config.yaml` requires manual model entry
- `routing-policy.yaml` requires manual model entry
- Continue client configuration is manual (`vscode-continue/`)
