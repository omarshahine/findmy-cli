#!/usr/bin/env bash
# Thin wrapper around the findmy CLI. Resolves the plugin root, builds
# the binaries on first use (no committed artifacts), then shells out.
#
# Usage:
#   findmy.sh people [--json]
#   findmy.sh person "<name>" [--json]
set -euo pipefail

ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)}"
BIN="$ROOT/bin/findmy"
HELPER="$ROOT/bin/findmy-helper"

if [[ ! -x "$BIN" || ! -x "$HELPER" ]]; then
    echo "findmy: building binaries (first run)..." >&2
    if ! command -v go >/dev/null 2>&1; then
        echo "findmy: 'go' is not on PATH; install Go 1.22+ then re-run" >&2
        exit 127
    fi
    if ! command -v swiftc >/dev/null 2>&1; then
        echo "findmy: 'swiftc' is not on PATH; install Xcode Command Line Tools then re-run" >&2
        exit 127
    fi
    ( cd "$ROOT" && make ) >&2
fi

# Ensure the Go CLI can find its sibling helper regardless of cwd.
export FINDMY_HELPER="$HELPER"
exec "$BIN" "$@"
