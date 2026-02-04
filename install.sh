#!/bin/bash
set -e

# sure-cli installer
# Usage: curl -sSL https://raw.githubusercontent.com/dgilperez/sure-cli/main/install.sh | bash

REPO="dgilperez/sure-cli"

# Default install dir: /usr/local/bin on macOS (always in PATH), ~/.local/bin elsewhere
if [ "$(uname -s)" = "Darwin" ]; then
  DEFAULT_DIR="/usr/local/bin"
else
  DEFAULT_DIR="$HOME/.local/bin"
fi
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_DIR}"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to get latest version"
  exit 1
fi

echo "Installing sure-cli $VERSION for $OS/$ARCH..."

# Download
EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"

FILENAME="sure-cli_${VERSION#v}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

echo "Downloading $URL..."
curl -sSL "$URL" -o "$TMP_DIR/$FILENAME"

# Extract
cd "$TMP_DIR"
if [ "$EXT" = "zip" ]; then
  unzip -q "$FILENAME"
else
  tar xzf "$FILENAME"
fi

# Install
mkdir -p "$INSTALL_DIR"
mv sure-cli "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/sure-cli"

echo ""
echo "✓ Installed sure-cli to $INSTALL_DIR/sure-cli"

# Check PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo ""
  echo "⚠ $INSTALL_DIR is not in your PATH"
  echo "  Add this to your shell config:"
  echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
fi

echo ""
echo "Run 'sure-cli --help' to get started"
