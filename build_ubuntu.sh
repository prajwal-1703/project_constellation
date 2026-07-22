#!/bin/bash
set -e

# Run this script on an Ubuntu machine to compile and package the release.

# Ensure newly installed rust and go binaries are always found first
export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"

echo "✦ Constellation Packaging Script for Ubuntu ✦"

# Prerequisites check
if ! command -v go &> /dev/null; then
    echo "Error: 'go' is not installed."
    exit 1
fi

if ! command -v cargo &> /dev/null; then
    echo "Error: 'cargo' is not installed."
    exit 1
fi

if ! command -v npm &> /dev/null; then
    echo "Error: 'npm' is not installed."
    exit 1
fi

VERSION="v2.0"
DIST_DIR="release/constellation-${VERSION}-ubuntu"
TAR_FILE="constellation-${VERSION}-ubuntu.tar.gz"

echo "Cleaning previous builds..."
rm -rf release/
mkdir -p "$DIST_DIR/bin"
mkdir -p "$DIST_DIR/systemd"
mkdir -p "$DIST_DIR/certs"
mkdir -p "$DIST_DIR/dashboard/dist"

# 1. Build Dashboard
echo "[1/3] Building Dashboard..."
cd dashboard
npm install
npm run build
cp -r dist/* ../$DIST_DIR/dashboard/dist/
cd ..

# 2. Build Go Controller & CLI
echo "[2/3] Building Controller and CLI..."
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=1
cd controller
go build -o ../$DIST_DIR/bin/constellation-controller ./cmd/controller
cd ..
cd cli
go build -o ../$DIST_DIR/bin/constellation .
cd ..

# 3. Build Rust Agent
echo "[3/3] Building Rust Agent..."
cd agent
cargo build --release
cp target/release/constellation-agent ../$DIST_DIR/bin/
cd ..

# Copy systemd templates and cert generation script
cp deploy/constellation-controller.service $DIST_DIR/systemd/
cp deploy/constellation-agent.service $DIST_DIR/systemd/
cp deploy/install.sh $DIST_DIR/

echo "Creating tarball..."
cd release
tar -czvf $TAR_FILE constellation-${VERSION}-ubuntu/
cd ..

echo "✅ Build complete! Release package available at: release/$TAR_FILE"
