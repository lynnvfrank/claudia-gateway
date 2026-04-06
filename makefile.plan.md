# Makefile expansion plan

Design spec for evolving [Makefile](Makefile) and README-driven workflows. **Not all targets exist yet**; this file is the source of requirements.

---

## A. Non-negotiable requirements (current direction)

### A.1 No `make doctor`

Do **not** add a separate **`doctor`** / **`check-prereqs`** target. **`make install`** must be **idempotent** and, when run, **report health and status** of the toolchain and repo state (what is present, what was skipped, what failed, suggested next step). Verification is part of **install**, not a standalone diagnostic command.

### A.2 `make install`

- Installs or verifies **everything required** to build and run the stack from pinned **`deps.lock`** (BiFrost under **`.deps/`**, Qdrant + **`bifrost-http`** in **`bin/`**, plus documented host deps: Go, Node, git, etc.).
- **Idempotent**: safe to run repeatedly; skips work already satisfied and **says so**.
- **No “build BiFrost from a separate src tree”**: drop **`bifrost-from-src`**, **`BIFROST_SRC`**, and **`bifrost-node-check`** as user-facing flows. BiFrost comes only from the **pinned bootstrap** path (e.g. **`make install`** / **`scripts/install-bootstrap.sh`**), not from **`$HOME/src/bifrost`**.

### A.3 `make clean` and `make clean-all`

- **`make clean`**: remove **local build artifacts** (e.g. **`claudia`**, **`claudia-gui`**, **`dist/`**, object caches if any) — **not** downloaded **`bin/bifrost-http[.exe]`**, **`bin/qdrant[.exe]`**, and **not** **`.deps/`**, unless explicitly documented otherwise.
- **`make clean-all`**: stronger reset — additionally remove **`bin/`** third-party binaries and/or **`.deps/`** (and any **run**/ **logs**/ PID state), with clear messaging and optional confirmation for destructive steps.

Exact file sets should be listed in the Makefile comment block when implemented.

### A.4 `make fmt` and `make logs`

- **`make fmt`**: **automatically fix** formatting — run **`gofmt -w`** on **`cmd/`**, **`internal/`**, **`gui/`** (same scope as CI / **`fmt-check`**). No separate “fmt-fix” alias; **`fmt`** is the canonical fix target.
- **`make logs`**: show **recent log output** for supervised / background runs (e.g. tail **`logs/`** or per-service files under a **gitignored** log directory). Depends on the logging layout chosen for **`claudia` background / file logging** (see §C).

### A.5 `make claudia-status`

- Reports **runtime status** of Claudia-related services: e.g. whether **PID files** exist and processes are alive, **listening ports** (gateway / BiFrost / Qdrant), and **URLs** to open — using **`config/gateway.yaml`** / defaults where applicable.
- Distinct from **`make install`**: **install** focuses on **dependencies and artifacts**; **claudia-status** focuses on **whether the stack is up and reachable**.

### A.6 No duplicate aliases

- **One canonical target name** per behavior. Do **not** keep parallel aliases such as **`ci`** vs **`precommit`**, **`test`** vs **`test-gateway`**, **`claudia-serve-local`** vs **`claudia-serve`**, etc.
- Pick single names (e.g. only **`precommit`**, only **`test-gateway`**) and update README / docs / CI scripts accordingly.

### A.7 One command: install → configure → build → run

- A **single Makefile target** runs the full happy path in order:
  1. **Install** (idempotent deps + bootstrap BiFrost/Qdrant per **`deps.lock`** + status report).
  2. **Configure** (ensure **`.env`**, **`config/tokens.yaml`**, and any required files exist — interactive or non-interactive flags as designed).
  3. **Build** (**`claudia`** binary and anything else required for the chosen run mode).
  4. **Run** (start the stack — foreground or background per design; see §C).

Exact target name is TBD (**`make up`**, **`make dev`**, **`make start`**, etc.) but there must be **exactly one** documented entry point for “everything.”

---

## B. README alignment

| README concern | Planned Makefile role |
|----------------|------------------------|
| Prerequisites + BiFrost/Qdrant pins | Covered by **`make install`** (and the all-in-one target §A.7). |
| Copy/edit **`.env`**, **`tokens.yaml`**, **`gateway.yaml`**, **`bifrost.config.json`** | **`make configure`** (or a step inside the all-in-one command). |
| Run gateway / supervisor / stack | **`make claudia-run`**, **`make claudia-serve`**, background + **`make claudia-stop`**, **`make claudia-status`**, **`make logs`** — final names TBD but **no alias sprawl**. |
| Pre-commit quality gate | **One** target (e.g. **`precommit`** only): **`fmt-check`**, **`vet`**, **`test`** (split by module as needed, but **no duplicate “meta” aliases**). |

---

## C. Run modes (supervisor, background, PIDs)

- **Foreground**: gateway only vs full **`claudia serve`** (BiFrost ± Qdrant) — keep a small, clear set of targets.
- **Background**: optional **`make claudia-start`** (or folded into the all-in-one “run” step): logs under **gitignored** **`logs/`**, **PID files** under **gitignored** **`run/`** (or **`.run/`**), print **URLs**; **`make claudia-stop`** tears down using PID files.
- **`claudia-status`** reads those PIDs / probes ports / health endpoints.

**Caution**: **`claudia serve`** is one process supervising children; PID file semantics must match whether you track the **supervisor** only or each child.

---

## D. Quality, test, release (single names)

| Concern | Canonical target (proposal) |
|---------|------------------------------|
| Auto-format | **`fmt`** |
| CI-style format check | **`fmt-check`** |
| Vet / test gateway module | **`vet-gateway`**, **`test-gateway`** (no **`test`** alias) |
| Vet / test gui module | **`vet-gui`**, **`test-gui`** |
| Full local gate before commit | **`precommit`** only (chains the above; **`SKIP_GUI=1`** when Fyne/CGO unavailable) |
| GoReleaser snapshot | **`release-snapshot`** |

**`./scripts/precommit-smoke.sh`**: optional wrapper around **`make precommit`**.

---

## E. Cross-platform (bash + PowerShell)

- **`make install`** and bootstrap logic should remain implementable via **bash** and **PowerShell** siblings where shell is required; **`deps.lock`** stays the single version source.
- **Windows**: **`bifrost-http.exe`**, **`qdrant.exe`**, and **`go vet -C` / `go test -C`** / CGO / `-race` constraints remain implementation details called out in README.

---

## F. Gitignore (planned / ongoing)

- **`logs/`** — service logs.
- **`run/`** or **`.run/`** — PID files and optional locks.
- Existing ignores for **`bin/`** artifacts, **`.deps/`**, etc. — align with **`clean`** vs **`clean-all`**.

---

## G. Removed from scope (do not bring back)

- **`make doctor`** / standalone prereq-only targets.
- **`bifrost-from-src`**, **`BIFROST_SRC`**, **`bifrost-node-check`** as documented user flows.
- **Duplicate Makefile aliases** for the same behavior (§A.6).

---

## H. Implementation status

The following are **implemented** in the repo (see root **`Makefile`** and **`scripts/`**):

1. Deprecated **Makefile** targets removed: **`bifrost-from-src`**, **`BIFROST_SRC`**, **`bifrost-node-check`**, **`bootstrap-deps`**, **`claudia-serve-local`**, **`claudia-serve-stack`** (behavior is **`make claudia-serve`**), **`ci`**, **`test-all`**, **`test`** (alias).
2. **`make install`** — **`scripts/install.sh`**: toolchain report + **`install-bootstrap.sh`**.
3. **`make configure`** — **`scripts/configure.sh`**.
4. **`make clean`** / **`make clean-all CONFIRM=1`** — **`scripts/clean.sh`**, **`scripts/clean-all.sh`**.
5. **`make fmt`** / **`make fmt-check`**, **`make logs`**, **`make claudia-status`**.
6. **`make up`** — **`install`** → **`configure`** → **`claudia-build`** → **`claudia-start`** (background; **`UP_STACK=0`** omits Qdrant).
7. **`make claudia-start`** / **`claudia-stop`** — **`scripts/claudia-start.sh`**, **`claudia-stop.sh`**; logs **`logs/claudia.log`**, PID **`run/claudia.pid`**.
8. **`make precommit`** only (no **`ci`**); gateway **`test-gateway`**, **`vet-gateway`**, gui **`test-gui`**, **`vet-gui`**; **`SKIP_GUI=1`** supported.

**Optional / follow-ups:** PowerShell twins for every shell script (§E); richer **`claudia-status`** (parse **`gateway.yaml`** for ports); process-group kill on **`claudia-stop`** if orphans appear.

---

## I. Doc updates when implementing

- [README.md](README.md): primary onboarding is **`make up`**.
- [docs/installation.md](docs/installation.md), [docs/supervisor.md](docs/supervisor.md): use **`make install`** / **`deps.lock`** (no **`bifrost-from-src`**).
- Keep this file updated when target names change.
