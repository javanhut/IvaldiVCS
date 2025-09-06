# Makefile for Ivaldi VCS
# Builds and installs the Ivaldi binary

# Binary name
BINARY_NAME=ivaldi

# Build directory
BUILD_DIR=build

# Installation directory
INSTALL_DIR=/usr/local/bin

# Go build flags
BUILD_FLAGS=-ldflags "-s -w"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./main.go
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

# Install the binary to /usr/local/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@sudo chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "$(BINARY_NAME) installed to $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo "You can now use: $(BINARY_NAME) --help"

# Uninstall the binary
.PHONY: uninstall
uninstall:
	@echo "Removing $(BINARY_NAME) from $(INSTALL_DIR)..."
	@sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "$(BINARY_NAME) uninstalled"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Build directory cleaned"

# Test the application
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Run linting
.PHONY: lint
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		@echo "golangci-lint not found, running go vet instead..."; \
		go vet ./...; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Development build (with debug info)
.PHONY: dev
dev:
	@echo "Building $(BINARY_NAME) with debug info..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./main.go
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME) (debug)"

# Run the application locally
.PHONY: run
run: dev
	./$(BUILD_DIR)/$(BINARY_NAME) --help

# Check if the binary is installed
.PHONY: check
check:
	@if [ -f "$(INSTALL_DIR)/$(BINARY_NAME)" ]; then \
		echo "$(BINARY_NAME) is installed at $(INSTALL_DIR)/$(BINARY_NAME)"; \
		$(INSTALL_DIR)/$(BINARY_NAME) --help | head -3; \
	else \
		echo "$(BINARY_NAME) is not installed"; \
		echo "Run 'make install' to install it"; \
	fi

# Show help
.PHONY: help
help:
	@echo "Ivaldi VCS Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build      - Build the binary in $(BUILD_DIR)/"
	@echo "  install    - Install $(BINARY_NAME) to $(INSTALL_DIR)/ (requires sudo)"
	@echo "  uninstall  - Remove $(BINARY_NAME) from $(INSTALL_DIR)/ (requires sudo)"
	@echo "  clean      - Remove build artifacts"
	@echo "  test       - Run tests"
	@echo "  lint       - Run linters"
	@echo "  fmt        - Format code"
	@echo "  dev        - Build with debug info"
	@echo "  run        - Build and run locally"
	@echo "  check      - Check if $(BINARY_NAME) is installed"
	@echo "  help       - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make install   # Build and install the binary"
	@echo "  make check     # Check installation status"
	@echo "  make clean     # Clean up build files"