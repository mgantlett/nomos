#!/usr/bin/env bash
set -eu
(set -o pipefail) 2>/dev/null && set -o pipefail

# ==============================================================================
# Nomos: Zero-Friction Deterministic Engineering Substrate Installer
# Usage:
#   curl -fsSL https://sophialabs.ca/install | bash
#
# Environment Overrides:
#   NOMOS_INSTALL_DIR : Custom destination directory (defaults to GOBIN / GOPATH/bin)
#   NOMOS_VERSION     : Explicit release tag (e.g. v4.1.0)
#   NOMOS_LOCAL_TAR   : Local tar.gz archive for offline/testing validation
#   NOMOS_LOCAL_BIN   : Local binary path for zero-download testing validation
#   NOMOS_REPO        : GitHub repository (default: mgantlett/nomos)
# ==============================================================================

echo "🚀 Installing Nomos Deterministic Engineering Substrate..."

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

if [ "$ARCH" = "x86_64" ] || [ "$ARCH" = "amd64" ]; then
    ARCH="x86_64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ARCH="arm64"
else
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
fi

if [ "$OS" = "darwin" ]; then
    OS="Darwin"
elif [ "$OS" = "linux" ]; then
    OS="Linux"
else
    case "$OS" in
        mingw*|msys*|cygwin*)
            echo "❌ Native Windows shell detected ($OS)."
            echo "   Recommended on Windows: run this installer inside WSL2 (Ubuntu/Debian), or download precompiled Windows .zip binaries from:"
            echo "   https://github.com/mgantlett/nomos/releases/latest"
            exit 1
            ;;
    esac
    echo "❌ Unsupported OS: $OS"
    exit 1
fi

# 1. Determine destination directory (Standard Go bin SSoT first, then XDG user bin, then /usr/local/bin)
GO_BIN_CANDIDATE=""
if [ -n "${GOBIN:-}" ]; then
    GO_BIN_CANDIDATE="$GOBIN"
elif command -v go >/dev/null 2>&1; then
    GO_ENV_GOBIN="$(go env GOBIN 2>/dev/null || true)"
    GO_ENV_GOPATH="$(go env GOPATH 2>/dev/null || true)"
    if [ -n "$GO_ENV_GOBIN" ]; then
        GO_BIN_CANDIDATE="$GO_ENV_GOBIN"
    elif [ -n "$GO_ENV_GOPATH" ]; then
        GO_BIN_CANDIDATE="${GO_ENV_GOPATH%%:*}/bin"
    fi
elif [ -d "$HOME/go/bin" ]; then
    GO_BIN_CANDIDATE="$HOME/go/bin"
fi

if [ -n "${NOMOS_INSTALL_DIR:-}" ]; then
    INSTALL_DIR="$NOMOS_INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
    SUDO_CMD=""
elif [ -n "$GO_BIN_CANDIDATE" ] && mkdir -p "$GO_BIN_CANDIDATE" 2>/dev/null; then
    INSTALL_DIR="$GO_BIN_CANDIDATE"
    SUDO_CMD=""
elif [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    INSTALL_DIR="$HOME/.local/bin"
    SUDO_CMD=""
elif [ -d "$HOME/bin" ] || mkdir -p "$HOME/bin" 2>/dev/null; then
    INSTALL_DIR="$HOME/bin"
    SUDO_CMD=""
elif [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
    SUDO_CMD=""
elif command -v sudo >/dev/null 2>&1; then
    INSTALL_DIR="/usr/local/bin"
    SUDO_CMD="sudo"
else
    INSTALL_DIR="/usr/local/bin"
    SUDO_CMD=""
fi

verify_sha256_checksum() {
    local dir="$1"
    local tar_name="$2"
    local sums_file="$dir/checksums.txt"

    local expected
    expected="$(grep -E "[[:space:]]${tar_name}\$" "$sums_file" | awk '{print $1}' | head -n 1 || true)"
    if [ -z "$expected" ]; then
        echo "❌ Error: Archive '$tar_name' not found in checksums.txt."
        exit 1
    fi

    local actual=""
    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$dir/$tar_name" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "$dir/$tar_name" | awk '{print $1}')"
    elif command -v openssl >/dev/null 2>&1; then
        actual="$(openssl dgst -sha256 "$dir/$tar_name" | awk '{print $NF}')"
    else
        echo "❌ Error: No SHA-256 tool (sha256sum, shasum, openssl) available to verify archive."
        exit 1
    fi

    if [ "$actual" != "$expected" ]; then
        echo "❌ Error: SHA-256 checksum mismatch for $tar_name!"
        echo "   Expected: $expected"
        echo "   Actual  : $actual"
        exit 1
    fi
    echo "🔒 Verified SHA-256 checksum ($actual)"
}

# 2. Source resolution inside isolated mktemp directory
if [ -n "${NOMOS_LOCAL_BIN:-}" ] && [ -f "$NOMOS_LOCAL_BIN" ]; then
    echo "📦 Using local nomos binary: $NOMOS_LOCAL_BIN"
    $SUDO_CMD cp "$NOMOS_LOCAL_BIN" "$INSTALL_DIR/nomos"
    $SUDO_CMD chmod +x "$INSTALL_DIR/nomos"
else
    TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'nomos-install')"
    trap 'rm -rf "$TMP_DIR"' EXIT

    TAR_FILE="nomos_${OS}_${ARCH}.tar.gz"
    if [ -n "${NOMOS_LOCAL_TAR:-}" ] && [ -f "$NOMOS_LOCAL_TAR" ]; then
        echo "📦 Using local release archive: $NOMOS_LOCAL_TAR"
        cp "$NOMOS_LOCAL_TAR" "$TMP_DIR/$TAR_FILE"
    else
        REPO="${NOMOS_REPO:-mgantlett/nomos}"
        LATEST_RELEASE="${NOMOS_VERSION:-}"
        if [ -z "$LATEST_RELEASE" ]; then
            EFFECTIVE_URL="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"
            case "$EFFECTIVE_URL" in
                */releases/tag/*)
                    LATEST_RELEASE="${EFFECTIVE_URL##*/releases/tag/}"
                    LATEST_RELEASE="${LATEST_RELEASE%%\?*}"
                    LATEST_RELEASE="${LATEST_RELEASE%%/}"
                    ;;
            esac
        fi
        if [ -z "$LATEST_RELEASE" ]; then
            LATEST_RELEASE="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/' || true)"
        fi

        if [ -z "$LATEST_RELEASE" ]; then
            echo "❌ Error: Could not determine release version (GitHub API rate limit or no network)."
            echo "   Provide an explicit release version via NOMOS_VERSION=v4.1.0 or local tarball via NOMOS_LOCAL_TAR."
            exit 1
        fi

        BASE_URL="https://github.com/${REPO}/releases/download/${LATEST_RELEASE}"
        DOWNLOAD_URL="${BASE_URL}/${TAR_FILE}"
        CHECKSUMS_URL="${BASE_URL}/checksums.txt"

        echo "🌐 Downloading $DOWNLOAD_URL..."
        if ! curl -fsSL -o "$TMP_DIR/$TAR_FILE" "$DOWNLOAD_URL"; then
            echo "❌ Error: Failed to download release archive ($DOWNLOAD_URL)."
            exit 1
        fi
        if ! curl -fsSL -o "$TMP_DIR/checksums.txt" "$CHECKSUMS_URL"; then
            echo "❌ Error: Failed to download release checksums ($CHECKSUMS_URL)."
            exit 1
        fi
        verify_sha256_checksum "$TMP_DIR" "$TAR_FILE"
    fi

    echo "📂 Extracting..."
    tar -xzf "$TMP_DIR/$TAR_FILE" -C "$TMP_DIR" nomos
    echo "📦 Installing nomos to $INSTALL_DIR..."
    $SUDO_CMD mv "$TMP_DIR/nomos" "$INSTALL_DIR/nomos"
    $SUDO_CMD chmod +x "$INSTALL_DIR/nomos"
fi

# 3. Alpine Linux / musl libc fallback check (only if binary is dynamically linked against glibc)
if [ "$OS" = "Linux" ] && ! "$INSTALL_DIR/nomos" version >/dev/null 2>&1; then
    if ldd --version 2>&1 | grep -qi "musl" || [ -f /lib/ld-musl-x86_64.so.1 ] || [ -f /lib/ld-musl-aarch64.so.1 ]; then
        if ! command -v gcompat >/dev/null 2>&1 && [ ! -f /lib/ld-linux-x86-64.so.2 ] && [ ! -f /lib/ld-linux-aarch64.so.1 ]; then
            echo "ℹ️  Note: Alpine Linux / musl libc detected. If required, install compatibility libraries: 'apk add gcompat libstdc++'"
        fi
    fi
fi

# 4. Shell PATH check
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo ""
        echo "⚠️  Note: $INSTALL_DIR is not in your \$PATH."
        echo "   To make 'nomos' globally accessible in your shell, run:"
        if [ -n "${ZSH_VERSION:-}" ] || [ -f "$HOME/.zshrc" ]; then
            echo "     echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.zshrc && export PATH=\"$INSTALL_DIR:\$PATH\""
        else
            echo "     echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.bashrc && export PATH=\"$INSTALL_DIR:\$PATH\""
        fi
        echo ""
        ;;
esac

# 5. Verification
if "$INSTALL_DIR/nomos" version >/dev/null 2>&1; then
    VERSION_STR="$("$INSTALL_DIR/nomos" version 2>/dev/null | head -n 1 || echo "nomos")"
    echo "✨ Installation verified: $VERSION_STR ($INSTALL_DIR/nomos)"
else
    echo "✨ Installation complete! ($INSTALL_DIR/nomos)"
fi

echo ""
echo "Next steps:"
echo "  1. Run 'nomos init' in your repository to configure workspace governance."
echo "  2. Prompt your coding agent naturally (or run 'nomos handshake' to inspect state)."
