#!/bin/bash

# TARR Annunciator Package Creation Script
# This script helps create distribution packages for all platforms

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() { echo -e "${GREEN}✓${NC} $1"; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; }
print_info() { echo -e "${BLUE}ℹ${NC} $1"; }

echo "==========================================="
echo "🎯 TARR Annunciator Package Creator"
echo "==========================================="

# Check if we're in the right directory
if [[ ! -f "main.go" ]] || [[ ! -f "Makefile" ]]; then
    print_error "This script must be run from the TARR Annunciator project root directory"
    exit 1
fi

# Create dist directory if it doesn't exist
mkdir -p dist packages

print_info "Building executables for all platforms..."

# Build all platform executables
print_info "Building Windows x64..."
make build-windows

print_info "Building Linux x64..."
make build-linux

print_info "Building Raspberry Pi variants..."
make build-raspberry-pi        # ARM64
make build-raspberry-pi-32     # ARM32
make build-raspberry-pi-zero   # ARMv6

print_status "All executables built successfully!"

echo ""
echo "=== Creating Packages ==="

# Function to copy core files
copy_core_files() {
    local dest_dir="$1"
    
    # Configuration files
    cp -r json/ "$dest_dir/"
    
    # Web templates
    cp -r templates/ "$dest_dir/"
    
    # Audio files
    cp -r static/ "$dest_dir/"
    
    # Create logs directory
    mkdir -p "$dest_dir/logs"
    
    print_status "Core files copied to $dest_dir"
}

# Windows Package
print_info "Creating Windows x64 package..."
WIN_DIR="packages/TARR_Annunciator_Windows_x64"
mkdir -p "$WIN_DIR"

# Copy Windows executable
cp dist/windows/tarr-annunciator.exe "$WIN_DIR/"

# Copy Windows-specific scripts
cp build_windows.bat "$WIN_DIR/"
cp run_windows_go.bat "$WIN_DIR/run.bat"
cp install_windows.bat "$WIN_DIR/"
cp install_windows.ps1 "$WIN_DIR/"
cp cleanup.bat "$WIN_DIR/"

# Copy Windows documentation
cp README_Windows.md "$WIN_DIR/"
cp README_CrossPlatform.md "$WIN_DIR/"

# Copy core files
copy_core_files "$WIN_DIR"

# Create Windows ZIP package
cd packages
zip -r "TARR_Annunciator_Windows_x64.zip" "TARR_Annunciator_Windows_x64/" -q
cd ..
print_status "Windows package created: packages/TARR_Annunciator_Windows_x64.zip"

# Linux Package
print_info "Creating Linux x64 package..."
LINUX_DIR="packages/TARR_Annunciator_Linux_x64"
mkdir -p "$LINUX_DIR"

# Copy Linux executable
cp dist/linux/tarr-annunciator "$LINUX_DIR/"
chmod +x "$LINUX_DIR/tarr-annunciator"

# Copy Linux-specific scripts
cp build_linux.sh "$LINUX_DIR/"
cp run_linux.sh "$LINUX_DIR/run.sh"
cp test_crossplatform.sh "$LINUX_DIR/"
cp Makefile "$LINUX_DIR/"
chmod +x "$LINUX_DIR"/*.sh

# Copy Linux documentation
cp README_CrossPlatform.md "$LINUX_DIR/"
cp README_Go.md "$LINUX_DIR/"

# Copy core files
copy_core_files "$LINUX_DIR"

# Create Linux TAR.GZ package
cd packages
tar -czf "TARR_Annunciator_Linux_x64.tar.gz" "TARR_Annunciator_Linux_x64/"
cd ..
print_status "Linux package created: packages/TARR_Annunciator_Linux_x64.tar.gz"

# Raspberry Pi ARM64 Package
print_info "Creating Raspberry Pi ARM64 package..."
PI_ARM64_DIR="packages/TARR_Annunciator_RaspberryPi_ARM64"
mkdir -p "$PI_ARM64_DIR"

# Copy Pi ARM64 executable
cp dist/raspberry-pi/tarr-annunciator "$PI_ARM64_DIR/"
chmod +x "$PI_ARM64_DIR/tarr-annunciator"

# Copy Pi-specific scripts
cp install_raspberry_pi.sh "$PI_ARM64_DIR/"
cp run_raspberry_pi.sh "$PI_ARM64_DIR/run.sh"
cp build_linux.sh "$PI_ARM64_DIR/"
cp test_crossplatform.sh "$PI_ARM64_DIR/"
cp Makefile "$PI_ARM64_DIR/"
chmod +x "$PI_ARM64_DIR"/*.sh

# Copy Pi documentation
cp README_RaspberryPi.md "$PI_ARM64_DIR/"
cp README_CrossPlatform.md "$PI_ARM64_DIR/"

# Copy core files
copy_core_files "$PI_ARM64_DIR"

# Create Pi ARM64 TAR.GZ package
cd packages
tar -czf "TARR_Annunciator_RaspberryPi_ARM64.tar.gz" "TARR_Annunciator_RaspberryPi_ARM64/"
cd ..
print_status "Raspberry Pi ARM64 package created: packages/TARR_Annunciator_RaspberryPi_ARM64.tar.gz"

# Raspberry Pi ARM32 Package
print_info "Creating Raspberry Pi ARM32 package..."
PI_ARM32_DIR="packages/TARR_Annunciator_RaspberryPi_ARM32"
mkdir -p "$PI_ARM32_DIR"

# Copy Pi ARM32 executable
cp dist/raspberry-pi-32/tarr-annunciator "$PI_ARM32_DIR/"
chmod +x "$PI_ARM32_DIR/tarr-annunciator"

# Copy Pi-specific scripts (same as ARM64)
cp install_raspberry_pi.sh "$PI_ARM32_DIR/"
cp run_raspberry_pi.sh "$PI_ARM32_DIR/run.sh"
cp build_linux.sh "$PI_ARM32_DIR/"
cp test_crossplatform.sh "$PI_ARM32_DIR/"
cp Makefile "$PI_ARM32_DIR/"
chmod +x "$PI_ARM32_DIR"/*.sh

# Copy Pi documentation
cp README_RaspberryPi.md "$PI_ARM32_DIR/"
cp README_CrossPlatform.md "$PI_ARM32_DIR/"

# Copy core files
copy_core_files "$PI_ARM32_DIR"

# Create Pi ARM32 TAR.GZ package
cd packages
tar -czf "TARR_Annunciator_RaspberryPi_ARM32.tar.gz" "TARR_Annunciator_RaspberryPi_ARM32/"
cd ..
print_status "Raspberry Pi ARM32 package created: packages/TARR_Annunciator_RaspberryPi_ARM32.tar.gz"

# Raspberry Pi ARMv6 Package
print_info "Creating Raspberry Pi ARMv6 package..."
PI_ARMV6_DIR="packages/TARR_Annunciator_RaspberryPi_ARMv6"
mkdir -p "$PI_ARMV6_DIR"

# Copy Pi ARMv6 executable
cp dist/raspberry-pi-zero/tarr-annunciator "$PI_ARMV6_DIR/"
chmod +x "$PI_ARMV6_DIR/tarr-annunciator"

# Copy Pi-specific scripts (same as others)
cp install_raspberry_pi.sh "$PI_ARMV6_DIR/"
cp run_raspberry_pi.sh "$PI_ARMV6_DIR/run.sh"
cp build_linux.sh "$PI_ARMV6_DIR/"
cp test_crossplatform.sh "$PI_ARMV6_DIR/"
cp Makefile "$PI_ARMV6_DIR/"
chmod +x "$PI_ARMV6_DIR"/*.sh

# Copy Pi documentation
cp README_RaspberryPi.md "$PI_ARMV6_DIR/"
cp README_CrossPlatform.md "$PI_ARMV6_DIR/"

# Copy core files
copy_core_files "$PI_ARMV6_DIR"

# Create Pi ARMv6 TAR.GZ package
cd packages
tar -czf "TARR_Annunciator_RaspberryPi_ARMv6.tar.gz" "TARR_Annunciator_RaspberryPi_ARMv6/"
cd ..
print_status "Raspberry Pi ARMv6 package created: packages/TARR_Annunciator_RaspberryPi_ARMv6.tar.gz"

echo ""
echo "==========================================="
echo "🎉 Package Creation Complete!"
echo "==========================================="

print_info "Created packages:"
ls -lh packages/*.zip packages/*.tar.gz 2>/dev/null | while read -r line; do
    echo "  $line"
done

echo ""
print_info "Package sizes:"
du -h packages/*.zip packages/*.tar.gz 2>/dev/null

echo ""
print_info "Package contents verification:"
echo "Each package should contain:"
echo "  - Executable (platform-specific)"
echo "  - Configuration files (json/)"
echo "  - Web templates (templates/)"
echo "  - Audio files (static/mp3/)"
echo "  - Platform-specific scripts"
echo "  - Documentation"
echo "  - Empty logs/ directory"

echo ""
print_status "All packages ready for distribution!"
print_info "Upload these packages to your distribution platform"