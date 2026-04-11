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

## Identifiers, keys, and picking the right Qdrant collection (v0.2 product plan)

This is the **current** system the gateway plan targets for **v0.2** — the anchor for how requests and index payloads map to storage.

| Concept | Where it comes from | Role |
|--------|---------------------|------|
| **`tenant_id`** | Gateway-issued **Bearer token** (server-side; not chosen per request by the client for chat) | Scopes **all** RAG data; retrieval and ingest apply only within this tenant. |
| **`project_id`** | **`X-Claudia-Project`** header on chat (when RAG applies) and on ingest, else **token default** | Selects the **project** / corpus namespace within the tenant. |
| **`flavor_id`** | Optional **`X-Claudia-Flavor-Id`**, else **token default** | Selects a **variant** corpus (e.g. branch, profile) within tenant + project. |
| **Qdrant collection** | Derived **deterministically** by the gateway from **`(tenant_id, project_id, flavor_id)`** | **One collection per triple**; naming follows encoding rules in [`claudia-gateway.plan.md`](claudia-gateway.plan.md) (lowercase, slug-safe, collision hash suffix). **No** reliance on payload filters for tenancy at v0.2 — isolation is by **collection**. |
| **`source` (indexed paths)** | Indexer / ingest client | **Relative path** under configured roots in [`indexer.plan.md`](indexer.plan.md); avoids leaking absolute host paths in bodies. |

**Operational note:** Operators still configure **how** the gateway reaches Qdrant (URL, API key, adapter). [`claudia-gateway.plan.md`](claudia-gateway.plan.md) defaults to an HTTP health probe (e.g. **`6333`** in Compose); a local **gRPC** client on **`6334`** remains compatible with the same **collection naming** and payload contract as long as the adapter uses one consistent Qdrant API mode.

---

## Future update plan: enhanced local vector RAG (vectordb-cli + custom LLM gateway)

The following is a **forward-looking** architecture (single developer machine, **no containers**, **vectordb-cli** as the indexer populator). It is **not** the locked v0.2 contract above; use it to reason about **keys**, **collection naming**, **embedding alignment**, and **retrieval quality** when evolving beyond gateway-mediated **`POST /v1/ingest`** + remote embeddings. **Goal:** high-relevance code/text retrieval with low latency, strong validation, and optional iterative refinement while staying **deterministic** where the pipeline is pinned (models, collection names, preprocessing).

### 1. Connection information, ports, paths, and configuration

- **Qdrant ports** (firewall / localhost only):
  - **Primary:** **6334/TCP (gRPC)** — intended for indexing and querying in this design.
  - **Optional:** **6333/TCP (HTTP/REST)** — dashboard, manual checks, or health-style probes.
  - No external exposure; bind to **localhost** or same-machine private network. **TLS** only if traffic leaves the host.
- **Connection details:**
  - **`QDRANT_URL`** (example default: `http://localhost:6334` — **note:** URL scheme must match client library expectations for gRPC vs REST; align with Qdrant client docs) or equivalent in `config.toml`.
  - Optional **`QDRANT_API_KEY`** shared between indexer manager, gateway, and Qdrant when enabled.
  - **Manager** process injects these per indexing run; **gateway** reuses the same logical connection (singleton + pooling).
- **Key paths** (manager / gateway):
  - **Source directories:** **absolute** paths resolved from gateway **project config** (contrasts with v0.2 **relative `source`** in HTTP ingest — if both worlds coexist, define an explicit mapping at integration time).
  - **ONNX embedding model + tokenizer:** fixed **read-only** paths to the **`.onnx`** file and tokenizer assets; **must match exactly** between indexer (**vectordb-cli**) and gateway at query time.
  - **vectordb-cli config:** prefer **environment variables + CLI flags** over `~/.config/vectordb-cli/config.toml` to reduce file-locking and stale state.
- **Collection naming** (deterministic; **shared** manager + router code):
  - Derive stable name from **user + project** (e.g. `repo_user-abc123-proj-xyz789`).
  - Sanitize for Qdrant (no slashes, respect length limits).
  - **Per-project collections** — isolation without payload filters for tenancy (similar *shape* to v0.2’s **one collection per triple**, different **key inputs**).

### 2. Indexing flow (manager process)

- **Manager** (separate **Go** process) periodically or via webhook **pulls project config** from the gateway (user keys + file paths).
- **Per project:** derive **repo/collection name**, run **vectordb-cli** repo management + sync/index with retries and exponential backoff.
- **Full re-index vs delta** depending on **Git repo** vs plain directory.
- Indexing = **short-lived CLI invocations** (not a daemon); data lands in Qdrant and is **immediately** queryable.
- **Watch-outs:** **fsnotify** or gateway push for change detection; schedule work on **separate CPU cores** so indexing does not starve the gateway.

### 3. Query-time flow (router / gateway layer)

Target **request-scoped** pipeline (**under ~600 ms** end-to-end where practical):

1. Extract **user + project** identifiers from the incoming request.
2. Compute the **exact Qdrant collection name** (same derivation as the manager).
3. **Enrich** the raw query text (see §4).
4. **Embed** enriched text with the **identical ONNX model** as the indexer.
5. **Vector search** on that collection.
6. **Validate and rerank** top‑k (score thresholds, intra-file checks, micro-judging).
7. **Optional** iterative refinement (**≤ 2** rounds): follow-up queries → re-search → merge.
8. Attach validated top‑k chunks (metadata: **`file_path`**, **`language`**, **`chunk_type`**) to the final LLM prompt.
9. **Graceful fallback:** if the collection is missing or Qdrant is unreachable, return **empty context** rather than failing the chat request.

### 4. Embedding the query + enrichment strategies

- **Core embedding:** always the **same ONNX model and tokenizer** as **vectordb-cli** at index time. Input = **enriched** query text; output vector goes straight to Qdrant search. **Dimension and normalization** must match.
- **Enrichment** (before embedding), examples:
  - **Simple rewrite:** small LLM reframes the query as a precise dev-style search (symbols, file patterns, edge cases).
  - **Multi-query:** **3–5** variants; embed each and fuse (**RRF** or vector averaging).
  - **HyDE:** LLM drafts a short hypothetical snippet that would answer the query; embed the hypothetical.
  - **Context injection:** prefix with **project hints** from gateway config (language, framework, etc.).
- **Normalization:** final enriched text should follow the **same whitespace / newline rules** as the indexer to stay in the same embedding space.
- **Alignment test:** index a known snippet → enrich a matching query → expect **self-retrieval score > ~0.85** (tune per model).

### 5. Model size and type recommendations (CPU-friendly, local)

Aim for **~4–6 GB RAM** total, **quantized** execution, **sub‑300 ms** per hot path on a typical dev machine (targets, not guarantees).

- **Embedding (index + query):** e.g. **BGE-M3**, **bge-base-en-v1.5** (dense + sparse hybrid where supported); alternatives **Nomic Embed Text v1.5**, **E5-base-v2**, **Jina Code Embeddings v2** (code-heavy). Require **ONNX/GGUF**, **8-bit** quantization where used; **fixed dimension**.
- **Small LLM** (enrichment, HyDE, follow-ups, micro-judge): e.g. **Phi-4-mini-instruct** (~3.8B); alternatives **Llama 3.2** 1B/3B, **Gemma 3** 1B/4B, **Qwen3** small, **SmolLM2** 1.7B. Run **4-bit/8-bit GGUF** via **llama.cpp** / **Ollama** or ONNX bindings.
- **Dedicated reranker:** classic **cross-encoder** (e.g. **ms-marco-MiniLM** L-6 / L-12) on top‑20–50; or **bge-reranker-base**, **mxbai-rerank-xsmall**.

### 6. Caching, better matching, validation, and iteration

- **Caching:**
  - **Embedding cache:** key ≈ hash(enriched query + user + project + model hash) → vector; in-memory or **BoltDB**; **5–15 min TTL** or invalidate on re-index for that collection.
  - **Full result cache:** top‑k + scores; invalidate on **any indexer run** for that collection.
- **Better matching:** hybrid **dense + sparse/BM25** at collection creation where supported; **rerank** post-retrieval; **metadata** filters (`file_path`, `language`, `chunk_type`); optional **pseudo-relevance** feedback (average top‑k vectors or text → new search).
- **Validation** before prompt attachment: hard **cosine** floor (e.g. **> 0.75**); **intra-file** neighborhood embedding check; **self-similarity** across top‑k; **LLM micro-judge** (batched, confidence **> 0.7**); code signals (AST/symbols) where available.
- **Iterative loop:** router-controlled; **max 2** rounds; **relevance-delta** stop + **overall timeout**; enable only for **complex** queries.

### 7. Implementation watch-outs and best practices

- **Embedding alignment** is non-negotiable — **golden** test projects.
- **Collection naming** must be **identical** and **collision-free** in manager and router.
- Keep router decisions **request-scoped** and **unit-testable** (enrichment + validation).
- **Latency budget:** enrichment + validation + optional iteration **~300–600 ms** total when features are on.
- **Resource isolation:** indexer/manager vs gateway **CPU affinity**; **fallback** paths always available.
- **Test loop:** small golden codebase → full manager cycle → end-to-end gateway request → assert relevant chunks.
- **Operations:** monitor Qdrant **disk**; **payload indexes** on frequently filtered fields.

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

This document also carries a **future** local **vectordb-cli** stack (§ **Future update plan**) for retrieval enhancement; reconcile its **collection naming** and **path** conventions with the v0.2 **triple** + **relative `source`** model when implementing a bridge.
