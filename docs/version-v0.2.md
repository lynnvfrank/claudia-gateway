# Claudia — toward v0.2 (implementation plan)

This document pulls together **everything scoped to product v0.2** from [`claudia-gateway.plan.md`](claudia-gateway.plan.md) (authoritative product roadmap), [`overview.md`](overview.md), [`network.md`](network.md), [`configuration.md`](configuration.md), and cross-links the **file indexer** work in a **separate** plan: [`indexer.plan.md`](indexer.plan.md).

**Tone:** normative items below track **locked** product decisions in the gateway plan; where the **in-tree** stack differs from the original LiteLLM + TypeScript + Compose description, treat this document as the **capability target** and align the Go gateway + BiFrost implementation to the same **HTTP contracts** and **behavior**. See the implementation note at the top of [`claudia-gateway.plan.md`](claudia-gateway.plan.md).

**Companion:** v0.1 working notes and checklist live in [`version-v0.1.md`](version-v0.1.md).

---

## What v0.2 is

**v0.2** is the **RAG baseline**: gateway-mediated **ingestion**, **query-time retrieval**, **Qdrant** (or another backend behind the **vector-store adapter**), **tenant-scoped** access to ingested data, and **indexer-facing REST** so an external **`claudia-index`** (and operators) can drive indexing without embedding locally.

**Release roadmap summary** (from [`claudia-gateway.plan.md`](claudia-gateway.plan.md)):

- **`POST /v1/ingest`**
- **Indexer REST:** **`GET /v1/indexer/config`**, **`GET /v1/indexer/storage/health`**, **`GET /v1/indexer/storage/stats`** (live Qdrant readings; no persisted metric history in-gateway)
- **Chunking defaults:** **512** UTF-8 code units, **128** overlap (configurable; surfaced via indexer config)
- **Qdrant adapter** + **query-time retrieval** + **prompt assembly**
- **Collection** naming rules; **`X-Claudia-Project`** / **`X-Claudia-Flavor-Id`** headers
- **`GET /health`** includes **Qdrant** probe when **RAG is enabled**

---

## Gateway and stack (v0.2)

### Authentication, tenant, and headers

- **Bearer token** (same as chat) defines **tenant**; **from v0.2** the token **authorizes RAG** so retrieval and ingested memory are **only** for that tenant’s data (gateway plan **#13**).
- **`X-Claudia-Project: <slug>`** on chat (when RAG applies) and on **ingestion**; falls back to token default ( **#14** ).
- Optional **`X-Claudia-Flavor-Id: <key>`** (or token default **`flavor_id`**) selects the **corpus** within tenant + project.

### Virtual model and RAG

- **`GET /v1/models`**: same virtual **`Claudia-<semver>`** id pattern as v0.1; **v0.2+** the virtual model **adds RAG when enabled** (explicit upstream model ids still **direct proxy**).

### Ingestion API

- **`POST /v1/ingest`** — **one document per request** (multipart **`file`** and/or JSON with **`text`**, **`source`**, etc.); finalize and document the **exact schema** in `docs/` and implementation.
- Accept **client-supplied `content_hash`** (algorithm and field name per contract) for **inventory / change detection**; gateway stores it as specified in [`indexer.plan.md`](indexer.plan.md) (indexer v0.2–v0.3 uses client hash as local truth until server-authoritative hash lands in later milestones).

### Indexer REST (gateway-owned)

- **`GET /v1/indexer/config`** — effective **`chunk_size`**, **`chunk_overlap`**, **`embedding_model`**, **`ingest_method`** + **`ingest_path`**, required/optional headers (**`X-Claudia-Project`**, **`X-Claudia-Flavor-Id`**), minimum Qdrant payload fields, collection naming summary, **`gateway_version`**, and related knobs from **running** config.
- **`GET /v1/indexer/storage/health`** — vector store reachability; **degraded**/ok; scoped to token **tenant**.
- **`GET /v1/indexer/storage/stats`** — **live** per-collection **point counts**, **vector dimension**, safe Qdrant metrics (document response fields).
- Optional additional **`GET`** under **`/v1/indexer/…`** as needed; document paths and keep stable within a **minor** release.

**Joint delivery note:** [`indexer.plan.md`](indexer.plan.md) calls for a future **corpus inventory** contract (e.g. path + hash listing) for reconciliation; that is **not** a hard requirement for labeling “gateway v0.2” complete in the product plan, but gateway and indexer teams should track it as a **coordination** item when implementing startup reconciliation.

### Chunking, embedding, and Qdrant

- **Chunking** happens **server-side** after ingest; defaults **512** / **128** overlap; configurable.
- **Embeddings** via the configured embed path (product plan: LiteLLM **`/v1/embeddings`**; in-tree: equivalent via BiFrost/embed configuration).
- **Qdrant defaults:** cosine (or dot if normalized — document with embed model); vector size **must** match embedding model dimension; default HNSW unless profiling says otherwise.
- **One collection** per **`(tenant_id, project_id, flavor_id)`**; **collection name encoding:** lowercase, spaces → hyphens, collapse repeats, strip illegal characters, deterministic short hash suffix on collision.
- **Qdrant payload (minimum):** **`tenant_id`**, **`project_id`**, **`text`**, **`source`**, optional **`created_at`**, optional **`flavor_id`**.

### Retrieval and prompt assembly

- **Query-time retrieval** for the virtual model when RAG is enabled.
- **Defaults:** **top_k = 8**; drop chunks below **~0.72** cosine similarity (configurable); optional **`created_at`** recency boost (default off unless config enables).
- **Prompt assembly:** inject retrieved chunks as a **single delimited section** before the user turn (e.g. markdown **`### Retrieved context`** with **numbered** chunks and a blank line before the rest of the conversation).

### Health and operations

- **`GET /health`**: when **RAG is enabled**, also probe **Qdrant** (e.g. **`GET http://qdrant:6333/`** in Compose); if RAG disabled, **omit** Qdrant. Failure → **503**, **`degraded`: true**, per-check detail (**#10**).
- **Structured logging (v0.1 baseline, v0.2 extension):** **DEBUG** (and appropriate levels) should cover **RAG** path activity — retrieve, ingest, collection id (**v0.2+**).

### Operator documentation (delta for v0.2)

The gateway plan requires **`docs/`** to cover overview, network, install, Docker cookbook, and configuration reference **for v0.1**; **v0.2** adds:

- Data flow **IDE → gateway → embed path → Qdrant** (and **indexer → gateway** for ingest).
- **Ingest** and **indexer** API paths, auth, and headers.
- **Continue** (or client) samples: **`X-Claudia-Project`** and **`X-Claudia-Flavor-Id`** on the OpenAI-compatible provider entry (**v0.2+** custom headers) — see **`vscode-continue/`** convention in the gateway plan.

[`network.md`](network.md) already notes **v0.2+**: Claudia → Qdrant for retrieval and indexer-backed workflows. [`configuration.md`](configuration.md) notes **`tenant_id`** in logs and **v0.2+** RAG scoping by tenant — keep these aligned as behavior lands.

---

## File indexer (v0.2)

All **indexer** milestones, configuration schema, gateway client behavior, Makefile targets, and **checklists** live in:

**[`indexer.plan.md`](indexer.plan.md)**

**Summary for this release:** the first shippable **`claudia-index`** **aligns with gateway v0.2** — whole-file **`POST /v1/ingest`**, **`GET /v1/indexer/config`**, storage **health** (and related APIs), client **`content_hash`**, env-based token, watch roots + ignore rules, **no symlink follow** by default, debouncing/backpressure, and documented behavior for **oversized files** under whole-file-only ingest until **indexer v0.4** dual-mode exists.

---

## Explicitly not v0.2

Keep these on later roadmap entries (see [`claudia-gateway.plan.md`](claudia-gateway.plan.md) **Release roadmap**):

- **v0.3** — peer LiteLLM, virtual keys, cross-host publishing, per-key observability (**#46**), etc.
- **v0.4** — ensembles, escalation, **dual-mode / streaming large-file ingest**, server-authoritative hash in ingest response (indexer plan **v0.4**).
- **v0.5+** — gateway MCP, conversation archive ingestion, etc.
- **v0.7** — TLS, hardening, **`/health`** lockdown on untrusted networks.
- **v0.8** — queues / priority scheduling (**#47**).

**Exploration from v0.1** (e.g. embedded vector store to avoid a dedicated Qdrant process) remains **research**; v0.2 still assumes a **vector-store adapter** boundary so embedded and remote backends can swap under the same interface ([`version-v0.1.md`](version-v0.1.md) §4c and gateway plan).

---

## Implementation checklist (high level)

Use this to track cross-cutting v0.2 work; gate detailed indexer items in [`indexer.plan.md`](indexer.plan.md).

| Area | Action |
|------|--------|
| **Config** | Gateway config to enable/disable RAG, embedding model id, Qdrant (or adapter) connection, chunking knobs, retrieval thresholds, feature flags as needed. |
| **HTTP API** | Implement **`POST /v1/ingest`**, **`GET /v1/indexer/config`**, **`GET /v1/indexer/storage/health`**, **`GET /v1/indexer/storage/stats`**; document schemas and limits (e.g. max body size for whole-file ingest). |
| **Chat path** | Virtual model: when RAG enabled, run retrieval + prompt assembly; honor **`X-Claudia-Project`** / **`X-Claudia-Flavor-Id`**. |
| **Qdrant / adapter** | Collections per triple; payload fields; collection naming; cosine/dot and dimension checks. |
| **Health** | Extend **`GET /health`** with Qdrant probe when RAG enabled. |
| **Docs** | Update **`docs/overview.md`**, **`docs/network.md`**, **`docs/configuration.md`**, ingestion/indexer references; **`vscode-continue/`** samples with v0.2 headers. |
| **Indexer** | Follow [`indexer.plan.md`](indexer.plan.md) **Indexer v0.2** checklist and **Gateway coordination** section. |

---

## Quick reference — related plans

| Document | Role |
|----------|------|
| [`claudia-gateway.plan.md`](claudia-gateway.plan.md) | Authoritative product requirements and roadmap |
| [`indexer.plan.md`](indexer.plan.md) | **`claudia-index`** milestones and gateway coordination |
| [`version-v0.1.md`](version-v0.1.md) | v0.1 delivery notes and exploration |
| [`overview.md`](overview.md) | Repo-oriented product summary |
| [`network.md`](network.md) | Ports and v0.2+ Qdrant data path |
| [`configuration.md`](configuration.md) | Config files and v0.2+ tenant scoping note |
