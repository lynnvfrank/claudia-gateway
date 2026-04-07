#!/usr/bin/env bash
# Installs native dependencies to build the Fyne GUI (CGO) on Ubuntu/Debian, macOS, or Windows (Git Bash).
set -euo pipefail

os=$(uname -s)

if [[ "$os" == "Linux" ]]; then
  if [[ ! -f /etc/debian_version ]]; then
    echo "gui-install: non-Debian Linux. Install deps for your distro, then re-run make gui-build." >&2
    echo "  https://developer.fyne.io/started/" >&2
    exit 1
  fi
  echo "gui-install: Debian/Ubuntu (incl. 14.04+) — apt-get install build deps for Fyne..."
  sudo apt-get update
  sudo apt-get install -y \
    build-essential \
    gcc \
    pkg-config \
    libgl1-mesa-dev \
    libx11-dev \
    libxrandr-dev \
    libxinerama-dev \
    libxcursor-dev \
    libxi-dev \
    libxxf86vm-dev
  echo "gui-install: done. Next: make gui-build"

elif [[ "$os" == "Darwin" ]]; then
  echo "gui-install: macOS — checking Xcode Command Line Tools (required for CGO/Fyne)..."
  if xcode-select -p >/dev/null 2>&1 && command -v clang >/dev/null 2>&1; then
    echo "gui-install: Xcode CLT and clang are present."
    exit 0
  fi
  echo "gui-install: Launching Apple installer for Command Line Tools (GUI approval may be required)..."
  xcode-select --install 2>/dev/null || true
  echo "gui-install: When the install finishes, run: xcode-select -p && clang --version"
  echo "gui-install: Then: make gui-build"

elif [[ "$os" =~ ^MINGW ]] || [[ "$os" =~ ^MSYS ]] || [[ "$os" =~ ^CYGWIN ]]; then
  REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
  # shellcheck source=scripts/msys2-gcc-path.sh
  source "$REPO_ROOT/scripts/msys2-gcc-path.sh"

  # Append a guarded PATH block to ~/.bashrc so new Git Bash sessions find MSYS2 gcc.
  _win_gui_ensure_bashrc_path() {
    local mroot="$1"
    local envname="$2"
    local brc="${HOME}/.bashrc"
    local sentinel="# claudia-gateway gui-install MSYS2 PATH"
    [[ -f "$brc" ]] && grep -Fq "$sentinel" "$brc" 2>/dev/null && return 0
    local p_tool p_usr
    p_tool="${mroot}/${envname}/bin"
    p_usr="${mroot}/usr/bin"
    {
      echo ""
      echo "$sentinel (Fyne/CGO)"
      echo "MSYS2_TOOLCHAIN_BIN=${p_tool@Q}"
      echo "MSYS2_USR_BIN=${p_usr@Q}"
      echo 'export PATH="$MSYS2_TOOLCHAIN_BIN:$MSYS2_USR_BIN:$PATH"'
    } >>"$brc"
    echo "gui-install: appended MSYS2 toolchain PATH to $brc (remove that block to undo)."
  }

  echo "gui-install: Windows — C compiler on PATH is required for Go CGO (Fyne)."
  if has_cc; then
    echo "gui-install: gcc found:"
    gcc --version | head -1
    exit 0
  fi

  winget_cmd=""
  if command -v winget.exe >/dev/null 2>&1; then
    winget_cmd=winget.exe
  elif command -v winget >/dev/null 2>&1; then
    winget_cmd=winget
  elif command -v powershell.exe >/dev/null 2>&1; then
    w=$(powershell.exe -NoProfile -Command "try { (Get-Command winget -ErrorAction Stop).Source } catch { '' }" 2>/dev/null | tr -d '\r')
    if [[ -n "$w" ]]; then
      winget_cmd="$w"
    fi
  fi

  if [[ -n "$winget_cmd" ]]; then
    echo "gui-install: Installing MSYS2 via winget (administrator prompt may appear)..."
    "$winget_cmd" install -e --id MSYS2.MSYS2 --accept-package-agreements --accept-source-agreements || true
  else
    echo "gui-install: winget not found. Install MSYS2 from https://www.msys2.org/" >&2
  fi

  mroot=""
  if mroot="$(msys2_resolve_root)"; then
    :
  else
    mroot=""
  fi

  if [[ -n "$mroot" && "${SKIP_MSYS_PACMAN:-}" != "1" ]]; then
    echo "gui-install: MSYS2 at $mroot — pacman sync + MinGW GCC (may take a few minutes)..."
    mbash="$mroot/usr/bin/bash.exe"
    "$mbash" -l -c "pacman -Syu --noconfirm" || true
    "$mbash" -l -c "pacman -Syu --noconfirm" || true
    if ! "$mbash" -l -c "pacman -S --needed --noconfirm mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkg-config"; then
      echo "gui-install: UCRT GCC install failed; trying MINGW64 packages..." >&2
      "$mbash" -l -c "pacman -S --needed --noconfirm mingw-w64-x86_64-gcc mingw-w64-x86_64-pkg-config" || true
    fi
  elif [[ -z "$mroot" ]]; then
    echo "gui-install: MSYS2 not found under MSYS2_ROOT, /c/msys64, or \"Program Files\". Install from https://www.msys2.org/" >&2
  fi

  msys2_prepend_gcc_path || true

  if has_cc; then
    echo "gui-install: gcc found:"
    gcc --version | head -1
    if [[ "${SKIP_BASHRC_PATH:-}" != "1" && -n "${CLAUDIA_MSYS2_TOOLCHAIN_ROOT:-}" && -n "${CLAUDIA_MSYS2_TOOLCHAIN_ENV:-}" ]]; then
      _win_gui_ensure_bashrc_path "${CLAUDIA_MSYS2_TOOLCHAIN_ROOT}" "${CLAUDIA_MSYS2_TOOLCHAIN_ENV}"
    fi
    echo "gui-install: done. Next: make gui-build"
    exit 0
  fi

  echo "" >&2
  echo "gui-install: automatic setup did not expose gcc on PATH." >&2
  echo "  • Open \"MSYS2 UCRT64\" from the Start menu and run: pacman -Syu  (repeat until stable)" >&2
  echo "  • Then: pacman -S --needed mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkg-config" >&2
  echo "  • Or set MSYS2_ROOT if MSYS2 is not under C:\\msys64, then re-run make gui-install" >&2
  echo "  • Optional: SKIP_MSYS_PACMAN=1 to skip pacman; SKIP_BASHRC_PATH=1 to skip ~/.bashrc edits" >&2
  echo "  https://developer.fyne.io/started/" >&2
  exit 1

else
  echo "gui-install: unsupported OS: $os" >&2
  exit 1
fi
