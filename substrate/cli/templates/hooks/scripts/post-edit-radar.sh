#!/usr/bin/env bash
set -e

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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ALERT_FILE="${WORKSPACE_DIR}/.tmp/nomos_radar_alert.txt"

IS_POST_INVOCATION=$(printf '%s' "$PAYLOAD" | jq -r 'has("invocationNum")' 2>/dev/null || echo "false")
if [[ "$IS_POST_INVOCATION" == "true" ]]; then
    if [[ -f "$ALERT_FILE" ]]; then
        ALERT_MSG=$(cat "$ALERT_FILE" || true)
        rm -f "$ALERT_FILE"
        if [[ -n "$ALERT_MSG" ]]; then
            jq -Rn --arg msg "$ALERT_MSG" '{injectSteps: [{ephemeralMessage: $msg}]}'
            exit 0
        fi
    fi
    echo "{}"
    exit 0
fi

TARGET_FILE=$(printf '%s' "$PAYLOAD" | jq -r '.toolCall.args.TargetFile // .toolCall.args.Path // .tool_input.file_path // .tool_input.path // .args.file_path // .args.path // empty' 2>/dev/null || true)
if [[ -z "$TARGET_FILE" || "$TARGET_FILE" == "null" ]]; then
    echo "{}"
    exit 0
fi

if [[ "$TARGET_FILE" != /* ]]; then
    BASE_CWD=$(printf '%s' "$PAYLOAD" | jq -r '.cwd // empty' 2>/dev/null || true)
    if [[ -z "$BASE_CWD" ]]; then
        BASE_CWD="$WORKSPACE_DIR"
    fi
    TARGET_FILE="${BASE_CWD}/${TARGET_FILE}"
fi

if [[ "$TARGET_FILE" != *"/worktrees/"* ]]; then
    echo "{}"
    exit 0
fi

RADAR_OUT=$("$NOMOS_BIN" verify --fast-file "$TARGET_FILE" 2>/dev/null || true)
if [[ -z "$RADAR_OUT" ]]; then
    echo "{}"
    exit 0
fi

HAS_TOOL_INPUT=$(printf '%s' "$PAYLOAD" | jq -r 'has("tool_input")' 2>/dev/null || echo "false")
if [[ "$HAS_TOOL_INPUT" == "true" ]]; then
    jq -Rn --arg ctx "$RADAR_OUT" '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $ctx}}'
    exit 0
fi

mkdir -p "${WORKSPACE_DIR}/.tmp"
printf '%s\n' "$RADAR_OUT" >> "$ALERT_FILE"
echo "{}"

