# Quick Start Guide for Pusher

## Prerequisites Installation

### 1. Install Go

#### macOS (using Homebrew)
```bash
brew install go
```

#### macOS (manual)
1. Download from: https://go.dev/dl/
2. Install the `.pkg` file
3. Verify installation:
```bash
go version
```

### 2. Install ADB (Android Debug Bridge)

```bash
brew install android-platform-tools
```

Verify installation:
```bash
adb version
```

## Building Pusher

### Option 1: Using Makefile (Recommended)

```bash
# Clone and navigate to the project
cd /Users/andreibanu/Pusher

# Download dependencies
make deps

# Build the binary
make build

# Install to system path
make install
```

### Option 2: Using build script

```bash
./build.sh
sudo cp pusher /usr/local/bin/
```

### Option 3: Manual build

```bash
go mod download
go build -ldflags="-s -w" -o pusher
```

## First-Time Setup

After building and installing:

```bash
# Run pusher for the first time
pusher

# You'll be prompted to enter:
# - Robot Wi-Fi SSID
# - Robot Wi-Fi Password

# This creates your default profile
```

## Common Commands

```bash
# Deploy to robot (default command)
pusher

# Disconnect ADB only
pusher dc

# Disconnect ADB and restore Wi-Fi
pusher exit

# Manage profiles
pusher profile list
pusher profile add
pusher profile use <name>

# Get help
pusher help
```

## Project Requirements

Your Android Studio project must have:
- `gradlew` (Gradle wrapper) in the project root
- `assembleDebug` and `installDebug` Gradle tasks configured

## Troubleshooting

### "adb not found"
```bash
brew install android-platform-tools
```

### "gradlew not found"
Ensure you're running `pusher` from within your Android Studio project directory or its subdirectories (up to 3 levels deep).

### Permission errors with Wi-Fi
Some Wi-Fi operations may require elevated privileges:
```bash
sudo pusher
```

### Build errors
```bash
# Clean and rebuild
go clean -cache
go mod tidy
go build -o pusher
```

## Development Workflow

1. Make changes to code
2. Test locally: `make build && ./pusher`
3. Create release: `make release VERSION=1.0.0`
4. Test installation: `make install`

## Next Steps

- See [README.md](README.md) for complete documentation
- See [BUILD.md](BUILD.md) for detailed build instructions
- See [HOMEBREW.md](HOMEBREW.md) for distribution setup
