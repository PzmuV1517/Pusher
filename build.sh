#!/bin/bash

set -e

VERSION=${VERSION:-"dev"}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS="-s -w"

echo "Building pusher..."
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Build for current platform
go build -ldflags="${LDFLAGS}" -o pusher

echo "[OK] Build complete: ./pusher"
echo ""
echo "To install globally, run:"
echo "  sudo cp pusher /usr/local/bin/"
