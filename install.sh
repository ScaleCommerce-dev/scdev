#!/bin/sh
set -e

REPO="ScaleCommerce-DEV/scdev"
BINARY="scdev"
BIN_DIR="$HOME/.scdev/bin"
SYMLINK_DIR="/usr/local/bin"

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
mkdir -p "$BIN_DIR"
curl -fsSL -o "$BIN_DIR/$BINARY" "$URL"
chmod +x "$BIN_DIR/$BINARY"

# Remove macOS quarantine attribute
if [ "$OS" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$BIN_DIR/$BINARY" 2>/dev/null || true
fi

# Symlink into a PATH location. The real binary stays in the user-owned
# $BIN_DIR so future `scdev self-update` runs never need sudo.
if [ -w "$SYMLINK_DIR" ]; then
  ln -sfn "$BIN_DIR/$BINARY" "$SYMLINK_DIR/$BINARY"
else
  echo "Creating symlink in ${SYMLINK_DIR} (requires sudo, one-time)..."
  sudo ln -sfn "$BIN_DIR/$BINARY" "$SYMLINK_DIR/$BINARY"
fi

echo "scdev installed to ${BIN_DIR}/${BINARY}"
echo "  symlinked from ${SYMLINK_DIR}/${BINARY}"
scdev version
