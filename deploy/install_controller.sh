#!/bin/bash
set -e

echo "=================================================="
echo "    ✦ Constellation Controller Setup ✦"
echo "=================================================="

if [ "$EUID" -eq 0 ]; then
  echo "Please DO NOT run this script directly as root."
  echo "Run as your normal user, and the script will use sudo when needed."
  exit 1
fi

echo "[1/6] Installing dependencies (requires sudo)..."
sudo apt-get update
sudo apt-get install -y software-properties-common curl wget git build-essential libc6-dev gcc pkg-config libssl-dev protobuf-compiler

if grep -qi ubuntu /etc/os-release; then
    # dqlite PPA might fail on non-ubuntu (like Debian/Kali), ignore errors if it does
    sudo add-apt-repository -y ppa:dqlite/dev || true
    sudo apt-get update || true
fi
sudo apt-get install -y libdqlite-dev libraft-dev

echo "[2/6] Setting up Node.js 20.x..."
if ! command -v node &> /dev/null || ! node -v | grep -q "v20\|v21\|v22\|v23"; then
    curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi

echo "[3/6] Setting up Go 1.25.0..."
GO_VERSION="1.25.0"
if ! command -v go &> /dev/null || ! go version | grep -q "go1.25"; then
    wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
    rm go${GO_VERSION}.linux-amd64.tar.gz
fi
export PATH=/usr/local/go/bin:$PATH

echo "[4/6] Building Controller and Dashboard..."
cd dashboard
npm install
npm run build
cd ..

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=1

cd controller
go build -o ../constellation-controller ./cmd/controller
cd ..

cd cli
go build -o ../constellation .
cd ..

echo "Generating cluster certificates..."
go run gen_certs.go

echo "[5/6] Installing Systemd Services..."
# Stop service if it is already running to avoid "Text file busy" error
sudo systemctl stop constellation-controller 2>/dev/null || true

sudo mkdir -p /opt/constellation/bin
sudo mkdir -p /opt/constellation/certs
sudo mkdir -p /opt/constellation/dashboard

sudo rm -f /opt/constellation/bin/constellation-controller
sudo rm -f /opt/constellation/bin/constellation

sudo cp constellation-controller /opt/constellation/bin/
sudo cp constellation /opt/constellation/bin/
sudo cp -r certs/* /opt/constellation/certs/
sudo cp -r dashboard/dist /opt/constellation/dashboard/

sudo ln -sf /opt/constellation/bin/constellation /usr/local/bin/constellation

sudo mkdir -p /var/lib/constellation
sudo chmod 755 /var/lib/constellation

sudo cp deploy/constellation-controller.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now constellation-controller

echo "[6/6] Initializing Cluster..."
echo "Wiping old database state for a fresh cluster..."
sudo systemctl stop constellation-controller 2>/dev/null || true
sudo rm -rf /var/lib/constellation/*
sudo systemctl start constellation-controller

sleep 3 # Wait for controller to start

# Initialize a fresh cluster
INIT_OUTPUT=$(constellation init 2>&1) || true

echo ""
echo "=================================================================="
echo "✅ Controller Installation Complete!"
echo "=================================================================="
echo ""
echo "Your Controller is running."
if echo "$INIT_OUTPUT" | grep -q "Join Token"; then
    echo "$INIT_OUTPUT" | grep -E "Cluster:|Controller:|Join Token:"
else
    echo "Cluster is already initialized."
    echo "Run 'constellation login -u admin -p admin' to manage your cluster."
fi
echo ""
echo "NEXT STEPS FOR WORKER NODES:"
echo "Copy the project repository to your worker node, and run:"
echo "  bash deploy/install_worker.sh"
echo "=================================================================="
