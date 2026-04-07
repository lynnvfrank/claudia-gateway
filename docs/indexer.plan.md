# Plan: Claudia file indexer (`claudia-index`)

This document plans a **portable Go binary** that watches configured directories, respects ignore rules, **chunks content locally**, and synchronizes with the **Claudia Gateway** ingest and (future) indexer APIs. It complements the **gateway** responsibilities described in [`claudia-gateway.plan.md`](claudia-gateway.plan.md) (RAG, Qdrant, `POST /v1/ingest`, `GET /v1/indexer/config`, etc.).

**Related docs:** [`cli-tool.plan.md`](cli-tool.plan.md) (configuration precedence pattern), [`claudia-gateway.plan.md`](claudia-gateway.plan.md), [`overview.md`](overview.md), [`network.md`](network.md).

---

## Goals

1. **Security-conscious identifiers** — stable document identity and `source` metadata use **paths relative to configured workspace roots**, never absolute host paths, so payloads sent to the gateway do not leak usernames, drive letters, or internal mount layouts.
2. **Portable artifact** — single **Go** binary (`claudia-index` / `claudia-index.exe`) shipped alongside or independently of `claudia`, same cross-platform story as the gateway.
3. **Incremental indexing** — on startup, compute the watch set, **reconcile with gateway-held state** (when APIs exist), enqueue work, then run incrementally with debouncing and backpressure consistent with common file-watcher tooling.
4. **Layered configuration** — `.claudia/indexer.config.yaml` (and optional global override file) with explicit **precedence**; casual users can run with **one root** and minimal YAML.
5. **Defer complex lifecycle** — **delete/rename/tombstone** semantics follow **prior art** (e.g. OpenClaw-style agents, mature indexers) in later milestones; v0.1 focuses on **add/update** paths and documented gaps.

---

## Non-goals (initial milestones)

- **Continue** as the indexer runtime (Continue remains a **chat client**; headers must **match** indexer scope per gateway plan).
- **Embedding inside the indexer** — embeddings stay on the **gateway** (LiteLLM/BiFrost path per product plan) unless a future version explicitly adds local embed models.
- **Full VS Code UI** in v0.1–v0.2 — see [§ Visual Studio Code integration](#visual-studio-code-integration).

---

## Versioning (indexer milestones)

Indexer releases are **numbered separately** from the **gateway** semver but are **paired in docs** (e.g. “requires gateway ≥ v0.2 for ingest”).

### Indexer v0.1

**Scope**

- **Tenant scope** — `tenant_id` is implied by the **gateway-issued Bearer token** (same token model as chat); no separate tenant field in YAML required.
- **Single or multiple roots** — configurable **watch roots** (directories); each root is a **security boundary** for relative paths (see [§ Stable document identity](#stable-document-identity)).
- **Ignore rules** — skip binary files; honor **`.claudiaignore`** (shipped template or generated defaults including entries such as `.env`); also honor **`.gitignore`** and, where feasible, other common `*ignore` patterns documented in config.
- **Symlinks** — default **do not follow** symlinks when walking the tree (more secure); no toggle in v0.1.
- **Chunking** — performed **in the indexer** before upload (see [§ Chunking and gateway contract](#chunking-and-gateway-contract)).
- **Auth** — read gateway URL and **API token from environment** (e.g. `.env` loaded by the user’s shell or documented env vars); no token in YAML yet.
- **Operational behavior** — **debouncing**, **coalescing**, and **backpressure** align with **common file-watcher / sync tools** (bounded worker pool, queue depth limits, exponential backoff on failures); exact constants are implementation details, not normative in this plan.
- **Offline / HTTP 503** — same class of behavior as those tools: **pause or retry with backoff**, optional **durable queue** stub (implementation may start with in-memory + disk spill in a later patch; document chosen behavior in README when implemented).

**Not in indexer v0.1**

- Per-path **`project_id` / `workspace_id` / `flavor_id`** overrides (deferred to **indexer v0.2**).
- Gateway **reconciliation API** (full “list remote files + mtime”) if not yet implemented on the gateway — indexer may **fallback to full backfill** of local watch set until the API exists (see [§ Startup reconciliation](#startup-reconciliation)).

### Indexer v0.2

**Scope**

- **`project_id` / `workspace_id`** and **`flavor_id`** — support **global defaults** in YAML, plus **per-root** and **per-glob** overrides (merge order documented in [§ Configuration schema](#configuration-schema-evolution)).
- **Alignment with Continue** — same values must be sent as **`X-Claudia-Project`** / **`X-Claudia-Flavor-Id`** on chat for RAG to hit the same corpus ([`claudia-gateway.plan.md`](claudia-gateway.plan.md) § Client integration).

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

- **Canonical id** — derived from **`(tenant_id, root_id, path_relative_to_that_root, content_revision)`** where:
  - **`tenant_id`** comes from the token (server-side); indexer does not send raw tenant in path ids unless the gateway contract requires it in payload.
  - **`root_id`** is a stable slug for each configured watch root (config-defined or hash of normalized root path **local only**, never sent as absolute path).
  - **`path_relative_to_that_root`** is the **only** path form stored in **`source`** and used for human-readable citations.
- **Absolute paths** must not appear in **HTTP bodies** or logs in production modes; debug logging may redact or hash paths.

This keeps **multi-root** setups correct while avoiding **cross-machine path leakage**.

---

## Deletes, renames, and corpus lifecycle

**v0.1 — Explicitly deferred.** Behavior is **undefined** beyond “best effort”: renamed file may appear as **delete + add** once lifecycle APIs exist.

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

**Indexer v0.1 shortcut:** support **only** env vars + a **single explicit `--config` pointing at `.claudia/indexer.config.yaml`** if full merge is not ready day one; still document the **target** precedence above.

### Configuration schema (evolution)

**v0.1 — minimal**

- `gateway_url` (or env `CLAUDIA_GATEWAY_URL`)
- `roots`: list of directory paths to watch
- Optional `ignore_extra`: list of glob patterns added to `.claudiaignore` semantics
- Chunking parameters **may** duplicate gateway defaults temporarily; **preferred** is to call **`GET /v1/indexer/config`** when the gateway implements it and use returned **`chunk_size` / `chunk_overlap`** unless overridden.

**v0.2 — scoped overrides**

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

**Product decision (this plan):** the **indexer performs chunking** before upload.

**Coordination with [`claudia-gateway.plan.md`](claudia-gateway.plan.md):** that document currently describes gateway-side chunking on **`POST /v1/ingest`**. Implementers must **reconcile** one of:

- **A.** Ingest accepts **pre-chunked** records (array of chunks with shared `source`) in one request, or
- **B.** Indexer issues **multiple** `POST /v1/ingest` calls per file (one chunk per request), or
- **C.** Gateway adds an explicit **`POST /v1/ingest/chunks`** (name TBD) for batch chunk upload.

Pick **one** contract during gateway v0.2 implementation and document it in `docs/` next to this file.

Embedding model and vector dimensions remain **gateway-owned**; indexer **must** refresh config when **`GET /v1/indexer/config`** reports changes (see [§ Version skew and embedding settings](#version-skew-and-embedding-settings)).

---

## Authentication

- **v0.1:** Bearer token from **environment** (e.g. `CLAUDIA_GATEWAY_TOKEN`); document loading from `.env` via user workflow (shell, `direnv`, etc.).
- **Later:** read token (or path to token file) from **YAML** per [§ Configuration precedence](#configuration-precedence); never commit secrets; recommend `.gitignore` for `.claudia/indexer.config.yaml` when it holds tokens.

---

## Path allowlist and symlinks

- **v0.1:** Only index under configured **`roots`**; **do not follow symlinks** by default when enumerating files.
- **Later:** configuration toggle to **follow symlinks** with explicit warning in docs (security + duplicate path risk).

---

## Startup reconciliation

**Desired behavior**

1. On start, compute the **candidate file set** from all roots (after ignores).
2. Call the gateway (or indexer API) to obtain **remote inventory** for the authenticated **tenant** (and, from v0.2, **project** / **flavor** scope): e.g. **paths + last modified** or **content revision** the gateway stores or proxies from Qdrant payload.
3. Compute **diff**: enqueue **uploads** for missing or stale local files.
4. Run workers with **backpressure**; on gateway **503** / storage unhealthy, **retry with backoff** (behavior aligned with common sync tools).

**Gateway gap:** [`claudia-gateway.plan.md`](claudia-gateway.plan.md) currently specifies **`GET /v1/indexer/storage/stats`** (aggregate) but not per-point **path inventory**. Add a **normative endpoint or contract extension** (e.g. **`GET /v1/indexer/corpus/state`** with pagination) as part of **gateway + indexer joint delivery**; until then, indexer may **queue full scan** on each cold start (document cost).

---

## Version skew and embedding settings

On **every startup** (and periodically during long runs), the indexer **SHOULD** call **`GET /v1/indexer/config`** with the same **Bearer token** and (from v0.2) **`X-Claudia-Project` / `X-Claudia-Flavor-Id`** as appropriate.

**Use returned fields for:**

- **`embedding_model`**, **`chunk_size`**, **`chunk_overlap`** (if indexer is allowed to override local chunking to match server), **`ingest_path`**, required headers.
- **`gateway_version`** — log and optionally trigger **full reindex** if major embedding/collection rules change.

**Optional future:** same response (or **`GET /v1/indexer/storage/stats`**) includes **point counts** or **per-corpus checksums** to inform reconciliation (depends on gateway implementation).

---

## Binary and module layout

| Item | Proposal |
|------|----------|
| **Go package** | `cmd/claudia-index` |
| **Artifact name** | `claudia-index` (Unix), `claudia-index.exe` (Windows) |
| **Shared logic** | `internal/indexer/*` — config load/merge, ignore engine, chunker, queue, gateway client |
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

- **Unit:** ignore matching, relative path canonicalization, chunk boundary rules, config merge order (v0.8+).
- **Integration:** `httptest` for gateway client; optional testcontainers or mocked **`GET /v1/indexer/config`** / ingest.

---

## Documentation deliverables (when implemented)

- **`README.md`** (or `docs/indexer.md`) — install, env vars, `.claudia/indexer.config.yaml` example, Continue header alignment for v0.2.
- **Security** — no absolute paths in payloads; symlink default; secret handling.
- **Gateway API** — link to finalized ingest chunk contract and any **corpus state** endpoint.

---

## Open decisions

1. **Ingest API shape** for **indexer-side chunking** (single batch vs multiple POSTs vs new route) — **blocker** for joint gateway v0.2 + indexer release.
2. **Corpus inventory endpoint** — schema (path key, mtime vs hash, pagination); **authz** per tenant/project/flavor.
3. **Delete/rename** — first gateway primitive (tombstone, delete-by-filter, or reindex-only).
4. **Durable queue format** — SQLite vs JSONL vs embedded store for offline resilience.
5. **Binary name** — `claudia-index` vs shorter alias; align with `make` targets and docs.

---

## Implementation checklist (summary)

**Indexer v0.1**

- [ ] `cmd/claudia-index`: config discovery (minimal), env-based token, watch roots, ignores (.claudiaignore + .gitignore), no symlink follow by default.
- [ ] Local chunking; gateway HTTP client for **`GET /v1/indexer/config`** (when available) and **`POST /v1/ingest`** per chosen chunk contract.
- [ ] Debounced change handling, worker pool, backoff on failure.
- [ ] Stable **relative** `source` and document ids (no absolute paths on wire).
- [ ] Makefile targets + help text + clean script updates.
- [ ] README / docs snippet for operators.

**Indexer v0.2**

- [ ] `project_id` / `flavor_id` (and `workspace_id` alias if adopted) in YAML: defaults, per-root, per-glob.
- [ ] Send matching headers on ingest; document parity with Continue `config.yaml`.

**Indexer v0.8**

- [ ] Full config precedence: defaults → `~/.claudia/indexer.config.yaml` → `./.claudia/indexer.config.yaml` → flags.

**Indexer v0.9**

- [ ] Optional LLM-assisted strategy generation (API TBD).

**Gateway coordination (not indexer-only)**

- [ ] Define and implement **ingest** contract for **pre-chunked** content.
- [ ] Define **corpus state / inventory** API for startup reconciliation.
- [ ] Document point payload fields for **mtime** / revision for incremental sync.

---

*Plan status: **draft for implementation** — aligns with product direction in [`claudia-gateway.plan.md`](claudia-gateway.plan.md); indexer milestones are **independent semver** but **gate** on gateway APIs where noted.*
