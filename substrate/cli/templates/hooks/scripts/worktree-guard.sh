#!/usr/bin/env bash
set -e

if [ -n "${GOBIN:-}" ] && [ -x "${GOBIN}/nomos" ]; then
    exec "${GOBIN}/nomos" hook pre-tool
elif [ -x "${HOME}/go/bin/nomos" ]; then
    exec "${HOME}/go/bin/nomos" hook pre-tool
elif command -v nomos >/dev/null 2>&1; then
    exec nomos hook pre-tool
elif [ -x "${NOMOS_HOME:-$HOME/.nomos}/bin/nomos" ]; then
    exec "${NOMOS_HOME:-$HOME/.nomos}/bin/nomos" hook pre-tool
elif [ -x "${HOME}/.local/bin/nomos" ]; then
    exec "${HOME}/.local/bin/nomos" hook pre-tool
fi

echo '{"decision": "allow"}'
