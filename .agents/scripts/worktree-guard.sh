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

normalize_path() {
    local raw="$1"
    local base="$2"
    if [[ "$raw" != /* ]]; then
        raw="${base}/${raw}"
    fi
    local IFS='/'
    local parts=()
    local resolved=()
    read -ra parts <<< "$raw"
    for part in "${parts[@]}"; do
        if [[ -z "$part" || "$part" == "." ]]; then
            continue
        elif [[ "$part" == ".." ]]; then
            if (( ${#resolved[@]} > 0 )); then
                resolved=("${resolved[@]:0:${#resolved[@]}-1}")
            fi
        else
            resolved+=("$part")
        fi
    done
    local out=""
    for seg in "${resolved[@]}"; do
        out="${out}/${seg}"
    done
    if [[ -z "$out" ]]; then
        out="/"
    fi
    printf '%s' "$out"
}

TARGET_FILE=$(printf '%s' "$PAYLOAD" | jq -r '.toolCall.args.TargetFile // .toolCall.args.Path // .tool_input.file_path // .tool_input.path // .args.file_path // .args.path // empty' 2>/dev/null || true)
TOOL_NAME=$(printf '%s' "$PAYLOAD" | jq -r '.toolCall.name // .tool_name // "file_mutation"' 2>/dev/null || true)
BASE_CWD=$(printf '%s' "$PAYLOAD" | jq -r '.cwd // empty' 2>/dev/null || true)
if [[ -z "$BASE_CWD" ]]; then
    BASE_CWD="$(pwd)"
fi

if [[ -n "$TARGET_FILE" && "$TARGET_FILE" != "null" ]]; then
    if [[ "$TARGET_FILE" == "~"* || "$TARGET_FILE" == *"$"* ]]; then
        REASON="Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '${TARGET_FILE}'. Pass a canonical resolved path."
        jq -Rn --arg reason "$REASON" '{decision: "deny", reason: $reason}'
        exit 0
    fi
    RESOLVED_FILE="$(normalize_path "$TARGET_FILE" "$BASE_CWD")"
    if [[ "$RESOLVED_FILE" != *"/worktrees/"* && "$RESOLVED_FILE" != *"/.tmp/"* && "$RESOLVED_FILE" != *"/.gemini/"* && "$RESOLVED_FILE" != *"/.gemini" ]]; then
        CHORE_HINT="nomos task chore \"<title>\""
        if [[ "$RESOLVED_FILE" == *"/intent/"* || "$RESOLVED_FILE" == *"/nomos.yaml" ]]; then
            CHORE_HINT="nomos task chore \"<title>\" --plane intent"
        fi
        REASON="Hermetic Worktree Violation (Law 3): Direct trunk mutation of '${TARGET_FILE}' (resolved: '${RESOLVED_FILE}') via '${TOOL_NAME}' is prohibited. Provision an isolated EDIT worktree in 1 turn via '${CHORE_HINT}' (or 'nomos task start <KEY> --edit'), then edit strictly inside '/worktrees/'."
        jq -Rn --arg reason "$REASON" '{decision: "deny", reason: $reason}'
        exit 0
    fi
fi

echo '{"decision": "allow"}'
