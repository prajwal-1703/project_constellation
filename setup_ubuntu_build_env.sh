#!/bin/bash
set -e

echo "✦ Constellation Build Environment Setup for Ubuntu ✦"

# Update package lists
echo "Updating apt package lists..."
sudo apt-get update

# Install basic requirements and C Build Tools
echo "Installing C build tools and utilities..."
sudo apt-get install -y software-properties-common curl wget git build-essential libc6-dev gcc pkg-config libssl-dev
if grep -qi ubuntu /etc/os-release; then
    sudo add-apt-repository -y ppa:dqlite/dev
    sudo apt-get update
fi
sudo apt-get install -y libdqlite-dev libraft-dev

# 1. Install Node.js (20.x)
if ! command -v node &> /dev/null || ! node -v | grep -q "v20\\|v21\\|v22\\|v23"; then
    echo "Installing Node.js 20.x..."
    curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
    sudo apt-get install -y nodejs
else
    echo "Node.js $(node -v) is already installed."
fi

# 2. Install Go (1.22.5)
GO_VERSION="1.22.5"
if ! command -v go &> /dev/null || ! go version | grep -q "go1.2"; then
    echo "Installing Go ${GO_VERSION}..."
    wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
    rm go${GO_VERSION}.linux-amd64.tar.gz
    
    # Add to bashrc if not already there
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin
else
    echo "Go is already installed: $(go version)"
fi

# 3. Install Rust
if ! command -v cargo &> /dev/null; then
    echo "Installing Rust..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
    # Ensure cargo is in PATH for the remainder of this script
    source "$HOME/.cargo/env"
else
    echo "Rust is already installed: $(cargo --version)"
fi

echo ""
echo "✅ All build dependencies have been installed successfully!"
echo "⚠️  IMPORTANT: To update your current terminal session with the new paths, please run:"
echo "    source ~/.bashrc"
echo "    source ~/.cargo/env"
echo ""
echo "After that, you can run: ./build_ubuntu.sh"
