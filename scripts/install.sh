#!/usr/bin/env bash
# Meerkat one-line installer (POSIX shells).
#
#   curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | bash
#
# Detects OS/arch, downloads the matching prebuilt binary from
# https://github.com/ujjwalredd/meerkat/releases, installs to
# $INSTALL_DIR (default /usr/local/bin if writable, else ~/.local/bin).
#
# Falls back to `go install` if no binary matches and Go >= 1.22 is present.
#
# Env overrides:
#   MEERKAT_VERSION=v0.4.1         pin a specific release
#   INSTALL_DIR=/path/to/bin       install location
#   MEERKAT_REPO=owner/name        for forks
#   MEERKAT_SETUP_CLAUDE=1         also install Claude Code /meerkat hooks
#   MEERKAT_REQUIRE_CHECKSUM=1     fail if checksums.txt is unavailable
#   MEERKAT_INSTALL_NO_GO_FALLBACK=1
#                                   fail instead of falling back to go install

set -euo pipefail

REPO="${MEERKAT_REPO:-ujjwalredd/meerkat}"
VERSION="${MEERKAT_VERSION:-latest}"
SETUP_CLAUDE="${MEERKAT_SETUP_CLAUDE:-0}"
REQUIRE_CHECKSUM="${MEERKAT_REQUIRE_CHECKSUM:-0}"
NO_GO_FALLBACK="${MEERKAT_INSTALL_NO_GO_FALLBACK:-0}"

err() { printf 'meerkat-install: %s\n' "$*" >&2; exit 1; }
say() { printf '==> %s\n' "$*"; }
warn() { printf 'meerkat-install: warning: %s\n' "$*" >&2; }

need() { command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"; }
need curl
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
  fi
fi
mkdir -p "$INSTALL_DIR"

install_via_go() {
  local ver="$1"
  if [ "$NO_GO_FALLBACK" = "1" ] || [ "$NO_GO_FALLBACK" = "true" ]; then
    err "no prebuilt binary available and MEERKAT_INSTALL_NO_GO_FALLBACK=1"
  fi
  command -v go >/dev/null 2>&1 || err "no prebuilt binary and Go not installed.
Install Go 1.22+ (https://go.dev/dl) and rerun, or use the npm path:
  npx meerkat-cli@latest init wizard"
  say "falling back to: go install (ref=${ver})"
  GOBIN="$INSTALL_DIR" go install "github.com/${REPO}/cmd/meerkat@${ver}" \
    || err "go install failed for ref ${ver}"
  say "installed via go: ${INSTALL_DIR}/meerkat${EXT}"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return 0
  fi
  return 1
}

verify_checksum() {
  local file="$1"
  local checksums="$TMP/checksums.txt"
  local checksums_url="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

  if ! curl -fsSL -o "$checksums" "$checksums_url" 2>/dev/null; then
    if [ "$REQUIRE_CHECKSUM" = "1" ] || [ "$REQUIRE_CHECKSUM" = "true" ]; then
      err "checksums.txt unavailable at ${checksums_url}"
    fi
    warn "checksums.txt unavailable for ${VERSION}; continuing without checksum verification"
    return 0
  fi

  local expected actual
  expected=$(awk -v asset="$ASSET" '$2 == asset {print $1}' "$checksums" | head -n1)
  if [ -z "$expected" ]; then
    err "checksums.txt does not contain an entry for ${ASSET}"
  fi
  actual=$(sha256_file "$file") || err "missing sha256sum or shasum for checksum verification"
  if [ "$actual" != "$expected" ]; then
    err "checksum mismatch for ${ASSET}: expected ${expected}, got ${actual}"
  fi
  say "verified checksum for ${ASSET}"
}

finish_install() {
  "${INSTALL_DIR}/meerkat${EXT}" version || true

  case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) say "add to your shell profile:  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
  esac

  if [ "$SETUP_CLAUDE" = "1" ] || [ "$SETUP_CLAUDE" = "true" ]; then
    say "configuring Claude Code hooks + /meerkat command"
    "${INSTALL_DIR}/meerkat${EXT}" claude install \
      || err "Claude Code setup failed. The CLI is installed; run 'meerkat claude install' manually after checking the error above."
  else
    say "Claude Code optional setup:  meerkat claude install"
    say "one-command install + Claude setup:  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | MEERKAT_SETUP_CLAUDE=1 bash"
  fi

  say "project setup:  cd into a repo and run:  meerkat init --profile=agent"
  say "daily use:      /meerkat <task> in Claude Code, or: meerkat run -- <command>"
}

# Resolve version. No releases yet -> jump straight to go install of main.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if [ "$VERSION" = "latest" ]; then
  say "resolving latest release"
  HTTP_CODE=$(curl -fsSL -o "$TMP/rel.json" -w '%{http_code}' \
              "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || echo "000")
  if [ "$HTTP_CODE" = "200" ]; then
    VERSION=$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$TMP/rel.json" | head -n1)
  else
    VERSION=""
  fi
  if [ -z "$VERSION" ]; then
    say "no published releases for ${REPO} yet (HTTP ${HTTP_CODE})"
    install_via_go "latest"
    finish_install
    exit 0
  fi
else
  case "$VERSION" in
    v*) ;;
    *) VERSION="v${VERSION}" ;;
  esac
fi
say "installing meerkat ${VERSION} for ${OS}/${ARCH}"

URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

if curl -fsSL -o "$TMP/${ASSET}" "$URL" 2>/dev/null; then
  verify_checksum "$TMP/${ASSET}"
  chmod +x "$TMP/${ASSET}"
  mv "$TMP/${ASSET}" "${INSTALL_DIR}/meerkat${EXT}"
  say "installed: ${INSTALL_DIR}/meerkat${EXT}"
else
  say "no prebuilt asset at ${URL}"
  install_via_go "${VERSION}"
fi

finish_install
