# Plan: Claudia file indexer (`claudia-index`)

This document plans a **portable Go binary** that watches configured directories, respects ignore rules, and sends **whole-file** bodies to the **Claudia Gateway** for **server-side chunking and embedding** (same strategy as [`claudia-gateway.plan.md`](claudia-gateway.plan.md): one document per request; gateway owns chunk boundaries and can change them without indexer upgrades). It complements gateway **ingest** and **indexer** APIs (`POST /v1/ingest`, `GET /v1/indexer/config`, etc.).

**Related docs:** [`cli-tool.plan.md`](cli-tool.plan.md) (configuration precedence pattern), [`claudia-gateway.plan.md`](claudia-gateway.plan.md), [`overview.md`](overview.md), [`network.md`](network.md).

---

## Goals

1. **Security-conscious identifiers** — stable document identity and `source` metadata use **paths relative to configured workspace roots**, never absolute host paths, so payloads sent to the gateway do not leak usernames, drive letters, or internal mount layouts.
2. **Portable artifact** — single **Go** binary (`claudia-index` / `claudia-index.exe`) shipped alongside or independently of `claudia`, same cross-platform story as the gateway.
3. **Incremental indexing** — on startup, compute the watch set, **reconcile with gateway-held state** (when APIs exist), enqueue work, then run incrementally with debouncing and backpressure consistent with common file-watcher tooling.
4. **Layered configuration** — `.claudia/indexer.config.yaml` (and optional global override file) with explicit **precedence**; casual users can run with **one root** and minimal YAML.
5. **Defer complex lifecycle** — **delete/rename/tombstone** semantics follow **prior art** (e.g. OpenClaw-style agents, mature indexers) in later milestones; first indexer release focuses on **add/update** paths and documented gaps.

---

## Non-goals (initial milestones)

- **Continue** as the indexer runtime (Continue remains a **chat client**; headers must **match** indexer scope per gateway plan).
- **Embedding inside the indexer** — embeddings stay on the **gateway** (LiteLLM/BiFrost path per product plan) unless a future version explicitly adds local embed models.
- **Full VS Code UI** in early indexer releases — see [§ Visual Studio Code integration](#visual-studio-code-integration).

---

## Versioning (indexer milestones)

**Indexer and gateway v0.2 align:** the first shippable **`claudia-index`** targets **gateway v0.2** (ingest + indexer config/storage APIs). Later indexer versions may add features without a gateway bump, but **v0.2** is the shared baseline for “RAG indexing works end-to-end.”

### Indexer v0.2 (initial release)

**Scope**

- **Tenant scope** — `tenant_id` is implied by the **gateway-issued Bearer token** (same token model as chat); no separate tenant field in YAML required.
- **Single or multiple roots** — configurable **watch roots** (directories); each root is a **security boundary** for relative paths (see [§ Stable document identity](#stable-document-identity)).
- **Ignore rules** — skip binary files; honor **`.claudiaignore`** (shipped template or generated defaults including entries such as `.env`); also honor **`.gitignore`** and, where feasible, other common `*ignore` patterns documented in config.
- **Symlinks** — default **do not follow** symlinks when walking the tree (more secure); no toggle in v0.2.
- **Ingest unit** — **one whole file per `POST /v1/ingest`**; **gateway** chunks, embeds, and writes vectors (see [§ Chunking and gateway contract](#chunking-and-gateway-contract)).
- **Auth** — read gateway URL and **API token from environment** (e.g. `.env` loaded by the user’s shell or documented env vars); no token in YAML yet.
- **Operational behavior** — **debouncing**, **coalescing**, and **backpressure** (bounded worker pool, queue depth limits); **failure handling** follows [§ Failure handling (normative)](#failure-handling-normative).

**Not in indexer v0.2**

- Per-path **`project_id` / `workspace_id` / `flavor_id`** overrides (deferred to **indexer v0.3**).
- Gateway **reconciliation API** (full “list remote files + content hash”) if not yet implemented on the gateway — indexer may **fallback to full backfill** of local watch set until the API exists (see [§ Startup reconciliation](#startup-reconciliation)).

### Indexer v0.3

**Scope**

- **`project_id` / `workspace_id`** and **`flavor_id`** — support **global defaults** in YAML, plus **per-root** and **per-glob** overrides (merge order documented in [§ Configuration schema](#configuration-schema-evolution)).
- **Alignment with Continue** — same values must be sent as **`X-Claudia-Project`** / **`X-Claudia-Flavor-Id`** on chat for RAG to hit the same corpus ([`claudia-gateway.plan.md`](claudia-gateway.plan.md) § Client integration).

### Indexer v0.4 — large files: dual-mode ingest + authoritative server hash

**Goal:** keep **whole-file** ingest as the default (see **v0.2**), and add an optional **second path** for **large files** that would exceed HTTP body limits or waste bandwidth on retries.

**Indexer + gateway must implement both modes** (negotiated per file or per config):

1. **Mode A — whole-file** (unchanged from v0.2): single **`POST /v1/ingest`** per file; gateway chunks server-side.
2. **Mode B — streaming / client-chunked upload** for large bodies — **normative wire format TBD** (e.g. resumable session id + ordered chunk uploads, or HTTP chunked encoding with gateway-defined framing). Gateway still **owns embedding and vector writes**; the split is **transport**, not a promise that the client chooses semantic chunk boundaries for RAG (unless explicitly specified later).

**Configuration:** threshold **e.g. file size or `GET /v1/indexer/config` field** (`max_whole_file_bytes` or similar) selects Mode A vs B.

**Content hash (this milestone):**

- **v0.2–v0.3:** **client-computed SHA-256** (or agreed algorithm) is the **source of truth** the indexer uses for change detection and sends on ingest; reconciliation compares **local client hash** to **remote stored hash** from inventory when available.
- **v0.4 adds:** gateway **computes hash over the bytes it actually ingested** (after decoding/normalization as defined in the contract) and returns **`content_sha256`** (name TBD) in the **ingest response** (and persists it for **corpus inventory**). Indexer **updates local bookkeeping** to that value so **server truth** can override client preflight hash when they differ (normalization, transcoding, or bug diagnosis).

**Deliverables:** documented APIs for Mode B, size thresholds, error/retry semantics per chunk/session, and **response body** fields for **server-side SHA**.

### Indexer v0.8+ (configuration parity with `claudiactl`)

- **Layered config files** mirroring [`cli-tool.plan.md`](cli-tool.plan.md): global `~/.claudia/indexer.config.yaml`, optional flags — see [§ Configuration precedence](#configuration-precedence).

### Indexer v0.9

- **Model-assisted indexing strategy** — optional flow: indexer (or a companion tool) sends a **directory tree summary**, **effective ignore sets**, and **config** to a **gateway or LLM** endpoint and receives a **recommended indexing strategy** (patterns, priorities, exclusions). Normative API shape is **TBD**; depends on gateway/tooling roadmap.

### Visual Studio Code integration

**Later releases** (not tied to a single indexer semver in this draft):

1. **Early extension** — surface **progress**, **logs**, and **errors** from the running `claudia-index` process (spawned by the extension or attached to an existing process).
2. **Config assistance** — open or generate **`.claudia/indexer.config.yaml`** with **sensible defaults**, wizards, or **prompt text** the user can paste to an assistant to produce a config.
3. **Richer UX** — status views, queue depth, per-root health, links to gateway storage stats.
4. **Multi-project RAG** — help users run or attach indexers for **other workspaces** / corpora (organization-dependent).

---

## Stable document identity

- **Canonical id** — derived from **`(tenant_id, root_id, path_relative_to_that_root, content_hash)`** where:
  - **`tenant_id`** comes from the token (server-side); indexer does not send raw tenant in path ids unless the gateway contract requires it in payload.
  - **`root_id`** is a stable slug for each configured watch root (config-defined or hash of normalized root path **local only**, never sent as absolute path).
  - **`path_relative_to_that_root`** is the **only** path form stored in **`source`** and used for human-readable citations.
  - **`content_hash`** — **cryptographic hash** of file bytes (e.g. **SHA-256**).
    - **Indexer v0.2–v0.3:** computed **on the client**; treated as **truth** for **local change detection** and sent with ingest so the gateway can **store** it for inventory (exact header or JSON field name is part of the ingest contract). Reconciliation uses **client hash vs remote stored hash** when the inventory API exists.
    - **Indexer v0.4+:** gateway **also** computes hash over **canonical ingested bytes** and returns it in the **ingest response**; indexer **prefers server-reported SHA** for persisted sync state when present (see [Indexer v0.4](#indexer-v04--large-files-dual-mode-ingest--authoritative-server-hash)).
- **Absolute paths** must not appear in **HTTP bodies** or logs in production modes; debug logging may redact or hash paths.

This keeps **multi-root** setups correct while avoiding **cross-machine path leakage**.

---

## Deletes, renames, and corpus lifecycle

**Indexer v0.2 — Explicitly deferred.** Behavior is **undefined** beyond “best effort”: renamed file may appear as **delete + add** once lifecycle APIs exist.

**Future** — adopt patterns from **mature indexers** and **agent platforms** (e.g. OpenClaw-style tooling) for:

- Tombstones vs hard deletes in the vector store.
- **Rename detection** (inode / content hash heuristics).
- Gateway support for **delete-by-source** or **replace-collection** operations.

Track as **open design** once gateway exposes stable operations beyond ingest.

---

## Configuration file

### Primary path

- **Repository / workspace local:** **`.claudia/indexer.config.yaml`**

(If a repo already uses `.claudia/` for other Claudia artifacts, keep a single subdirectory layout documented in the main README when implemented.)

### Configuration precedence

Aligned with [`cli-tool.plan.md`](cli-tool.plan.md) where applicable.

**From indexer v0.8** (and optionally introduced earlier in simplified form):

1. **Built-in defaults** (compiled into the binary).
2. **Global user config:** `~/.claudia/indexer.config.yaml` (resolve home via `os.UserHomeDir()`; on Windows `%USERPROFILE%\.claudia\indexer.config.yaml`).
3. **Local project config:** `./.claudia/indexer.config.yaml` relative to the **current working directory** when the indexer starts (or explicit `--config` path that overrides “local” discovery).
4. **CLI flags** — override merged YAML for the same keys (e.g. `--gateway-url`, `--token-file`, `--root`).

**Merge rule:** later layers override earlier for the same key; missing keys fall through.

**Indexer v0.2 shortcut:** support **only** env vars + a **single explicit `--config` pointing at `.claudia/indexer.config.yaml`** if full merge is not ready day one; still document the **target** precedence above.

### Configuration schema (evolution)

**v0.2 — minimal**

- `gateway_url` (or env `CLAUDIA_GATEWAY_URL`)
- `roots`: list of directory paths to watch
- Optional `ignore_extra`: list of glob patterns added to `.claudiaignore` semantics
- **Backoff / recovery** — optional overrides for [§ Failure handling (normative)](#failure-handling-normative): e.g. `retry_max_attempts`, `retry_base_delay`, `retry_max_delay`, `recovery_poll_interval` (see gateway contract for health URLs).

**v0.3 — scoped overrides**

```yaml
# Illustrative only — final schema when implementing.
defaults:
  project_id: "my-app"
  flavor_id: "default"

roots:
  - path: "./apps/web"
    project_id: "web"
  - path: "./legacy"
    flavor_id: "legacy-corpus"

overrides:
  - glob: "**/*.md"
    flavor_id: "docs"
```

**Later** — per-glob **lifecycle** and **priority** rules (queue ordering, inclusion/exclusion strategies).

---

## Ignore rules

1. **Binary detection** — skip non-text files via extension allowlist/denylist + content sniff where appropriate.
2. **`.claudiaignore`** — first-class file with **sensible defaults** (e.g. `.env`, secrets, large artifacts); `claudia-index init` (future) may generate it.
3. **`.gitignore`** — honored when present (reuse a well-tested Go library or stdlib + gitignore parser).
4. **Other `*ignore` files** — optional phased support; document which names (e.g. `.dockerignore`) are honored per release.

---

## Chunking and gateway contract

**Product decision (this plan):** the **indexer sends the whole file** (multipart **`file`** and/or JSON fields per gateway schema); the **gateway** applies **`chunk_size`**, **`chunk_overlap`**, **embedding**, and **Qdrant** writes. That matches [`claudia-gateway.plan.md`](claudia-gateway.plan.md) (**one document per request**; gateway chunking defaults configurable and surfaced via **`GET /v1/indexer/config`**).

**Rationale:** the service works at **file** (document) granularity for ingest APIs and storage evolution; chunking strategy can improve **without** shipping a new indexer.

**Indexer responsibilities (v0.2):** read file bytes, compute **client `content_hash`**, set **`source`** to the **relative path**, call **`POST /v1/ingest`**; obey **max request size** limits — files over the limit are **skipped or errored** until [Indexer v0.4](#indexer-v04--large-files-dual-mode-ingest--authoritative-server-hash) **dual-mode** ingest exists.

**Future (v0.4):** same logical **file** may use **whole-file** or **streaming/chunked** transport per threshold; gateway remains responsible for **chunking for embedding** after assembly.

Embedding model and vector dimensions remain **gateway-owned**; indexer **must** refresh config when **`GET /v1/indexer/config`** reports changes (see [§ Version skew and embedding settings](#version-skew-and-embedding-settings)).

---

## Failure handling (normative)

For **ingest failures** (transient HTTP errors, **503**, **429**, network errors) where the response does **not** explicitly require the client to **stop permanently** (contrast: **401** / **403** — treat as **fatal / operator action**, do not infinite-retry):

1. **Retry with exponential backoff** — **configurable** **`retry_max_attempts`** (small integer, e.g. default **5**), **`retry_base_delay`**, **`retry_max_delay`** (cap per wait). Jitter optional. Apply per failing operation or per batch per implementation, but **must** bound total attempts.
2. **After the last backoff attempt fails** — **pause** the ingest **queue** (do **not** discard queued work; continue **collecting** filesystem events if desired, subject to backpressure limits).
3. **Recovery polling** — while paused, periodically call gateway **status** endpoints to determine whether **ingest / RAG storage** is available again:
   - **`GET /v1/indexer/storage/health`** (Bearer token; scoped per [`claudia-gateway.plan.md`](claudia-gateway.plan.md) **indexer REST**), and
   - optionally **`GET /health`** for overall gateway / upstream readiness.
4. **Resume** when responses indicate **healthy / not degraded** for the paths relevant to ingest (exact JSON fields documented with gateway implementation). **Reset** backoff state for subsequent failures.

**Configurable** **`recovery_poll_interval`** (e.g. default **30s**) governs how often to poll while paused.

Document defaults and env overrides in the indexer **README** when implemented.

---

## Authentication

- **v0.2:** Bearer token from **environment** (e.g. `CLAUDIA_GATEWAY_TOKEN`); document loading from `.env` via user workflow (shell, `direnv`, etc.).
- **Later:** read token (or path to token file) from **YAML** per [§ Configuration precedence](#configuration-precedence); never commit secrets; recommend `.gitignore` for `.claudia/indexer.config.yaml` when it holds tokens.

---

## Path allowlist and symlinks

- **v0.2:** Only index under configured **`roots`**; **do not follow symlinks** by default when enumerating files.
- **Later:** configuration toggle to **follow symlinks** with explicit warning in docs (security + duplicate path risk).

---

## Startup reconciliation

**Desired behavior**

1. On start, compute the **candidate file set** from all roots (after ignores).
2. Call the gateway (or indexer API) to obtain **remote inventory** for the authenticated **tenant** (and, from **v0.3**, **project** / **flavor** scope): e.g. **paths + `content_hash`** the gateway stores or aggregates from Qdrant payload.
3. Compute **diff**: enqueue **uploads** for missing files or paths whose **local hash ≠ remote hash**.
4. Run workers with **backpressure**; transient failures follow [§ Failure handling (normative)](#failure-handling-normative).

**Gateway gap:** [`claudia-gateway.plan.md`](claudia-gateway.plan.md) currently specifies **`GET /v1/indexer/storage/stats`** (aggregate) but not per-document **path + hash inventory**. Add a **normative endpoint or contract extension** (e.g. **`GET /v1/indexer/corpus/state`** with pagination) as part of **gateway + indexer joint delivery**; until then, indexer may **queue full scan** on each cold start (document cost).

---

## Version skew and embedding settings

On **every startup** (and periodically during long runs), the indexer **SHOULD** call **`GET /v1/indexer/config`** with the same **Bearer token** and (from **v0.3**) **`X-Claudia-Project` / `X-Claudia-Flavor-Id`** as appropriate.

**Use returned fields for:**

- **`embedding_model`**, **`chunk_size`**, **`chunk_overlap`** (inform logging / version skew only; **indexer does not chunk**), **`ingest_path`**, required headers.
- **`gateway_version`** — log and optionally trigger **full reindex** if major embedding/collection rules change.

**Optional future:** same response (or **`GET /v1/indexer/storage/stats`**) includes **point counts** or **per-corpus checksums** to inform reconciliation (depends on gateway implementation).

**v0.4+:** **`GET /v1/indexer/config`** (or ingest response) may advertise **`max_whole_file_bytes`** and **dual-mode** capability flags so the indexer selects Mode A vs B without hardcoding.

---

## Binary and module layout

| Item | Proposal |
|------|----------|
| **Go package** | `cmd/claudia-index` |
| **Artifact name** | `claudia-index` (Unix), `claudia-index.exe` (Windows) |
| **Shared logic** | `internal/indexer/*` — config load/merge, ignore engine, hashing, queue, gateway client |
| **Import path** | Same module as gateway (`go.mod` at repo root) unless packaging later splits modules |

---

## Makefile (when implemented)

| Target | Behavior |
|--------|----------|
| **`make indexer-build`** | `go build -o claudia-index[.exe] ./cmd/claudia-index` |
| **`make indexer-run`** | `go run ./cmd/claudia-index` with passthrough args |
| **`make indexer-install`** | `go install ./cmd/claudia-index` |

Update `scripts/print-make-help.sh` and **clean** scripts to remove `claudia-index[.exe]` consistently with other binaries.

---

## Testing

- **Unit:** ignore matching, relative path canonicalization, **content hash** computation, config merge order (v0.8+).
- **Integration:** `httptest` for gateway client; optional testcontainers or mocked **`GET /v1/indexer/config`** / ingest.

---

## Documentation deliverables (when implemented)

- **`README.md`** (or `docs/indexer.md`) — install, env vars, `.claudia/indexer.config.yaml` example, Continue header alignment for **v0.3**, [§ Failure handling (normative)](#failure-handling-normative) defaults.
- **Security** — no absolute paths in payloads; symlink default; secret handling.
- **Gateway API** — link to finalized **whole-file ingest** schema (v0.2), **v0.4** dual-mode / streaming protocol, **ingest response** (**server SHA**), and **corpus state** endpoint.

---

## Open decisions

1. **v0.2 — Large files under whole-file-only** — max body size for **`POST /v1/ingest`**; indexer behavior when over limit (**skip** vs **fail loud**) until **v0.4** dual-mode exists.
2. **v0.4 — Mode B wire protocol** — session lifecycle, chunk size, idempotency keys, resume after partial failure; must align with gateway streaming implementation.
3. **Corpus inventory endpoint** — schema (**path key**, **`content_hash`**, pagination); **authz** per tenant/project/flavor; **v0.4** stores **server-computed** hash for truth after ingest.
4. **Delete/rename** — first gateway primitive (tombstone, delete-by-filter, or reindex-only).
5. **Durable queue format** — SQLite vs JSONL vs embedded store for offline resilience while **paused**.
6. **Binary name** — `claudia-index` vs shorter alias; align with `make` targets and docs.

---

## Implementation checklist (summary)

**Indexer v0.2**

- [ ] `cmd/claudia-index`: config discovery (minimal), env-based token, watch roots, ignores (.claudiaignore + .gitignore), no symlink follow by default.
- [ ] **Whole-file** ingest; **`content_hash`** (e.g. SHA-256) for local change detection and future reconciliation.
- [ ] Gateway HTTP client: **`GET /v1/indexer/config`**, **`POST /v1/ingest`**, **`GET /v1/indexer/storage/health`**, optional **`GET /health`** — implement [§ Failure handling (normative)](#failure-handling-normative).
- [ ] Debounced change handling, worker pool, bounded queue / backpressure.
- [ ] Stable **relative** `source` (no absolute paths on wire).
- [ ] Makefile targets + help text + clean script updates.
- [ ] README / docs snippet for operators.

**Indexer v0.3**

- [ ] `project_id` / `flavor_id` (and `workspace_id` alias if adopted) in YAML: defaults, per-root, per-glob.
- [ ] Send matching headers on ingest; document parity with Continue `config.yaml`.

**Indexer v0.4**

- [ ] **Dual-mode ingest:** whole-file (Mode A) + streaming/chunked large-file path (Mode B); threshold from config or **`GET /v1/indexer/config`**.
- [ ] Parse **ingest response** **server `content_sha256`** (name TBD); persist for reconciliation / conflict vs client hash.
- [ ] Retry and **pause/resume** semantics extended to **chunk sessions** per open decisions.

**Indexer v0.8**

- [ ] Full config precedence: defaults → `~/.claudia/indexer.config.yaml` → `./.claudia/indexer.config.yaml` → flags.

**Indexer v0.9**

- [ ] Optional LLM-assisted strategy generation (API TBD).

**Gateway coordination (not indexer-only)**

- [ ] **`POST /v1/ingest`** — **whole-file** document schema (v0.2); **server-side** chunking per existing gateway plan; accept **client** `content_hash` field and store for inventory.
- [ ] Define **corpus state / inventory** API for startup reconciliation (**path** + **`content_hash`**).
- [ ] Document **`GET /v1/indexer/storage/health`** (and **`GET /health`**) fields used by indexer **resume** logic.
- [ ] **v0.4:** **Mode B** streaming/chunked ingest API; **compute and return** canonical **server SHA** on success; persist for inventory; advertise limits/capability in **`GET /v1/indexer/config`**.

---

*Plan status: **draft for implementation** — aligns with product direction in [`claudia-gateway.plan.md`](claudia-gateway.plan.md); **indexer v0.2** and **gateway v0.2** are the shared baseline for ingest; **indexer v0.4** adds dual-mode large-file ingest and **server-returned SHA** for authoritative sync state.*
