# TARR Annunciator - Source Code

This directory contains all the source code and development files needed to build the TARR Annunciator application.

## Contents

### Go Source Files
- `main.go` - Main application entry point and web server
- `api.go` - REST API endpoints and handlers
- `audio.go` - Audio playback functions
- `audio_devices.go` - Audio device management
- `announcement_queue.go` - Announcement queuing and prioritization system
- `utils.go` - Utility functions for JSON handling, scheduling, and file operations

### Build Configuration
- `go.mod` - Go module dependencies
- `go.sum` - Go module checksums
- `Makefile` - Cross-platform build targets

### Build Scripts
- `build_linux.sh` - Linux build script
- `build_windows.bat` - Windows build script (verification only)
- `create_packages.sh` - Package creation script for all platforms
- `test_crossplatform.sh` - Cross-platform testing script

### Documentation
- `README_Go.md` - Go-specific development documentation
- `README_CrossPlatform.md` - Cross-platform development guide
- `PACKAGE_CONTENTS.md` - Package contents documentation
- `PACKAGING_SUMMARY.md` - Packaging process summary

### Package Manifests
- `package_windows.manifest` - Windows package manifest
- `package_linux.manifest` - Linux package manifest
- `package_raspberry_pi.manifest` - Raspberry Pi package manifest

## Building the Application

### Prerequisites
- Go 1.21 or higher
- Make (for using Makefile)

### Build Commands

#### Current Platform
```bash
cd source
go build -o tarr-annunciator .
```

#### Cross-Platform Builds
```bash
cd source
make build-windows    # Windows x64
make build-linux      # Linux x64
make build-raspberry-pi # ARM64 (Raspberry Pi 4/5)
```

#### All Platforms
```bash
cd source
make build-all
```

### Package Creation
```bash
cd source
./create_packages.sh
```

## Development

### Adding Dependencies
```bash
cd source
go mod tidy
```

### Testing
```bash
cd source
go test ./...
```

### Code Formatting
```bash
cd source
go fmt ./...
```

## Cross-Compilation Notes

The audio libraries (faiface/beep and hajimehoshi/oto) require platform-specific drivers. For successful cross-compilation:

1. **Windows to Linux/ARM**: Requires CGO and appropriate cross-compilation toolchain
2. **Native builds recommended**: Build on target platform for best results
3. **Docker alternative**: Use golang Docker images for cross-platform builds

## Directory Structure After Build

```
source/
├── dist/
│   ├── windows/tarr-annunciator.exe
│   ├── linux/tarr-annunciator
│   └── raspberry-pi/tarr-annunciator
└── packages/
    ├── TARR_Annunciator_Windows_x64.zip
    ├── TARR_Annunciator_Linux_x64.tar.gz
    └── TARR_Annunciator_RaspberryPi_ARM64.tar.gz
```

## Notes

- All source code is required for building
- The parent directory contains runtime files and install packages
- Built executables are placed in `dist/` subdirectory
- Distribution packages are created in `packages/` subdirectory