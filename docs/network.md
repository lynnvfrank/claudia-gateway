# Network architecture

## Logical flow

- **IDE / Continue** → **Claudia Gateway** (`POST /v1/chat/completions`, `GET /v1/models`) with `Authorization: Bearer <gateway token>`.
- **Claudia Gateway** → **BiFrost** (default) or **LiteLLM** (`/v1/chat/completions`, `/v1/models`) with `Authorization: Bearer <CLAUDIA_UPSTREAM_API_KEY>` (BiFrost often accepts a placeholder unless governance keys are enabled).
- **BiFrost** → providers using **`GROQ_API_KEY`**, **`GEMINI_API_KEY`**, etc. per `config/bifrost.config.json`. **LiteLLM** → providers using `config/litellm_config.yaml` and env keys.
- **v0.2+**: **Claudia** → **Qdrant** for retrieval and indexer-backed workflows. **v0.1**: Qdrant runs idle unless you use it manually.

## Docker Compose topology

All services attach to the user-defined bridge network **`claudianet`** (see `docker-compose.yml`).

| Service (DNS name) | Internal ports | Typical host publish | Role |
|--------------------|----------------|----------------------|------|
| **claudia** | `3000` | `3000` | Client-facing gateway |
| **bifrost** | `8080` | `8080` (optional) | AI gateway (default upstream) |
| **litellm** | `4000` | `4000` (optional) | LiteLLM proxy (optional) |
| **qdrant** | `6333`, `6334` | `6333`, `6334` (optional) | Vectors (v0.2+) |

**Inside the stack**, use Compose DNS hostnames and **internal** ports—for example `http://bifrost:8080` (default `config/gateway.yaml`) or `http://litellm:4000`, never `localhost`, from the `claudia` container.

**On the host**, use `http://localhost:3000` for Continue’s `apiBase` (plus `/v1` path as required by your client).

## Trust boundary (v0.1)

Traffic is **plain HTTP** by default on the loopback or trusted LAN. Hardening and TLS are **out of scope for v0.1**; see the plan (**v0.7** security).
