# Installation

This document covers **installing toolchains and third-party binaries** so you can build and run Claudia Gateway from source. It does **not** cover gateway configuration (tokens, `gateway.yaml`, provider keys) or verifying that the server is healthy — see [configuration.md](configuration.md) and the **Execution** section in the repo [README.md](../README.md).

## What gets installed

From a clean clone you typically need:

1. **Language runtimes and build driver** on your machine — **Go** (to build this repo and BiFrost’s Go code), **Node.js** (BiFrost’s UI is built with `npm`; `make bootstrap-deps` runs that step), **Git** (clone BiFrost), and **GNU Make** (this repo’s **`Makefile`** and BiFrost’s **`make build`**).
2. **BiFrost** — a checkout under **`.deps/bifrost`** and a compiled **`bifrost-http`** binary copied to **`./bin/bifrost-http`**. The gateway talks to BiFrost over HTTP (upstream URL in `gateway.yaml` when you configure it later).
3. **Qdrant** (optional for the full local stack) — a prebuilt **`./bin/qdrant`** binary downloaded from GitHub releases, matching the version pinned in **`deps.lock`**.

Pinned versions live in repo-root **`deps.lock`** (single place to bump them). The important keys are **`BIFROST_GIT_URL`**, **`BIFROST_GIT_REF`** (commit, tag, or branch), and **`QDRANT_RELEASE`**. **`scripts/bootstrap-deps.sh`** and **`scripts/deps-lock.sh`** read that file; always treat **`deps.lock`** as the source of truth for exact pins.

## Prerequisites

You need **Go 1.22+**, **Node.js 20+**, **Git**, **GNU Make**, plus **`bash`**, **`curl`**, and **`tar`** for **`make bootstrap-deps`**. Below: **Ubuntu**, **macOS**, and **Windows** for each.

Go drives **`go build`** / **`make claudia-build`** here and inside BiFrost’s **`make build`**. Node is only for building BiFrost (including **`bootstrap-deps`**); the **`claudia`** binary does not embed Node.

### Go

- **Ubuntu:** Default **`golang-go`** in the archive can be older than 1.22. Prefer: **`sudo snap install go --classic`** (tracks current stable), or install from [go.dev/dl](https://go.dev/dl/) (Linux tarball: extract to **`/usr/local/go`**, then add **`export PATH=$PATH:/usr/local/go/bin`** to **`~/.profile`** and open a new shell). Verify: **`go version`** (need **1.22+**).
- **macOS:** **`brew install go`**, or install the **macOS** package from [go.dev/dl](https://go.dev/dl/). Verify: **`go version`**.
- **Windows:** MSI from [go.dev/dl](https://go.dev/dl/) or **`winget install GoLang.Go`**. If **`go`** is missing in a new terminal: **Settings → System → About → Advanced system settings → Environment Variables** → add **`C:\Program Files\Go\bin`** (or your install location) to **Path**. Verify: **`go version`**.

### Node.js (20 or later)

- **Ubuntu:** The stock **`nodejs`** package may be too old. Use [NodeSource’s Node 20 setup](https://github.com/nodesource/distributions#installation-instructions) (e.g. their **`setup_20.x`** script then **`sudo apt-get install -y nodejs`**), or install [nvm](https://github.com/nvm-sh/nvm) and run **`nvm install 20`**. Verify: **`node -v`** (major version **≥ 20**).
- **macOS:** **`brew install node`** (upgrade with **`brew upgrade node`** if **`node -v`** is below **20**), or install **LTS** from [nodejs.org](https://nodejs.org/). Verify: **`node -v`** (major **≥ 20**).
- **Windows:** LTS installer from [nodejs.org](https://nodejs.org/) or **`winget install OpenJS.NodeJS.LTS`**. Verify: **`node -v`**.

### Git

- **Ubuntu:** **`sudo apt update && sudo apt install git`**. Verify: **`git --version`**.
- **macOS:** **`xcode-select --install`** ([Xcode Command Line Tools](https://developer.apple.com/xcode/resources/)) includes **`git`**, or **`brew install git`**. Verify: **`git --version`**.
- **Windows:** [git-scm.com/download/win](https://git-scm.com/download/win) or **`winget install Git.Git`**. Open a **new** terminal; verify: **`git --version`**.

### Make

Use **GNU Make**, not MSVC **`nmake`**. Verify: **`make --version`** (look for *GNU Make*).

- **Ubuntu:** **`sudo apt update && sudo apt install build-essential`** ( **`make`** plus a C toolchain BiFrost may need) or **`sudo apt install make`**. Verify: **`make --version`**.
- **macOS:** **Xcode Command Line Tools** (**`xcode-select --install`**) provide **`make`**. Or **`brew install make`** — if the command is **`gmake`**, run **`gmake`** wherever this doc says **`make`**, or put GNU **`make`** first on **`PATH`**. Verify: **`make --version`** or **`gmake --version`**.
- **Windows:** [Git for Windows](https://git-scm.com/download/win) does **not** ship **`make`**. Options (use the same environment you run **`make`** in): **WSL (Ubuntu):** **`sudo apt update && sudo apt install make`**; **Scoop:** **`scoop install make`**; **Chocolatey:** **`choco install make`**; **MSYS2:** **`pacman -S make`**. Verify inside that environment: **`make --version`**.

### `bash`, `curl`, and `tar` (for `make bootstrap-deps`)

**`scripts/bootstrap-deps.sh`** is a **bash** script and uses **`curl`** and **`tar`** for Qdrant.

- **Ubuntu:** **`sudo apt install bash curl tar`** (**`bash`** is usually already present). Verify: **`bash --version`**, **`curl --version`**, **`tar --version`**.
- **macOS:** **bash**, **curl**, and **tar** ship with the OS; Xcode CLT is enough if prompted. Verify the same three commands.
- **Windows:** **WSL (Ubuntu)** — install as on Ubuntu above (minimal images: **`sudo apt install curl tar`**). **Git Bash** includes **`bash`** and **`curl.exe`**; you still need **GNU Make** separately, and **`scripts/fetch-qdrant-local.sh`** does **not** support native Windows — see below. **cmd.exe** is not supported for **`make bootstrap-deps`**.

### Windows and `make bootstrap-deps`

For **Windows**, run **`make bootstrap-deps`** from **WSL** (same Ubuntu steps as above inside the distro) so **bash**, **curl**, **tar**, **GNU Make**, and the **Qdrant** download path all match **Linux**. You can install **Go** and **Node** inside WSL as well, or use Windows-native Go/Node only if you consistently invoke **`make`** from WSL with a consistent **`PATH`** (advanced).

**Qdrant binary download:** **`scripts/fetch-qdrant-local.sh`** only supports **Linux** and **macOS**. On **Windows** without WSL, install Qdrant manually from [Qdrant releases](https://github.com/qdrant/qdrant/releases) using **`QDRANT_RELEASE`** in **`deps.lock`**, or use **WSL** for the full **`make bootstrap-deps`** flow.

## Clone the repository

```bash
git clone <your-fork-or-upstream-url> claudia-gateway
cd claudia-gateway
```

Use whichever remote you develop against; the install steps are the same.

## Bootstrap BiFrost and Qdrant (`make bootstrap-deps`)

Run **once** per machine (or after you delete **`.deps`** / **`bin`**) from the repository root:

```bash
make bootstrap-deps
```

### What this does

1. Creates **`.deps/bifrost`** (default) if needed, **clones** BiFrost from **`BIFROST_GIT_URL`**, **fetches** objects until the commit in **`BIFROST_GIT_REF`** is available, and **checks out** that exact revision so your build matches everyone else using the same lock file.
2. Runs BiFrost’s **`make setup-workspace`** and **`make build LOCAL=1`** in that tree. That can take **several minutes** the first time: it may run **`npm ci`** under BiFrost’s UI directory and compile Go with a workspace that lines up BiFrost’s local modules (see comments in **`scripts/bootstrap-deps.sh`** for why **`LOCAL=1`** is used).
3. Copies **`tmp/bifrost-http`** from the BiFrost build into **`./bin/bifrost-http`** and marks it executable.
4. Runs **`scripts/fetch-qdrant-local.sh`**, which downloads the **musl** (Linux) or **darwin** archive for your CPU from GitHub and installs **`./bin/qdrant`**.

### Re-runs and disk layout

- If **`.deps/bifrost`** already exists, the script **reuses** it, fetches updates, and checks out the pinned ref again — useful after a **`deps.lock`** bump.
- Expect a **large** **`.deps/bifrost`** directory (clone + `node_modules` + build artifacts). **`./bin`** holds the two binaries you run directly.

### Common failures

| Symptom | Likely cause |
|--------|----------------|
| `bootstrap-deps: install Node.js 20+` | Node missing or too old; install or upgrade and open a new shell. |
| `git` / `curl` / `make` / `bash` not found, or **`make`** is not GNU Make | Install the missing tools (see above); on Windows use **GNU Make** from WSL / Scoop / Chocolatey / MSYS2 — not MSVC **`nmake`**. |
| BiFrost `make` errors | Often network or incomplete clone; try removing **`.deps/bifrost`** and re-running, or check BiFrost’s docs for extra OS packages. |
| Qdrant script exits on Windows | Use **WSL** for **`make bootstrap-deps`**, or install Qdrant manually (see **Windows** above). |

## Alternative: build BiFrost from your own checkout (`make bifrost-from-src`)

If you already maintain a BiFrost tree (for example at **`$HOME/src/bifrost`**), set **`BIFROST_SRC`** to that path and run:

```bash
make bifrost-from-src
```

That builds from **your** branch or commit and copies **`bifrost-http`** to **`./bin/bifrost-http`**. It does **not** download Qdrant; use **`make qdrant-from-release`** (or the fetch script) if you still need **`./bin/qdrant`**. Use this path when you are developing or patching BiFrost and want the gateway to run your local build instead of the **`deps.lock`** pin.

## Build the `claudia` binary

After dependencies are in place (BiFrost is only required when you run a supervised stack or point the gateway at a live upstream — not strictly required **only** to compile **`claudia`**):

```bash
make claudia-build
```

This produces the **`./claudia`** executable in the repo root (or use **`go build -o claudia ./cmd/claudia`** if you prefer invoking **`go`** directly).

## Next steps

- **Configuration** (environment file, **`config/tokens.yaml`**, **`config/gateway.yaml`**, **`config/bifrost.config.json`**): [configuration.md](configuration.md).
- **Running** BiFrost and the gateway together (**`claudia serve`**, local binaries): [supervisor.md](supervisor.md).
