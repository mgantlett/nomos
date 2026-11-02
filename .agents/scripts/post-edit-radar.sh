#!/usr/bin/env bash
set -e

PAYLOAD=$(cat || true)

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

if "$NOMOS_BIN" hook --help >/dev/null 2>&1; then
    printf '%s' "$PAYLOAD" | "$NOMOS_BIN" hook post-tool
    exit 0
fi

echo "{}"
