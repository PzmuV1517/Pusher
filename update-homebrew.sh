#!/bin/bash

# update-homebrew.sh - Helper script to update Homebrew formula with latest release info
# Usage: ./update-homebrew.sh <version> <sha256>

set -e

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <version> <sha256>"
    echo ""
    echo "Example:"
    echo "  $0 v1.0.20260131120000 abc123def456..."
    echo ""
    echo "You can find these values in the latest GitHub release:"
    echo "  https://github.com/andreibanu/pusher/releases/latest"
    exit 1
fi

VERSION="$1"
SHA256="$2"

echo "[*] Updating Homebrew formula for Pusher $VERSION"
echo ""
echo "Copy this to your Homebrew formula at:"
echo "  https://github.com/PzmuV1517/homebrew-PzmuV1517"
echo ""
echo "---"
echo ""

cat << EOF
class Pusher < Formula
  desc "FTC Robot Deployment Tool - Connect, build, and deploy to FTC robots"
  homepage "https://github.com/andreibanu/pusher"
  url "https://github.com/andreibanu/pusher/archive/${VERSION}.tar.gz"
  sha256 "${SHA256}"
  version "${VERSION#v}"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}")
  end

  test do
    assert_match "FTC Robot Deployment Tool", shell_output("#{bin}/pusher help")
  end
end
EOF

echo ""
echo "---"
echo ""
echo "[OK] Formula ready! Update your Homebrew tap with the above content."
