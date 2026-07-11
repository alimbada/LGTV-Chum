#!/bin/bash
set -e

# LGTV Chum Installer

INSTALL_DIR="${1:-$HOME/bin}"

# Expand path to absolute
mkdir -p "$INSTALL_DIR"
INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd)

echo "=== LGTV Chum Installation ==="
echo "Installing to: $INSTALL_DIR"
echo "Current user:  $USER"

# Check if INSTALL_DIR is in PATH
case ":$PATH:" in
  *:"$INSTALL_DIR":*) ;;
  *)
    echo "Warning: $INSTALL_DIR is not in your PATH environment variable."
    echo "You may need to add it to your shell configuration (e.g., ~/.bashrc or ~/.zshrc)."
    ;;
esac

# Check for LGWebOSRemote dependency
if [ ! -f "$HOME/lgtv-venv/bin/activate" ]; then
    echo "--------------------------------------------------------"
    echo "Warning: klattimer/LGWebOSRemote virtual environment not found at $HOME/lgtv-venv."
    echo "Please ensure you have installed it before using lgtv-chum."
    echo "Refer to the README.md for instructions."
    echo "--------------------------------------------------------"
fi

# Create a temporary directory for substitutions and build
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Preparing files..."

echo "Compiling Go binary..."
(cd src && go build -o "$TEMP_DIR/lgtv-chum" .)

# Copy binary and control script
echo "Copying binary and control script to $INSTALL_DIR..."
cp "$TEMP_DIR/lgtv-chum" "$INSTALL_DIR/lgtv-chum"
cp lgtv-control.sh "$INSTALL_DIR/lgtv-control.sh"
chmod +x "$INSTALL_DIR/lgtv-chum" "$INSTALL_DIR/lgtv-control.sh"

# Process service files
sed "s|{{BIN_DIR}}|$INSTALL_DIR|g; s|{{USER}}|$USER|g" lgtv-boot.service > "$TEMP_DIR/lgtv-boot.service"
sed "s|{{BIN_DIR}}|$INSTALL_DIR|g" lgtv-chum.service > "$TEMP_DIR/lgtv-chum.service"

# Install user service
USER_SERVICE_DIR="$HOME/.config/systemd/user"
mkdir -p "$USER_SERVICE_DIR"
echo "Installing user service to $USER_SERVICE_DIR..."
cp "$TEMP_DIR/lgtv-chum.service" "$USER_SERVICE_DIR/lgtv-chum.service"

echo "Enabling and starting user service..."
systemctl --user daemon-reload
systemctl --user enable --now lgtv-chum.service

# Install system service (requires sudo)
echo "Installing system-level boot/shutdown service (requires sudo privileges)..."
sudo cp "$TEMP_DIR/lgtv-boot.service" /etc/systemd/system/lgtv-boot.service
echo "Enabling system-level boot/shutdown service..."
sudo systemctl daemon-reload
sudo systemctl enable lgtv-boot.service

echo "=== LGTV Chum Installation Completed Successfully! ==="
