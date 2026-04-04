# End-to-end path: install → first chat (CLI)

Documented **operator** path from a **fresh machine** (or clean checkout) to a **first successful** non-streaming chat via **`curl`**. The **Fyne GUI** (`./claudia-gui`) does not start the gateway; use **`claudia`** / **`claudia serve`** for API access.

## Prerequisites

- **Go 1.22+** (if building from source) *or* a **release archive** from [packaging.md](packaging.md).
- **BiFrost** (or LiteLLM) reachable at the URL in **`litellm.base_url`**, with provider keys configured upstream.
- **`config/tokens.yaml`** with at least one gateway token (copy from `config/tokens.example.yaml`).
- **`CLAUDIA_UPSTREAM_API_KEY`** set (or the env name in `gateway.yaml`).

## Path A — Release binary

1. Download and extract the archive for your OS/arch; `cd` into the extracted directory.
2. Copy **`config/gateway.yaml`**, **`config/tokens.yaml`**, **`config/routing-policy.yaml`** from this repo (or your existing deploy) next to **`claudia`**, preserving relative paths referenced in YAML.
3. Export env: `export CLAUDIA_UPSTREAM_API_KEY=…` (and provider keys if BiFrost reads them from the environment).
4. Start the gateway:  
   `./claudia`  
   Or with BiFrost as a subprocess:  
   `./claudia serve` (see [supervisor.md](supervisor.md) for flags).
5. Verify: `curl -sS http://127.0.0.1:3000/health`
6. Run:  
   `./scripts/e2e-first-chat-curl.sh http://127.0.0.1:3000 '<gateway-token>' 'Claudia-<semver>'`  
   Use the virtual model id from **`gateway.yaml`** `gateway.semver` (`Claudia-` + semver), or any upstream model id your routing allows.

## Path B — Build from source

1. Clone the repo; `cp config/tokens.example.yaml config/tokens.yaml` and edit.
2. `make claudia-build`
3. Set `CLAUDIA_UPSTREAM_API_KEY` and start `./claudia` or `./claudia serve`.
4. Same **`curl`** checks as Path A (invoke **`scripts/e2e-first-chat-curl.sh`** from repo root).

## Regression automation

CI and local development: **`go test ./...`** (and **`scripts/smoke-go-gateway.sh`**). Phase 2 parity tests remain the authoritative **regression** suite for HTTP behavior.
