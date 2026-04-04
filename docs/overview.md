# High-level overview

**Claudia Gateway** is a small **Go** service in front of **BiFrost** (or any OpenAI-compatible upstream). IDEs and agents (for example **VS Code Continue**) use a **single OpenAI-compatible base URL** and one **virtual model id** (`Claudia-<semver>`, e.g. `Claudia-0.1.0`) instead of switching models manually in the UI. The gateway validates **gateway-issued API tokens**, optionally applies **routing rules** to pick an initial backend model, then walks a **configured fallback chain** on **429** or **5xx** from the upstream.

**BiFrost** holds **provider API keys** and talks to Groq, Gemini, and other backends per **`config/bifrost.config.json`**. Claudia calls the upstream **only over HTTP**.

**Qdrant** can be supervised alongside BiFrost via **`claudia serve`** for **v0.2+ RAG**. In **v0.1** the gateway does **not** read or write Qdrant.

**v0.1** delivers: virtual model + fallback chain, YAML tokens and routing policy with **mtime reload**, **`GET /health`**, structured logging, and operator documentation. **v0.2** adds ingest, indexer APIs, and query-time RAG — see the plan roadmap.
