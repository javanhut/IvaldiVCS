#!/bin/bash

# setup.sh - Ivaldi VCS Setup Script
# Checks OS, installs Go if needed, and installs Ivaldi VCS

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Minimum Go version required
MIN_GO_VERSION="1.19"
REPO_URL="https://github.com/javanhut/IvaldiVCS"
INSTALL_DIR="/tmp/ivaldi-install-$$"

# Print colored messages
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect operating system
detect_os() {
    print_info "Detecting operating system..."

    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        OS="linux"
        if [ -f /etc/os-release ]; then
            . /etc/os-release
            DISTRO=$ID
            print_success "Detected Linux distribution: $NAME"
        else
            DISTRO="unknown"
            print_warning "Linux distribution unknown"
        fi
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
        DISTRO="macos"
        print_success "Detected macOS"
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "win32" ]]; then
        OS="windows"
        DISTRO="windows"
        print_error "Windows is not fully supported by this script. Please install Go manually and run 'make install'."
        exit 1
    else
        OS="unknown"
        DISTRO="unknown"
        print_error "Unknown operating system: $OSTYPE"
        exit 1
    fi
}

# Compare semantic versions
version_ge() {
    # Returns 0 if $1 >= $2
    printf '%s\n%s\n' "$2" "$1" | sort -V -C
}

# Check if Go is installed and meets minimum version
check_go() {
    print_info "Checking for Go installation..."

    if command -v go &> /dev/null; then
        CURRENT_GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        print_info "Found Go version: $CURRENT_GO_VERSION"

        if version_ge "$CURRENT_GO_VERSION" "$MIN_GO_VERSION"; then
            print_success "Go version is sufficient (>= $MIN_GO_VERSION)"
            return 0
        else
            print_warning "Go version $CURRENT_GO_VERSION is older than required $MIN_GO_VERSION"
            return 1
        fi
    else
        print_warning "Go is not installed"
        return 1
    fi
}

# Install Go on Linux
install_go_linux() {
    print_info "Installing Go on Linux..."

    # Detect architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            GO_ARCH="amd64"
            ;;
        aarch64|arm64)
            GO_ARCH="arm64"
            ;;
        armv6l)
            GO_ARCH="armv6l"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    # Get latest Go version
    GO_VERSION=$(curl -s https://go.dev/VERSION?m=text | head -n 1)
    if [ -z "$GO_VERSION" ]; then
        print_warning "Could not fetch latest Go version, using fallback"
        GO_VERSION="go1.22.0"
    fi

    GO_TAR="${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    GO_URL="https://go.dev/dl/${GO_TAR}"

    print_info "Downloading Go $GO_VERSION for linux-${GO_ARCH}..."

    # Download Go
    if ! curl -L -o "/tmp/${GO_TAR}" "${GO_URL}"; then
        print_error "Failed to download Go"
        exit 1
    fi

    print_info "Extracting Go..."
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm "/tmp/${GO_TAR}"

    # Add Go to PATH if not already present
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        print_info "Adding Go to PATH in ~/.bashrc"
        echo "" >> ~/.bashrc
        echo "# Go programming language" >> ~/.bashrc
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
    fi

    # Also check ~/.profile
    if [ -f ~/.profile ] && ! grep -q "/usr/local/go/bin" ~/.profile; then
        print_info "Adding Go to PATH in ~/.profile"
        echo "" >> ~/.profile
        echo "# Go programming language" >> ~/.profile
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
        echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.profile
    fi

    # Export for current session
    export PATH=$PATH:/usr/local/go/bin

    print_success "Go installed successfully!"
    /usr/local/go/bin/go version
}

# Install Go on macOS
install_go_macos() {
    print_info "Installing Go on macOS..."

    # Check if Homebrew is available
    if command -v brew &> /dev/null; then
        print_info "Using Homebrew to install Go..."
        brew install go
        print_success "Go installed via Homebrew"
    else
        print_info "Homebrew not found, installing Go manually..."

        # Detect architecture
        ARCH=$(uname -m)
        case $ARCH in
            x86_64)
                GO_ARCH="amd64"
                ;;
            arm64)
                GO_ARCH="arm64"
                ;;
            *)
                print_error "Unsupported architecture: $ARCH"
                exit 1
                ;;
        esac

        # Get latest Go version
        GO_VERSION=$(curl -s https://go.dev/VERSION?m=text | head -n 1)
        if [ -z "$GO_VERSION" ]; then
            print_warning "Could not fetch latest Go version, using fallback"
            GO_VERSION="go1.22.0"
        fi

        GO_PKG="${GO_VERSION}.darwin-${GO_ARCH}.pkg"
        GO_URL="https://go.dev/dl/${GO_PKG}"

        print_info "Downloading Go $GO_VERSION for darwin-${GO_ARCH}..."

        # Download and install Go
        if ! curl -L -o "/tmp/${GO_PKG}" "${GO_URL}"; then
            print_error "Failed to download Go"
            exit 1
        fi

        print_info "Installing Go (may require password)..."
        sudo installer -pkg "/tmp/${GO_PKG}" -target /
        rm "/tmp/${GO_PKG}"

        print_success "Go installed successfully!"
    fi

    # Add Go to PATH if needed
    if ! grep -q "/usr/local/go/bin" ~/.zshrc 2>/dev/null && ! grep -q "/usr/local/go/bin" ~/.bash_profile 2>/dev/null; then
        SHELL_RC=""
        if [ -n "$ZSH_VERSION" ]; then
            SHELL_RC=~/.zshrc
        else
            SHELL_RC=~/.bash_profile
        fi

        print_info "Adding Go to PATH in $SHELL_RC"
        echo "" >> "$SHELL_RC"
        echo "# Go programming language" >> "$SHELL_RC"
        echo 'export PATH=$PATH:/usr/local/go/bin' >> "$SHELL_RC"
        echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> "$SHELL_RC"
    fi

    go version
}

# Install Go based on OS
install_go() {
    if [ "$OS" == "linux" ]; then
        install_go_linux
    elif [ "$OS" == "macos" ]; then
        install_go_macos
    else
        print_error "Cannot install Go on unsupported OS: $OS"
        exit 1
    fi
}

# Detect if running from git repo or via curl
detect_installation_method() {
    if [ -f "Makefile" ] && [ -f "main.go" ] && [ -d ".git" ]; then
        print_info "Detected local git repository"
        INSTALL_METHOD="local"
    else
        print_info "Detected remote installation (curl-based)"
        INSTALL_METHOD="remote"
    fi
}

# Download and extract source code from GitHub
download_source() {
    print_info "Downloading Ivaldi VCS source code..."

    # Create temporary directory
    mkdir -p "$INSTALL_DIR"

    # Download the latest release tarball
    TARBALL_URL="${REPO_URL}/archive/refs/heads/main.tar.gz"

    if ! curl -L -o "${INSTALL_DIR}/ivaldi.tar.gz" "$TARBALL_URL"; then
        print_error "Failed to download source code"
        rm -rf "$INSTALL_DIR"
        exit 1
    fi

    print_info "Extracting source code..."
    if ! tar -xzf "${INSTALL_DIR}/ivaldi.tar.gz" -C "$INSTALL_DIR" --strip-components=1; then
        print_error "Failed to extract source code"
        rm -rf "$INSTALL_DIR"
        exit 1
    fi

    # Remove tarball to save space
    rm "${INSTALL_DIR}/ivaldi.tar.gz"

    print_success "Source code downloaded and extracted to $INSTALL_DIR"
}

# Install make on Linux
install_make_linux() {
    print_info "Installing make..."

    case $DISTRO in
        ubuntu|debian|pop)
            print_info "Installing build-essential via apt..."
            sudo apt-get update
            sudo apt-get install -y build-essential
            ;;
        fedora|rhel|centos)
            print_info "Installing Development Tools via dnf/yum..."
            if command -v dnf &> /dev/null; then
                sudo dnf group install -y "Development Tools"
            else
                sudo yum groupinstall -y "Development Tools"
            fi
            ;;
        arch|manjaro)
            print_info "Installing base-devel via pacman..."
            sudo pacman -S --noconfirm base-devel
            ;;
        alpine)
            print_info "Installing build-base via apk..."
            sudo apk add build-base
            ;;
        opensuse*|sles)
            print_info "Installing development pattern via zypper..."
            sudo zypper install -y -t pattern devel_basis
            ;;
        *)
            print_error "Unknown distribution. Please install make manually:"
            print_error "  Ubuntu/Debian: sudo apt-get install build-essential"
            print_error "  Fedora/RHEL: sudo dnf group install 'Development Tools'"
            print_error "  Arch: sudo pacman -S base-devel"
            print_error "  Alpine: sudo apk add build-base"
            return 1
            ;;
    esac

    print_success "make installed successfully"
}

# Install make on macOS
install_make_macos() {
    print_info "Installing make and build tools..."

    # Check if Xcode Command Line Tools are installed
    if ! xcode-select -p &> /dev/null; then
        print_info "Installing Xcode Command Line Tools (this may take a while)..."
        xcode-select --install

        # Wait for installation to complete
        print_info "Please complete the Xcode Command Line Tools installation in the dialog box."
        print_info "Press any key once installation is complete..."
        read -n 1 -s
    fi

    print_success "Build tools installed successfully"
}

# Check for required tools
check_requirements() {
    print_info "Checking for required tools..."

    # Check for make
    if ! command -v make &> /dev/null; then
        print_warning "make is not installed"
        echo ""
        read -p "Do you want to install make and build tools? (y/n) " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            if [ "$OS" == "linux" ]; then
                if ! install_make_linux; then
                    exit 1
                fi
            elif [ "$OS" == "macos" ]; then
                if ! install_make_macos; then
                    exit 1
                fi
            fi
        else
            print_error "make is required to build Ivaldi VCS. Installation cancelled."
            exit 1
        fi
    fi

    # Check for curl (needed for remote installation)
    if [ "$INSTALL_METHOD" == "remote" ] && ! command -v curl &> /dev/null; then
        print_error "curl is not installed. Please install curl to proceed."
        exit 1
    fi

    # Check for tar (needed for remote installation)
    if [ "$INSTALL_METHOD" == "remote" ] && ! command -v tar &> /dev/null; then
        print_error "tar is not installed. Please install tar to proceed."
        exit 1
    fi

    print_success "All required tools are available"
}

# Build and install Ivaldi VCS
install_ivaldi() {
    print_info "Building and installing Ivaldi VCS..."

    # Set working directory based on installation method
    if [ "$INSTALL_METHOD" == "remote" ]; then
        WORK_DIR="$INSTALL_DIR"
    else
        WORK_DIR="$(pwd)"
    fi

    # Verify we're in the right directory
    if [ ! -f "${WORK_DIR}/Makefile" ] || [ ! -f "${WORK_DIR}/main.go" ]; then
        print_error "Makefile or main.go not found in ${WORK_DIR}"
        if [ "$INSTALL_METHOD" == "remote" ]; then
            rm -rf "$INSTALL_DIR"
        fi
        exit 1
    fi

    # Run make install
    if (cd "$WORK_DIR" && make install); then
        print_success "Ivaldi VCS installed successfully!"
        print_info "You can now use: ivaldi --help"

        # Cleanup temporary directory if remote install
        if [ "$INSTALL_METHOD" == "remote" ]; then
            print_info "Cleaning up temporary files..."
            rm -rf "$INSTALL_DIR"
        fi
    else
        print_error "Failed to install Ivaldi VCS"
        if [ "$INSTALL_METHOD" == "remote" ]; then
            rm -rf "$INSTALL_DIR"
        fi
        exit 1
    fi
}

# Main script execution
main() {
    echo ""
    echo "========================================"
    echo "  Ivaldi VCS Setup Script"
    echo "========================================"
    echo ""

    # Detect installation method
    detect_installation_method
    echo ""

    # Detect OS
    detect_os
    echo ""

    # Check and install Go if needed
    if ! check_go; then
        echo ""
        print_warning "Go needs to be installed or updated"
        read -p "Do you want to install/update Go? (y/n) " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            install_go
        else
            print_error "Go installation cancelled. Cannot proceed without Go."
            exit 1
        fi
    fi
    echo ""

    # Check for other requirements
    check_requirements
    echo ""

    # Download source code if remote installation
    if [ "$INSTALL_METHOD" == "remote" ]; then
        download_source
        echo ""
    fi

    # Install Ivaldi VCS
    install_ivaldi
    echo ""

    print_success "Setup complete!"
    echo ""
    echo "========================================"
    echo "  Next Steps:"
    echo "========================================"
    echo "1. Restart your terminal or run:"
    echo "   source ~/.bashrc  (Linux)"
    echo "   source ~/.zshrc   (macOS with zsh)"
    echo ""
    echo "2. Verify installation:"
    echo "   ivaldi --version"
    echo ""
    echo "3. Get help:"
    echo "   ivaldi --help"
    echo ""
}

# Run main function
main
