# Plan: Claudia desktop UI (webview) + gateway admin surface

This document plans a **cross-platform desktop shell** that wraps a **system webview** and loads **operator UI served by the Claudia gateway**. The goal is **one** web-based control experience shared with the browser (and eventually a PWA). **Version 0.1 removes** the legacy **Fyne** desktop app ([`gui/`](../gui/)) entirely; the webview shell does not use Fyne and is the only desktop UI.

**Implementation direction:** work toward **one primary executable** (`claudia` / `claudia.exe`) that the user launches in **desktop mode**: it starts the **supervised stack** (optional Qdrant, BiFrost, and the **HTTP gateway** in-process, per [`supervisor.md`](supervisor.md)), then opens the **webview** against the gateway’s `/ui/…` entry. The same binary also supports **headless** operation (no webview) for servers, automation, and future **platform installers** that install a bundle without a desktop shell.

**Related docs:** [`cli-tool-plan.md`](cli-tool-plan.md) (operator CLI, shared BiFrost assumptions), [`supervisor.md`](supervisor.md), [`bifrost-discovery.md`](bifrost-discovery.md), [`configuration.md`](configuration.md), [`vscode-continue/`](../vscode-continue/) (Continue examples).

---

## Versioning

### Version 0.1 — Webview wrapper + gateway admin UI

**Desktop shell (webview)**

- Embeds a **native webview** (platform WebView2 / WKWebView / WebKitGTK, or a small helper such as Wails/Tauri if the team standardizes on one). **No Fyne** — CGO is only required if the chosen webview stack requires it (unlike the old Fyne GUI).
- **Build entry (target):** integrate the webview into **[`cmd/claudia`](../cmd/claudia)** so **one binary** runs **desktop mode** (supervisor + gateway + window). A **temporary** separate package (e.g. [`cmd/claudia-gui`](../cmd/claudia-gui)) is acceptable only if it accelerates early integration; the **deliverable** to optimize for is **single `claudia`**. Makefile / script names such as **`make gui-build`** may continue to produce a **`claudia-gui`** artifact during transition, or may be retargeted to the desktop-capable `claudia` build once merged — document whichever layout the repo uses after the cutover.
- **Remove in v0.1:** delete the Fyne [`gui/`](../gui/) module; retarget [`scripts/gui-build.sh`](../scripts/gui-build.sh), [`scripts/gui-install.sh`](../scripts/gui-install.sh), and [`scripts/gui-run.sh`](../scripts/gui-run.sh) at the **webview-capable** build; update [`Makefile`](../Makefile) **`vet-gui`** / **`test-gui`** / **`fmt`** paths and drop **`CGO_ENABLED=1`** unless the webview stack needs CGO; update [`scripts/clean.sh`](../scripts/clean.sh), [`scripts/print-make-help.sh`](../scripts/print-make-help.sh), [`docs/gui-testing.md`](../docs/gui-testing.md), README, and CI (e.g. `.github/workflows`) so they describe webview deps, not Fyne.
- **Default navigation target:** Claudia gateway **operator entry** served by `claudia` (e.g. `http://127.0.0.1:3000/ui/` or a concrete path agreed in implementation — **must** be a page shipped **from the gateway**, not only bundled inside the wrapper).
- **Static assets bundled with the wrapper** (not the gateway): a **gateway unreachable** page (HTML/CSS) shown when the wrapper cannot connect to the configured base URL (connection refused, timeout, DNS failure). No token or secrets on that page.
- **v0.1 default base URL:** `http://127.0.0.1:3000` (hard-coded or single compile-time default; configurable persistence is **v0.8**).

**Gateway-served UI (same origin as gateway)**

1. **Default / landing** — First paint from Claudia: welcome or redirect into the login flow.
2. **Login** — User enters the **gateway token** (same class of secret as `Authorization: Bearer` on `/v1/*`, or a dedicated **admin** token if split in implementation; v0.1 must document which). Submission **authenticates** the session for admin UI routes only.
3. **Authentication model (v0.1)** — After successful login, use a **session the browser/webview can reuse** without putting the token in the URL:
   - Preferred: **`POST /api/ui/login`** (name illustrative) validates token against the existing token store ([`config/tokens.yaml`](../config/tokens.yaml) / gateway auth), then responds with **`Set-Cookie`**: **httpOnly**, **SameSite=Lax**, path scoped to `/ui` and `/api/ui` (or equivalent).
   - Subsequent `fetch()` from the control panel sends the cookie automatically inside the webview.
   - **401** on any admin call → return to login; clear stale session.
4. **Control panel** — Single page (or small multi-step) that:
   - **Displays current values** for BiFrost **Groq**, **Gemini**, and **Ollama** (as surfaced by the gateway from BiFrost’s management API — key metadata **masked**, Ollama base URL as plain text).
   - **Edits per row** — One row (or card) per concern: Groq API key, Gemini API key, Ollama base URL. User saves **one row at a time** (explicit Save per row); avoids losing half-completed multi-field forms.
   - **Inline errors** — Each row shows validation/API errors **next to that row** (HTTP 4xx/5xx from BiFrost or gateway BFF mapped to readable text). No silent failure.
5. **VS Code Continue snippet** — On the control panel (or a dedicated subsection), show a **copy-ready** configuration block: gateway **base URL**, **Bearer token** placeholder or instructions to paste the user’s token, and **model id** guidance aligned with [`vscode-continue/`](../vscode-continue/) (e.g. virtual `Claudia-<semver>` from [`config/gateway.yaml`](../config/gateway.yaml)). User copies into Continue `config.json` / YAML.

**BiFrost prerequisite (v0.1)**

- [`config/bifrost.config.json`](../config/bifrost.config.json) **must** ship with **`config_store` enabled** so management APIs persist and return consistent state for the control panel. Align with [`cli-tool-plan.md`](cli-tool-plan.md) § BiFrost API + config store.

**Gateway backend essentials (v0.1)** — implied by the UI above; all in scope for 0.1:

- **BFF (server-side)** from gateway to BiFrost management HTTP API (`/api/providers/...` per pinned `deps.lock` / OpenAPI). Browser **never** calls BiFrost directly (avoids CORS, hides BiFrost admin auth if enabled later).
- **Read path:** aggregate **Groq keys**, **Gemini keys**, **Ollama** URL (or key config) for display (masked secrets).
- **Write path:** update or create keys / Ollama URL per row; map errors to JSON the UI can show inline.
- **Session/login** and **authorization middleware** for `/ui/*` HTML and `/api/ui/*` JSON (cookie session tied to validated gateway token).

**Out of scope for v0.1** (see **Version 0.8**):

- User-configurable gateway URL persisted in the wrapper (beyond default).
- PWA manifest / service worker.
- Multi-user RBAC, audit log UI, non-localhost hardening beyond documenting HTTPS for remote deploy.

### Version 0.8

Everything **not** required to satisfy v0.1 above, including but not limited to:

- Saved **gateway base URL** (and optional port) in wrapper config; optional **profiles** (dev/prod).
- **Deep links**, **offline** PWA behavior, **installable** manifest served from gateway.
- Richer **observability** UI (logs, metrics), **BiFrost dashboard** parity, additional providers beyond Groq/Gemini/Ollama.
- **Unified** styling system, **i18n**, accessibility audit beyond baseline.
- **Automated** E2E tests for webview + gateway (CI matrix).

---

## Desktop launcher, bundled release, and lifecycle

**Single executable, not a single file for the whole product**

- The **user-facing launcher** is **one** `claudia` binary that, in **desktop mode**, starts everything that belongs in-process and via the existing **supervisor** (optional **Qdrant** and **BiFrost** subprocesses, plus the **Go HTTP gateway**).
- A **release** for end users is still a **bundle**: that executable **plus** the other programs the supervisor runs (**`bifrost-http`**, optional **Qdrant** binary), **configuration** (`config/gateway.yaml`, tokens, `bifrost.config.json`, etc.), and **data directories** as documented in installation / [`supervisor.md`](supervisor.md). Installers (future) ship this layout; nothing requires stuffing BiFrost or Qdrant *into* the same PE/ELF file.

**Headless vs desktop (same binary)**

- **Headless:** e.g. `claudia serve` with flags as today (or an explicit `--headless` / build tag that omits webview linkage for smaller CI and server artifacts). No window; shutdown is driven by **OS signals** only (unless extended later).
- **Desktop:** same binary opens the webview after (or while) the gateway is listening; default URL remains **`http://127.0.0.1:3000`** (or the resolved listen address) for `/ui/…`.

**Unified shutdown**

- Implement **one** internal shutdown path (e.g. cancel a **root context** and/or a dedicated `shutdown()` used everywhere). **Both** of the following must invoke it:
  - **OS signals** (**SIGINT**, **SIGTERM**) — same semantics as today’s `claudia serve`.
  - **Webview window close** (`OnClose` or the framework’s equivalent).
- **Order (conceptual):** graceful **HTTP server shutdown**, then **cancel supervisor child context** so **Qdrant** / **BiFrost** processes stop. Avoid duplicating teardown logic between signal handlers and UI callbacks.

**Backend failures vs. the shell**

- If **Qdrant** or **BiFrost** exits or never becomes healthy, the **desktop process** (and webview) **should keep running** so the operator can see **degraded state** (gateway **`GET /status`**, failure page, or in-app messaging). **User-driven quit** (close window or signal) still tears down the whole operation. Exact restart policy (auto-restart children vs. report-only) is implementation detail; v0.1 should at minimum **surface** failures without killing the window immediately.

---

## Architecture (v0.1)

**Logical components** (desktop mode): one OS process hosts the **webview** and the **gateway**; **BiFrost** and optional **Qdrant** remain **child processes** started by the supervisor.

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│  Process: claudia (desktop mode)                                             │
│  ┌─────────────────────┐     HTTP (cookie session)      ┌───────────────────┐  │
│  │  Webview            │ ────────────────────────────► │  Gateway (in-proc)│  │
│  │  (bundled failure   │   GET /ui/…, POST /api/ui/…   │  HTML/JS + BFF    │  │
│  │   page only local)  │                               │  token store      │  │
│  └─────────────────────┘                               └─────────┬─────────┘  │
└──────────────────────────────────────────────────────────────────┼───────────┘
                                                                   │ exec / supervise
                                          ┌────────────────────────┴────────────────────────┐
                                          ▼                                                 ▼
                                 ┌─────────────────┐                               ┌─────────────────┐
                                 │  Qdrant (child) │                               │  BiFrost (child)│
                                 │  optional       │                               │  config_store   │
                                 └─────────────────┘                               └─────────────────┘
```

**Headless** omits the webview box; the gateway + supervisor layout matches [`supervisor.md`](supervisor.md).

---

## Control panel UX (v0.1) — row model

| Row | Display | Action | Error surface |
|-----|---------|--------|----------------|
| **Groq** | Masked key fingerprint or “not set”; optional last-updated | Input + **Save** | Inline under row |
| **Gemini** | Same | Input + **Save** | Inline under row |
| **Ollama** | Current `base_url` | Input + **Save** | Inline under row |

Optional **Refresh** control to re-fetch state from BiFrost without full page reload.

---

## VS Code Continue (v0.1)

- **Static template** in the gateway UI with placeholders: `{gateway_url}`, `{token_hint}`, `{virtual_model_id}` filled from server-known values (semver/virtual model from runtime config).
- Link to repo [`vscode-continue/`](../vscode-continue/) for full examples.
- Warn: token in Continue config is **user-local**; do not commit.

---

## Security notes (v0.1)

- **No token in query strings** for navigation.
- **httpOnly** session cookie for admin UI; **CSRF** consideration for state-changing `POST`: use **SameSite**, or anti-CSRF token in form body / header for v0.1 if using cookie session.
- **localhost-only** by default; document that remote access requires **HTTPS** and tighter binding (`listen_host` in [`gateway.yaml`](../config/gateway.yaml)).

---

## Implementation checklist (v0.1)

**Gateway**

- [ ] `config_store` in [`config/bifrost.config.json`](../config/bifrost.config.json) + docs update.
- [ ] Admin session: login endpoint + cookie + auth middleware for `/ui` and `/api/ui`.
- [ ] BFF: read/write Groq, Gemini, Ollama via BiFrost management API (pinned OpenAPI).
- [ ] Serve **default** operator HTML/JS (embed or `html/template` / static files under e.g. `internal/server/ui/`).
- [ ] Control panel: per-row save + inline errors + masked display.
- [ ] Continue configuration snippet block.

**Desktop shell + launcher**

- [ ] **Single `claudia` binary:** **desktop mode** starts supervisor + gateway and opens **webview** with default base URL `http://127.0.0.1:3000` (or resolved listen address).
- [ ] **Headless mode** (e.g. `serve` / `--headless` / build tag) shares the same process model without linking or starting the webview.
- [ ] **Unified shutdown:** **SIGINT** / **SIGTERM** and **window close** call the **same** teardown path (HTTP shutdown, then supervised children stopped).
- [ ] On load failure → bundled **static failure** page; retry / quit.
- [ ] On success → load gateway `/ui/...` entry.

**Repo hygiene**

- [ ] **Remove Fyne [`gui/`](../gui/)** and repurpose existing targets: **`make gui-install`**, **`make gui-build`**, **`make gui-run`** align with the **webview + `claudia`** layout (artifact names documented after cutover).
- [ ] README: **bundled desktop release** (launcher + companion binaries + config + data dirs) vs **headless** `claudia serve` for servers and installers.

---

## Relationship to `claudiactl`

- **CLI** and **web UI** should call the **same** gateway BFF where practical so behavior and validation stay aligned ([`cli-tool-plan.md`](cli-tool-plan.md)).
- v0.1 may implement **either** CLI-first or **UI-first** BFF; the second consumer should reuse the same internal package (e.g. `internal/bifrostadmin`).

---

*Plan status: **draft for implementation** — v0.1 webview + gateway admin UI, single desktop launcher with headless variant and unified shutdown; v0.8 for persistence, PWA, and extended features.*
