# GUI testing (Phase 5 + status view)

The **Claudia** desktop shell lives in the nested module **`gui/`** ([Fyne](https://fyne.io/) v2). It **always** shows the Phase 5 footer string **`mew mew, Love Claudia`** (verbatim). The main area **polls** the Go gateway **`GET /status`** (see `internal/server/status.go`) so you can see **supervisor layout** when **`claudia serve`** is running, or **gateway-only** info when **`claudia`** is running without subprocesses.

**Connection model:** the GUI does **not** embed the supervisor. Run the gateway (**Go** **`claudia`** / **`claudia serve`**); the GUI is an **HTTP client** to **`GET /status`**.

Automated UI/E2E in CI is **not** wired (no display server in the default pipeline). This checklist is for **manual** verification after **`make gui-build`** or **`cd gui && go build -o claudia-gui .`** on your machine.

## Prerequisites

Run **`make gui-install`** once per machine (uses Git Bash on Windows, same as other **`make`** scripts):

- **Ubuntu / Debian** (incl. **14.04+**): **`sudo apt-get install`** for **gcc**, **OpenGL**, and **X11** development packages required by Fyne.
- **macOS**: ensures **Xcode Command Line Tools** / **`clang`** (launches Apple’s installer if needed).
- **Windows 11**: uses **`winget`** to install **MSYS2** when possible, then runs **`pacman`** (non-interactive) inside MSYS2 to install **UCRT MinGW GCC** and **`pkg-config`**, prepends **`ucrt64`** (or **`mingw64`**) **`bin`** to **`PATH`** for the current session, and appends a guarded block to **`~/.bashrc`** so **interactive** Git Bash picks up **`gcc`**. **`make gui-build`** runs a **non-interactive** bash that does **not** read **`~/.bashrc`**; **`scripts/gui-build.sh`** sources **`scripts/msys2-gcc-path.sh`** so **MSYS2** **`gcc`** is on **`PATH`** for **`go build`** / **CGO** anyway. Override **`MSYS2_ROOT`** if MSYS2 is not under **`C:\\msys64`**. Set **`SKIP_MSYS_PACMAN=1`** to skip **`pacman`**, or **`SKIP_BASHRC_PATH=1`** to skip editing **`~/.bashrc`**. If automation still fails (fresh MSYS2 sometimes needs an extra **`pacman -Syu`** cycle in the **UCRT64** terminal), follow the script’s printed fallback steps.

For non-Debian Linux or manual setup, see [Fyne — Getting started](https://developer.fyne.io/started/).

## Build

```bash
make gui-build
# → ./claudia-gui (or ./claudia-gui.exe on Windows) in repo root
```

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| **`-gateway`** | `http://127.0.0.1:3000` | Base URL for **`GET /status`** |
| **`-poll`** | `2s` | Refresh interval |

Example:

```bash
./claudia serve -bifrost-bin ./bin/bifrost-http   # terminal 1
./claudia-gui                                      # terminal 2 (same host, default URL)
```

## Manual checklist

- [ ] The application **window title** is **Claudia** (or the OS equivalent).
- [ ] Footer text is **`mew mew, Love Claudia`** (spelling and punctuation match exactly).
- [ ] With **no** gateway running, the status area shows an **offline** message.
- [ ] With **`./claudia`** or **`./claudia serve`** running, status shows **listen**, **virtual model**, **upstream probe**, and **supervisor: active** only for **`claudia serve`**.
- [ ] No crash on launch on **linux-amd64**, **darwin-arm64/amd64**, or **windows-amd64** (as applicable).

## Supervisor integration

Starting **`claudia serve` from inside the GUI** is **not** implemented; use a terminal per [supervisor.md](supervisor.md). The GUI **displays** supervisor details via **`/status`** when the supervised gateway is already running.

## CI

The **Go** workflow **`gui`** job compiles **`gui/`** on **ubuntu-latest** with **`CGO_ENABLED=1`** to satisfy **“Build verification: linux-amd64”** in the migration plan.
