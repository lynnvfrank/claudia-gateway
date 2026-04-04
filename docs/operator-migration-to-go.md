# Operator migration: Docker + TypeScript → Go binary

This guide is for operators already running the **Compose** stack (or the **TypeScript** gateway under `npm run dev`) who want to run the **Go** `claudia` implementation with the **same** YAML configuration.

## Why migrate

- **Single static binary** (plus optional `qdrant` from releases) — no Node runtime for the gateway.
- **`claudia serve`** can supervise **BiFrost** (and optionally **Qdrant**) on bare metal or VMs.
- **Feature parity** with v0.1 TypeScript for `config/gateway.yaml`, `tokens.yaml`, and `routing-policy.yaml` (see [configuration.md](configuration.md)).

## What stays the same

| Item | Notes |
|------|--------|
| `config/gateway.yaml` | Same fields; `litellm.base_url` still names the upstream (BiFrost or LiteLLM). |
| `config/tokens.yaml` | Same format; mtime reload. |
| `config/routing-policy.yaml` | Same format; mtime reload. |
| Env | `CLAUDIA_UPSTREAM_API_KEY` (or name from `api_key_env`), `CLAUDIA_GATEWAY_CONFIG`, `LOG_LEVEL`, provider keys for BiFrost. |

## Migration steps

1. **Install or build Go `claudia`**  
   - From source: `make claudia-build` → `./claudia`  
   - Or extract a release archive per [packaging.md](packaging.md).

2. **Stop the TypeScript gateway only** (if it holds port 3000), or point Go at another port:  
   `./claudia -listen :3001`  
   Keep **BiFrost** (and **LiteLLM**/Postgres if used) running as today, or use **`claudia serve`** to start BiFrost locally — [supervisor.md](supervisor.md).

3. **Point `litellm.base_url` at a URL reachable from the Go process**  
   - Same as for Node: e.g. `http://127.0.0.1:8080` when BiFrost is on the host, or `http://bifrost:8080` only if Go runs **inside** the same Docker network.

4. **Smoke checks**  
   - `curl -sS http://127.0.0.1:3000/health`  
   - One chat completion: [e2e-operator-path.md](e2e-operator-path.md) and **`scripts/e2e-first-chat-curl.sh`**.

5. **Compose**  
   The default **`docker-compose.yml`** **`claudia`** service still builds the **TypeScript** image until that Dockerfile is switched. To use **Go** under Compose, run `claudia` on the host with `-listen` and published ports, or add a separate service image when available — no change is required in this repo for a host-side Go deployment.

## Rollback

Run the **Compose** `claudia` service again (or `npm run dev`) with the same config files; no schema change is required between TS and Go for v0.1 parity.

## Security

See [SECURITY.md](../SECURITY.md) at the repo root (logging redaction, bind address, supervisor binaries).
