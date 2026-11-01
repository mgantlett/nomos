#!/usr/bin/env bash
set -e

# Read SessionStart JSON payload from stdin
PAYLOAD=$(cat || true)

if ! command -v jq >/dev/null 2>&1; then
    echo "{}"
    exit 0
fi

NOMOS_BIN=""
if command -v nomos >/dev/null 2>&1; then
    NOMOS_BIN="nomos"
elif [ -n "${GOBIN:-}" ] && [ -x "${GOBIN}/nomos" ]; then
    NOMOS_BIN="${GOBIN}/nomos"
elif [ -x "${HOME}/go/bin/nomos" ]; then
    NOMOS_BIN="${HOME}/go/bin/nomos"
elif [ -x "${HOME}/.local/bin/nomos" ]; then
    NOMOS_BIN="${HOME}/.local/bin/nomos"
else
    echo "{}"
    exit 0
fi

WORKSPACE_DIR=$(echo "$PAYLOAD" | jq -r '.workspacePaths[0] // empty' 2>/dev/null || true)
if [ -z "$WORKSPACE_DIR" ] || [ ! -d "$WORKSPACE_DIR" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    WORKSPACE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
fi

HANDSHAKE_OUT=$(cd "$WORKSPACE_DIR" && "$NOMOS_BIN" handshake 2>/dev/null || true)
if [ -z "$HANDSHAKE_OUT" ]; then
    echo "{}"
    exit 0
fi

printf '%s\n' "$HANDSHAKE_OUT" | jq -Rs '{injectSteps: [{systemMessage: {systemMessage: .}}]}'

