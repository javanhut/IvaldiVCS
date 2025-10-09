#!/bin/bash

# uninstall.sh - Ivaldi VCS Uninstall Script
# Removes Ivaldi VCS binary and optionally cleans up source files

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Installation directory
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="ivaldi"

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

# Check if Ivaldi is installed
check_installation() {
    if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        return 0
    else
        return 1
    fi
}

# Remove the binary
remove_binary() {
    print_info "Removing ${BINARY_NAME} from ${INSTALL_DIR}..."

    if sudo rm -f "${INSTALL_DIR}/${BINARY_NAME}"; then
        print_success "${BINARY_NAME} binary removed successfully"
        return 0
    else
        print_error "Failed to remove ${BINARY_NAME} binary"
        return 1
    fi
}

# Clean up source directory (if running from git repo)
clean_source() {
    if [ -f "Makefile" ] && [ -f "main.go" ]; then
        print_info "Found source directory. Cleaning build artifacts..."

        if [ -d "build" ]; then
            rm -rf build
            print_success "Build directory removed"
        fi

        # Ask if user wants to remove the entire source directory
        if [ -d ".git" ]; then
            echo ""
            print_warning "This appears to be a git repository."
            read -p "Do you want to remove the entire source directory? (y/n) " -n 1 -r
            echo ""
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                PARENT_DIR=$(dirname "$(pwd)")
                CURRENT_DIR=$(basename "$(pwd)")
                print_info "Removing source directory..."
                cd "$PARENT_DIR"
                rm -rf "$CURRENT_DIR"
                print_success "Source directory removed"
                return 0
            else
                print_info "Source directory kept intact"
            fi
        fi
    fi
}

# Remove PATH entries from shell rc files
clean_path_entries() {
    print_info "Checking for PATH entries in shell configuration files..."

    local files=(~/.bashrc ~/.profile ~/.zshrc ~/.bash_profile)
    local removed=false

    for file in "${files[@]}"; do
        if [ -f "$file" ]; then
            # Check if file contains Go GOPATH bin entries added by this installer
            if grep -q "go env GOPATH" "$file" 2>/dev/null; then
                print_warning "Found Go PATH entries in $file"
                echo "These entries were likely added by the installer:"
                grep -n "go env GOPATH" "$file" || true
                echo ""
                read -p "Remove Go-related PATH entries from $file? (y/n) " -n 1 -r
                echo ""
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    # Create backup
                    cp "$file" "${file}.backup.$(date +%s)"
                    # Remove Go-related lines
                    sed -i '/# Go programming language/d' "$file"
                    sed -i '/export PATH=\$PATH:\/usr\/local\/go\/bin/d' "$file"
                    sed -i '/export PATH=\$PATH:\$(go env GOPATH)\/bin/d' "$file"
                    print_success "Removed entries from $file (backup created)"
                    removed=true
                fi
            fi
        fi
    done

    if [ "$removed" = true ]; then
        print_info "You may need to restart your shell or run 'source <file>' for changes to take effect"
    else
        print_info "No PATH entries found to remove"
    fi
}

# Main uninstall function
main() {
    echo ""
    echo "========================================"
    echo "  Ivaldi VCS Uninstall Script"
    echo "========================================"
    echo ""

    # Check if Ivaldi is installed
    if ! check_installation; then
        print_warning "${BINARY_NAME} is not installed at ${INSTALL_DIR}/${BINARY_NAME}"

        # Still offer to clean up source if present
        if [ -f "Makefile" ] && [ -f "main.go" ]; then
            echo ""
            read -p "Clean up source directory anyway? (y/n) " -n 1 -r
            echo ""
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                clean_source
            fi
        fi

        exit 0
    fi

    # Show current installation
    print_info "Found ${BINARY_NAME} at ${INSTALL_DIR}/${BINARY_NAME}"
    if command -v ivaldi &> /dev/null; then
        VERSION=$(ivaldi --version 2>/dev/null || echo "unknown")
        print_info "Version: $VERSION"
    fi
    echo ""

    # Confirm uninstallation
    read -p "Do you want to uninstall Ivaldi VCS? (y/n) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Uninstallation cancelled"
        exit 0
    fi
    echo ""

    # Remove the binary
    if ! remove_binary; then
        print_error "Uninstallation failed"
        exit 1
    fi
    echo ""

    # Ask about cleaning PATH entries
    read -p "Do you want to clean up PATH entries from shell config files? (y/n) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo ""
        clean_path_entries
    fi
    echo ""

    # Clean up source directory
    clean_source
    echo ""

    print_success "Ivaldi VCS has been uninstalled!"
    echo ""
    echo "========================================"
    echo "  Uninstallation Complete"
    echo "========================================"
    echo ""
    echo "Note: This script does not remove Go itself."
    echo "If you want to remove Go, please do so manually."
    echo ""
}

# Run main function
main
