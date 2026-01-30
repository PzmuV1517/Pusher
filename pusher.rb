class Pusher < Formula
  desc "FTC Robot deployment tool - automate building and deploying Android apps to robots"
  homepage "https://github.com/andreibanu/pusher"
  version "1.0.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/andreibanu/pusher/releases/download/v#{version}/pusher-darwin-arm64"
      sha256 "PUT_ARM64_SHA256_HERE"
    else
      url "https://github.com/andreibanu/pusher/releases/download/v#{version}/pusher-darwin-amd64"
      sha256 "PUT_AMD64_SHA256_HERE"
    end
  end

  on_linux do
    url "https://github.com/andreibanu/pusher/releases/download/v#{version}/pusher-linux-amd64"
    sha256 "PUT_LINUX_SHA256_HERE"
  end

  def install
    # Determine the binary name based on OS and arch
    binary_name = if OS.mac?
                    "pusher-darwin-#{Hardware::CPU.arch}"
                  else
                    "pusher-linux-amd64"
                  end
    
    bin.install binary_name => "pusher"
  end

  def caveats
    <<~EOS
      Pusher requires ADB (Android Debug Bridge) to be installed.
      Install it with:
        brew install android-platform-tools

      Run 'pusher help' to get started.
    EOS
  end

  test do
    assert_match "FTC Robot Deployment Tool", shell_output("#{bin}/pusher help")
  end
end
