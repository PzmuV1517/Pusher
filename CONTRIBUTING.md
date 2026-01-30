# Contributing to Pusher

Thank you for your interest in contributing to Pusher! This document provides guidelines and instructions for contributing.

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help maintain a welcoming environment for all contributors

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in Issues
2. Include detailed steps to reproduce
3. Provide system information (OS, Go version, etc.)
4. Include relevant logs and error messages

### Suggesting Features

1. Check if the feature has already been requested
2. Clearly describe the feature and its use case
3. Explain why this feature would benefit FTC developers

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Test thoroughly
5. Commit with clear messages
6. Push to your fork
7. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or later
- macOS (for testing macOS-specific features)
- ADB installed
- An Android Studio project for testing

### Setup Steps

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/pusher
cd pusher

# Add upstream remote
git remote add upstream https://github.com/andreibanu/pusher

# Download dependencies
make deps

# Build
make build

# Test
make test
```

## Code Style

### Go Style Guidelines

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `go vet` before committing
- Keep functions small and focused
- Add comments for exported functions

### Formatting

```bash
# Format code
go fmt ./...

# Vet code
go vet ./...

# Run linter (if installed)
golangci-lint run
```

## Project Structure

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed project structure.

### Key Principles

- **cmd/**: Command definitions only, no business logic
- **internal/**: All business logic, properly encapsulated
- **OS-specific code**: Abstract behind interfaces (see wifi package)
- **Error handling**: Always return errors, don't panic
- **Testing**: Add tests for new functionality

## Testing Guidelines

### Manual Testing Checklist

Before submitting a PR, test:

- [ ] First-run experience (delete ~/.config/pusher)
- [ ] Default push command
- [ ] Profile management (add, list, edit, use)
- [ ] Disconnect command
- [ ] Exit command
- [ ] Help command
- [ ] Error cases (no ADB, no gradlew, wrong Wi-Fi password)

### Writing Tests

```go
// Example test structure
func TestWiFiManager_GetCurrentSSID(t *testing.T) {
    mgr := NewManager()
    
    ssid, err := mgr.GetCurrentSSID()
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if ssid == "" {
        t.Error("expected non-empty SSID")
    }
}
```

## Adding Platform Support

### Supporting a New OS

1. Create OS-specific file (e.g., `wifi_linux.go`)
2. Use build tags: `//go:build linux`
3. Implement the same interface as other platforms
4. Update documentation
5. Test thoroughly on target platform

Example:

```go
//go:build linux

package wifi

import "os/exec"

func (m *Manager) GetCurrentSSID() (string, error) {
    // Linux-specific implementation using nmcli
    cmd := exec.Command("nmcli", "-t", "-f", "active,ssid", "dev", "wifi")
    // ...
}
```

## Commit Message Guidelines

Use conventional commits format:

```
type(scope): subject

body (optional)

footer (optional)
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting
- `refactor`: Code restructuring
- `test`: Adding tests
- `chore`: Maintenance

Examples:
```
feat(wifi): add Linux support for Wi-Fi management

fix(adb): handle connection timeout gracefully

docs(readme): update installation instructions
```

## Release Process

(For maintainers)

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create and push tag: `git tag -a v1.0.0 -m "Release 1.0.0"`
4. Run release build: `make release VERSION=1.0.0`
5. Create GitHub release with binaries
6. Update Homebrew formula

## Areas Needing Help

Current priorities:

- [ ] Linux support (NetworkManager/nmcli)
- [ ] Windows support (netsh)
- [ ] Unit tests for all packages
- [ ] Integration tests
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Encrypted password storage
- [ ] Build output filtering/prettification
- [ ] Custom ADB address support
- [ ] Documentation improvements
- [ ] Video tutorials

## Questions?

- Open an issue for questions
- Tag with `question` label
- Check existing issues and documentation first

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

Thank you for contributing to Pusher!
