#!/bin/bash
set -e

echo "=== TimeReg CLI Installer ==="
echo ""

# Determine install directory
INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/timereg"
REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed."
    echo "Install it from https://go.dev/doc/install"
    exit 1
fi

echo "Go found: $(go version)"
echo ""

# Check for systray dependencies
SYSTRAY_TAGS=""
if pkg-config --exists ayatana-appindicator3-0.1 2>/dev/null && pkg-config --exists gtk+-3.0 2>/dev/null; then
    echo "Systray dependencies found — building with systray support"
    SYSTRAY_TAGS="-tags systray"
else
    echo "Systray dependencies not found — building CLI only"
    echo "  To enable systray, run:"
    echo "    sudo apt install libayatana-appindicator3-dev libgtk-3-dev"
    echo "  Then re-run this script."
fi
echo ""

# Build
echo "Building timereg..."
cd "$REPO_DIR"
go build $SYSTRAY_TAGS -o "$INSTALL_DIR/timereg" ./cmd/timereg/
echo "Installed to $INSTALL_DIR/timereg"
echo ""

# Ensure ~/.local/bin is on PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    SHELL_RC=""
    if [ -f "$HOME/.bashrc" ]; then
        SHELL_RC="$HOME/.bashrc"
    elif [ -f "$HOME/.zshrc" ]; then
        SHELL_RC="$HOME/.zshrc"
    fi

    if [ -n "$SHELL_RC" ]; then
        if ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' "$SHELL_RC" 2>/dev/null; then
            echo '' >> "$SHELL_RC"
            echo '# TimeReg CLI' >> "$SHELL_RC"
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
            echo "Added $INSTALL_DIR to PATH in $SHELL_RC"
        fi
    fi

    export PATH="$INSTALL_DIR:$PATH"
fi

echo ""
echo "=== Installation complete ==="
echo ""
echo "Available commands:"
echo "  timereg              Interactive menu"
echo "  timereg guide        Setup guide for Google API"
echo "  timereg guide status Check configuration status"
echo "  timereg -h           Show all commands"
echo ""

# Check if Google is configured
if [ ! -f "$CONFIG_DIR/credentials.json" ]; then
    echo "Next step: Set up Google integration"
    echo "  Run: timereg guide"
else
    echo "Google credentials found."
    if [ ! -f "$CONFIG_DIR/token.json" ]; then
        echo "Next step: Authenticate with Google"
        echo "  Run: timereg auth"
    else
        echo "Already authenticated. Run 'timereg' to start."
    fi
fi
