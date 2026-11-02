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
fi

if [ -n "$NOMOS_BIN" ] && "$NOMOS_BIN" hook --help >/dev/null 2>&1; then
    printf '%s' "$PAYLOAD" | "$NOMOS_BIN" hook pre-tool
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    echo '{"decision": "allow"}'
    exit 0
fi

COMMAND_LINE=$(printf '%s' "$PAYLOAD" | jq -r '.toolCall.args.CommandLine // .tool_input.command // .args.command // empty' 2>/dev/null || true)

deny_command() {
    jq -Rn --arg reason "$1" '{decision: "deny", reason: $reason}'
    exit 0
}

if [[ -n "$COMMAND_LINE" && "$COMMAND_LINE" != "null" ]]; then
    if [[ "$COMMAND_LINE" =~ rm[[:space:]]+(-[a-zA-Z]*r[a-zA-Z]*f[a-zA-Z]*|-[a-zA-Z]*f[a-zA-Z]*r[a-zA-Z]*|--recursive)[[:space:]]+(/|\*+|~|\.\.)([[:space:]]|$|/) ]]; then
        deny_command "❌ [NOMOS INTERCEPT] Wildcard or root recursive deletion ('rm -rf /', 'rm -rf *', 'rm -rf ~', 'rm -rf ..') is strictly prohibited (Law 7)."
    fi

    if [[ "$COMMAND_LINE" == *"go install"* && "$COMMAND_LINE" == *"nomos"* ]]; then
        deny_command "❌ [NOMOS INTERCEPT] Manual binary installation ('go install' of nomos) during active task sessions is prohibited by Law 4 (Single Source of Truth). Invoke the installed 'nomos' CLI binary directly."
    fi

    if [[ "$COMMAND_LINE" == *"sqlite3"* ]] && ([[ "$COMMAND_LINE" == *".nomos"* ]] || [[ "$COMMAND_LINE" == *".db"* ]]); then
        deny_command "❌ [NOMOS INTERCEPT] Direct SQLite access prohibited (Law 1 & ADR-0062). Use canonical CLI commands: 'nomos mem recall', 'nomos mem list', or 'nomos task show'."
    fi

    if [[ "$COMMAND_LINE" == *"git stash"* ]] && ! [[ "$COMMAND_LINE" == *"git stash clear"* || "$COMMAND_LINE" == *"git stash list"* ]]; then
        deny_command "❌ [NOMOS INTERCEPT] 'git stash' is prohibited (Law 2 & Law 3). Use 'nomos task chore \"<title>\"' or 'git restore <file>'."
    fi

    if [[ "$COMMAND_LINE" == *".git/hooks"* ]] && ([[ "$COMMAND_LINE" == *"rm "* ]] || [[ "$COMMAND_LINE" == *"unlink "* ]] || [[ "$COMMAND_LINE" == *"chmod "* ]] || [[ "$COMMAND_LINE" == *"mv "* ]]); then
        deny_command "❌ [NOMOS INTERCEPT] Tampering with CapBAC Git defense hooks (.git/hooks) is strictly prohibited (Law 3 & Law 7)."
    fi
fi

echo '{"decision": "allow"}'
