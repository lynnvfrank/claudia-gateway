# Makefile and workflow plan

Design notes for the root [Makefile](../Makefile) and bash-driven scripts. **The Makefile is the source of truth** for target names; this document records intent, what shipped, and what is no longer worth tracking.

---

## Quick status

| Area | Status |
|------|--------|
| **`make up`** (`install` → `claudia-build` → `desktop-build` → `desktop-run`) | **Done** |
| **`make install`** (`claudia-install` + `desktop-install`) / **`claudia-install`** / **`configure`** / **`clean`** / **`clean-all`** / **`clean-data`** | **Done** |
| Foreground **`claudia-serve`**, background **`claudia-start`** / **`stop`** / **`logs`** / **`claudia-status`** | **Done** |
| **`UP_STACK=0`** (BiFrost only, no Qdrant) | **Done** |
| Desktop: **`desktop-install`** / **`desktop-build`** / **`desktop-run`**, **`vet-desktop`** | **Done** |
| Quality gate **`precommit`** (`fmt-check`, **`vet`**, **`test`**; desktop slice omitted with **`SKIP_DESKTOP=1`**) | **Done** |
| Catalog tools **`catalog-free`** / **`catalog-available`** / **`config-provider-free-tier`** | **Done** |
| Release **`release-install`** / **`release-snapshot`** / **`package`** | **Done** |
| No **`make doctor`**; no duplicate meta-targets (**`ci`**, etc.) | **Done** |
| PowerShell twin for every **`scripts/*.sh`** | **Optional / not important** — install uses bash; **`install-make.ps1`** exists for make only |
| Separate **`gui/`** module targets (**`vet-gui`**, **`test-gui`**, **`SKIP_GUI`**) | **Superseded** — webview lives in **`cmd/claudia`** with **`vet-desktop`** / **`SKIP_DESKTOP`** |
| Richer **`claudia-status`** (read ports from **`gateway.yaml`**) | **Optional follow-up** |

---

## Principles (still in force)

### No standalone “doctor”

Do not add **`make doctor`**. **`make claudia-install`** (BiFrost/Qdrant bootstrap via **`scripts/install.sh`**) should stay **idempotent** and report what it checked, what it skipped, and what failed. **`make install`** chains **`claudia-install`** then **`desktop-install`**. Deeper diagnostics stay in docs or optional scripts, not a competing Make entry point.

### Single canonical names

One target per behavior: e.g. **`precommit`** (not **`ci`**), **`test`** (aggregates **`test-*`** slices), **`claudia-serve`** (not local/stack aliases). Avoid parallel aliases for the same workflow.

### Bootstrap from **`deps.lock`** only

BiFrost and Qdrant come from **`scripts/install-bootstrap.sh`** / pinned **`deps.lock`**, not ad hoc “build from **`$HOME/src/bifrost`**” flows. Removed targets stay removed: **`bifrost-from-src`**, **`BIFROST_SRC`**, **`bifrost-node-check`**, **`bootstrap-deps`**, etc.

### **`make clean`** vs **`make clean-all`**

- **`clean`**: local artifacts — **`claudia`**, **`claudia-desktop`**, **`dist/`** (see Makefile header).
- **`clean-all CONFIRM=1`**: also **`bin/`**, **`packaging/qdrant-bundles/`**, **`packages/`**, **`node_modules/`**, **`.deps/`**, **`run/`**, **`logs/`**, then **`clean`**.

### Formatting scope

**`make fmt`** / **`fmt-check`**: **`cmd/`** and **`internal/`** only (same as CI). There is no separate **`gui/`** tree in this layout.

---

## Historical / not important to implement

These appeared in older versions of this plan; the product moved to **webview + `cmd/claudia`** and the names below are **not** Makefile targets today.

- **`vet-gui`**, **`test-gui`**, **`SKIP_GUI`** — replaced by **`vet-desktop`** / **`test-desktop`** and **`SKIP_DESKTOP=1`** for precommit when CGO/WebView is unavailable.
- **`fmt`** over a top-level **`gui/`** module — obsolete; Fyne **`gui/`** is not the shipping desktop path.
- “All-in-one entry point name TBD” — settled on **`make up`**.
- PID under **`.run/`** — repo uses **`run/claudia.pid`**; no need to migrate.

---

## Run modes (reference)

- **Foreground:** **`make claudia-run`** (bare gateway **`go run`**) vs **`make claudia-serve`** (supervisor + **`bin/bifrost-http`** + **`bin/qdrant`**).
- **Background:** **`make claudia-start`** (same supervisor as serve; **`--stack`** unless **`UP_STACK=0`**), **`logs/`**, **`run/claudia.pid`**, **`make claudia-stop`** / **`make claudia-status`**.
- **`claudia serve`** supervises children; the PID file tracks the **supervisor** process.

---

## README alignment (reference)

| Concern | Makefile role |
|--------|----------------|
| Toolchain + BiFrost/Qdrant pins | **`make claudia-install`** (or **`make install`** for desktop OS deps too) |
| **`.env`**, tokens, config files | **`make configure`** (+ UI / manual for **`tokens.yaml`**) |
| Run stack | **`claudia-run`**, **`claudia-serve`**, **`claudia-start`** / **`stop`** / **`status`**, **`logs`** |
| Local gate before commit | **`make precommit`** |

---

## Optional follow-ups (low priority)

- **`claudia-status`**: optionally parse **`config/gateway.yaml`** for bind ports instead of env defaults.
- **`claudia-stop`**: process-group / child teardown if orphan processes become a real issue.
- More PowerShell-first onboarding — only if Windows operators routinely avoid Git Bash.

---

## When editing the Makefile

Update **[README.md](../README.md)** and **[docs/installation.md](installation.md)** / **[docs/supervisor.md](supervisor.md)** if behavior or names change, and adjust **this file** so the “Quick status” table stays honest.
