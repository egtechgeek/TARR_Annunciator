#!/bin/bash

# Audio Fix for Raspberry Pi 5 (No 3.5mm jack)
# Pi 5 only supports HDMI audio or USB audio devices

set -e

echo "🔧 Fixing audio for Raspberry Pi 5 (HDMI/USB audio only)..."

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

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

if [ "$USER" != "pi" ]; then
    print_error "This script must be run as the 'pi' user"
    exit 1
fi

# Stop existing sessions
print_status "Stopping existing annunciator sessions..."
if screen -list | grep -q annunciator-system; then
    screen -S annunciator-system -X quit 2>/dev/null
    print_success "Screen session stopped"
fi

# Kill audio processes
print_status "Cleaning up audio processes..."
sudo pkill pulseaudio 2>/dev/null || true
sudo pkill pipewire 2>/dev/null || true
sleep 2

# Detect available audio hardware on Pi 5
print_status "Detecting Raspberry Pi 5 audio hardware..."
echo ""
echo "Available audio devices:"
if [ -f /proc/asound/cards ]; then
    cat /proc/asound/cards
else
    print_warning "No audio cards detected"
fi

echo ""
echo "USB audio devices:"
lsusb | grep -i audio || echo "No USB audio devices found"

echo ""
echo "HDMI audio status:"
aplay -l 2>/dev/null | grep -i hdmi || echo "No HDMI audio detected"

# Configure audio for Pi 5
print_status "Configuring audio for Raspberry Pi 5..."

# Set HDMI audio as primary (Pi 5 typically uses HDMI 0)
print_status "Setting HDMI audio as primary output..."
if command -v amixer >/dev/null 2>&1; then
    # Try to set HDMI audio
    amixer -c 0 set 'PCM' 100% 2>/dev/null || print_warning "Could not set PCM volume"
else
    print_warning "amixer not available"
fi

# Create Pi 5 specific ALSA configuration
print_status "Creating Pi 5 ALSA configuration..."
cat > /tmp/asound.conf << 'EOF'
# Raspberry Pi 5 ALSA Configuration
# Pi 5 supports HDMI and USB audio only (no 3.5mm jack)

# Default to first available device (usually HDMI)
pcm.!default {
    type asym
    playback.pcm "dmix"
}

pcm.dmix {
    type dmix
    ipc_key 1024
    slave {
        pcm "hw:0,0"  # First HDMI output
        rate 44100
        channels 2
        format S16_LE
        period_time 0
        period_size 1024
        buffer_time 0
        buffer_size 4096
    }
}

ctl.!default {
    type hw
    card 0
}

# HDMI outputs (Pi 5 has multiple HDMI ports)
pcm.hdmi0 {
    type hw
    card 0
    device 0
}

pcm.hdmi1 {
    type hw
    card 0
    device 1
}

# USB audio fallback
pcm.usb {
    type hw
    card 1
    device 0
}
EOF

sudo mv /tmp/asound.conf /etc/asound.conf
print_success "ALSA configuration updated for Pi 5"

# Configure PulseAudio for Pi 5
print_status "Configuring PulseAudio for Pi 5..."
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
mkdir -p "$XDG_RUNTIME_DIR"
mkdir -p ~/.config/pulse

cat > ~/.config/pulse/default.pa << 'EOF'
#!/usr/bin/pulseaudio -nF
# PulseAudio configuration for Raspberry Pi 5

.fail
load-module module-device-restore
load-module module-stream-restore
load-module module-card-restore
load-module module-augment-properties
load-module module-switch-on-port-available

# Detect all available audio devices
load-module module-udev-detect tsched=0

# Load specific HDMI audio for Pi 5
load-module module-alsa-sink device=hw:0,0 sink_name=hdmi0 sink_properties=device.description="HDMI0"
load-module module-alsa-sink device=hw:0,1 sink_name=hdmi1 sink_properties=device.description="HDMI1"

# Protocol modules
load-module module-native-protocol-unix
load-module module-default-device-restore
load-module module-rescue-streams
load-module module-always-sink
load-module module-intended-roles
load-module module-suspend-on-idle

# Set default sink to first HDMI
set-default-sink hdmi0
EOF

# Start PulseAudio
print_status "Starting PulseAudio..."
pulseaudio --kill 2>/dev/null || true
sleep 1
pulseaudio --start --verbose 2>/dev/null || print_warning "PulseAudio start may have failed"
sleep 3

# Test audio setup
print_status "Testing audio setup..."
echo ""
if command -v pactl >/dev/null 2>&1; then
    echo "Available PulseAudio sinks:"
    pactl list sinks short 2>/dev/null || print_warning "PulseAudio not responding"
    echo ""
    echo "Default sink:"
    pactl get-default-sink 2>/dev/null || print_warning "Could not get default sink"
else
    print_warning "PulseAudio tools not available"
fi

# Test basic audio
if command -v speaker-test >/dev/null 2>&1; then
    print_status "Running brief audio test..."
    timeout 2 speaker-test -t sine -f 1000 -l 1 2>/dev/null || print_warning "Audio test failed"
else
    print_warning "speaker-test not available"
fi

# Create Pi 5 optimized screen startup script
print_status "Creating Pi 5 optimized screen startup script..."
cd /home/pi/annunciator

cat > screen_start_annunciator.sh << 'EOF'
#!/bin/bash
cd /home/pi/annunciator
echo "Starting Annunciator System on Raspberry Pi 5..."

# Kill existing sessions
screen -ls | grep annunciator-system && screen -S annunciator-system -X quit 2>/dev/null
sleep 1

# Pi 5 Audio Environment Setup
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
export PULSE_RUNTIME_PATH="$XDG_RUNTIME_DIR/pulse"
export PULSE_SERVER="unix:$PULSE_RUNTIME_PATH/native"

# Ensure PulseAudio is running
if ! pgrep pulseaudio > /dev/null; then
    echo "Starting PulseAudio for Pi 5..."
    pulseaudio --start --verbose >/dev/null 2>&1
    sleep 2
fi

# Show audio hardware detection
echo "=== Pi 5 Audio Hardware Detection ==="
if [ -f /proc/asound/cards ]; then
    echo "Detected sound cards:"
    cat /proc/asound/cards
else
    echo "⚠️  No sound cards detected"
fi

echo ""
if command -v pactl >/dev/null 2>&1; then
    echo "Available audio outputs:"
    pactl list sinks short 2>/dev/null | grep -v "Dummy" || echo "No audio outputs (using dummy)"
    echo ""
    echo "Default audio output:"
    pactl get-default-sink 2>/dev/null || echo "No default sink"
else
    echo "PulseAudio not available for audio detection"
fi

# Test audio before starting
echo ""
echo "Testing audio output..."
if command -v pactl >/dev/null 2>&1 && pactl list sinks short 2>/dev/null | grep -v "Dummy" >/dev/null; then
    echo "✅ Real audio output detected"
    # Quick test
    timeout 1 speaker-test -t sine -f 440 -l 1 >/dev/null 2>&1 && echo "✅ Audio test successful" || echo "⚠️  Audio test failed"
else
    echo "❌ Only dummy output available - check HDMI/USB audio connection"
fi

# Start screen session
screen -dmS annunciator-system bash -c '
    cd /home/pi/annunciator
    
    # Full environment setup
    export XDG_RUNTIME_DIR="/run/user/$(id -u)"
    export PULSE_RUNTIME_PATH="$XDG_RUNTIME_DIR/pulse"
    export PULSE_SERVER="unix:$PULSE_RUNTIME_PATH/native"
    export HOME="/home/pi"
    export USER="pi"
    
    echo "=== Annunciator System Started on Pi 5 ==="
    echo "Audio Environment:"
    echo "  PULSE_SERVER: $PULSE_SERVER"
    echo "  XDG_RUNTIME_DIR: $XDG_RUNTIME_DIR"
    
    if command -v pactl >/dev/null 2>&1; then
        echo "  Default sink: $(pactl get-default-sink 2>/dev/null || echo "none")"
    fi
    
    echo "Web interface: http://$(hostname -I | awk '"'"'{print $1}'"'"'):8080"
    echo "================================"
    
    ./annunciator'

echo ""
echo "Screen session started: annunciator-system"
echo "To attach: screen -r annunciator-system"
EOF

chmod +x screen_start_annunciator.sh

# Show current audio status
echo ""
print_success "Pi 5 audio configuration completed!"
echo ""
echo "=== Pi 5 Audio Status ==="
if [ -f /proc/asound/cards ] && [ -s /proc/asound/cards ]; then
    echo "✅ Audio hardware detected:"
    cat /proc/asound/cards
else
    echo "❌ No audio hardware detected"
fi

echo ""
if command -v pactl >/dev/null 2>&1; then
    echo "PulseAudio sinks:"
    pactl list sinks short 2>/dev/null | head -5
else
    echo "PulseAudio not available"
fi

echo ""
echo "=== Pi 5 Audio Connection Guide ==="
echo "Raspberry Pi 5 audio options:"
echo "1. 🖥️  HDMI Audio - Connect Pi to HDMI monitor/TV with speakers"
echo "2. 🔌 USB Audio - Connect USB speakers/headphones/sound card" 
echo "3. 🎧 USB-C Audio - Use USB-C to 3.5mm adapter"
echo "4. 📶 Bluetooth Audio - Pair Bluetooth speakers/headphones"
echo ""
echo "💡 If you see 'Dummy Output', you need to connect one of the above!"

# Start the session
print_status "Starting optimized Pi 5 session..."
./screen_start_annunciator.sh

echo ""
print_warning "Next Steps:"
echo "1. Connect HDMI monitor with speakers OR USB audio device"
echo "2. Check that audio output shows up (not 'Dummy')"
echo "3. Test TTS from web interface"
echo "4. If still 'Dummy Output', try: sudo reboot"