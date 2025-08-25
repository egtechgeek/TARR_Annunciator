#!/bin/bash

echo "=== TARR Annunciator Cross-Platform Test ==="
echo "Testing platform detection and audio device enumeration..."
echo ""

# Test build for current platform
echo "1. Testing build for current platform..."
if make build; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi

# Test platform detection
echo ""
echo "2. Testing platform detection..."
echo "Platform: $(go env GOOS)/$(go env GOARCH)"

# Test cross-platform builds
echo ""
echo "3. Testing cross-platform builds..."

echo "Building for Windows..."
if GOOS=windows GOARCH=amd64 go build -o dist/windows/tarr-annunciator.exe .; then
    echo "✅ Windows build successful"
else
    echo "❌ Windows build failed"
fi

echo "Building for Linux..."
if GOOS=linux GOARCH=amd64 go build -o dist/linux/tarr-annunciator .; then
    echo "✅ Linux build successful"
else
    echo "❌ Linux build failed"
fi

echo "Building for macOS..."
if GOOS=darwin GOARCH=amd64 go build -o dist/darwin/tarr-annunciator .; then
    echo "✅ macOS build successful"
else
    echo "❌ macOS build failed"
fi

# List created binaries
echo ""
echo "4. Created binaries:"
if [ -d "dist" ]; then
    find dist -name "tarr-annunciator*" -type f -exec ls -lh {} \;
else
    echo "No dist directory found"
fi

# Test audio device detection (if possible)
echo ""
echo "5. Testing audio system detection..."

case "$(go env GOOS)" in
    "linux")
        echo "Linux detected - checking audio systems:"
        if command -v pactl &> /dev/null; then
            echo "✅ PulseAudio available (pactl found)"
        else
            echo "❌ PulseAudio not available"
        fi
        
        if command -v aplay &> /dev/null; then
            echo "✅ ALSA available (aplay found)"
        else
            echo "❌ ALSA not available"
        fi
        ;;
    "windows")
        echo "Windows detected - checking PowerShell modules:"
        if powershell -Command "Get-Module -ListAvailable -Name AudioDeviceCmdlets" &> /dev/null; then
            echo "✅ AudioDeviceCmdlets available"
        else
            echo "⚠️ AudioDeviceCmdlets not available (will use WMI fallback)"
        fi
        ;;
    "darwin")
        echo "macOS detected - checking system_profiler:"
        if command -v system_profiler &> /dev/null; then
            echo "✅ system_profiler available"
        else
            echo "❌ system_profiler not available"
        fi
        ;;
    *)
        echo "Unknown platform: $(go env GOOS)"
        ;;
esac

echo ""
echo "=== Cross-Platform Test Complete ==="
echo ""
echo "Next steps:"
echo "- Test the application: ./tarr-annunciator (or .exe on Windows)"
echo "- Check platform info: curl http://localhost:8080/api/platform"
echo "- Access admin panel: http://localhost:8080/admin"