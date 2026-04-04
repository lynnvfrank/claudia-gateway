# Installation, setup, and startup

## Prerequisites

- **Go 1.22+** (for building from source)
- **BiFrost** HTTP binary (`bifrost-http`), e.g. from **`make bifrost-from-src`** — see [supervisor.md](supervisor.md)
- Provider API keys for BiFrost (**`GROQ_API_KEY`**, **`GEMINI_API_KEY`**, etc. per **`config/bifrost.config.json`**)

## Steps

1. **Clone** this repository and `cd` into it.

2. **Environment (optional)**  
   Copy **`env.example`** to **`.env`** in the directory from which you run **`claudia`**, or export:
   - **`CLAUDIA_UPSTREAM_API_KEY`** — Bearer token the gateway sends upstream (BiFrost: any non-empty placeholder unless governance keys are enabled). Must match **`litellm.api_key_env`** in **`config/gateway.yaml`**.
   - **`GROQ_API_KEY`** / **`GEMINI_API_KEY`** — as referenced in **`config/bifrost.config.json`** (inherited by BiFrost when using **`claudia serve`**).

3. **Gateway tokens**  
   **`config/tokens.yaml`** is not committed (see **`.gitignore`**). Copy **`config/tokens.example.yaml`** to **`config/tokens.yaml`** and set a strong secret. Clients send it as **`Authorization: Bearer …`**.

4. **Align model names**  
   **`config/gateway.yaml`** → **`routing.fallback_chain`** must list upstream model ids (**BiFrost**: **`provider/model`**) that exist in your BiFrost catalog.

5. **Start BiFrost and the gateway**  
   Easiest path:

   ```bash
   export CLAUDIA_UPSTREAM_API_KEY=bifrost-local-dummy
   make bifrost-from-src    # once
   make claudia-serve-local
   ```

   Or run **`./claudia`** alone if BiFrost is already listening at **`litellm.base_url`** in **`config/gateway.yaml`** (default **`http://127.0.0.1:8080`**).

6. **Verify health**

   ```bash
   curl -sS http://127.0.0.1:3000/health
   ```

   Expect **`200`** JSON with **`"status": "ok"`** and **`checks.litellm.ok": true`** when the upstream is reachable.

7. **List models (authenticated)**

   ```bash
   curl -sS -H "Authorization: Bearer <your-gateway-token>" http://127.0.0.1:3000/v1/models
   ```

   The first model **`id`** should be **`Claudia-0.1.0`** (or your configured semver).
