#!/bin/bash
set -e

# Constellation Installation Script for Ubuntu

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (use sudo)"
  exit 1
fi

echo "Installing Constellation..."

# 1. Copy binaries and assets
mkdir -p /opt/constellation
cp -r bin /opt/constellation/
cp -r dashboard /opt/constellation/
cp -r certs /opt/constellation/

# Create data directory
mkdir -p /var/lib/constellation
chmod 755 /var/lib/constellation

# 2. Add CLI to PATH
ln -sf /opt/constellation/bin/constellation /usr/local/bin/constellation

# 3. Setup Systemd services
echo "Setting up systemd services..."
cp systemd/constellation-controller.service /etc/systemd/system/
cp systemd/constellation-agent.service /etc/systemd/system/

systemctl daemon-reload

echo ""
echo "Installation complete!"
echo "To start the Controller:"
echo "  sudo systemctl enable --now constellation-controller"
echo ""
echo "To start a Node Agent:"
echo "  sudo systemctl enable --now constellation-agent"
echo ""
echo "Use 'constellation login' to authenticate on this machine."
