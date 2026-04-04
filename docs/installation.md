# Installation, setup, and startup

## Prerequisites

- **Docker** and **Docker Compose V2**
- At least one **provider API key** for **BiFrost** (`config/bifrost.config.json`, e.g. **`GROQ_API_KEY`** / **`GEMINI_API_KEY`**) and/or models in `config/litellm_config.yaml` if you use LiteLLM
- A shell on the host to copy config templates and create `.env`

## Steps

1. **Clone or copy** this repository and `cd` into it.

2. **Environment file**  
   Copy `env.example` to `.env` and set:
   - **`CLAUDIA_UPSTREAM_API_KEY`** — Bearer token the gateway sends to its upstream (**BiFrost**: any non-empty placeholder unless governance keys are enabled; default in Compose: `bifrost-local-dummy`). Must match `litellm.api_key_env` in `config/gateway.yaml`.
   - **`LITELLM_MASTER_KEY`** — LiteLLM proxy key (admin/UI and proxy API); required for the **litellm** service, not for the gateway when `api_key_env` is **`CLAUDIA_UPSTREAM_API_KEY`**.
   - **`LITELLM_UI_USERNAME`** / **`LITELLM_UI_PASSWORD`** — credentials for **http://localhost:4000/ui**. These only work when LiteLLM has **`DATABASE_URL`**; the Compose stack includes **PostgreSQL** and sets `DATABASE_URL` from **`POSTGRES_*`**. If you omit or mismatch Postgres vars, the UI often reports *Invalid credentials* even when the password is “right”.
   - **`POSTGRES_USER`**, **`POSTGRES_PASSWORD`**, **`POSTGRES_DB`** — must stay consistent with the `DATABASE_URL` line in `docker-compose.yml` (same user/password/db as in `.env`).
   - **`GROQ_API_KEY`** / **`GEMINI_API_KEY`** / **`OPENAI_API_KEY`** — as needed for BiFrost and/or `litellm_config.yaml`.

3. **Gateway tokens**  
   `config/tokens.yaml` is **not committed** (see `.gitignore`). Copy `config/tokens.example.yaml` to `config/tokens.yaml` and replace the placeholder token with a strong secret. Clients send this value as **`Authorization: Bearer …`** (Continue **`apiKey`**).

4. **Align model names**  
   `config/gateway.yaml` → `routing.fallback_chain` must list upstream model ids (**BiFrost**: `provider/model`; **LiteLLM**: `model_name` from `config/litellm_config.yaml`).

5. **Start the stack**

   ```bash
   docker compose up -d --build
   ```

6. **Verify health**

   ```bash
   curl -sS http://localhost:3000/health
   ```

   Expect **`200`** JSON with **`status": "ok"`** and **`checks.litellm.ok": true`** when the configured upstream (default BiFrost) is reachable.

7. **List models (authenticated)**

   ```bash
   curl -sS -H "Authorization: Bearer <your-gateway-token>" http://localhost:3000/v1/models
   ```

   The first model **`id`** should be **`Claudia-0.1.0`** (or your configured semver).

## LiteLLM documentation

Configure the proxy (providers, keys, advanced options) using the upstream docs:

- [LiteLLM documentation hub](https://docs.litellm.ai/docs/)
- [LiteLLM Proxy — deploy / Docker](https://docs.litellm.ai/docs/proxy/deploy)

## Local gateway + Docker LiteLLM (optional)

For fast gateway development, run LiteLLM in Compose and the gateway on the host with `npm run dev`, set `litellm.base_url` in a local `gateway.yaml` to `http://localhost:4000`, and export **`LITELLM_MASTER_KEY`** to match `.env`.
