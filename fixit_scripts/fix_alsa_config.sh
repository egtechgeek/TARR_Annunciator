#!/bin/bash

# Quick fix for corrupted ALSA configuration
echo "🔧 Fixing corrupted ALSA configuration..."

# Backup and remove the broken config
sudo mv /etc/asound.conf /etc/asound.conf.backup 2>/dev/null || true

# Create a simple, working ALSA config for USB Sound Blaster
sudo tee /etc/asound.conf > /dev/null << 'EOF'
# Simple ALSA configuration for USB Sound Blaster Play! 3
pcm.!default {
    type hw
    card 0
}

ctl.!default {
    type hw
    card 0
}

# USB Sound Blaster specific
pcm.usb {
    type hw
    card S3
}

ctl.usb {
    type hw
    card S3
}
EOF

echo "✅ ALSA configuration fixed"

# Test the fix
echo "Testing ALSA configuration..."
if aplay -l > /dev/null 2>&1; then
    echo "✅ ALSA is working"
    aplay -l
else
    echo "❌ ALSA still has issues"
fi

# Test alsamixer
echo ""
echo "Testing alsamixer (press 'q' to quit if it opens)..."
timeout 3 alsamixer 2>/dev/null || echo "alsamixer test completed"

echo ""
echo "Now restart the annunciator session:"
echo "1. screen -r annunciator-system"
echo "2. Press Ctrl+C to stop"
echo "3. ./screen_start_annunciator.sh"
echo "4. Test TTS from web interface"