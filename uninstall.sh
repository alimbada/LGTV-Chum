#!/bin/bash
# LGTV Chum Uninstaller

INSTALL_DIR="${1:-$HOME/bin}"

# Expand path to absolute
if [ -d "$INSTALL_DIR" ]; then
    INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd)
else
    echo "Warning: Installation directory $INSTALL_DIR does not exist."
fi

echo "=== LGTV Chum Uninstall ==="
echo "Removing from: $INSTALL_DIR"

echo "Stopping and disabling user service..."
systemctl --user disable --now lgtv-chum.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/lgtv-chum.service"
systemctl --user daemon-reload

echo "Stopping and disabling system service (requires sudo privileges)..."
sudo systemctl disable lgtv-boot.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/lgtv-boot.service
sudo systemctl daemon-reload

echo "Removing binary and control script..."
rm -f "$INSTALL_DIR/lgtv-chum"
rm -f "$INSTALL_DIR/lgtv-control.sh"

echo "=== LGTV Chum Uninstall Completed Successfully! ==="
