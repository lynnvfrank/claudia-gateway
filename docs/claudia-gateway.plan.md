# Claudia Gateway (LiteLLM-backed)

**Claudia Gateway** is the primary name for the thin orchestrator clients call. It exposes a **single OpenAI-compatible** entrypoint (e.g. for Continue) and hides manual model switching. Clients **authenticate** with a **gateway-issued API token**; that token defines the **tenant** and (**from v0.2**) **authorizes RAG** so retrieval and ingested memory are **only** available for data belonging to that tenant (see **#13**). **LiteLLM** handles multi-provider plumbing; the **gateway** implements Claudia-specific routing and policy (**v0.1**), **RAG indexing and query-time retrieval** (**v0.2**), and (from **v0.4**) **ensembles** and escalation. **TLS, gateway hardening, and broader security architecture** are **v0.7** (see **Release roadmap** and **#54–56**). The reference implementation is **TypeScript** (see **Implementation decisions**).

## Architecture

- **LiteLLM** — Deployed from the **official LiteLLM Docker image** (separate container from the gateway). Unified access to Groq, Gemini, local runners (Ollama, vLLM, llama.cpp, etc.), load balancing, retries, basic fallbacks, key handling, **embeddings** (`/v1/embeddings`) for **v0.2** gateway **RAG**, and **v0.4+**: **executing** parallel chat completions when the gateway’s **ensemble** orchestration requests them. It does **not** own “when to ensemble,” judge prompts, or human escalation copy. **v0.3**: **Another person’s machine** as a remote **model** endpoint is **their LiteLLM** OpenAI base URL (host/LAN/VPN-reachable)—not **their Claudia Gateway** (see **#25**); peers use **LiteLLM virtual keys** (see **Release roadmap**).
- **Claudia Gateway** — **TypeScript** service; sits *above* LiteLLM via **HTTP** (no in-process LiteLLM SDK). **Prior to v0.7**, the stack defaults to **plain HTTP** inside Compose and for typical **localhost** / trusted-LAN clients unless the operator adds their own terminator. **v0.7**: **TLS**, trust stores, and **security hardening** (see **#54–56**). **v0.1**: per-turn routing, quota/key policy, **virtual model** + **fallback chain**—**no** gateway-mediated **RAG** (no ingest, no Qdrant, no retrieval into prompts). **v0.2**: **RAG indexing** (`POST /v1/ingest`, chunking, embeddings via LiteLLM, Qdrant adapter), **query-time retrieval**, **`GET /v1/indexer/config`** for external indexers, and **tenant-scoped** RAG auth per **#13**. **v0.4+**: **two-phase ensemble** orchestration and external escalation handoff messages. **v0.2+**: talks to **Qdrant** through a **vector-store adapter** so another vector backend can replace Qdrant later. **v0.1**: **`GET /health`** probes **LiteLLM** (and **Qdrant** only when **v0.2 RAG** is enabled—see **#10**). **Locking down** `/health` beyond trusted use is **v0.7** (**#55**). Advanced behavior is implemented here, not only via stock LiteLLM config.
- **MCP** — Optional, for tools and context servers in the **IDE** (Continue); **not** the primary mechanism for choosing which LLM runs. **Gateway-native MCP** and **conversation archive ingestion** target **v0.5** (see **Release roadmap**). See **MCP: boundaries** and **#44** under Requirements.

## Implementation decisions (locked)

| Topic | Decision |
|-------|----------|
| **Language / runtime** | **TypeScript** (Node). Gateway calls **LiteLLM over HTTP** only. |
| **Containers** | **Official LiteLLM** image + **separate Claudia Gateway** image; **Docker Compose**; **default `docker-compose.yml`** runs the **full** stack (**#11**); publish ports as needed for IDE (**v0.1**) and, from **v0.3**, for LAN peer access to LiteLLM where enabled. **TLS** / **mTLS** / exposure hardening for those surfaces: **v0.7** (**#54–56**). |
| **Streaming (v0.1)** | **SSE / streaming** MUST match **OpenAI-compatible** behavior expected by Continue on **day one** (non-streaming-only is not enough). |
| **Gateway API tokens (v0.1)** | Valid tokens and tenant bindings load from a **static YAML file** (path via config/env). The gateway **caches** the parsed document and **reloads** when the file’s **modification time** changes. |
| **RAG indexing & retrieval (v0.2)** | Gateway is the **HTTP entrypoint** for **ingest** and **query-time retrieval**; **Qdrant adapter**; **embeddings** via **LiteLLM** `/v1/embeddings` (or configured embed base). Indexers call **`GET /v1/indexer/config`** for **chunking and related settings**, then **`POST /v1/ingest`**. **Direct Qdrant writes** remain **allowed anytime** (operator responsibility for **tenant_id** / **project_id** / **flavor_id** consistency). **Not in v0.1**. |
| **Indexer REST API — config (v0.2)** | **`GET /v1/indexer/config`** — **`Authorization: Bearer <gateway token>`** (**same** as chat). JSON includes **effective** **`chunk_size`**, **`chunk_overlap`**, **`embedding_model`** (LiteLLM model id), **`ingest_method`** + **`ingest_path`** (**`POST /v1/ingest`**), **required / optional HTTP headers** (**`X-Claudia-Project`**, **`X-Claudia-Flavor-Id`**), **minimum Qdrant payload fields**, **collection naming** summary, **`gateway_version`**, and other **indexer-relevant** knobs from **running** config. |
| **Indexer REST API — storage & live stats (v0.2)** | **Full REST** surface (Bearer-auth) so the **file indexer** can read **live** state about the **Qdrant** storage it writes to—**no** gateway-persisted history or time-series in this version (each response is an **on-demand** read from Qdrant / gateway checks). Minimum: **`GET /v1/indexer/storage/health`** — vector store **reachability** and **degraded**/ok for RAG storage (scoped to token **tenant**); **`GET /v1/indexer/storage/stats`** — **live** per-collection **point counts**, **vector dimension**, and any **storage/size** metrics Qdrant exposes that are safe to return (document fields). Optional additional **`GET`** sub-resources under **`/v1/indexer/…`** as needed; document paths in **`docs/`** and keep stable within a **minor** release. |
| **Structured logging (v0.1)** | Gateway uses **standard log levels** (`error`, `warn`, `info`, `debug`, …). **INFO**: **all** inbound and outbound **HTTP** connections (client route, upstream LiteLLM/Qdrant targets, **status codes**, duration); **key request/response parameters** useful for troubleshooting (**redact** secrets and full bodies). **DEBUG**: **controller / routing** branch choices; **RAG** path activity (retrieve, ingest, collection id—**v0.2+**); **filesystem** reads (config/token/policy paths); **configuration reload** events (path, success/failure); **LiteLLM** relay (model id, stream vs non-stream, error summaries). |
| **Operator documentation (v0.1)** | **`docs/`** (or repo-agreed root) MUST ship: **(1)** **High-level overview** — what Claudia Gateway, LiteLLM, and Qdrant each do and how they differ. **(2)** **Network architecture** — Compose service DNS, **published** vs **internal** ports, data flow (IDE → **claudia** → **litellm** → providers; **v0.2+** **claudia** → **qdrant**). **(3)** **Installation, setup, and startup** — prerequisites, **`.env`**, first **`docker compose up`**, verify **`GET /health`**, links to **LiteLLM** official docs below. **(4)** **Docker command cookbook** — common **`docker compose`** / **`docker`** commands (up, down, logs, exec, rebuild, volumes). **(5)** **Configuration reference** — every file and env var the gateway reads (token YAML, routing policy, main config): field semantics, defaults, reload behavior. **LiteLLM** (external): [LiteLLM documentation hub](https://docs.litellm.ai/docs/), [LiteLLM Proxy — deploy / Docker](https://docs.litellm.ai/docs/proxy/deploy). |
| **Compose & Dockerfile commentary (v0.1)** | **`docker-compose.yml`**: a **leading comment block** summarizing the stack; **per-service** comments on **each** **`environment`** entry and **each** **`volumes`** mapping (host ↔ container path, purpose, data lifetime). **Claudia Gateway `Dockerfile`**: a **block comment at the top** describing **image purpose**, base image, **build stages** (if any), exposed port, default **CMD**, and what operators typically override. |
| **VS Code Continue samples (v0.1)** | Directory **`vscode-continue/`** (or existing repo convention): **`README.md`** — how to connect Continue to Claudia (**`apiBase`**, **`apiKey`**, virtual **`Claudia-<semver>`** **`model`** from **`GET /v1/models`**, **v0.2+** custom headers). **`config.yml`** example (use **`config.yaml`** if Continue’s version requires that filename) including **pre-defined snippets** (or copy-paste blocks) showing **`X-Claudia-Project`** and **`X-Claudia-Flavor-Id`** on the OpenAI-compatible provider entry. |
| **Ingestion HTTP API (v0.2)** | **`POST /v1/ingest`** — **one document per request** (multipart **`file`** and/or JSON body with **`text`**, **`source`**, etc.—document exact schema). **`Authorization: Bearer <gateway token>`** — **same** as **`/v1/chat/completions`**. |
| **Ingest chunking defaults (v0.2)** | Default **512** UTF-8 code units per chunk, **128** overlap—**configurable** (surfaced via **`GET /v1/indexer/config`**). |
| **RAG prompt assembly (v0.2)** | Inject retrieved chunks as a **single delimited section** before the model sees the user turn—e.g. markdown **`### Retrieved context`** (or equivalent) with **numbered** chunks and a **blank line** before the rest of the conversation. There is **no universal standard**; this follows common **RAG** / cookbook patterns and works with most **instruct** chat models. |
| **Qdrant defaults (v0.2)** | **Cosine** distance (or **dot** if embeddings are normalized—document with embed model). **Vector size** MUST match the configured **embedding model** output dimension (operator-set). Use Qdrant **default HNSW** index params unless profiling says otherwise. |
| **Collection name encoding (v0.2)** | **Core rule**: **lowercase**; **spaces → hyphens** (ASCII hyphen). That is **enough** for typical **`tenant_id` / `project_id` / `flavor_id`** slugs. **Also**: **collapse** repeated hyphens; replace/strip characters **illegal** for Qdrant collection names (keep **alphanumeric**, **`-`**, **`_`**). If two distinct triples still **collide** after normalization, append a **short deterministic hash** of the triple. |
| **Project header (v0.2)** | **`X-Claudia-Project: <slug>`** on chat/completions (when **RAG** applies) and on **ingestion** (falls back to token default per **#14**). |
| **Flavor id header (v0.2)** | Optional **`X-Claudia-Flavor-Id: <key>`** (or token default **`flavor_id`**) selects the **corpus** within a tenant+project—see **Qdrant collections** row. |
| **Client model IDs & catalog (v0.1)** | Gateway **`GET /v1/models`**: call **LiteLLM** **`GET /v1/models`**, then **prepend** a **virtual** entry whose **`id`** is **`Claudia-`** concatenated with the **gateway semantic version** (e.g. **`Claudia-0.1.0`**—must match the string clients send as **`model`**). **Metadata** may duplicate version for UIs. **v0.1** virtual model: **routing + fallback chain only** (**no RAG**). **v0.2+**: same **`id` pattern**; virtual model adds **RAG** when enabled. Remaining **`id`s** are **explicit** LiteLLM model names; requests with those ids are **direct proxy** to LiteLLM. |
| **Routing fallback chain (v0.1)** | For **`model: Claudia-<gateway_semver>`** (see **`GET /v1/models`**), use an **ordered list of LiteLLM model id strings** in **gateway configuration** (operator-maintained). On failure or **429**, try the **next** entry (**fail-fast**—**#47**). |
| **Qdrant collections (v0.2)** | **One** Qdrant **collection** per **`(tenant_id, project_id, flavor_id)`**; collection **names** follow **Collection name encoding** row. |
| **Retrieval defaults (v0.2)** | Default **top_k = 8**; drop chunks below **~0.72** cosine similarity—**configurable**. **Recency**: optional **`created_at`** boost (default **off** unless config enables). |
| **Qdrant payload (v0.2)** | Minimum point/payload fields: **`tenant_id`**, **`project_id`**, **`text`**, **`source`**, optional **`created_at`**, optional **`flavor_id`**. Additional keys allowed as the implementation evolves. |
| **HTTP health / Compose readiness (v0.1)** | Gateway **`GET /health`** (**no auth**; **#55**). **v0.1**: probe **LiteLLM** only (**configurable** URL; default **`GET http://litellm:<port>/health`** or image-documented **200** URL); **expect 200**; **no retries**; timeout **configurable**, **default 5 seconds**. **v0.2+** when **RAG is enabled**: also probe **Qdrant** (default **`GET http://qdrant:6333/`**); if **RAG disabled**, **omit** Qdrant. If an **included** check fails, return **503**, **`degraded`: true**, **per-check** detail. **200** when every **included** check passes. **#10**. |
| **Compose service names (v0.1)** | **`docker-compose.yml`** services **`claudia`**, **`litellm`**, **`qdrant`**. **Named volumes** (and **`container_name`** if set) **match** those service names—e.g. volume **`claudia_data`** → service **`claudia`** (document exact names in the sample file). Intra-stack DNS: **`http://claudia:<port>`**, **`http://litellm:<port>`**, **`http://qdrant:6333`**. |
| **Ensemble (“heavy thinking”) (v0.4)** | **#34–36**. **Critique/synthesize** design and **streaming errors** during multi-phase ensemble runs are **out of scope for v0.1**—defer specification until **v0.4** (no normative contract before then). |
| **Peer access to another host’s LiteLLM (v0.3)** | Use **LiteLLM virtual keys** (proxy) so the host operator can **mint** per-peer (or per-use) keys with revocation and optional model limits; peers send the key per LiteLLM’s auth contract (typically `Authorization: Bearer …`). See [LiteLLM virtual keys](https://docs.litellm.ai/docs/proxy/virtual_keys). **TLS/mTLS** for peer URLs: **v0.7** (**#54**). |
| **Security & TLS (v0.7)** | **Normative** requirements for **TLS** termination (client→gateway and operator-chosen surfaces), optional **mTLS**, **corporate/custom CA** trust, **restricting or authenticating `/health`** on untrusted networks, **rate limits** / abuse controls, and **audit / redaction / secrets** hygiene—see **#54–56**. **Out of scope prior to v0.7** except what operators layer themselves (reverse proxy, VPN, etc.). |

## Release roadmap

| Version | Scope |
|---------|--------|
| **v0.1** | Services **`claudia`**, **`litellm`**, **`qdrant`** in **default `docker-compose.yml`** (with **documented** env vars, volumes, and **Dockerfile** header comments); **`GET /v1/models`** = virtual **`Claudia-<semver>`** **prepended** to LiteLLM list; **manual** **fallback chain**; **OpenAI-compatible streaming**; YAML tokens + **mtime** reload; **`GET /health`** (**LiteLLM** only; **5s** default timeout). **Structured logging** (**INFO** connections + key params; **DEBUG** routing, config reload, LiteLLM relay). **Operator `docs/`**: overview, **network architecture**, install/setup/start, **Docker command** cookbook, **configuration file** reference, links to [LiteLLM docs](https://docs.litellm.ai/docs/) and [proxy deploy](https://docs.litellm.ai/docs/proxy/deploy). **`vscode-continue/`** samples: **`README.md`** + **`config.yml`** (or **`config.yaml`**) with **snippet** examples for custom headers. **No** gateway RAG/ingest/indexer storage APIs yet (**v0.2**). **Single-host**. **MCP** only via **Continue**. |
| **v0.2** | **RAG indexing & retrieval**: **`POST /v1/ingest`**, **full indexer REST** — **`GET /v1/indexer/config`** plus **`GET /v1/indexer/storage/health`** and **`GET /v1/indexer/storage/stats`** (**live** Qdrant readings; **no** persisted metric history in-gateway), chunking **512** / **128** overlap, **Qdrant** adapter, **query-time retrieval** + **prompt assembly**; **collection** rules; **project** / **`flavor_id`** headers; **`GET /health`** + **Qdrant** probe when RAG on. See **Implementation decisions** rows tagged **v0.2**. |
| **v0.3** | **Peer-to-peer model backends**: call **another operator’s LiteLLM** over **host-routable** URL + **published** port; **LiteLLM virtual keys** (or equivalent) for cross-host auth; gateway/LiteLLM config and operator docs for **#24–27**, **#30** peer paths, and **#9** cross-host publishing—see **Peer operators** requirements. **Per-key / usage observability** (**#46**). |
| **v0.4** | **Ensemble** (“heavy thinking”): **#34–36**—parallel drafts, triggers (`//deep`), **LiteLLM introspection** for **`N`** cap; **critique/synthesize** and **ensemble streaming errors** specified here (deferred from v0.1). Human escalation: **#50** “exhausted + low confidence” **signal design**; paste-back **session/state** vs purely stateless handling—fully realize **#48–53** where deferred. |
| **v0.5** | **Gateway MCP** (or unified tool surface): optional; **undetermined** until scoped. **Conversation archive ingestion** (**#44**): automated / folder-based export pipeline calling **`POST /v1/ingest`** (**requires v0.2** indexing). |
| **v0.7** | **Security & TLS** (**#54–56**): encryption in transit for gateway-facing and optional inter-service paths, trust-store / CA story, **health** endpoint hardening when exposed, **rate limiting** and related abuse controls, **audit logging** and **redaction**, documented **threat-model** assumptions vs **pre-v0.7** trusted **HTTP** defaults. |
| **v0.8** | **Queues** and priority scheduling for degraded / busy backends (**#47**)—**fail-over / fail-fast** only prior to this. |

## Requirements

Subsections follow this **order**: naming & client API contract → **system context & dependencies** → responsibility split → **deployment & networking** → **operator documentation & samples (v0.1)** → **indexer REST storage (v0.2)** → **authentication, tenant & project (IDEs)** → gateway runtime → backends & peer topology → routing policy & implementation → MCP → **RAG & ingestion (v0.2)** → optional hooks → **observability (v0.3+)** → **ensemble (v0.4)** → human escalation → **security & TLS (v0.7)**. **Queues (v0.8)** in **#47**. **Implementation decisions** and **Release roadmap** are authoritative.

### Naming & client surface

1. **Product name** — Use **Claudia Gateway** as the canonical name for the entrypoint in docs, config, and operator runbooks (orchestrator / router are fine as synonyms where helpful).

2. **Single stable URL** — One base URL for clients (e.g. Continue); no manual per-request model switching in the UI.

3. **OpenAI-compatible surface** — Chat/completions (and related) shapes expected by common IDEs and agents.

3a. **Virtual vs explicit model id (v0.1)** — **`GET /v1/models`** returns the **virtual** entry **first**: **`id`** = **`Claudia-`** + **gateway semver** (e.g. **`Claudia-0.1.0`**), then **LiteLLM**’s models. Clients set **`model`** to that exact **`id`** for the orchestrated path (**v0.1**: routing + **fallback** only; **v0.2+**: adds **RAG** when enabled). **Explicit** ids **proxy** to LiteLLM (see **Implementation decisions**).

### System context — components & dependencies

The runnable system is **not** only the gateway binary: several **services** and **external APIs** interact. Implementation and operator docs MUST keep this inventory explicit (names, protocols, and who calls whom).

| Component | Role | Depends on (typical) | Consumed by | Interface (typical) |
|-----------|------|----------------------|-------------|---------------------|
| **Claudia Gateway** (TypeScript) | **v0.1**: routing + **virtual `Claudia-<semver>`** + **fallback chain** + **structured logs**. **v0.2+**: **RAG**, **Qdrant**, **indexer REST** (config + storage health/stats). **v0.4+**: **ensembles** + escalation | **LiteLLM** (HTTP: chat; **v0.2+** **embeddings** for RAG); **v0.2+** **Qdrant**; **gateway token YAML** | **Continue**, indexers, HTTP clients | **v0.1**: **`GET /v1/models`**, **`/v1/chat/completions`**, **`GET /health`**. **v0.2+**: **`POST /v1/ingest`**, **`GET /v1/indexer/config`**, **`GET /v1/indexer/storage/health`**, **`GET /v1/indexer/storage/stats`**, RAG headers |
| **LiteLLM** (official image) | Multi-provider model proxy; embeddings for RAG | **Groq**, **Gemini**, **OpenAI-compatible** local servers (llama.cpp, Ollama, vLLM, …); optional **peer LiteLLM** (**v0.3**) | Claudia Gateway | HTTP OpenAI-compatible `/v1` (chat + embeddings) |
| **Qdrant** | Vector persistence for gateway-mediated RAG (**v0.2+**) | Persistent volume / ops backup policy | **Claudia Gateway** (**v0.2+**); direct writes allowed anytime | HTTP REST / gRPC (per Qdrant) |
| **Groq / Gemini** (APIs) | Cloud inference | Vendor accounts & API keys | LiteLLM | HTTPS |
| **Local LLM servers** | On-box or LAN inference | GPU/CPU, model files | LiteLLM | OpenAI-compatible or provider-specific |
| **Peer LiteLLM** (**v0.3**) | Remote model capacity on another host | Host-published port; **LiteLLM virtual key** (or equivalent) from peer operator | Your gateway/LiteLLM `api_base` | **HTTP** to routable host + port + `Authorization: Bearer <virtual-key>` (per LiteLLM); **HTTPS**/mTLS **v0.7** (**#54**) |
| **MCP servers** (**v0.5**) | Tools, resources | Per server | **Continue** Agent mode (workspace config)—**not** the gateway until **v0.5** | MCP transports—**not** the primary model-routing path; **gateway-native MCP undetermined** until v0.5 scoping |

### Responsibility split & delivery constraints

4. **LiteLLM responsibilities** — Provider keys, retries, streaming, OpenAI-shaped requests to many backends, and **v0.4+**: **invoking** multiple completions in parallel when orchestrated by the gateway for **ensembles**.

5. **Gateway responsibilities** — **v0.1**: When **`model: Claudia-<semver>`**, walk the **configured fallback chain** of LiteLLM model ids (**no** vector retrieval). **v0.2+**: **if and what** to retrieve from vector memory for the virtual model when **RAG** is enabled. **v0.4+**: **when** to run ensembles, **critique/synthesize**, escalation messages, paste-back merge.

6. **v0.1 delivery** — Ship **LiteLLM + Claudia Gateway** (+ **Qdrant** in Compose for **v0.2** readiness) per **Release roadmap**: **OpenAI-compatible streaming**; **virtual model id** + **fallback chain**; **no** gateway RAG APIs yet. Do **not** build a full custom LLM proxy from scratch unless a hard requirement forces it. **v0.2**: **`GET /health`** with **Qdrant** probe when **RAG enabled**; **degraded** on failure (**#10**). **v0.1**: **LiteLLM-only** health probes.

### Runtime & deployment (how to run it)

7. **Docker Compose is the default operator path** — Run **`litellm`**, **`claudia`**, and **`qdrant`** as **services** in **one** Compose project (**default `docker-compose.yml`**, **#11**) so **`docker compose up -d`** brings up the **documented** stack; **v0.1** uses **claudia** + **litellm** for core chat (**qdrant** present for **v0.2** readiness). Secrets in **`.env`** / Compose secrets, not baked into images.

8. **RAG stack (v0.2+)** — When **v0.2 RAG** is **enabled** in configuration, **Qdrant** and the **embedding** path MUST be **up** and **reachable**. When **disabled** (or **v0.1**), the gateway does **not** require Qdrant for correctness; **`GET /health`** omits the Qdrant probe (**#10**). Docs MUST list DNS (**`litellm`**, **`qdrant`**, **`claudia`**) and health behavior per version.

9. **Per-stack internal network** — Attach **`claudia`**, **`litellm`**, and **`qdrant`** to the same **user-defined bridge network** (same pattern as this repo’s stacks, e.g. `mcpnet`). Use **Compose DNS**—e.g. **`http://litellm:<port>`**, **`http://qdrant:6333`**, **`http://claudia:<port>`**—not `localhost`, for **intra-stack** traffic. **v0.3**: **cross-operator** peer access uses **host-reachable** addresses and **published** ports (see **#25–26**).

10. **Published ports & Compose health** — Publish ports for the **IDE** (gateway front door; optional LiteLLM debug). **Prior to v0.7**, **trusted** reachability; **TLS** / lockdown **v0.7** (**#54–55**). **`GET /health`** (no API token): **v0.1** — **LiteLLM** probe only (**Implementation decisions**). **v0.2+** with **RAG enabled** — add **Qdrant** probe; on failure **503** + **`degraded`**. **Configurable** URLs; **expect 200**; **no retries**; **default 5s** timeout. **Docker Compose** **`healthcheck`** on **`claudia`** MUST hit **`GET /health`**.

11. **Default single Compose file** — **`docker-compose.yml`** at the **repository root** declares services **`claudia`**, **`litellm`**, **`qdrant`** on a shared network; **container names** and **named volumes** align with those service names (**Implementation decisions**). **`docker compose up -d`** is the default bootstrap. Optional overlays **`docker-compose.*.yml`** / **`profiles`** allowed but must not replace the single-file default.

12. **Developer iteration** — For fast edits to gateway code only, it is acceptable to run the **gateway locally** (Node / `tsx` / your package runner) while **LiteLLM stays in Docker**, pointing at `http://localhost:<published-litellm-port>`. Production-like checks should still use the full Compose stack.

### Operator documentation, containers, logging, and Continue samples — **v0.1**

57. **Human-facing overview** — **`docs/`** MUST include a **high-level** description of the project: purpose of **Claudia Gateway**, how **LiteLLM** and **Qdrant** fit in, and what **v0.1** vs **v0.2+** deliver (see **Release roadmap**).

58. **Network architecture** — Document **logical** and **Compose** topology: services **`claudia`**, **`litellm`**, **`qdrant`**; **internal DNS** vs **host-published** ports; traffic from IDE/clients, indexers, and outbound to cloud providers via LiteLLM.

59. **Installation, setup, startup** — Step-by-step: clone/checkout, **`.env`** from example, **`docker compose up`**, confirm **`GET /health`**, set **gateway token** YAML, optional LiteLLM provider keys. **Link** prominently to **LiteLLM** official **[documentation](https://docs.litellm.ai/docs/)** and **[proxy deployment / setup (incl. Docker)](https://docs.litellm.ai/docs/proxy/deploy)** so operators configure the proxy correctly alongside Claudia.

60. **Docker commands & tools** — Document common **`docker compose`** and **`docker`** invocations: logs (`-f`), `ps`, `exec` into **`claudia`** / **`litellm`**, rebuild after code changes, `down` / volume management, and where **Ops** helper scripts live if the repo provides them.

61. **Configuration reference** — Detailed documentation for **every** configuration source the gateway loads at runtime: **token YAML** (schema, tenant binding, defaults), **routing / fallback** files, environment-to-config mapping, and reload (**mtime**) semantics—aligned with **Implementation decisions**.

62. **Inline container documentation** — **`docker-compose.yml`**: top **comment** summarizing the stack; **per-variable** and **per-volume** comments (see **Implementation decisions**). **Claudia `Dockerfile`**: **header comment block** describing image role, ports, and default process.

63. **Structured logging (v0.1)** — Gateway emits **leveled** logs per **Implementation decisions**: **INFO** for **all** HTTP connections and high-signal params/responses (**redact** secrets); **DEBUG** for controller decisions, **RAG** (**v0.2+**), file reads, **config reload**, and **LiteLLM** relay details.

64. **VS Code Continue samples** — Ship **`vscode-continue/README.md`** and an example **`config.yml`** (or **`config.yaml`** per Continue version) with **pre-defined snippets** (or equivalent) for **`X-Claudia-Project`** and **`X-Claudia-Flavor-Id`** on the OpenAI-compatible provider; document **`model: Claudia-<semver>`** from **`GET /v1/models`**.

### Indexer REST — storage & live metrics — **v0.2**

65. **File indexer REST surface** — Beyond **`GET /v1/indexer/config`**, implement authenticated **REST** **`GET`** endpoints so the **file indexer** can read **live** **Qdrant**-oriented state: at minimum **`/v1/indexer/storage/health`** (RAG storage health for the token’s scope) and **`/v1/indexer/storage/stats`** (live **point counts**, **storage** / size signals Qdrant exposes, **per collection** as applicable). **This version** does **not** persist historical metrics or run a time-series store—responses are **on-demand** only (**Implementation decisions**).

### Client integration: tenant + per-project RAG (e.g. VS Code Continue)

**v0.2+** (RAG): Continue (and similar clients) do not know your **project** unless you pass it on the HTTP request. **One Qdrant instance**, **one collection per `(tenant_id, project_id, flavor_id)`** (**Implementation decisions**). **v0.1**: **`X-Claudia-Project`** / **`X-Claudia-Flavor-Id`** are **not** required for chat (**no** retrieval).

13. **Gateway API token — authentication and RAG tenancy** — Clients MUST authenticate with a gateway-issued **API token** (e.g. `Authorization: Bearer …` or Continue **`apiKey`**). Valid tokens load from **static YAML** (**Implementation decisions**); **reload** on **mtime**. **v0.1**: token **authenticates** to the gateway. **v0.2+**: token also **authorizes RAG**—retrieval, **`POST /v1/ingest`**, **`GET /v1/indexer/config`**, **`GET /v1/indexer/storage/*`**, and archive pipelines MUST scope by **tenant**; **default `project_id`** / **`flavor_id`** when headers omitted.

14. **Project scope on the wire (v0.2+)** — **`X-Claudia-Project`** on chat (when RAG applies) and **ingestion**. Resolve **`project_id`** from header or token default; allowlists for unknown projects. Optional **`X-Claudia-Flavor-Id`** selects **collection** (**Implementation decisions**).

15. **Per-workspace Continue config** — Use **Continue’s OpenAI-compatible** fields per [Continue’s config reference](https://docs.continue.dev/reference). **Placeholders**: `apiBase: <CLAUDIA_GATEWAY_URL>`, `apiKey: <GATEWAY_TOKEN>`, `model: <VIRTUAL_MODEL_ID>` (use the **`id`** from **`GET /v1/models`**, e.g. **`Claudia-0.1.0`**), **`X-Claudia-Project`**, optional **`X-Claudia-Flavor-Id`** (**v0.2+** RAG)—exact keys per Continue’s **OpenAI** provider schema. **Workspace-local** `.continue/config.yaml`; copy from repo **`vscode-continue/`** samples (**#64**).

16. **Ingestion parity (v0.2+)** — Prefer **`POST /v1/ingest`** + **Bearer**; indexers SHOULD call **`GET /v1/indexer/config`** first, then use **`GET /v1/indexer/storage/health`** and **`GET /v1/indexer/storage/stats`** for **live** corpus state (**#65**). **Direct Qdrant writes** **allowed anytime**; writers **must** keep **`tenant_id`**, **`project_id`**, **`flavor_id`** consistent with Continue / indexer headers or risk leaks.

### Gateway runtime behavior

17. **Long-lived service** — The gateway runs continuously while Claudia is in use; no process restart between ordinary user messages.

18. **Per-turn dispatch** — **Every** user message is evaluated anew for routing (**v0.2+**: retrieval when RAG enabled; **v0.4+**: ensemble triggers per **#34–36**).

### Backends, keys & sequential fallback

19. **Local / multi-machine models** — Configure backends for **this** stack’s LiteLLM (same Compose network or routable from it): local runners, cloud APIs, etc.; gateway policy can prefer by health, capacity, or task type (e.g. heavy vs fast). **v0.3**: add **peer** machines via **Peer operators** (**#24–27**) and LiteLLM `api_base` entries.

20. **Groq** — Support multiple API keys where each key is a **separate org/account** so rotation increases effective quota; keys in the same account share one bucket (document this).

21. **Gemini** — Same multi-key / multi-account pattern as Groq where applicable.

22. **Key rotation** — LiteLLM (and provider config) distributes across keys (e.g. round-robin) and reacts to rate-limit signals (headers, 429): backoff, switch key, or fall through.

23. **Sequential fallback (`model: Claudia-<semver>`)** — **v0.1+**: Gateway uses the **operator-configured ordered list of LiteLLM model ids** for the **virtual** model (**Implementation decisions**): on failure or **429**, **fail fast** to the **next** id; exhausted list → error. **No queue** until **v0.8** (**#47**). LiteLLM-internal fallback remains separate.

### Peer operators (separate machines, separate Compose networks) — **v0.3**

**Product scope begins in v0.3.** v0.1 is **single-operator, single-host**; no configuration surface is required for remote **peer LiteLLM** backends until then.

Each person runs **their own** Compose project: **Claudia Gateway + LiteLLM** (+ **Qdrant** / **v0.2+ RAG** as configured) on **that machine’s** default **user-defined bridge network** (isolated per project/host). Operators do **not** attach two people’s stacks to **one** shared Docker network. **IDEs** on each machine point at **localhost** (or that host’s published gateway port) with **that person’s** API token (separate tenants, separate RAG). When **one** gateway uses **the other’s computer** as **extra model capacity**, traffic crosses **hosts** via **routable URLs** (LAN, Tailscale, etc.) and **published** ports—not Compose service DNS from another project.

24. **Independent stacks, independent tenants** — Each operator’s **Claudia Gateway** is the sole **client-facing** entrypoint on **their** machine; each has its own **API tokens**, **vector store** (or collection partition), **Compose network**, and policy. Do not assume one gateway “owns” another’s RAG or auth.

25. **Peer as model backend → peer LiteLLM, not peer Gateway** — When configuring **your** gateway (or **your** LiteLLM `custom_openai` / deployment entry) to call **another operator’s machine**, set OpenAI-compatible **`api_base`** to **their LiteLLM** endpoint reachable **from your** containers or host (e.g. `http://<peer-hostname>.tailnet-name.ts.net:<published-litellm-port>/v1`, `http://192.168.x.x:<port>/v1`, or `http://host.docker.internal:<peer-port>/v1` when Docker routes that correctly on your platform). Include authentication using a **LiteLLM virtual key** (or equivalent proxy credential) **issued by that peer** for your stack—see [LiteLLM virtual keys](https://docs.litellm.ai/docs/proxy/virtual_keys). Do **not** use a **Compose-only** hostname from **their** stack (e.g. `http://litellm:4000`)—that name resolves only **inside their** project network, not from yours. Do **not** use **their Claudia Gateway** URL for this pattern: that chains **two** orchestrators and applies **their** token/RAG/ensemble policy to forwarded traffic.

26. **Document gateway vs LiteLLM ports (per host)** — Operator docs MUST distinguish URLs on **each** machine: which **published** port is for **humans/Continue** (Claudia Gateway) vs which **published** port is for **remote** model calls **to that machine’s LiteLLM**. Peers configuring `api_base` need the **latter**, on a **host-routable** address. Document **firewall** and **VPN** expectations for cross-host access; **TLS/mTLS** and **deep exposure** guidance: **v0.7** (**#54–56**).

27. **Gateway-on-gateway out of default scope** — **Gateway → peer Gateway** is **not** the default integration; avoid documenting it as the normal way to share compute. If ever required, it needs explicit **dual-auth** and policy design (**v0.7** security scope, **#54–56**) and is separate from peer LiteLLM routing.

### Model selection, uncertainty & cloud vs local

28. **Best model for the request** — Policy-driven selection (task class, latency vs quality, context length, cost heuristic: prefer the *cheapest model that can solve correctly*, not always the largest).

29. **Uncertainty default** — When routing is **ambiguous**, default to a **safe, capable** model (favor answer quality over marginal speed); avoid picking a weak model just because the heuristic is unsure.

**Implementation (#28–29, v0.1+)** — Realized in **gateway TypeScript** as **deterministic** policy (**no** extra LLM per turn; **#33** may later assist). Operators add a **routing policy** file (YAML/JSON; path via config/env; **reload** on **mtime** or together with token YAML—implementation choice). **Rules** are evaluated in **order**; each rule matches **cheap signals** (e.g. message length, simple patterns/keywords, optional task header) and supplies an **ordered list of LiteLLM model ids**—ordering encodes **#28** (cost vs capability: “cheapest first among those the operator deems adequate for this rule”). Every id MUST appear in the **fallback chain** (**#23**) so fail-over stays consistent. **First matching rule** → use **first** id in that list for the **initial** LiteLLM call. **#29**: if **no** rule matches or **tie** at the same priority, call **`ambiguous_default_model`** (configured LiteLLM id, also in the fallback chain)—a **capable generalist**, not the smallest model in the pool. **#30–31** extend the same file with **cloud vs local** and **RAG-on-cloud** flags where applicable (**v0.2+**).

30. **Cloud vs local policy** — Prefer **cloud** (Groq/Gemini, etc.) for high-volume, generic work where in-thread context suffices. Prefer **local** (often with **selective** RAG **v0.2+**) for long-term continuity, large **private** context, **v0.4+** **heavy ensemble**, or **cloud quota** exhaustion—configurable rules. **v0.3**: **Peer LiteLLM** as **remote-runner** `api_base`.

31. **Selective context for cloud (v0.2+)** — Do **not**, by default, inject **large** personal RAG into **every** cloud call. Retrieve only when policy says the turn needs memory.

### Routing implementation (how the gateway decides)

32. **Rules and heuristics first** — When **`model`** matches **`Claudia-<semver>`**, combine heuristics with the **configured fallback chain**—not an LLM call every turn (**v0.2+**: add RAG policy). **Explicit** LiteLLM ids → **direct proxy**.

33. **Optional routing judge** — Later enhancement: a **small, fast** model may assist routing on ambiguous turns; not required for v0.1.

### MCP: boundaries

37. **MCP is not the model router** — MCP remains **optional** for tools, resources, and standardized tool loops; it is **not** required for basic routing to Groq/Gemini/local OpenAI-compatible servers. **v0.1**: configure MCP in **Continue** (per workspace) only—**no** MCP client inside Claudia Gateway (**v0.5** target; see **Release roadmap**).

38. **No mid-inference model switching via MCP** — The gateway chooses the model(s) **before** generation. MCP must not be relied on to “switch brains” mid-stream; the underlying LLM for a given completion is fixed for that turn’s pipeline unless the **gateway** starts a new orchestrated step.

39. **Avoid LLM-as-MCP-tool for primary routing** — Do **not** use “call another LLM through an MCP tool” as the **main** way to pick backends; that path is higher latency, harder to debug, and less reliable than gateway policy. (A custom MCP tool that calls a model for a **specific** tool use-case is a separate concern.)

40. **Deterministic routing preference** — Routing-critical behavior should be **gateway-controlled** (rules, policy, thresholds). Do not depend on the model’s ad hoc choice of MCP tools for **which** provider or model answers the user’s primary request.

### RAG, memory & ingestion — **v0.2** (indexing + retrieval)

41. **Prompt assembly** — Forward messages, system prompt, and client context. **v0.2+**: retrieved chunks use the **delimited section** pattern (**Implementation decisions**). **Gateway** orchestrates retrieval/injection when policy requires (**not** inside raw LiteLLM).

42. **Retrieval contract (v0.2+)** — **Semantic search** via **Qdrant adapter** on the **`(tenant_id, project_id, flavor_id)`** collection; **top‑k** per **Implementation decisions**. **Qdrant** payloads per **Implementation decisions**.

43. **RAG quality controls (v0.2+)** — **Similarity floor**, optional **recency**, **`flavor_id` + project** boundaries; optional system text vs irrelevant callbacks.

44. **Conversation archive ingestion** — **Target: v0.5**; **depends on v0.2** **`POST /v1/ingest`**. Automated pipeline from a **configured folder** of exports: **one file per request**, correct **tenant** / **project** / **`flavor_id`**.

### Optional hooks & tool integration

45. **Enrichment hooks** — Extension points for long-context / memory-heavy detection and for delegating **tool** loops to **MCP servers** (the standard pattern for tool servers when used).

### Observability & resilience

46. **Per-key tracking** — **v0.3+**: Track which key/backend was used and exposure to RPM/TPM-style limits where headers exist (**not** normative for **v0.1**).

47. **Graceful degradation** — **Prior to v0.8**: **fail-over** within the **configured model chain** where applicable, otherwise **fail fast**—**no** gateway queue. **v0.3+**: when a **peer** LiteLLM exists, same rule (no queue until **v0.8**). **v0.8**: optional **queues** and priority (**Release roadmap**).

### Ensemble (“heavy thinking”) — **v0.4**

**Not in v0.1.** **Critique/synthesize** behavior and **streaming** behavior when ensemble phases fail mid-stream are **deferred** to **v0.4**—no normative design before then.

34. **Two-phase ensemble** — For configured or detected “deep” tasks: run **N parallel** draft completions (same or mixed backends, multi-node, varied temperature/seed where useful), then a **dedicated critique/synthesize** pass (typically a strong model) to produce **one** user-facing answer. **`N`** is **configurable**; **default `N` = 3**. If **`N`** exceeds the number of models/backends **actually available** for that turn (after policy, health, and limits), use **that smaller count** (cap). **Availability** comes from **LiteLLM introspection**—not a parallel hand-maintained list. **v0.4**: specify **critique/synthesize** and **streaming error** semantics.

35. **Ensemble triggers** — Support **automatic** triggers and **manual** **`//deep`** (trimmed leading whitespace), only when **`model`** is the **virtual `Claudia-<semver>`** id (or future gateway-orchestrated id). The gateway **may** strip **`//deep`** before upstream models. **`N`** follows **#34**.

36. **Ensemble integration** — Ensemble orchestration, judge prompts, and merge policy live in the **gateway**; LiteLLM supplies fast multi-call execution, not the decision to run the workflow.

### External human-in-the-loop escalation (manual third-party assist)

When internal routing cannot produce a satisfactory answer under policy, the gateway may **fall back** to a **human-in-the-loop** path: the user copies a prompt to an **approved external** web UI, pastes the external reply back, and the gateway continues. This is **not** an API integration to that third party; the user is the channel.

**Target: v0.4** for full productization of signals (**#50**), paste-back **session/state**, and operator polish. Requirements **#48–53** remain the **design contract**; **v0.1** may omit or stub escalation until then.

48. **Configurable external surfaces** — **One or more** entries in configuration, each with a **service name** and **URL** (e.g. `Grok` → `https://grok.com`). The escalation message may reference one or more of these; which to suggest is policy- or template-driven.

49. **Privacy disclosure** — Every escalation response **must** include an explicit **privacy** message: using an external service may send **task or context** outside the operator-controlled stack (and any org boundary). Wording should be consistent and reviewable (template, not ad hoc).

50. **When policy engages** — Use this escalation path only when **both** are true: internal attempts are **exhausted** (e.g. fallback chain and any configured parallel/ensemble paths did not yield an acceptable result), **and** assessed **confidence is low** per gateway policy (thresholds configurable). **Concrete signals and thresholds: v0.4** (see **Release roadmap**).

51. **Escalation message contents** — The assistant message should: state that internal backends could not answer satisfactorily; point to **configured** name/URL pairs; provide a **single copy-paste prompt** for the external service that restates the task and constraints; and include **instructions to the external service** so its answer is formatted for re-ingestion—specifically so the answer contains a **required sequence** (marker, delimiter, or structured line) that unambiguously marks the payload as the official “paste-back” block.

52. **Recognizing paste-back** — On a later user message, if the content **contains** the required **sequence** (or delimited block per policy), treat that as the **external answer**, merge it into context, and **continue** to the next phase of work (synthesize, verify, implement) with internal models as usual. **v0.4**: explicit session state if needed beyond message-history parsing.

53. **Continuing without an external answer** — If the user’s message **does not** include the required sequence, **assume** they are **not** supplying the external paste and intend to **continue the thread** without that payload (normal chat, clarification, or alternate approach). Do **not** block waiting for a paste unless a separate explicit UX is added later.

### Security & TLS — **v0.7**

**Prior to v0.7**, ship **functional** tenancy (**API tokens**, virtual keys for peers) on **plain HTTP** by default inside the stack and for typical **trusted** clients. **Normative** product requirements for **encryption in transit**, **trust stores**, **health exposure**, **abuse controls**, and **audit/redaction** are **deferred** to **v0.7**. Operators may always front the gateway with their own **TLS** terminator or VPN before then.

54. **TLS and trust (v0.7)** — Specify and implement **TLS** termination for client→gateway (and optional paths to LiteLLM/Qdrant where operators require it), optional **mTLS** between components, and **corporate/custom CA** trust when deployments require it—documented alongside a **threat model** (“trusted LAN” vs “internet-adjacent”).

55. **Health and attack surface (v0.7)** — **`/health`** may stay **unauthenticated** **prior to v0.7** in trusted setups (**#10**). **v0.7** adds **normative** options: bind to internal interfaces only, gate with auth or network policy, or reduce sensitive detail in JSON when exposed.

56. **Abuse resistance and observability hygiene (v0.7)** — Gateway-level **rate limiting**, request-size limits, **audit** logging with **redaction** policy, and documented **secrets** practices beyond YAML **mtime** reload where operators require stricter hygiene.
