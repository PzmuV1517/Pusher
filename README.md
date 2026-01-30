# Pusher - FTC Robot Deployment Tool

A production-quality CLI tool for FTC robotics developers that automates connecting to robots, building, and deploying Android Studio projects.

## Features

- Automatic robot Wi-Fi connection with retry logic
- ADB connection management
- Gradle build and deployment automation
- Robot profile management
- Wi-Fi state restoration
- Clean, animated terminal UI

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/andreibanu/pusher
cd pusher

# Build the binary
go build -o pusher

# Move to your PATH (optional)
sudo mv pusher /usr/local/bin/
```

### Using Homebrew (coming soon)

```bash
brew tap PzmuV1517/pusher
brew install pusher
```

## Prerequisites

- **macOS** (Linux and Windows support coming soon)
- **ADB (Android Debug Bridge)** - Install via Android SDK Platform-Tools
- **Gradle wrapper** in your Android Studio project
- **Go 1.21+** (for building from source)

## Quick Start

### First Run

On your first run, Pusher will prompt you to set up a robot profile:

```bash
pusher
```

You'll be asked to enter:
- Robot Wi-Fi SSID
- Robot Wi-Fi Password

This profile will be saved as your default profile.

### Basic Usage

#### Deploy to Robot

```bash
# Connect to robot, build, and deploy
pusher
```

This command will:
1. Save your current Wi-Fi connection
2. Connect to the robot's Wi-Fi
3. Establish ADB connection at 192.168.43.1
4. Detect Gradle wrapper
5. Build and deploy your app

#### Disconnect ADB

```bash
# Disconnect ADB only (keep Wi-Fi)
pusher dc
# or
pusher disconnect
```

#### Disconnect and Restore Wi-Fi

```bash
# Disconnect ADB and restore previous Wi-Fi
pusher exit
```

## Profile Management

### List Profiles

```bash
pusher profile list
```

Shows all saved robot profiles with the default marked.

### Add a Profile

```bash
pusher profile add
```

Interactive prompt to add a new robot profile.

### Edit a Profile

```bash
pusher profile edit <profile-name>
```

Update SSID or password for an existing profile.

### Set Default Profile

```bash
pusher profile use <profile-name>
```

Set which profile to use by default.

## Configuration

Pusher stores its configuration in `~/.config/pusher/config.yaml`.

Example configuration:
```yaml
default_profile: default
last_wifi: MyHomeWiFi
profiles:
  default:
    name: default
    ssid: DIRECT-RobotController
    password: mypassword
  team_robot:
    name: team_robot
    ssid: FTC-12345
    password: teampassword
```

## Commands Reference

| Command | Alias | Description |
|---------|-------|-------------|
| `pusher` | - | Connect, build, and deploy (default action) |
| `pusher dc` | `disconnect` | Disconnect ADB only |
| `pusher exit` | - | Disconnect ADB and restore Wi-Fi |
| `pusher profile list` | - | List all robot profiles |
| `pusher profile add` | - | Add a new profile |
| `pusher profile edit` | - | Edit an existing profile |
| `pusher profile use` | - | Set default profile |
| `pusher help` | - | Show help with ASCII art |

## Troubleshooting

### ADB Not Found

Install Android SDK Platform-Tools:
```bash
brew install android-platform-tools
```

### Wi-Fi Connection Issues

- Ensure you have the correct Wi-Fi SSID and password
- Check that you're in range of the robot's Wi-Fi
- Verify you have permission to change Wi-Fi settings
- Pusher automatically retries connection 3 times

### Gradle Wrapper Not Found

- Ensure you're running Pusher from within or near your Android Studio project directory
- Pusher searches up to 3 parent directories for `gradlew`
- Make sure `gradlew` exists in your project root

### Permission Issues

If you encounter permission errors with `networksetup`, you may need to run with elevated privileges:
```bash
sudo pusher
```

However, this should generally not be necessary on macOS.

## Development

### Building

```bash
# Build for current platform
go build -o pusher

# Build with version information
go build -ldflags "-X main.version=1.0.0" -o pusher

# Build for multiple platforms
GOOS=darwin GOARCH=amd64 go build -o pusher-darwin-amd64
GOOS=darwin GOARCH=arm64 go build -o pusher-darwin-arm64
GOOS=linux GOARCH=amd64 go build -o pusher-linux-amd64
```

### Running Tests

```bash
go test ./...
```

### Project Structure

```
pusher/
├── cmd/                    # Cobra commands
│   ├── root.go            # Root command setup
│   ├── push.go            # Main push logic
│   ├── disconnect.go      # Disconnect command
│   ├── exit.go            # Exit command
│   ├── profile.go         # Profile management
│   └── help.go            # Help command
├── internal/
│   ├── adb/               # ADB connection logic
│   ├── config/            # Configuration management
│   ├── gradle/            # Gradle build logic
│   ├── tui/               # Terminal UI components
│   └── wifi/              # Wi-Fi management (OS-specific)
├── main.go                # Entry point
├── go.mod                 # Go module definition
└── README.md              # This file
```

## Roadmap

- [ ] Linux support
- [ ] Windows support
- [ ] Enhanced build output filtering
- [ ] Support for custom ADB addresses
- [ ] Integration with FTC SDK tools
- [ ] Automated testing
- [ ] CI/CD pipeline

## Credits

Made with love by **Andrei Banu**

## License

MIT License - feel free to use and modify as needed.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
