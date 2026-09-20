#!/bin/bash

# Fix Screen Session Audio Access for Annunciator System
# Run this script on the Raspberry Pi to fix audio issues with screen sessions

set -e

echo "🔧 Fixing audio access for screen sessions..."

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

cd /home/pi/annunciator

# Stop existing screen session
print_status "Stopping existing screen session..."
if screen -list | grep -q annunciator-system; then
    screen -S annunciator-system -X quit 2>/dev/null
    print_success "Existing session stopped"
else
    print_status "No existing session found"
fi

# Create improved screen startup script with audio fixes
print_status "Creating improved screen startup script with audio fixes..."

cat > screen_start_annunciator.sh << 'EOF'
#!/bin/bash
cd /home/pi/annunciator
echo "Starting Annunciator System in GNU Screen session with audio support..."

# Kill any existing screen sessions
screen -ls | grep annunciator-system && screen -S annunciator-system -X quit 2>/dev/null
sleep 1

# Set up audio environment for screen session
export PULSE_RUNTIME_PATH="/run/user/$(id -u)/pulse"
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
export PULSE_SERVER="unix:${XDG_RUNTIME_DIR}/pulse/native"

# Ensure runtime directory exists
mkdir -p "${XDG_RUNTIME_DIR}/pulse" 2>/dev/null

# Ensure audio permissions
sudo usermod -a -G audio,pulse,pulse-access pi 2>/dev/null

# Start PulseAudio if not running (for pi user)
if ! pgrep -x "pulseaudio" > /dev/null; then
    echo "Starting PulseAudio for audio access..."
    pulseaudio --start --verbose 2>/dev/null || echo "PulseAudio may already be running"
fi

# Test audio access before starting
echo "Testing audio access..."
if command -v pactl >/dev/null 2>&1; then
    if pactl info >/dev/null 2>&1; then
        echo "✅ PulseAudio connection successful"
    else
        echo "⚠️  PulseAudio connection failed, but continuing..."
    fi
fi

# Start new screen session with full audio environment
screen -dmS annunciator-system bash -c '
    cd /home/pi/annunciator
    
    # Set up full audio environment inside screen
    export PULSE_RUNTIME_PATH="/run/user/$(id -u)/pulse"
    export XDG_RUNTIME_DIR="/run/user/$(id -u)"
    export PULSE_SERVER="unix:${XDG_RUNTIME_DIR}/pulse/native"
    export HOME="/home/pi"
    export USER="pi"
    export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    
    echo "=== Annunciator System Started ==="
    echo "Audio Environment:"
    echo "  USER: ${USER}"
    echo "  HOME: ${HOME}"
    echo "  PULSE_RUNTIME_PATH: ${PULSE_RUNTIME_PATH}"
    echo "  XDG_RUNTIME_DIR: ${XDG_RUNTIME_DIR}"
    echo "  PULSE_SERVER: ${PULSE_SERVER}"
    
    # Test espeak before starting main application
    echo "Testing TTS audio..."
    if command -v espeak >/dev/null 2>&1; then
        espeak -s 120 "Audio test successful" 2>/dev/null || echo "⚠️  TTS test failed but continuing..."
    fi
    
    echo "Web interface: http://$(hostname -I | awk '"'"'{print $1}'"'"'):8080"
    echo "================================"
    
    # Start the application
    ./annunciator'

echo "Screen session 'annunciator-system' started with audio support"
echo "To attach to session: screen -r annunciator-system"
echo "To detach from session: Ctrl+A then D"
echo "Web interface: http://$(hostname -I | awk '{print $1}'):8080"

# Show audio status
echo ""
echo "Audio Status:"
if command -v pactl >/dev/null 2>&1; then
    pactl info | grep "Server Name\|User Name" || echo "PulseAudio info not available"
else
    echo "PulseAudio tools not available"
fi
EOF

chmod +x screen_start_annunciator.sh
print_success "Improved screen startup script created"

# Fix audio permissions
print_status "Fixing audio permissions..."
sudo usermod -a -G audio,pulse,pulse-access pi
sudo chmod 666 /dev/snd/* 2>/dev/null || print_warning "Some audio device permissions could not be set"

# Update ALSA configuration for better compatibility
print_status "Updating ALSA configuration..."
cat > /tmp/asound.conf << 'EOF'
# ALSA configuration for Annunciator System
pcm.!default {
    type pulse
    fallback "sysdefault"
    hint {
        show on
        description "Default ALSA Output (currently PulseAudio Sound Server)"
    }
}

ctl.!default {
    type pulse
    fallback "sysdefault"
}

# Fallback to hardware if PulseAudio is not available
pcm.sysdefault {
    type hw
    card 0
}
ctl.sysdefault {
    type hw
    card 0
}
EOF

sudo mv /tmp/asound.conf /etc/asound.conf
print_success "ALSA configuration updated"

# Start the improved session
print_status "Starting improved annunciator session..."
./screen_start_annunciator.sh

print_success "Audio fixes applied and session started!"
echo ""
echo "=== Audio Fix Summary ==="
echo "✅ Improved screen startup script with audio environment"
echo "✅ PulseAudio integration for screen sessions" 
echo "✅ Audio permissions fixed"
echo "✅ ALSA configuration updated with PulseAudio support"
echo ""
echo "To monitor the session: screen -r annunciator-system"
echo "To check audio: attach to session and test TTS"