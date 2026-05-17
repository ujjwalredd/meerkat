#!/usr/bin/env bash
# Meerkat one-line installer (POSIX shells).
#
#   curl -fsSL https://cdn.jsdelivr.net/gh/ujjwalredd/meerkat@main/scripts/install.sh | bash
#
# Detects OS/arch, downloads the matching prebuilt binary from
# https://github.com/ujjwalredd/meerkat/releases, installs to
# $INSTALL_DIR (default /usr/local/bin if writable, else ~/.local/bin).
#
# Falls back to `go install` if no binary matches and Go >= 1.22 is present.
#
# Env overrides:
#   MEERKAT_VERSION=v0.3.0         pin a specific release
#   INSTALL_DIR=/path/to/bin       install location
#   MEERKAT_REPO=owner/name        for forks

set -euo pipefail

REPO="${MEERKAT_REPO:-ujjwalredd/meerkat}"
VERSION="${MEERKAT_VERSION:-latest}"

err() { printf 'meerkat-install: %s\n' "$*" >&2; exit 1; }
say() { printf '==> %s\n' "$*"; }

need() { command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"; }
need curl
need tar
need uname

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) err "unsupported arch: $ARCH" ;;
esac
case "$OS" in
  darwin|linux) ;;
  msys*|mingw*|cygwin*) OS=windows ;;
  *) err "unsupported OS: $OS (try the npm path: npx meerkat-cli@latest init)" ;;
esac

EXT=""
[ "$OS" = "windows" ] && EXT=".exe"
ASSET="meerkat-${OS}-${ARCH}${EXT}"

# Pick install dir
if [ -z "${INSTALL_DIR:-}" ]; then
  if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR=/usr/local/bin
  else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
  fi
fi

# Resolve version
if [ "$VERSION" = "latest" ]; then
  say "resolving latest release"
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || err "could not resolve latest version (set MEERKAT_VERSION=vX.Y.Z)"
fi
say "installing meerkat ${VERSION} for ${OS}/${ARCH}"

URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if curl -fsSL -o "$TMP/${ASSET}" "$URL" 2>/dev/null; then
  chmod +x "$TMP/${ASSET}"
  mv "$TMP/${ASSET}" "${INSTALL_DIR}/meerkat${EXT}"
  say "installed: ${INSTALL_DIR}/meerkat${EXT}"
else
  say "no prebuilt asset at ${URL}"
  if command -v go >/dev/null 2>&1; then
    say "falling back to: go install"
    GOBIN="$INSTALL_DIR" go install "github.com/${REPO#*/}/cmd/meerkat@${VERSION}" || \
    GOBIN="$INSTALL_DIR" go install "github.com/${REPO}/cmd/meerkat@${VERSION}"
    say "installed via go: ${INSTALL_DIR}/meerkat"
  else
    err "no prebuilt binary and Go not installed. Install Go 1.22+ or use: npx meerkat-cli@latest init"
  fi
fi

# PATH hint
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) say "add to your shell profile:  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac

"${INSTALL_DIR}/meerkat${EXT}" version || true
say "next: cd into a project and run:  meerkat init"
