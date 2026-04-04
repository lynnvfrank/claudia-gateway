# GUI testing (Phase 5 + status view)

The **Claudia** desktop shell lives in the nested module **`gui/`** ([Fyne](https://fyne.io/) v2). It **always** shows the Phase 5 footer string **`mew mew, Love Claudia`** (verbatim). The main area **polls** the Go gateway **`GET /status`** (see `internal/server/status.go`) so you can see **supervisor layout** when **`claudia serve`** is running, or **gateway-only** info when **`claudia`** is running without subprocesses.

**Connection model:** the GUI does **not** embed the supervisor. Run the gateway (**Go** **`claudia`** / **`claudia serve`**); the GUI is an **HTTP client** to **`GET /status`**.

Automated UI/E2E in CI is **not** wired (no display server in the default pipeline). This checklist is for **manual** verification after **`make claudia-gui-build`** or **`cd gui && go build -o claudia-gui .`** on your machine.

## Prerequisites (Linux)

Install build deps (names vary by distro; Debian/Ubuntu example):

```bash
sudo apt-get install -y gcc pkg-config libgl1-mesa-dev libx11-dev libxrandr-dev \
  libxinerama-dev libxcursor-dev libxi-dev libxxf86vm-dev
```

**macOS** / **Windows:** follow [Fyne — Getting started](https://developer.fyne.io/started/) (Xcode command line tools / MSVC + GCC as appropriate).

## Build

```bash
make claudia-gui-build
# → ./claudia-gui in repo root
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
