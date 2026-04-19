# claudia-index (v0.2)

`claudia-index` is the workspace file indexer that ships alongside the Claudia
Gateway v0.2. It walks configured directory roots, applies ignore rules,
hashes each file, and POSTs the bytes to `POST /v1/ingest`. The gateway
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

`claudia-index --config .claudia/indexer.config.yaml` is the v0.2 entry
point. A starter file lives at [`config/indexer.example.yaml`](../config/indexer.example.yaml).

Precedence (lowest → highest): built-in defaults → file → environment
(`CLAUDIA_GATEWAY_URL`) → CLI flags (`--gateway-url`, `--root`).

```yaml
gateway_url: "http://127.0.0.1:8080"
roots:
  - "."
ignore_extra:
  - "tmp/"
  - "*.snapshot"
```

`tenant_id` is implied by the bearer token; `project_id` and `flavor_id` per
root come in indexer **v0.3** (gateway already accepts the headers today).

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
  (default 30 s). The job is re-enqueued once the gateway reports healthy.
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
- Tokens stay in the environment; YAML never contains secrets in v0.2.
