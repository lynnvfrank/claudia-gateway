#!/usr/bin/env bash
# Install Qdrant binary for the current machine into ./bin/qdrant (or bin/qdrant.exe on Windows).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VER="$(tr -d '[:space:]' <"$ROOT/scripts/qdrant-pinned-version.txt")"
BASE="https://github.com/qdrant/qdrant/releases/download/${VER}"
mkdir -p "$ROOT/bin"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os" in
linux)
  case "$arch" in
  x86_64) asset="qdrant-x86_64-unknown-linux-musl.tar.gz" ;;
  aarch64 | arm64) asset="qdrant-aarch64-unknown-linux-musl.tar.gz" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
  tmp="$(mktemp -d)"
  curl -fsSL "${BASE}/${asset}" | tar xz -C "$tmp"
  mv "$tmp/qdrant" "$ROOT/bin/qdrant"
  chmod +x "$ROOT/bin/qdrant"
  rm -rf "$tmp"
  echo "Installed $ROOT/bin/qdrant ($VER)"
  ;;
darwin)
  case "$arch" in
  x86_64) asset="qdrant-x86_64-apple-darwin.tar.gz" ;;
  arm64) asset="qdrant-aarch64-apple-darwin.tar.gz" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
  tmp="$(mktemp -d)"
  curl -fsSL "${BASE}/${asset}" | tar xz -C "$tmp"
  mv "$tmp/qdrant" "$ROOT/bin/qdrant"
  chmod +x "$ROOT/bin/qdrant"
  rm -rf "$tmp"
  echo "Installed $ROOT/bin/qdrant ($VER)"
  ;;
*)
  echo "fetch-qdrant-local.sh: use WSL/Linux/macOS or download manually from ${BASE}" >&2
  exit 1
  ;;
esac
