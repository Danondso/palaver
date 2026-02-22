#!/usr/bin/env bash
set -euo pipefail

echo "=== Palaver Installer ==="
echo

OS="$(uname -s)"

# Check for Go
if ! command -v go &>/dev/null; then
    echo "Error: Go is not installed. Install Go 1.25+ from https://go.dev/dl/"
    exit 1
fi

echo "Go: $(go version)"

# Install system dependencies
echo
if [ "$OS" = "Darwin" ]; then
    # macOS
    if ! command -v brew &>/dev/null; then
        echo "Error: Homebrew is not installed. Install from https://brew.sh"
        exit 1
    fi
    BREW_NEEDED=""
    if ! brew list portaudio &>/dev/null; then
        BREW_NEEDED="portaudio"
    else
        echo "portaudio already installed."
    fi
    if ! brew list whisper-cpp &>/dev/null; then
        BREW_NEEDED="$BREW_NEEDED whisper-cpp"
    else
        echo "whisper-cpp already installed."
    fi
    if [ -n "$BREW_NEEDED" ]; then
        echo "Installing$BREW_NEEDED via Homebrew..."
        brew install $BREW_NEEDED
    fi
else
    # Linux
    SESSION_TYPE="${XDG_SESSION_TYPE:-x11}"
    AUDIO_PKGS=""
    DISPLAY_PKGS=""

    if command -v apt &>/dev/null; then
        PKG_MGR="apt"
        AUDIO_PKGS="libportaudio2 portaudio19-dev"
        if [ "$SESSION_TYPE" = "wayland" ]; then
            DISPLAY_PKGS="ydotool wl-clipboard"
        else
            DISPLAY_PKGS="xdotool xclip"
        fi
    elif command -v dnf &>/dev/null; then
        PKG_MGR="dnf"
        AUDIO_PKGS="portaudio portaudio-devel"
        if [ "$SESSION_TYPE" = "wayland" ]; then
            DISPLAY_PKGS="ydotool wl-clipboard"
        else
            DISPLAY_PKGS="xdotool xclip"
        fi
    elif command -v pacman &>/dev/null; then
        PKG_MGR="pacman"
        AUDIO_PKGS="portaudio"
        if [ "$SESSION_TYPE" = "wayland" ]; then
            DISPLAY_PKGS="ydotool wl-clipboard"
        else
            DISPLAY_PKGS="xdotool xclip"
        fi
    else
        PKG_MGR=""
    fi

    if [ -z "$PKG_MGR" ]; then
        echo "Warning: Could not detect package manager. Please install manually:"
        echo "  - libportaudio2 / portaudio19-dev"
        echo "  - xdotool + xclip (X11) or ydotool + wl-clipboard (Wayland)"
    else
        ALL_PKGS="$AUDIO_PKGS $DISPLAY_PKGS"
        echo "The following system packages will be installed (session: $SESSION_TYPE):"
        echo "  $ALL_PKGS"
        echo
        echo "Installing system dependencies..."
        case "$PKG_MGR" in
            apt)    sudo apt install -y $ALL_PKGS ;;
            dnf)    sudo dnf install -y $ALL_PKGS ;;
            pacman) sudo pacman -S --noconfirm $ALL_PKGS ;;
        esac
    fi
fi

# Build palaver
echo
echo "Building palaver..."
INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"

# Clone or use existing repo
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

if [ -f "go.mod" ] && grep -q "palaver" go.mod 2>/dev/null; then
    echo "Building from current directory..."
    go build -o "${INSTALL_DIR}/palaver" ./cmd/palaver/
else
    echo "Cloning palaver..."
    git clone https://github.com/Danondso/palaver.git "$TMPDIR/palaver"
    cd "$TMPDIR/palaver"
    go build -o "${INSTALL_DIR}/palaver" ./cmd/palaver/
fi

echo "Installed palaver to ${INSTALL_DIR}/palaver"

# Add to PATH if needed
if ! echo "$PATH" | grep -q "${INSTALL_DIR}"; then
    echo
    echo "Note: ${INSTALL_DIR} is not in your PATH."
    echo "Add this to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\${HOME}/.local/bin:\${PATH}\""
fi

if [ "$OS" = "Darwin" ]; then
    # macOS: run setup (downloads whisper model)
    echo
    "${INSTALL_DIR}/palaver" setup

    echo
    echo "=== macOS Permissions Required ==="
    echo "Palaver needs these macOS permissions to work:"
    echo "  1. Microphone: Granted automatically on first run"
    echo "  2. Accessibility: System Settings > Privacy & Security > Accessibility"
    echo "     Add your terminal app (Terminal, iTerm2, etc.) to the list."
    echo "  3. Input Monitoring: System Settings > Privacy & Security > Input Monitoring"
    echo "     Required for global hotkey detection."
    echo
    echo "=== Installation Complete ==="
    echo "Run 'palaver' to start."
else
    # Linux: run setup to download parakeet and models
    echo
    echo "Running palaver setup (downloads parakeet binary, ONNX Runtime, and ~670MB of model files)..."
    "${INSTALL_DIR}/palaver" setup

    # Add user to input group for evdev hotkey access
    echo
    if ! groups | grep -q '\binput\b'; then
        echo "Adding $USER to the 'input' group (required for global hotkey via evdev)..."
        echo "  sudo usermod -aG input $USER"
        echo
        sudo usermod -aG input "$USER"
        echo "You will need to log out and back in for the group change to take effect."
    else
        echo "User is already in the 'input' group."
    fi

    echo
    echo "=== Installation Complete ==="
    echo "Run 'palaver' to start. (Log out and back in if input group was just added.)"
fi
