#!/bin/bash
set -e

echo "=================================================="
echo "      ✦ Constellation Worker Setup ✦"
echo "=================================================="

if [ "$EUID" -eq 0 ]; then
  echo "Please DO NOT run this script directly as root."
  echo "Run as your normal user, and the script will use sudo when needed."
  exit 1
fi

echo "Please enter the details provided by your Controller:"
read -p "Controller IP Address (e.g., 100.103.238.127): " CONTROLLER_IP
read -p "Cluster Join Token (e.g., cst_abc123): " JOIN_TOKEN

if [ -z "$CONTROLLER_IP" ] || [ -z "$JOIN_TOKEN" ]; then
    echo "Error: Controller IP and Join Token are required."
    exit 1
fi

echo ""
echo "[1/4] Installing dependencies (requires sudo)..."
sudo apt-get update
sudo apt-get install -y curl build-essential pkg-config libssl-dev

echo "[2/4] Setting up Rust Toolchain..."
if ! command -v cargo &> /dev/null; then
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
    source "$HOME/.cargo/env"
fi

# Ensure cargo is in PATH for the rest of the script
export PATH="$HOME/.cargo/bin:$PATH"

echo "[3/4] Compiling Rust Agent Natively..."
cd agent
cargo build --release
cd ..

echo "[4/4] Installing Systemd Services..."
sudo mkdir -p /opt/constellation/bin
sudo cp agent/target/release/constellation-agent /opt/constellation/bin/

# Inject Controller IP and Token into systemd service
SERVICE_FILE="/tmp/constellation-agent.service"
cat <<EOF > $SERVICE_FILE
[Unit]
Description=Constellation Node Agent
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/constellation
ExecStart=/opt/constellation/bin/constellation-agent --controller http://${CONTROLLER_IP}:8080 --token ${JOIN_TOKEN}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

sudo mv $SERVICE_FILE /etc/systemd/system/constellation-agent.service
sudo systemctl daemon-reload
sudo systemctl enable --now constellation-agent

echo ""
echo "=================================================================="
echo "✅ Worker Installation Complete!"
echo "=================================================================="
echo "The agent is starting up and connecting to $CONTROLLER_IP..."
echo "You can view the logs at any time by running:"
echo "  sudo journalctl -u constellation-agent -f"
echo "=================================================================="
