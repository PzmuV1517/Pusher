#!/bin/bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
print_info() {
    echo -e "${BLUE}[i]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
    echo -e "${RED}[X]${NC} $1"
}

print_header() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  Pusher - FTC Robot Deployment Tool${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

check_os() {
    print_info "Checking operating system..."
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        print_success "macOS detected"
        return 0
    else
        print_error "This installer currently only supports macOS"
        print_info "For other platforms, see: https://github.com/andreibanu/pusher#installation"
        exit 1
    fi
}

check_go() {
    print_info "Checking for Go installation..."
    
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}')
        print_success "Go is installed: $GO_VERSION"
        return 0
    else
        print_warning "Go is not installed"
        return 1
    fi
}

install_go() {
    print_info "Installing Go..."
    
    if command -v brew &> /dev/null; then
        print_info "Using Homebrew to install Go..."
        brew install go
        print_success "Go installed successfully"
    else
        print_warning "Homebrew not found"
        print_info "Please install Go manually from: https://go.dev/dl/"
        print_info "Or install Homebrew first: https://brew.sh"
        exit 1
    fi
}

check_adb() {
    print_info "Checking for ADB installation..."
    
    if command -v adb &> /dev/null; then
        print_success "ADB is installed"
        return 0
    else
        print_warning "ADB is not installed"
        return 1
    fi
}

install_adb() {
    print_info "Installing ADB (Android Platform Tools)..."
    
    if command -v brew &> /dev/null; then
        brew install android-platform-tools
        print_success "ADB installed successfully"
    else
        print_warning "Homebrew not found"
        print_info "Please install Android Platform Tools manually"
        exit 1
    fi
}

build_pusher() {
    print_info "Building Pusher..."
    
    # Tidy dependencies
    print_info "Tidying dependencies..."
    go mod tidy
    
    # Build
    print_info "Compiling binary..."
    go build -ldflags="-s -w" -o pusher
    
    print_success "Pusher built successfully"
}

install_pusher() {
    print_info "Installing Pusher..."
    
    # Check if /usr/local/bin exists
    if [ ! -d "/usr/local/bin" ]; then
        print_info "Creating /usr/local/bin directory..."
        sudo mkdir -p /usr/local/bin
    fi
    
    # Install
    sudo cp pusher /usr/local/bin/
    sudo chmod +x /usr/local/bin/pusher
    
    print_success "Pusher installed to /usr/local/bin/pusher"
}

verify_installation() {
    print_info "Verifying installation..."
    
    if command -v pusher &> /dev/null; then
        print_success "Pusher is installed and available in PATH"
        return 0
    else
        print_error "Pusher is not in PATH"
        return 1
    fi
}

show_next_steps() {
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  Installation Complete!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Navigate to your Android Studio project:"
    echo "   cd /path/to/your/ftc/project"
    echo ""
    echo "2. Run Pusher:"
    echo "   pusher"
    echo ""
    echo "3. On first run, you'll be prompted to set up your robot profile"
    echo ""
    echo "For help:"
    echo "   pusher help"
    echo ""
    echo "Documentation:"
    echo "   https://github.com/andreibanu/pusher#readme"
    echo ""
}

# Main installation flow
main() {
    print_header
    
    # Check OS
    check_os
    
    # Check and install Go
    if ! check_go; then
        read -p "Would you like to install Go now? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            install_go
        else
            print_error "Go is required to build Pusher"
            print_info "Install Go and run this script again"
            exit 1
        fi
    fi
    
    # Check and install ADB
    if ! check_adb; then
        read -p "Would you like to install ADB now? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            install_adb
        else
            print_warning "ADB is required to connect to robots"
            print_info "You can install it later with: brew install android-platform-tools"
        fi
    fi
    
    # Build
    build_pusher
    
    # Install
    print_info "Pusher needs to be installed to /usr/local/bin (requires sudo)"
    read -p "Proceed with installation? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_pusher
        
        # Verify
        if verify_installation; then
            show_next_steps
        else
            print_error "Installation verification failed"
            print_info "Try running: export PATH=\"/usr/local/bin:\$PATH\""
        fi
    else
        print_info "Binary is available at: ./pusher"
        print_info "To install manually later, run: sudo cp pusher /usr/local/bin/"
    fi
}

# Run main installation
main
