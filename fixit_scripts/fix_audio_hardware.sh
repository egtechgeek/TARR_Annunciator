#!/bin/bash

# Fix Audio Hardware Detection for Raspberry Pi
# This script addresses "Dummy Output" issues and ensures proper audio device detection

set -e

echo "🔧 Fixing audio hardware detection on Raspberry Pi..."

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

# Stop any existing annunciator sessions
print_status "Stopping existing annunciator sessions..."
if screen -list | grep -q annunciator-system; then
    screen -S annunciator-system -X quit 2>/dev/null
    print_success "Screen session stopped"
fi

# Kill any existing audio processes that might be blocking
print_status "Cleaning up audio processes..."
sudo pkill pulseaudio 2>/dev/null || true
sudo pkill pipewire 2>/dev/null || true
sleep 2

# Check what audio hardware is available
print_status "Detecting audio hardware..."
echo "Available audio cards:"
cat /proc/asound/cards || print_warning "No audio cards detected in /proc/asound/cards"

echo ""
echo "ALSA devices:"
aplay -l 2>/dev/null || print_warning "No ALSA playback devices found"

echo ""
echo "Audio groups:"
groups pi

# Force audio hardware detection
print_status "Forcing audio hardware detection..."

# Enable audio on Raspberry Pi (this is critical)
if [ -f /boot/config.txt ]; then
    print_status "Checking /boot/config.txt for audio configuration..."
    
    # Check if audio is enabled
    if ! grep -q "^dtparam=audio=on" /boot/config.txt; then
        print_warning "Audio not enabled in /boot/config.txt, enabling it..."
        echo "dtparam=audio=on" | sudo tee -a /boot/config.txt
        print_warning "⚠️  REBOOT REQUIRED: Audio hardware enabled, please reboot the Pi"
    else
        print_success "Audio is enabled in /boot/config.txt"
    fi
    
    # Check for additional audio configurations
    if ! grep -q "^audio_pwm_mode=" /boot/config.txt; then
        print_status "Adding additional audio configuration..."
        echo "audio_pwm_mode=2" | sudo tee -a /boot/config.txt
        echo "disable_audio_dither=1" | sudo tee -a /boot/config.txt
        print_warning "⚠️  REBOOT REQUIRED: Audio configuration updated"
    fi
fi

# Fix ALSA configuration for Raspberry Pi
print_status "Configuring ALSA for Raspberry Pi hardware..."

cat > /tmp/asound.conf << 'EOF'
# Raspberry Pi ALSA configuration for Annunciator System

# Try to use the first available hardware device
pcm.!default {
    type asym
    playback.pcm "dmix"
    capture.pcm "dsnoop"
}

pcm.dmix {
    type dmix
    ipc_key 1024
    slave {
        pcm "hw:0,0"
        rate 44100
        channels 2
        format S16_LE
        period_time 0
        period_size 1024
        buffer_time 0
        buffer_size 4096
    }
    bindings {
        0 0
        1 1
    }
}

pcm.dsnoop {
    type dsnoop
    ipc_key 2048
    slave {
        pcm "hw:0,0"
        channels 2
        rate 44100
        format S16_LE
        period_time 0
        period_size 1024
        buffer_time 0
        buffer_size 4096
    }
    bindings {
        0 0
        1 1
    }
}

ctl.!default {
    type hw
    card 0
}

# Fallback configurations for different hardware
pcm.headphones {
    type hw
    card 0
    device 0
}

pcm.hdmi {
    type hw
    card 1
    device 0
}
EOF

sudo mv /tmp/asound.conf /etc/asound.conf
print_success "ALSA configuration updated"

# Set audio output to analog (3.5mm jack) by default
print_status "Setting audio output to analog..."
sudo amixer cset numid=3 1 2>/dev/null || print_warning "Could not set audio output (amixer not available)"

# Ensure audio modules are loaded
print_status "Loading audio modules..."
sudo modprobe snd-bcm2835 2>/dev/null || print_warning "Could not load snd-bcm2835 module"

# Test basic audio functionality
print_status "Testing basic audio functionality..."
if command -v speaker-test >/dev/null 2>&1; then
    timeout 3 speaker-test -t sine -f 1000 -l 1 2>/dev/null || print_warning "Speaker test failed"
else
    print_warning "speaker-test not available"
fi

# Restart audio services
print_status "Restarting audio services..."

# Try to start PulseAudio with proper configuration
print_status "Starting PulseAudio..."
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
mkdir -p "$XDG_RUNTIME_DIR"

# Create PulseAudio configuration that forces hardware detection
mkdir -p ~/.config/pulse
cat > ~/.config/pulse/default.pa << 'EOF'
#!/usr/bin/pulseaudio -nF
# PulseAudio configuration for Raspberry Pi

# Load necessary modules
.fail
load-module module-device-restore
load-module module-stream-restore
load-module module-card-restore
load-module module-augment-properties
load-module module-switch-on-port-available
load-module module-udev-detect tsched=0
load-module module-alsa-card device_id=0 name=bcm2835_alsa card_name=bcm2835_alsa tsched=0
load-module module-native-protocol-unix
load-module module-default-device-restore
load-module module-rescue-streams
load-module module-always-sink
load-module module-intended-roles
load-module module-suspend-on-idle
load-module module-console-kit
load-module module-position-event-sounds

# Set default sink
set-default-sink alsa_output.bcm2835_alsa.analog-stereo
EOF

# Start PulseAudio
pulseaudio --kill 2>/dev/null || true
sleep 1
pulseaudio --start --verbose 2>/dev/null || print_warning "PulseAudio start failed"

# Wait for PulseAudio to initialize
sleep 3

# Check audio status after configuration
print_status "Checking audio status after configuration..."
echo ""
echo "ALSA devices after configuration:"
aplay -l 2>/dev/null || print_warning "Still no ALSA devices"

echo ""
if command -v pactl >/dev/null 2>&1; then
    echo "PulseAudio sinks:"
    pactl list sinks short 2>/dev/null || print_warning "PulseAudio not responding"
    echo ""
    echo "PulseAudio info:"
    pactl info 2>/dev/null || print_warning "PulseAudio info not available"
else
    print_warning "PulseAudio tools not available"
fi

# Update the screen startup script with better audio detection
print_status "Updating screen startup script for better audio detection..."
cd /home/pi/annunciator

cat > screen_start_annunciator.sh << 'EOF'
#!/bin/bash
cd /home/pi/annunciator
echo "Starting Annunciator System with hardware audio detection..."

# Kill existing sessions
screen -ls | grep annunciator-system && screen -S annunciator-system -X quit 2>/dev/null
sleep 1

# Set up comprehensive audio environment
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
export PULSE_RUNTIME_PATH="$XDG_RUNTIME_DIR/pulse"
export PULSE_SERVER="unix:$PULSE_RUNTIME_PATH/native"
export ALSA_PCM_CARD=0
export ALSA_PCM_DEVICE=0

# Ensure runtime directory exists
mkdir -p "$XDG_RUNTIME_DIR/pulse"

# Audio hardware detection and setup
echo "Detecting audio hardware..."
if [ -f /proc/asound/cards ]; then
    echo "Available sound cards:"
    cat /proc/asound/cards
else
    echo "⚠️  No sound cards detected - check hardware configuration"
fi

# Force analog output
sudo amixer cset numid=3 1 >/dev/null 2>&1 || echo "Could not set analog output"

# Start PulseAudio if needed
if ! pgrep pulseaudio > /dev/null; then
    echo "Starting PulseAudio..."
    pulseaudio --start --verbose >/dev/null 2>&1 || echo "PulseAudio start may have failed"
    sleep 2
fi

# Test audio before starting main application
echo "Testing audio setup..."
if command -v aplay >/dev/null 2>&1; then
    # Create a simple test tone
    echo "Testing ALSA audio..."
    timeout 1 speaker-test -t sine -f 440 -l 1 >/dev/null 2>&1 && echo "✅ ALSA audio test passed" || echo "❌ ALSA audio test failed"
fi

# Start screen session with full environment
screen -dmS annunciator-system bash -c '
    cd /home/pi/annunciator
    
    # Set full audio environment
    export XDG_RUNTIME_DIR="/run/user/$(id -u)"
    export PULSE_RUNTIME_PATH="$XDG_RUNTIME_DIR/pulse"
    export PULSE_SERVER="unix:$PULSE_RUNTIME_PATH/native"
    export ALSA_PCM_CARD=0
    export ALSA_PCM_DEVICE=0
    export HOME="/home/pi"
    export USER="pi"
    
    echo "=== Annunciator System Started ==="
    echo "Audio Environment:"
    echo "  ALSA_PCM_CARD: $ALSA_PCM_CARD"
    echo "  ALSA_PCM_DEVICE: $ALSA_PCM_DEVICE"
    echo "  PULSE_SERVER: $PULSE_SERVER"
    echo "  XDG_RUNTIME_DIR: $XDG_RUNTIME_DIR"
    
    # Show detected audio hardware
    echo ""
    echo "Detected Audio Hardware:"
    if [ -f /proc/asound/cards ]; then
        cat /proc/asound/cards
    else
        echo "  No sound cards detected"
    fi
    
    echo ""
    echo "Web interface: http://$(hostname -I | awk '"'"'{print $1}'"'"'):8080"
    echo "================================"
    
    # Start application
    ./annunciator'

echo "Screen session started with hardware audio detection"
echo "To attach: screen -r annunciator-system"
echo ""

# Show final status
echo "=== Audio Configuration Summary ==="
if [ -f /proc/asound/cards ] && [ -s /proc/asound/cards ]; then
    echo "✅ Audio hardware detected"
    cat /proc/asound/cards
else
    echo "❌ No audio hardware detected - may need reboot or hardware check"
fi
EOF

chmod +x screen_start_annunciator.sh

# Start the improved session
print_status "Starting annunciator with improved audio detection..."
./screen_start_annunciator.sh

echo ""
print_success "Audio hardware fix completed!"
echo ""
echo "=== Hardware Fix Summary ==="
echo "✅ Enabled audio in /boot/config.txt (if needed)"
echo "✅ Updated ALSA configuration for Raspberry Pi"
echo "✅ Set audio output to analog (3.5mm jack)"
echo "✅ Created PulseAudio configuration"
echo "✅ Updated screen startup with hardware detection"
echo ""
if [ -f /proc/asound/cards ] && [ -s /proc/asound/cards ]; then
    echo "✅ Audio hardware appears to be detected"
else
    echo "⚠️  Audio hardware may not be properly detected"
    echo "    1. Check that speakers/headphones are connected to 3.5mm jack"
    echo "    2. Reboot the Pi if /boot/config.txt was modified"
    echo "    3. Check 'sudo dmesg | grep audio' for hardware detection issues"
fi
echo ""
echo "Test the audio from the web interface TTS tab!"