.PHONY: build clean test install release help

# Variables
BINARY_NAME=pusher
VERSION?=dev
BUILD_DIR=dist
INSTALL_PATH=/usr/local/bin

# Build flags
LDFLAGS=-s -w
GO_BUILD=go build -ldflags="$(LDFLAGS)"

help: ## Show this help
	@echo "Pusher - FTC Robot Deployment Tool"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME)..."
	@$(GO_BUILD) -o $(BINARY_NAME)
	@echo "[OK] Build complete: ./$(BINARY_NAME)"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR)
	@echo "[OK] Clean complete"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

install: build ## Install to system path
	@echo "Installing to $(INSTALL_PATH)..."
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
	@echo "[OK] Installed to $(INSTALL_PATH)/$(BINARY_NAME)"

uninstall: ## Uninstall from system path
	@echo "Uninstalling from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "[OK] Uninstalled"

release: ## Build release binaries for all platforms
	@echo "Building release for version $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building macOS (Intel)..."
	@GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	@echo "Building macOS (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 $(GO_BUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
	@echo "Building Linux..."
	@GOOS=linux GOARCH=amd64 $(GO_BUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	@echo "Building Windows..."
	@GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe
	@echo "Creating universal macOS binary..."
	@lipo -create -output $(BUILD_DIR)/$(BINARY_NAME)-darwin-universal \
		$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 \
		$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
	@echo ""
	@echo "[OK] Release builds complete in $(BUILD_DIR)/"
	@ls -lh $(BUILD_DIR)/

checksums: ## Calculate SHA256 checksums for release binaries
	@echo "SHA256 checksums:"
	@shasum -a 256 $(BUILD_DIR)/$(BINARY_NAME)-*

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "[OK] Dependencies downloaded"

tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	@go mod tidy
	@echo "[OK] Dependencies tidied"

run: build ## Build and run
	@./$(BINARY_NAME)

dev: ## Run without building (go run)
	@go run main.go
