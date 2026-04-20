# claudia-index (v0.4)

`claudia-index` is the workspace file indexer that ships alongside the Claudia
Gateway v0.2+. It walks configured directory roots, applies ignore rules,
hashes each file, and sends bytes via **`POST /v1/ingest`** (small/medium files)
or the **chunked session API** when the file is larger than `max_whole_file_bytes`
from `GET /v1/indexer/config` (see gateway `rag.ingest.max_whole_file_bytes`).
The gateway
chunks, embeds, and writes vectors to Qdrant, so the indexer never embeds or
chunks locally.

See [`docs/indexer.plan.md`](indexer.plan.md) for the full product plan and
non-goals; this document is the operator-facing quick start.

## Install / build

```sh
make indexer-build   # produces ./claudia-index[.exe]
make indexer-install # go install into $GOBIN
```

## Environment

| Variable                | Purpose                                    |
|-------------------------|--------------------------------------------|
| `CLAUDIA_GATEWAY_URL`   | Base URL of a running Claudia Gateway      |
| `CLAUDIA_GATEWAY_TOKEN` | Bearer token (required; never store in YAML) |

## Configuration

`claudia-index` loads **YAML config in layers** (each file optional except when
you pass **`--config`**, which must exist). Merge order (lowest → highest):

1. **`~/.claudia/indexer.config.yaml`** (user-wide; `os.UserHomeDir()` / Windows `%USERPROFILE%`)
2. **`<cwd>/.claudia/indexer.config.yaml`** (project-local)
3. **`--config path`** when set (highest among files)

Later files override earlier ones for the same keys (see `MergeFileConfig` in
`internal/indexer/config.go`). You can run with **only** layers (1)+(2) and no
`--config`, or add **`--config`** for an extra overlay. A starter overlay lives
at [`config/indexer.example.yaml`](../config/indexer.example.yaml).

After merged YAML: **environment** (`CLAUDIA_GATEWAY_URL`) overrides
`gateway_url`; **CLI** `--gateway-url` and `--root` override merged YAML for
those fields. **`CLAUDIA_GATEWAY_TOKEN`** is always from the environment (never
YAML).

```yaml
# Claudia Gateway base URL (default listen_port is 3000 in config/gateway.yaml).
# Do not point this at BiFrost (8080) — claudia-index talks to the gateway.
gateway_url: "http://127.0.0.1:3000"
roots:
  - "."
ignore_extra:
  - "tmp/"
  - "*.snapshot"
```

`tenant_id` is implied by the bearer token. **v0.3** adds optional
`defaults`, per-root, and per-glob `project_id` / `flavor_id` / `workspace_id`
in YAML. They are merged in order **defaults → root → overrides** (each
`overrides[]` glob that matches the file’s root-relative path applies on top;
later list entries win for the same field). Values are sent on every ingest as
`X-Claudia-Project` and `X-Claudia-Flavor-Id`, and the merged **defaults** are
also sent on `GET /v1/indexer/config` at startup. Match the same headers (or
Continue `config.yaml` project/flavor fields) as chat so RAG queries the same
Qdrant collection the indexer wrote to.

**v0.4:** Successful ingests record **client** and **server** SHA-256 digests under
`sync_state_path` (default `.claudia/indexer.sync-state.json`). If a file’s
client hash matches the last recorded value, the indexer **skips** re-upload.
Gateway responses include **`content_sha256`** (authoritative over UTF-8 text
bytes ingested). Optional YAML: `max_whole_file_bytes` (caps whole-body mode
when lower than the gateway), `sync_state_path`.

**Chunked ingest (large files):** each session step (**start session**, **PUT
chunk**, **complete**) retries transient errors with the same backoff settings
as whole-file ingest (`retry_max_attempts`, `retry_base_delay_ms`,
`retry_max_delay_ms`).

## Corpus inventory (reconciliation)

`GET /v1/indexer/corpus/inventory` (Bearer token; same **`X-Claudia-Project`**
/**`X-Claudia-Flavor-Id`** headers as ingest) returns **deduplicated** sources
for the scoped Qdrant collection. Query params:

- **`limit`** — max points to scan per page (default **256**, max **2000**).
- **`cursor`** — opaque value from the previous response’s **`next_cursor`**
  (omit on the first page).

Response JSON includes **`entries[]`** with **`source`**, **`content_sha256`**
(server digest over UTF-8 file bytes), optional **`client_content_hash`**
(indexer-supplied `content_hash` when present), plus **`has_more`** and
**`next_cursor`**. The gateway advertises the path on **`GET /v1/indexer/config`**
as **`corpus_inventory_path`**.

**`claudia-index`** loads all pages during the initial scan (after
**`GET /v1/indexer/config`**) and skips files whose **client** hash matches the
inventory when **`client_content_hash`** is set, or falls back to **sync state +
server SHA** when only server digests exist on older points.

## Ignore rules

The matcher is a layered gitignore-style engine that combines:

1. Built-in defaults (`.git/`, `node_modules/`, `*.bin`, `*.png`, secrets,
   etc.).
2. `ignore_extra` from the YAML config.
3. `.claudiaignore` at each root (created by you).
4. `.gitignore` at each root.

Binary files are also excluded via a NUL-byte sniff over the first ~8 KB.

## Failure handling

Per [§ Failure handling](indexer.plan.md#failure-handling-normative):

- Retry transient failures (`5xx`, `408`, `425`, `429`, network errors) with
  bounded exponential backoff (`retry_max_attempts`, `retry_base_delay_ms`,
  `retry_max_delay_ms`; defaults 5 / 500 ms / 30 s).
- After the last retry, the worker pauses and polls
  `GET /v1/indexer/storage/health` every `recovery_poll_interval_ms`
  (default 30 s). By default it **also** requires **`GET /health`** to report
  readiness (non-`503`, no `degraded` in JSON). Set **`recovery_include_root_health: false`**
  in YAML to only use storage health.
- `401`/`403` responses are treated as fatal and surfaced in logs without
  retry.

## Modes

```sh
claudia-index --config .claudia/indexer.config.yaml          # watch + ingest
claudia-index --config .claudia/indexer.config.yaml --one-shot  # scan + exit
claudia-index --root ./apps/web --gateway-url http://x:8080  # flag-only
```

In watch mode the indexer drains an initial scan, then incrementally ingests
files reported by `fsnotify` (debounced to coalesce save bursts; default
debounce 750 ms).

## Security notes

- `source` paths sent on the wire are always **relative to the configured
  root**. Absolute host paths are never transmitted.
- Symlinks are not followed (no toggle in v0.2).
- Tokens stay in the environment; YAML never contains secrets in supported releases.
