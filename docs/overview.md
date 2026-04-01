# High-level overview

**Claudia Gateway** is a small TypeScript service that sits in front of **LiteLLM**. IDEs and agents (for example **VS Code Continue**) use a **single OpenAI-compatible base URL** and one **virtual model id** (`Claudia-<semver>`, e.g. `Claudia-0.1.0`) instead of switching models manually in the UI. The gateway validates **gateway-issued API tokens**, optionally applies **routing rules** to pick an initial backend model, then walks a **configured fallback chain** on **429** or **5xx** from LiteLLM.

**LiteLLM** is the official multi-provider proxy: it holds **provider API keys**, talks to Groq, OpenAI, Gemini, Ollama, vLLM, and other backends, and exposes standard OpenAI-style **`/v1/chat/completions`** and **`/v1/models`**. Claudia calls LiteLLM **only over HTTP**; it does not embed the LiteLLM SDK.

**Qdrant** is included in the default **v0.1** Compose stack for **v0.2 RAG readiness**. In **v0.1** the gateway does **not** read or write Qdrant; health checks probe **LiteLLM only**. When RAG is enabled in a future version, the gateway will use Qdrant for retrieval and will extend **`GET /health`** accordingly.

**v0.1** delivers: virtual model + fallback chain, YAML tokens and routing policy with **mtime reload**, **`GET /health`**, structured logging, and operator documentation. **v0.2** adds ingest, indexer APIs, and query-time RAG—see the plan roadmap.
