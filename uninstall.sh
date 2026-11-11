#!/usr/bin/env bash
set -eu
(set -o pipefail) 2>/dev/null && set -o pipefail

# ==============================================================================
# Nomos: Deterministic Engineering Substrate Uninstaller (SSoT)
# Usage:
#   curl -fsSL https://sophialabs.ca/uninstall | bash
#
# Environment Overrides:
#   NOMOS_INSTALL_DIR : Custom installation directory to remove nomos from
#   NOMOS_HOME        : Custom global state directory (defaults to ~/.nomos)
#   NOMOS_PURGE_DATA  : Set to 1 to also remove global state (nomos_global.db, mneme.db)
# ==============================================================================

echo "🧹 Uninstalling Nomos Deterministic Engineering Substrate..."

GO_ENV_GOBIN=""
GO_ENV_GOPATH=""
if command -v go >/dev/null 2>&1; then
    GO_ENV_GOBIN="$(go env GOBIN 2>/dev/null || true)"
    GO_ENV_GOPATH="$(go env GOPATH 2>/dev/null || true)"
fi

REMOVED_COUNT=0
SEEN_DIRS=""
for dir in \
    "${NOMOS_INSTALL_DIR:-}" \
    "${GOBIN:-}" \
    "$GO_ENV_GOBIN" \
    "${GO_ENV_GOPATH:+${GO_ENV_GOPATH%%:*}/bin}" \
    "$HOME/go/bin" \
    "$HOME/.local/bin" \
    "$HOME/bin" \
    "/usr/local/bin" \
    "$HOME/.nomos/bin"; do
    [ -n "$dir" ] || continue
    case "
$SEEN_DIRS
" in
        *"
$dir
"*) continue ;;
    esac
    SEEN_DIRS="${SEEN_DIRS}
${dir}"
    BIN_PATH="$dir/nomos"
    if [ -f "$BIN_PATH" ]; then
        if [ -w "$BIN_PATH" ] || [ -w "$dir" ]; then
            rm -f "$BIN_PATH"
        elif command -v sudo >/dev/null 2>&1; then
            sudo rm -f "$BIN_PATH"
        else
            echo "⚠️  Cannot remove $BIN_PATH (insufficient permissions)."
            continue
        fi
        echo "✓ Removed binary: $BIN_PATH"
        REMOVED_COUNT=$((REMOVED_COUNT + 1))
    fi
done

if [ "$REMOVED_COUNT" -eq 0 ]; then
    echo "ℹ️  No installed nomos binary found in standard PATH/GOBIN locations."
fi

DATA_DIR="${NOMOS_HOME:-$HOME/.nomos}"
if [ "${NOMOS_PURGE_DATA:-0}" = "1" ]; then
    if [ -d "$DATA_DIR" ]; then
        rm -rf "$DATA_DIR"
        echo "✓ Purged global state directory: $DATA_DIR"
    fi
else
    if [ -d "$DATA_DIR" ]; then
        echo ""
        echo "📁 Preserved global state & Nomos Mneme memory at: $DATA_DIR"
        echo "   To permanently purge global SQLite state, re-run with NOMOS_PURGE_DATA=1 or run:"
        echo "     rm -rf \"$DATA_DIR\""
    fi
fi

echo ""
echo "✨ Nomos uninstall complete."
