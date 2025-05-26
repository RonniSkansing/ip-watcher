#!/bin/bash

# IP Watcher Installation Script
# This script installs the IP Watcher as a systemd service

set -e

# Define variables
SERVICE_NAME="ip-watcher"
BINARY_PATH="/usr/local/bin/ip-watcher"
CONFIG_DIR="/etc/ip-watcher"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
DEFAULT_INTERVAL=60 # 1 minute

# Make sure script is run as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root" >&2
    exit 1
fi

echo "Installing IP Watcher..."

# Create necessary directories
mkdir -p "${CONFIG_DIR}"

# Build and copy the binary
if [ -f "./ip-watcher" ]; then
    cp ./ip-watcher "${BINARY_PATH}"
    chmod +x "${BINARY_PATH}"
else
    echo "Building IP Watcher from source..."
    go build -o "${BINARY_PATH}"
    chmod +x "${BINARY_PATH}"
fi

# Create systemd service file
cat > "${SERVICE_FILE}" << EOF
[Unit]
Description=External IP Address Watcher
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${BINARY_PATH} -interval ${DEFAULT_INTERVAL}
Restart=on-failure
RestartSec=60
User=root

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd, enable and start service
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl start "${SERVICE_NAME}"

echo "IP Watcher has been installed and started as a systemd service."
echo "Default check interval: ${DEFAULT_INTERVAL} seconds (1 minute)"
echo ""
echo "You can check the service status with: systemctl status ${SERVICE_NAME}"
echo "You can view logs with: journalctl -u ${SERVICE_NAME}"
echo ""
echo "To customize the settings, stop the service, modify ${SERVICE_FILE},"
echo "then reload systemd and restart the service."
