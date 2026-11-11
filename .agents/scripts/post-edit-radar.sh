#!/usr/bin/env bash
set -e

if [ -n "${GOBIN:-}" ] && [ -x "${GOBIN}/nomos" ]; then
    exec "${GOBIN}/nomos" hook post-tool
elif [ -x "${HOME}/go/bin/nomos" ]; then
    exec "${HOME}/go/bin/nomos" hook post-tool
elif command -v nomos >/dev/null 2>&1; then
    exec nomos hook post-tool
elif [ -x "${NOMOS_HOME:-$HOME/.nomos}/bin/nomos" ]; then
    exec "${NOMOS_HOME:-$HOME/.nomos}/bin/nomos" hook post-tool
elif [ -x "${HOME}/.local/bin/nomos" ]; then
    exec "${HOME}/.local/bin/nomos" hook post-tool
fi

echo "{}"
