# End-to-end operator path (local Go)

Documented path from clone to a first successful **`curl`** chat.

## Preconditions

- **`config/tokens.yaml`** with at least one gateway token (copy from **`config/tokens.example.yaml`**).
- **`CLAUDIA_UPSTREAM_API_KEY`** set (or the env name in **`gateway.yaml`** **`litellm.api_key_env`**).
- **BiFrost** running and reachable at **`litellm.base_url`** (e.g. **`make claudia-serve-local`** after **`make bifrost-from-src`**), with provider keys in the environment for **`config/bifrost.config.json`**.

## Path

1. Clone the repo.
2. Copy **`config/gateway.yaml`** / **`tokens.yaml`** / **`routing-policy.yaml`** next to **`claudia`**, preserving paths referenced in YAML.
3. Export env: **`export CLAUDIA_UPSTREAM_API_KEY=…`** (and **`GROQ_API_KEY`** / **`GEMINI_API_KEY`** as needed for BiFrost).
4. Run **`./claudia`** or **`./claudia serve`** (see [supervisor.md](supervisor.md)).
5. **`curl`** health: **`curl -sS http://127.0.0.1:3000/health`**
6. First chat (replace token and model id):

   **`./scripts/e2e-first-chat-curl.sh http://127.0.0.1:3000 '<gateway-token>' 'Claudia-<semver>'`**

## From zero on one machine

1. **`cp config/tokens.example.yaml config/tokens.yaml`** and edit.
2. **`make bifrost-from-src`** (once) and **`make claudia-serve-local`**.
3. **`export CLAUDIA_UPSTREAM_API_KEY=bifrost-local-dummy`** (plus provider keys).
4. Run **`./scripts/e2e-first-chat-curl.sh`** as above.
