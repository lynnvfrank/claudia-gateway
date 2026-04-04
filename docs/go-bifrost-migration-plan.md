# Claudia Gateway — Go rewrite, BiFrost, packaging, and GUI

This document is a **phased migration plan**. Each phase has a **deliverable**, **verification (tests)**, and **TODO** items. Work proceeds **one phase at a time**: the **user asks the agent to implement the next phase**. When an agent finishes a phase, it **updates this file** (checkboxes, completion notes, and links to PRs/commits as appropriate).

**Product goals (end state)**

- **Claudia Gateway** implemented in **Go**, with BiFrost as the component that **manages API keys and provider connections** (replacing LiteLLM in the reference architecture).
- **One distributable** operators can run on **macOS, Windows, and Linux** that bundles or supervises **both** BiFrost and Claudia (exact layout decided in packaging phases).
- A **GUI** that displays the message: **`mew mew, Love Claudia`** (minimum viable UI; may grow into settings/service management later).

**Non-goals for this document**

- It does not replace [claudia-gateway.plan.md](claudia-gateway.plan.md) for normative product requirements; it **implements** a technical path toward v0.1+ goals described in [version-v0.1.md](version-v0.1.md).

---

## How agents should update this plan

After completing work for a phase:

1. Mark that phase’s **TODO** checkboxes as done (`[x]`).
2. Under **Phase completion log**, add a dated entry: phase name, summary, PR or commit SHA, and any **follow-ups** or **deferred items**.
3. If scope changed, edit the phase text in place and note **why** in the log.

---

## Phase completion log

| Date | Phase | Summary | Reference |
|------|--------|---------|-----------|
| — | — | *No entries yet.* | — |

---

## Phase 0 — Discovery: BiFrost with the **existing** TypeScript gateway

**Intent.** De-risk BiFrost **before** committing to Go. The agent and user share a **discovery** pass: install BiFrost, configure keys and models in BiFrost, point the **current** Node/Fastify gateway at BiFrost’s OpenAI-compatible base URL, and record gaps.

**Deliverable**

- **`docs/bifrost-discovery.md`** (or equivalent) containing:
  - Exact BiFrost version(s) tried and install method(s) (e.g. Docker image tag, released binary, `npx`).
  - Minimal **BiFrost configuration** needed for at least one real completion (chat) and **`GET /v1/models`**.
  - **`config/gateway.yaml`** (or env) settings required so Claudia uses BiFrost instead of LiteLLM (`litellm.base_url` → BiFrost URL until naming is generalized).
  - **Compatibility matrix**: streaming (SSE) vs non-streaming, auth header shape, error codes, timeouts, anything that differs from LiteLLM behavior.
  - **Go migration implications**: what must be abstracted in a future Go gateway (endpoints, headers, fallback triggers).
- Optional but valuable: a **Compose override** or **documented commands** to run BiFrost + Claudia together for local reproduction (without removing LiteLLM docs until a later phase).

**Tests / acceptance criteria**

- [ ] **Manual smoke**: With BiFrost up and keys configured in BiFrost (not duplicated in Claudia for provider secrets), **`curl` or equivalent** succeeds against Claudia for **non-streaming** chat using the virtual model path that exercises routing/fallback **or** a documented limitation if parity is impossible in TS without code changes.
- [ ] **Manual smoke**: **Streaming** completion works through Claudia → BiFrost **or** discovery doc states the gap and reproduction steps.
- [ ] **`GET /v1/models`** through Claudia returns expected model list including virtual model behavior **or** gap is documented with workaround.
- [ ] **`GET /health`** on Claudia reflects upstream reachability in a way that matches current semantics **or** documented delta.
- [ ] Discovery doc includes a **short “definition of done” checklist** the next phase can rely on.

**TODO (Phase 0)**

- [ ] Install and run BiFrost per official docs; record version and command lines.
- [ ] Configure at least one provider **only in BiFrost**; confirm completions work **directly** against BiFrost.
- [ ] Point existing gateway config at BiFrost; fix or document any gateway-side changes needed (minimal diff to TS allowed if required for discovery).
- [ ] Write **`docs/bifrost-discovery.md`** with the sections above.
- [ ] Execute and record results of the **acceptance** checks (pass/fail/skip with reason).

**Status:** ☐ Not started · ☐ In progress · ☐ Complete

---

## Phase 1 — Go gateway: project scaffold and HTTP parity spike

**Intent.** Create a **new Go module** (suggested layout: `cmd/claudia/`, `internal/...`) that can serve **`GET /health`** and proxy **`POST /v1/chat/completions`** and **`GET /v1/models`** to a configurable upstream (BiFrost URL), without yet full feature parity with TypeScript.

**Deliverable**

- Runnable **`claudia`** (or `claudia-gateway`) binary from `go build` with configuration via flags and/or a small config file.
- Documented **mapping** from current YAML/env concepts to Go config (can be subset in this phase).

**Tests / acceptance criteria**

- [ ] **`go test ./...`** passes in CI (add Go workflow if missing).
- [ ] **Integration or handler tests** (e.g. `httptest` + fake upstream) verify: request forwarding, required headers, streaming pass-through behavior at the HTTP level for a **minimal** SSE fixture.
- [ ] **Manual or scripted smoke**: binary against a fake upstream confirms listen address and timeout behavior.

**TODO (Phase 1)**

- [ ] Add Go module and `README` section for building the Go binary.
- [ ] Implement minimal reverse proxy or typed client for chat + models + health upstream probe.
- [ ] Add CI job running **`go test ./...`** (and `go vet` / formatting as project standard).

**Status:** ☐ Not started · ☐ In progress · ☐ Complete

---

## Phase 2 — Go gateway: feature parity with v0.1 Claudia (BiFrost upstream)

**Intent.** Port **virtual model**, **token auth**, **routing policy**, **fallback chain on 429/5xx**, **config reload** (or equivalent), and **logging** to match [version-v0.1.md](version-v0.1.md) behavior as closely as BiFrost allows.

**Deliverable**

- Go gateway passes a **parity checklist** derived from TypeScript behavior and `docs/bifrost-discovery.md`.
- Migration note: **deprecation timeline** for the TypeScript server (keep, dual-ship, or remove).

**Tests / acceptance criteria**

- [ ] **Unit tests** for routing policy evaluation and fallback ordering (fixtures from current YAML samples).
- [ ] **Integration tests** with **mock upstream** returning 429/5xx to assert fallback chain walk.
- [ ] **Golden or snapshot tests** for virtual model id and models list ordering where stable.
- [ ] **Optional**: black-box test script in `scripts/` that runs Go binary against BiFrost in CI (may be `workflow_dispatch` only if secrets/network heavy).

**TODO (Phase 2)**

- [ ] Port token loading and mtime reload (or chosen alternative).
- [ ] Port routing policy and virtual model + fallback logic.
- [ ] Align **all** public routes required for v0.1 Continue/client compatibility.
- [ ] Update operator docs to describe **BiFrost + Go Claudia** as the reference path.

**Status:** ☐ Not started · ☐ In progress · ☐ Complete

---

## Phase 3 — Process supervision: one command runs BiFrost + Claudia

**Intent.** A single **entry binary** (could be the same `claudia` with a `serve` subcommand) that **starts BiFrost** and **Go Claudia**, sets **inter-process URLs** (e.g. localhost ports), handles **signals** for graceful shutdown, and optionally **waits for upstream readiness**.

**Deliverable**

- Documented **runtime architecture** (two processes, ports, config dirs, data dirs for BiFrost).
- Supervisor behavior verified on **Linux** at minimum; macOS/Windows noted or tested in Phase 4.

**Tests / acceptance criteria**

- [ ] **Unit tests** for supervisor logic where testable without spawning real BiFrost (mock commands or interface injection).
- [ ] **Integration test** (optional in CI): job that downloads or uses a **pinned BiFrost binary** / Docker to verify end-to-end **one command** startup (may be nightly or manual job—document which).
- [ ] Manual checklist: SIGINT stops both children without zombie processes.

**TODO (Phase 3)**

- [ ] Implement subprocess management (start order, env, working directory).
- [ ] Embed or document **how BiFrost binary is obtained** (bundled path, `PATH`, or download helper—align with licensing).
- [ ] Expose flags for ports and config paths.

**Status:** ☐ Not started · ☐ In progress · ☐ Complete

---

## Phase 4 — Cross-platform packaging (macOS, Windows, Linux)

**Intent.** Produce **releasable artifacts** per OS/arch (e.g. `.tar.gz`, `.zip`, macOS bundle or signed `.app` TBD, Windows `.exe` installer or zip TBD) that include **everything needed to run** Phase 3’s combined stack without a compiler on the target machine.

**Deliverable**

- **Release checklist** and automation (e.g. GoReleaser or custom scripts) producing artifacts attached to a tag.
- **Signing / notarization** called out as **follow-up** if not implemented (especially macOS).

**Tests / acceptance criteria**

- [ ] **CI** builds artifacts for **linux-amd64** at minimum; **darwin** and **windows** targets configured (may run on tag only).
- [ ] **Smoke test** job: unpack artifact, run `--version` or `--help`, and optionally start with **mock** upstream (fast path).
- [ ] **Documentation**: install steps per OS, antivirus/first-run notes for Windows if relevant.

**TODO (Phase 4)**

- [ ] Select tooling (GoReleaser, etc.) and pin BiFrost versions per release.
- [ ] Define artifact layout (binary names, `LICENSE` files, third-party notices).
- [ ] Add `docs/packaging.md` for operators.

**Status:** ☐ Not started · ☐ In progress · ☐ Complete

---

## Phase 5 — GUI: “mew mew, Love Claudia”

**Intent.** Ship a **graphical** entry (desktop shell) that displays **`mew mew, Love Claudia`**. The GUI may be **Wails**, **Fyne**, or another agreed stack; it should **launch or attach** to the supervised stack from Phase 3–4 where feasible, or clearly document “GUI-only demo” until wired.

**Deliverable**

- GUI application included in **packaged releases** for **macOS, Windows, Linux** (or documented exceptions).
- On first launch, user sees **exactly** the required message (additional UI optional).

**Tests / acceptance criteria**

- [ ] **Automated UI test** where feasible (e.g. Wails/Fyne test hooks or screenshot/E2E in CI for one platform); if not feasible, **manual test script** with signed checklist in `docs/gui-testing.md`.
- [ ] **Build verification**: CI builds GUI flavor for at least **linux-amd64** (headless-friendly checks as appropriate).

**TODO (Phase 5)**

- [ ] Choose GUI framework; spike hello-world in repo.
- [ ] Implement required message string and window sizing/accessibility basics.
- [ ] Integrate with supervisor **or** document phased wiring; update packaging.

**Status:** ☐ Not started · ☐ In progress · ☐ Complete

---

## Phase 6 — Hardening, operator UX, and TypeScript gateway sunset

**Intent.** Security review pass, logging/redaction, upgrade path between versions, **migration guide** from Docker+LiteLLM stack, and final decision on **removing or archiving** the TypeScript implementation.

**Deliverable**

- **SECURITY.md** or equivalent notes (token handling, local attack surface).
- **Migration guide** for existing operators.
- Explicit **sunset** statement for TS gateway in README/plan.

**Tests / acceptance criteria**

- [ ] **Fuzz or static analysis** (optional): `go test -fuzz` on HTTP parsers or config loaders where valuable.
- [ ] **End-to-end script** (documented): fresh machine path from download to first successful chat via GUI or CLI.
- [ ] **Regression suite** from Phase 2 still green.

**TODO (Phase 6)**

- [ ] Audit secrets in logs and config reload paths.
- [ ] Finalize docs and archive TS server if applicable.
- [ ] Tag **v0.x** release candidate.

**Status:** ☐ Not started · ☐ In progress · ☐ Complete

---

## Quick reference

| Topic | Primary location after migration |
|--------|----------------------------------|
| Go entrypoint | `cmd/claudia/` (suggested) |
| Discovery artifacts | `docs/bifrost-discovery.md` |
| Packaging | `docs/packaging.md` |
| GUI testing | `docs/gui-testing.md` |

---

*Last updated: plan authored; no phases completed yet.*
