#!/usr/bin/env bash
set -euo pipefail

echo "=== Palaver Uninstaller ==="
echo

OS="$(uname -s)"

# Stop any running palaver processes
if pgrep -x palaver &>/dev/null; then
    echo "Stopping running palaver processes..."
    pkill -x palaver || true
    sleep 1
fi

# Stop any running parakeet server
if pgrep -f "parakeet" &>/dev/null; then
    echo "Stopping running parakeet server..."
    pkill -f "parakeet" || true
    sleep 1
fi

# Remove binary
INSTALL_DIR="${HOME}/.local/bin"
if [ -f "${INSTALL_DIR}/palaver" ]; then
    echo "Removing ${INSTALL_DIR}/palaver..."
    rm -f "${INSTALL_DIR}/palaver"
else
    echo "Binary not found at ${INSTALL_DIR}/palaver (already removed or installed elsewhere)."
fi

# Remove data directory (managed server, models, ONNX runtime)
DATA_DIR="${HOME}/.local/share/palaver"
if [ -d "$DATA_DIR" ]; then
    echo "Removing data directory ${DATA_DIR}..."
    rm -rf "$DATA_DIR"
else
    echo "Data directory not found at ${DATA_DIR} (already removed)."
fi

# Remove config
CONFIG_DIR="${HOME}/.config/palaver"
if [ -d "$CONFIG_DIR" ]; then
    echo
    read -rp "Remove config directory ${CONFIG_DIR}? [y/N] " answer
    if [[ "$answer" =~ ^[Yy]$ ]]; then
        rm -rf "$CONFIG_DIR"
        echo "Removed ${CONFIG_DIR}."
    else
        echo "Kept ${CONFIG_DIR}."
    fi
else
    echo "Config directory not found at ${CONFIG_DIR} (already removed)."
fi

echo
echo "=== Uninstall Complete ==="
echo "Note: System packages were not removed. Remove them manually if no longer needed:"
if [ "$OS" = "Darwin" ]; then
    echo "  Audio: brew uninstall portaudio"
else
    echo "  Audio:          libportaudio2, portaudio19-dev (apt) / portaudio, portaudio-devel (dnf) / portaudio (pacman)"
    echo "  X11 clipboard:  xdotool, xclip"
    echo "  Wayland clipboard: ydotool, wl-clipboard"
fi
