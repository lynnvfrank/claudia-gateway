#!/usr/bin/env bash
# Parse repository-root deps.lock (KEY=value). Source after setting REPO_ROOT.
# Optional: DEPS_LOCK_FILE overrides the lockfile path.
# shellcheck shell=bash
deps_lock_get() {
	local key="$1"
	local lockfile="${DEPS_LOCK_FILE:-$REPO_ROOT/deps.lock}"
	if [[ ! -f "$lockfile" ]]; then
		echo "deps_lock_get: lockfile not found: $lockfile" >&2
		return 1
	fi
	while IFS= read -r line || [[ -n "$line" ]]; do
		line="${line%$'\r'}"
		[[ "$line" =~ ^[[:space:]]*# ]] && continue
		[[ -z "${line// }" ]] && continue
		if [[ "$line" == "${key}="* ]]; then
			printf '%s\n' "${line#${key}=}"
			return 0
		fi
	done <"$lockfile"
	echo "deps_lock_get: missing key: $key in $lockfile" >&2
	return 1
}
