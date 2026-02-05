#!/bin/bash
set -e

# sure-cli installer
# Usage: curl -sSL https://raw.githubusercontent.com/we-promise/sure-cli/main/install.sh | bash

REPO="we-promise/sure-cli"

# Default install dir with fallback
if [ -z "$INSTALL_DIR" ]; then
  if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi

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

# Detect shell profile and add to PATH if needed
detect_profile() {
  if [ -n "$PROFILE" ]; then
    echo "$PROFILE"
    return
  fi
  
  local DETECTED=""
  if [ -n "$ZSH_VERSION" ] || [ "$SHELL" = *"zsh"* ]; then
    if [ -f "$HOME/.zshrc" ]; then
      DETECTED="$HOME/.zshrc"
    fi
  elif [ -n "$BASH_VERSION" ] || [ "$SHELL" = *"bash"* ]; then
    if [ -f "$HOME/.bashrc" ]; then
      DETECTED="$HOME/.bashrc"
    elif [ -f "$HOME/.bash_profile" ]; then
      DETECTED="$HOME/.bash_profile"
    fi
  fi
  
  # Fallback detection
  if [ -z "$DETECTED" ]; then
    if [ -f "$HOME/.zshrc" ]; then
      DETECTED="$HOME/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
      DETECTED="$HOME/.bashrc"
    elif [ -f "$HOME/.bash_profile" ]; then
      DETECTED="$HOME/.bash_profile"
    elif [ -f "$HOME/.profile" ]; then
      DETECTED="$HOME/.profile"
    fi
  fi
  
  echo "$DETECTED"
}

# Check if PATH modification is needed
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  SHELL_PROFILE=$(detect_profile)
  PATH_LINE="export PATH=\"\$PATH:$INSTALL_DIR\""
  
  if [ -n "$SHELL_PROFILE" ]; then
    # Check if already in profile
    if ! grep -q "$INSTALL_DIR" "$SHELL_PROFILE" 2>/dev/null; then
      echo "" >> "$SHELL_PROFILE"
      echo "# Added by sure-cli installer" >> "$SHELL_PROFILE"
      echo "$PATH_LINE" >> "$SHELL_PROFILE"
      echo ""
      echo "✓ Added $INSTALL_DIR to PATH in $SHELL_PROFILE"
      echo ""
      echo "Restart your terminal or run:"
      echo "  source $SHELL_PROFILE"
    else
      echo ""
      echo "PATH already configured in $SHELL_PROFILE"
    fi
  else
    echo ""
    echo "⚠ Could not detect shell profile"
    echo "  Add this to your shell config manually:"
    echo "    $PATH_LINE"
  fi
fi

echo ""
echo "Run 'sure-cli --help' to get started"
