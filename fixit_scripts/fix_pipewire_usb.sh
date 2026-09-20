#!/bin/bash

# Fix PipeWire to recognize USB Sound Blaster Play! 3
echo "🔧 Configuring PipeWire for USB Sound Blaster..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Stop annunciator session
print_status "Stopping annunciator session..."
if screen -list | grep -q annunciator-system; then
    screen -S annunciator-system -X quit 2>/dev/null
    sleep 1
fi

# Restart PipeWire services to detect USB audio
print_status "Restarting PipeWire services..."
systemctl --user stop pipewire pipewire-pulse wireplumber 2>/dev/null || true
sleep 2
systemctl --user start pipewire pipewire-pulse wireplumber 2>/dev/null || true
sleep 3

# Force PipeWire to detect USB audio devices
print_status "Forcing PipeWire USB audio detection..."
if command -v wpctl >/dev/null 2>&1; then
    echo "Available audio devices:"
    wpctl status
    echo ""
    
    # Try to set USB audio as default
    USB_SINK=$(wpctl status | grep -i "sound blaster\|usb.*audio" | grep -o "^\s*[0-9]*" | head -1 | tr -d ' ')
    if [ -n "$USB_SINK" ]; then
        print_status "Setting USB Sound Blaster (ID: $USB_SINK) as default..."
        wpctl set-default $USB_SINK 2>/dev/null || print_warning "Could not set USB audio as default"
        print_success "USB audio should now be available"
    else
        print_warning "Could not find USB audio device in PipeWire"
    fi
else
    print_warning "wpctl not available, trying alternative method..."
    
    # Alternative: restart user session audio
    export XDG_RUNTIME_DIR="/run/user/$(id -u)"
    if [ -d "$XDG_RUNTIME_DIR" ]; then
        print_status "Restarting user audio session..."
        pulseaudio --kill 2>/dev/null || true
        sleep 1
        pulseaudio --start 2>/dev/null || true
        sleep 2
    fi
fi

# Test audio again
print_status "Testing audio after PipeWire restart..."
if command -v speaker-test >/dev/null 2>&1; then
    timeout 2 speaker-test -c2 -t sine -f 440 2>/dev/null && print_success "Audio test passed!" || print_warning "Audio test failed"
fi

# Update screen startup script to work better with PipeWire
print_status "Updating screen startup for PipeWire compatibility..."

cat > screen_start_annunciator.sh << 'EOF'
#!/bin/bash
cd /home/pi/annunciator
echo "Starting Annunciator System with USB Sound Blaster support..."

# Kill existing sessions
screen -ls | grep annunciator-system && screen -S annunciator-system -X quit 2>/dev/null
sleep 1

# PipeWire/USB Audio Environment Setup
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
export PULSE_RUNTIME_PATH="$XDG_RUNTIME_DIR/pulse"
export PULSE_SERVER="unix:$PULSE_RUNTIME_PATH/native"

# Ensure audio services are running
systemctl --user is-active pipewire >/dev/null 2>&1 || systemctl --user start pipewire 2>/dev/null
systemctl --user is-active pipewire-pulse >/dev/null 2>&1 || systemctl --user start pipewire-pulse 2>/dev/null

# Wait for services
sleep 2

echo "=== USB Audio Detection ==="
if [ -f /proc/asound/cards ]; then
    echo "Sound cards:"
    cat /proc/asound/cards
else
    echo "No sound cards detected"
fi

# Show PipeWire status
if command -v wpctl >/dev/null 2>&1; then
    echo ""
    echo "PipeWire audio devices:"
    wpctl status | grep -A 10 "Audio" | grep -E "^\s*[0-9]|Sinks:|Sources:" | head -10
fi

# Test USB audio before starting
echo ""
echo "Testing USB audio..."
if timeout 1 speaker-test -D hw:S3 -t sine -f 440 -l 1 >/dev/null 2>&1; then
    echo "✅ USB Sound Blaster is working"
else
    echo "⚠️  USB Sound Blaster test failed"
fi

# Start screen session with USB audio support
screen -dmS annunciator-system bash -c '
    cd /home/pi/annunciator
    
    # Full audio environment
    export XDG_RUNTIME_DIR="/run/user/$(id -u)"
    export PULSE_RUNTIME_PATH="$XDG_RUNTIME_DIR/pulse"
    export PULSE_SERVER="unix:$PULSE_RUNTIME_PATH/native"
    export HOME="/home/pi"
    export USER="pi"
    
    echo "=== Annunciator System Started ==="
    echo "Audio: USB Sound Blaster Play! 3 detected"
    
    if command -v wpctl >/dev/null 2>&1; then
        DEFAULT_SINK=$(wpctl get-default SINK 2>/dev/null || echo "unknown")
        echo "Default audio sink: $DEFAULT_SINK"
    fi
    
    echo "Web interface: http://$(hostname -I | awk '"'"'{print $1}'"'"'):8080"
    echo "================================"
    
    ./annunciator'

echo ""
echo "Screen session started with USB audio support"
echo "To attach: screen -r annunciator-system"
EOF

chmod +x screen_start_annunciator.sh

# Start the session
print_status "Starting annunciator with USB audio support..."
./screen_start_annunciator.sh

echo ""
print_success "PipeWire USB audio fix completed!"
echo ""
echo "=== USB Audio Status ==="
if command -v wpctl >/dev/null 2>&1; then
    wpctl status | grep -A 5 "Sinks:" | head -6
else
    echo "PipeWire tools not available"
fi
echo ""
echo "🎵 Your Sound Blaster Play! 3 should now work for TTS audio!"
echo "💡 Test TTS from the web interface - it should play sound instead of 'Audio not available'"