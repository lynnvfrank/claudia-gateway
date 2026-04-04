# BiFrost (+ optional Qdrant) subprocesses + Claudia (`claudia serve`)

Phase 3 of [go-bifrost-migration-plan.md](go-bifrost-migration-plan.md): one command can start **Qdrant** (optional), **BiFrost**, and the **Go Claudia** HTTP server in the **same** parent process. **SIGINT** / **SIGTERM** triggers graceful HTTP **shutdown**, then **all** supervised children are **stopped** (reverse order is not guaranteed; context cancel ends them together).

## Runtime layout

| Piece | Role |
|-------|------|
| **Parent** | Go `claudia serve` — HTTP gateway (`config/gateway.yaml`, tokens, routing). |
| **Child (optional)** | **Qdrant** native binary — **`QDRANT__STORAGE__STORAGE_PATH`**, **`QDRANT__SERVICE__HOST`**, **`QDRANT__SERVICE__HTTP_PORT`**, **`QDRANT__SERVICE__GRPC_PORT`** (defaults align with Compose **6333** / **6334**). Readiness: **`GET /readyz`**. Omit by leaving **`-qdrant-bin`** empty. |
| **Child** | BiFrost HTTP binary (`bifrost-http`) — started with **`-app-dir`**, **`-host`**, **`-port`**, **`-log-level`**, **`-log-style`** (same as the Docker entrypoint). **`APP_HOST`** / **`APP_PORT`** are also set for compatibility. Working directory = **data dir**. |
| **Config copy** | Your `bifrost.config.json` is copied to **`<bifrost-data-dir>/config.json`** on each start (same idea as mounting `config/bifrost.config.json` in Docker). |

Claudia’s upstream URL is **overridden** to **`http://<upstream-host>:<bifrost-port>`** (default **`http://127.0.0.1:8080`**) so `gateway.yaml` may still say `http://bifrost:8080` for Compose while local supervise uses loopback.

The gateway exposes **`GET /status`** (JSON, no auth — same sensitivity as **`/health`**) with **`supervisor.active: true`**, BiFrost/Qdrant listen hints, and upstream probe results. The Fyne **`claudia-gui`** polls this endpoint; see [gui-testing.md](gui-testing.md).

## Obtaining the BiFrost binary

The repository does **not** vendor BiFrost. Install per [BiFrost documentation](https://docs.getbifrost.ai/) (release binary, package, or build from [source](https://github.com/maximhq/bifrost)). You need the **HTTP server** artifact (**`bifrost-http`** from a source build’s **`tmp/`**), not only the CLI **`bifrost`**.

### Build from a local clone (shortcut)

If you keep BiFrost at **`$HOME/src/bifrost`** (or set **`BIFROST_SRC`**):

```bash
make bifrost-from-src    # runs `make build` in BIFROST_SRC; copies tmp/bifrost-http → ./bin/bifrost-http
make claudia-serve-local # claudia serve -bifrost-bin ./bin/bifrost-http
```

Upstream **`make build`** includes the UI (**`build-ui`**) and needs **Node.js 20+** and a matching **npm** (not only Go). **`make bifrost-from-src`** checks this before calling BiFrost’s **`make build`**. Override the tree with **`make bifrost-from-src BIFROST_SRC=/path/to/bifrost`**.

Otherwise put **`bifrost-http`** (or a compatible binary) on **`PATH`** as **`bifrost`**, or pass **`-bifrost-bin /full/path`**.

### **`fork/exec ./bin/bifrost-http: no such file or directory`** (binary exists)

The kernel resolves a **relative** **`-bifrost-bin`** path against the **process current working directory**, not the repo root. If **`claudia serve`** starts with a different cwd (some IDE tasks, **`go run`** from another directory), **`./bin/bifrost-http`** misses. Claudia resolves **`./…`** and **`bin/…`** to an **absolute** path before exec; use **`-bifrost-bin /home/you/src/claudia-gateway/bin/bifrost-http`** if you still see issues, or run from the repo root.

### Troubleshooting **`npm ci`** / **`Cannot read property '@base-ui/react' of undefined`**

That error usually means **`npm`** is too old (e.g. **npm 6** with **Node 10**). On Ubuntu, **snap**’s **`node`** package is often **v10**; BiFrost’s UI expects a current **Node** (see BiFrost **`ui/package.json`** / Next 15). Fix by installing **Node 20+** (nvm, fnm, [nodejs.org](https://nodejs.org/), or your distro’s **`nodejs`** package) and ensuring **`which node`** points at it **before** snap’s **`/snap/bin/node`**. Then run **`make bifrost-from-src`** again.

Provider keys (**`GROQ_API_KEY`**, **`GEMINI_API_KEY`**, etc.) are read from the **environment** of the `claudia serve` process and inherited by the BiFrost child (same as Docker `environment:`). Qdrant inherits the same environment (optional **`QDRANT__*`** overrides).

## Qdrant binary

The gateway does **not** call Qdrant in **v0.1**; supervision is for **v0.2+ RAG** and local parity with Compose.

- **Pinned version:** **`scripts/qdrant-pinned-version.txt`** (used by fetch scripts and GoReleaser).
- **Local install:** **`make qdrant-from-release`** → **`./bin/qdrant`** (Linux/macOS via **`scripts/fetch-qdrant-local.sh`**). On Windows, download the matching **`.zip`** from [Qdrant releases](https://github.com/qdrant/qdrant/releases) and pass **`-qdrant-bin`** to that **`qdrant.exe`**.
- **Full local stack (Qdrant + BiFrost + gateway):** after **`make qdrant-from-release`** and **`make bifrost-from-src`**, run **`make claudia-serve-stack`**.

## Usage

From the repo root (with `config/gateway.yaml`, `config/tokens.yaml`, `config/bifrost.config.json`):

```bash
export CLAUDIA_UPSTREAM_API_KEY=bifrost-local-dummy
export GROQ_API_KEY=...   # as needed
go run ./cmd/claudia serve
# or: ./claudia serve
```

Common flags:

| Flag | Default | Meaning |
|------|---------|---------|
| **`-bifrost-bin`** | `bifrost` | **`bifrost-http`** (or name on PATH); use **`./bin/bifrost-http`** after **`make bifrost-from-src`** |
| **`-bifrost-config`** | `config/bifrost.config.json` | Source JSON copied into data dir |
| **`-bifrost-data-dir`** | `data/bifrost` | Writable BiFrost state directory |
| **`-bifrost-bind`** | `127.0.0.1` | **`-host`** (and **`APP_HOST`**) |
| **`-bifrost-port`** | `8080` | **`-port`** (and **`APP_PORT`**) |
| **`-bifrost-log-level`** | `info` | **`-log-level`** |
| **`-bifrost-log-style`** | `json` | **`-log-style`** (`json` or `pretty`) |
| **`-upstream-host`** | `127.0.0.1` | Host segment for Claudia → BiFrost URL (use when BiFrost binds `0.0.0.0`) |
| **`-wait-bifrost`** | `60s` | Max time to poll **`/health`** before exiting |
| **`-no-wait-bifrost`** | off | Skip readiness poll (debug only) |
| **`-qdrant-bin`** | *(empty)* | Qdrant executable; set e.g. **`./bin/qdrant`** to supervise Qdrant |
| **`-qdrant-storage`** | `data/qdrant` | On-disk vector storage (created) |
| **`-qdrant-bind`** | `127.0.0.1` | **`QDRANT__SERVICE__HOST`** |
| **`-qdrant-http-port`** | `6333` | HTTP API port |
| **`-qdrant-grpc-port`** | `6334` | gRPC port |
| **`-qdrant-health-host`** | `127.0.0.1` | Host for **`/readyz`** probe when **`qdrant-bind`** is **`0.0.0.0`** |
| **`-wait-qdrant`** | `60s` | Max time to poll **`/readyz`** |
| **`-no-wait-qdrant`** | off | Skip Qdrant readiness poll |

Gateway flags **`‑config`** and **`‑listen`** apply as in gateway-only mode. See **`claudia serve -h`**.

## Make / npm

- **`make claudia-serve`** → `go run ./cmd/claudia serve`
- **`make bifrost-from-src`** → build BiFrost in **`BIFROST_SRC`** and install **`./bin/bifrost-http`**
- **`make claudia-serve-local`** → serve with **`-bifrost-bin ./bin/bifrost-http`**
- **`make qdrant-from-release`** → **`./bin/qdrant`**
- **`make claudia-serve-stack`** → Qdrant + **`./bin/bifrost-http`**
- **`npm run go:serve`**, **`npm run go:serve:local`**, **`npm run bifrost:from-src`**, **`npm run qdrant:from-release`**, **`npm run go:serve:stack`**

## Manual checklist (Linux)

1. Build or install **`bifrost-http`** (e.g. **`make bifrost-from-src`** → **`./bin/bifrost-http`**) or use **`-bifrost-bin`**.
2. Run **`claudia serve`**; confirm **`GET http://127.0.0.1:3000/health`** (or your listen port) returns **`ok`** when BiFrost is up.
3. Send **SIGINT** to the parent; confirm child processes exit (no orphan **`bifrost`** / **`qdrant`** in **`ps`**).

## CI

End-to-end tests with real BiFrost/Qdrant binaries are **optional** (network, secrets). Unit tests cover config copy, env merge, **`WaitHealthy`**, and context-cancel kills **`sleep`** children on Unix.
