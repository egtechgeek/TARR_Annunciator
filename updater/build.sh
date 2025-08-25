#!/bin/bash

echo "Building TARR Annunciator Updater..."

# Build for Windows
echo "Building Windows x64..."
GOOS=windows GOARCH=amd64 go build -o tarr-updater-windows-x64.exe .
if [ $? -ne 0 ]; then
    echo "Error: Windows build failed"
    exit 1
fi

# Build for Linux x64
echo "Building Linux x64..."
GOOS=linux GOARCH=amd64 go build -o tarr-updater-linux-x64 .
if [ $? -ne 0 ]; then
    echo "Error: Linux build failed"
    exit 1
fi

# Build for Raspberry Pi ARM64
echo "Building Raspberry Pi ARM64..."
GOOS=linux GOARCH=arm64 go build -o tarr-updater-raspberry-pi-arm64 .
if [ $? -ne 0 ]; then
    echo "Error: Raspberry Pi ARM64 build failed"
    exit 1
fi

# Build for Raspberry Pi ARM32
echo "Building Raspberry Pi ARM32..."
GOOS=linux GOARCH=arm GOARM=7 go build -o tarr-updater-raspberry-pi-arm32 .
if [ $? -ne 0 ]; then
    echo "Error: Raspberry Pi ARM32 build failed"
    exit 1
fi

echo ""
echo "All updater builds completed successfully!"
echo ""
echo "Built files:"
ls -la tarr-updater-*
echo ""