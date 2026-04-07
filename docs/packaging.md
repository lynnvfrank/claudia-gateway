# Packaging and releases (Phase 4)

The **Go `claudia`** binary is built with **[GoReleaser](https://goreleaser.com/)** v2. Each release archive also includes the **Qdrant** binary for the matching OS/arch (**`QDRANT_RELEASE`** in repo-root **`deps.lock`**, fetched by **`scripts/release-snapshot-qdrant.sh`** before packaging). **BiFrost** is **not** bundled (license, size, CGO); operators use **`make install`** — see [supervisor.md](supervisor.md).

## Artifact layout

Each GitHub **Release** (git tag **`v*`, e.g. `v0.1.0`**) publishes:

| File | Contents |
|------|----------|
| **`claudia_<version>_<os>_<arch>.tar.gz`** | Linux/macOS: **`claudia`**, **`qdrant`** (or **`qdrant.exe`** on Windows zip), **`README.md`**, **`PACKAGING.md`** |
| **`claudia_<version>_windows_amd64.zip`** | Windows: **`claudia.exe`**, **`qdrant.exe`**, same docs |
| **`checksums.txt`** | SHA-256 checksums for the archives |

Architectures: **linux/darwin** **amd64** and **arm64**; **windows amd64** only (no **windows/arm64**).

## Prerequisites on the target machine

- **Config:** copy or mount **`config/gateway.yaml`**, **`config/tokens.yaml`**, **`config/bifrost.config.json`** (and **`routing-policy.yaml`** paths as in YAML). See [configuration.md](configuration.md).
- **Environment:** **`CLAUDIA_UPSTREAM_API_KEY`** and provider keys (**`GROQ_API_KEY`**, etc.) — or a **`.env`** file in the **working directory** (the binary loads it at startup).
- **Upstream:** BiFrost (or another OpenAI-compatible proxy) reachable at **`upstream.base_url`**.

## Install (quick)

**Linux / macOS**

```bash
tar xzf claudia_<version>_linux_amd64.tar.gz
cd claudia_<version>_linux_amd64   # or matching folder name inside the archive
./claudia -version
./claudia -h
# Optional: ./qdrant with env from Qdrant docs, or claudia serve -qdrant-bin ./qdrant
```

**Windows**

Extract the **`.zip`**, run **`claudia.exe`** from PowerShell or cmd. SmartScreen may warn on first run for unsigned binaries; **code signing** is a documented follow-up.

## Cutting a release (maintainers)

1. Ensure **`go test ./...`** and **`gofmt`** are clean.
2. Tag: **`git tag v0.x.y`** and **`git push origin v0.x.y`** (prerelease: **`v0.1.0-rc.1`**, etc.).
3. **GitHub Actions** workflow **Release** runs GoReleaser and uploads assets (requires default **`GITHUB_TOKEN`** with **contents: write**).

Release notes may reference [SECURITY.md](../SECURITY.md).

Local snapshot (no upload):

```bash
make release-snapshot
# or: goreleaser release --snapshot --clean
```

Artifacts appear under **`dist/`** (gitignored).

## Version string

Release builds embed the tag, commit, and commit date:

```bash
claudia -version
```

Plain **`go build`** without **`-ldflags`** reports **`dev`**, **`none`**, **`unknown`**.

## Qdrant licensing

Qdrant is **Apache-2.0**. The archive **redistributes** the official prebuilt binary from [github.com/qdrant/qdrant](https://github.com/qdrant/qdrant/releases). Bump **`QDRANT_RELEASE`** in **`deps.lock`** when you want a newer Qdrant in releases.

## BiFrost version pinning

Releases version **`claudia`** and bundle **Qdrant** as above. Record the **BiFrost** image tag or binary version you tested against in release notes; bundling BiFrost remains out of scope.

## GUI binary (`claudia-gui`)

The **Fyne** app is a **separate nested module** under **`gui/`** (CGO, platform UI libraries). It is **not** part of the static **`claudia`** GoReleaser archives. Build locally with **`make gui-build`** → **`./claudia-gui`** (or **`./claudia-gui.exe`** on Windows); see [gui-testing.md](gui-testing.md). Bundling the GUI into release zips may be added later (cross-compiling Fyne in CI is heavier than the gateway binary).

## Follow-ups (not in Phase 4)

- **macOS** notarization / **Windows** Authenticode signing.
- **`LICENSE`** / third-party notices in archives once a license is chosen for publication.
