#!/usr/bin/env bash
# Append one MSYS-style directory to the Windows *User* PATH if missing (registry / environment).
# Safe to source from Git Bash / MSYS; no-op on Linux/macOS.
# Embedded single quotes in the Windows path are doubled for PowerShell's single-quoted literal.
win_persist_user_path_dir() {
	local msys_dir="${1//$'\r'/}"
	msys_dir="${msys_dir//\\//}"
	[[ -n "$msys_dir" ]] || return 0
	case "$(uname -s 2>/dev/null || echo unknown)" in
	MINGW* | MSYS* | CYGWIN*) ;;
	*) return 0 ;;
	esac
	local winpath=""
	if command -v cygpath >/dev/null 2>&1; then
		winpath="$(cygpath -w "$msys_dir" 2>/dev/null || true)"
	fi
	[[ -n "${winpath:-}" ]] || return 0
	# PowerShell single-quoted literal: escape embedded ' as ''
	local ps_path="${winpath//\'/\'\'}"
	powershell.exe -NoProfile -Command "
		\$d = '$ps_path'
		if (-not (Test-Path -LiteralPath \$d)) { exit 0 }
		\$cur = [Environment]::GetEnvironmentVariable('Path', 'User')
		if (-not \$cur) { \$cur = '' }
		\$parts = \$cur -split ';' | Where-Object { \$_ -ne '' }
		foreach (\$p in \$parts) {
			if ([string]::Equals(\$p.Trim(), \$d.Trim(), [System.StringComparison]::OrdinalIgnoreCase)) { exit 0 }
		}
		\$new = if (\$cur.Trim()) { \$cur.TrimEnd(';') + ';' + \$d } else { \$d }
		[Environment]::SetEnvironmentVariable('Path', \$new, 'User')
	" 2>/dev/null || true
}
