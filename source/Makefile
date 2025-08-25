# TARR Annunciator Cross-Platform Build

.PHONY: all clean build-windows build-linux build-darwin run test

# Default target
all: build

# Detect current platform
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Build for current platform
build:
	@echo "Building for current platform ($(GOOS)/$(GOARCH))..."
	go mod download
	go build -o tarr-annunciator$(if $(filter windows,$(GOOS)),.exe) .
	@echo "Build completed: tarr-annunciator$(if $(filter windows,$(GOOS)),.exe)"

# Build for all platforms
build-all: build-windows build-linux build-darwin

# Build for all ARM platforms (Raspberry Pi focused)
build-arm-all: build-raspberry-pi build-raspberry-pi-32 build-raspberry-pi-zero

# Windows build
build-windows:
	@echo "Building for Windows..."
	@mkdir -p dist/windows
	GOOS=windows GOARCH=amd64 go build -o dist/windows/tarr-annunciator.exe .
	@echo "Windows build completed: dist/windows/tarr-annunciator.exe"

# Linux build  
build-linux:
	@echo "Building for Linux..."
	@mkdir -p dist/linux
	GOOS=linux GOARCH=amd64 go build -o dist/linux/tarr-annunciator .
	@echo "Linux build completed: dist/linux/tarr-annunciator"

# macOS build
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p dist/darwin
	GOOS=darwin GOARCH=amd64 go build -o dist/darwin/tarr-annunciator .
	@echo "macOS build completed: dist/darwin/tarr-annunciator"

# ARM builds for Raspberry Pi and other ARM devices
build-raspberry-pi:
	@echo "Building for Raspberry Pi (ARM64)..."
	@mkdir -p dist/raspberry-pi
	GOOS=linux GOARCH=arm64 go build -o dist/raspberry-pi/tarr-annunciator .
	@echo "Raspberry Pi ARM64 build completed: dist/raspberry-pi/tarr-annunciator"

build-raspberry-pi-32:
	@echo "Building for Raspberry Pi 32-bit (ARM)..."
	@mkdir -p dist/raspberry-pi-32
	GOOS=linux GOARCH=arm GOARM=7 go build -o dist/raspberry-pi-32/tarr-annunciator .
	@echo "Raspberry Pi ARM32 build completed: dist/raspberry-pi-32/tarr-annunciator"

build-raspberry-pi-zero:
	@echo "Building for Raspberry Pi Zero (ARMv6)..."
	@mkdir -p dist/raspberry-pi-zero
	GOOS=linux GOARCH=arm GOARM=6 go build -o dist/raspberry-pi-zero/tarr-annunciator .
	@echo "Raspberry Pi Zero ARMv6 build completed: dist/raspberry-pi-zero/tarr-annunciator"

# ARM64 builds
build-windows-arm64:
	@echo "Building for Windows ARM64..."
	@mkdir -p dist/windows-arm64
	GOOS=windows GOARCH=arm64 go build -o dist/windows-arm64/tarr-annunciator.exe .

build-linux-arm64:
	@echo "Building for Linux ARM64..."
	@mkdir -p dist/linux-arm64
	GOOS=linux GOARCH=arm64 go build -o dist/linux-arm64/tarr-annunciator .

build-darwin-arm64:
	@echo "Building for macOS ARM64 (Apple Silicon)..."
	@mkdir -p dist/darwin-arm64
	GOOS=darwin GOARCH=arm64 go build -o dist/darwin-arm64/tarr-annunciator .

# ARM32 builds
build-linux-arm32:
	@echo "Building for Linux ARM32..."
	@mkdir -p dist/linux-arm32
	GOOS=linux GOARCH=arm GOARM=7 go build -o dist/linux-arm32/tarr-annunciator .

build-linux-armv6:
	@echo "Building for Linux ARMv6..."
	@mkdir -p dist/linux-armv6
	GOOS=linux GOARCH=arm GOARM=6 go build -o dist/linux-armv6/tarr-annunciator .

# Run application
run: build
	./tarr-annunciator$(if $(filter windows,$(GOOS)),.exe)

# Test audio devices
test-audio:
	@echo "Testing audio device detection..."
	@go run . --test-devices || echo "Add --test-devices flag support to main.go"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f tarr-annunciator tarr-annunciator.exe
	@rm -rf dist/
	@echo "Clean completed"

# Dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Display help
help:
	@echo "TARR Annunciator Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  build                - Build for current platform"
	@echo "  build-all            - Build for all major platforms"
	@echo "  build-arm-all        - Build for all ARM/Raspberry Pi platforms"
	@echo ""
	@echo "Platform-specific builds:"
	@echo "  build-windows        - Build for Windows x64"
	@echo "  build-linux          - Build for Linux x64"
	@echo "  build-darwin         - Build for macOS x64"
	@echo ""
	@echo "Raspberry Pi builds:"
	@echo "  build-raspberry-pi   - Build for Raspberry Pi 4/5 (ARM64)"
	@echo "  build-raspberry-pi-32- Build for Raspberry Pi 2/3 (ARM32)"
	@echo "  build-raspberry-pi-zero - Build for Raspberry Pi Zero (ARMv6)"
	@echo ""
	@echo "ARM builds:"
	@echo "  build-linux-arm64    - Build for Linux ARM64"
	@echo "  build-linux-arm32    - Build for Linux ARM32"
	@echo "  build-linux-armv6    - Build for Linux ARMv6"
	@echo ""
	@echo "Development:"
	@echo "  run                  - Build and run application"
	@echo "  clean                - Remove build artifacts"
	@echo "  deps                 - Download and tidy dependencies"
	@echo "  fmt                  - Format source code"
	@echo "  vet                  - Vet source code"
	@echo "  help                 - Display this help"