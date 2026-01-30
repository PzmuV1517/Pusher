#!/bin/bash

set -e

VERSION=${1:-"dev"}

if [ "$VERSION" = "dev" ]; then
  echo "Usage: ./release.sh <version>"
  echo "Example: ./release.sh 1.0.0"
  exit 1
fi

COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS="-s -w"

OUTPUT_DIR="dist"
mkdir -p ${OUTPUT_DIR}

echo "Building release binaries for version ${VERSION}..."
echo ""

# macOS Intel
echo "[*] Building for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ${OUTPUT_DIR}/pusher-darwin-amd64

# macOS Apple Silicon
echo "[*] Building for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o ${OUTPUT_DIR}/pusher-darwin-arm64

# Linux
echo "[*] Building for Linux..."
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ${OUTPUT_DIR}/pusher-linux-amd64

# Windows
echo "[*] Building for Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ${OUTPUT_DIR}/pusher-windows-amd64.exe

# Create universal macOS binary
echo "[*] Creating universal macOS binary..."
lipo -create -output ${OUTPUT_DIR}/pusher-darwin-universal \
  ${OUTPUT_DIR}/pusher-darwin-amd64 \
  ${OUTPUT_DIR}/pusher-darwin-arm64

echo ""
echo "[OK] Release builds complete in ${OUTPUT_DIR}/"
echo ""
ls -lh ${OUTPUT_DIR}/

echo ""
echo "SHA256 checksums:"
shasum -a 256 ${OUTPUT_DIR}/pusher-*
