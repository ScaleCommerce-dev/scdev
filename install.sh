#!/bin/sh
set -e

REPO="ScaleCommerce-DEV/scdev"
BINARY="scdev"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64)  ARCH="arm64" ;;
  *)              echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Check for Docker
if ! command -v docker >/dev/null 2>&1; then
  echo "Warning: Docker is not installed. scdev requires Docker to run."
fi

URL="https://github.com/${REPO}/releases/latest/download/${BINARY}-${OS}-${ARCH}"

echo "Downloading scdev for ${OS}/${ARCH}..."
curl -fsSL -o "$BINARY" "$URL"
chmod +x "$BINARY"

# Remove macOS quarantine attribute
if [ "$OS" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$BINARY" 2>/dev/null || true
fi

if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "scdev installed to ${INSTALL_DIR}/${BINARY}"
scdev version
