#!/bin/sh
set -eu

REPO="fireynis/ralph-loop-tui"
PROJECT="ralph-loop-tui"
BINARY="ralph-loop-go"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest release tag
TAG="$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d '"' -f 4)"
VERSION="${TAG#v}"

echo "Installing ${BINARY} ${TAG} (${OS}/${ARCH})..."

# Build download URL
ARCHIVE="${PROJECT}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

# Create temp dir
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# Download archive and checksums
curl -sSfL -o "${TMP_DIR}/${ARCHIVE}" "$URL"
curl -sSfL -o "${TMP_DIR}/checksums.txt" "$CHECKSUMS_URL"

# Verify checksum
cd "$TMP_DIR"
if command -v sha256sum >/dev/null 2>&1; then
  grep "$ARCHIVE" checksums.txt | sha256sum -c --quiet
elif command -v shasum >/dev/null 2>&1; then
  grep "$ARCHIVE" checksums.txt | shasum -a 256 -c --quiet
else
  echo "Warning: no sha256sum or shasum found, skipping checksum verification" >&2
fi

# Extract
tar xzf "$ARCHIVE"

# Install
if [ -w /usr/local/bin ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ -d "${HOME}/.local/bin" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
else
  mkdir -p "${HOME}/.local/bin"
  INSTALL_DIR="${HOME}/.local/bin"
fi

mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

# Check if install dir is in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "Note: ${INSTALL_DIR} is not in your PATH. Add it with:" >&2
     echo "  export PATH=\"${INSTALL_DIR}:\$PATH\"" >&2 ;;
esac
