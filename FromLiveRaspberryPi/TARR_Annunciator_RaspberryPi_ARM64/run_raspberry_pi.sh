#!/bin/bash

echo "================================================"
echo "🍓 TARR Annunciator - Raspberry Pi Launcher"
echo "================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Check if running on Raspberry Pi
if [[ -f "/sys/firmware/devicetree/base/model" ]] && grep -q "Raspberry Pi" "/sys/firmware/devicetree/base/model" 2>/dev/null; then
    PI_MODEL=$(cat /sys/firmware/devicetree/base/model | tr -d '\0')
    print_status "Running on: $PI_MODEL"
elif grep -q "BCM" /proc/cpuinfo; then
    PI_MODEL="Raspberry Pi (detected via /proc/cpuinfo)"
    print_status "Running on: $PI_MODEL"
else
    print_warning "Raspberry Pi not detected, but continuing..."
fi

# Detect architecture
ARCH=$(uname -m)
print_info "Architecture: $ARCH"

echo ""
echo "=== System Check ==="

# Check for executable
EXECUTABLE=""
if [[ -f "tarr-annunciator" ]]; then
    EXECUTABLE="tarr-annunciator"
    print_status "Found: tarr-annunciator"
elif [[ -f "dist/raspberry-pi/tarr-annunciator" ]]; then
    EXECUTABLE="dist/raspberry-pi/tarr-annunciator"
    print_status "Found: dist/raspberry-pi/tarr-annunciator"
elif [[ -f "dist/raspberry-pi-32/tarr-annunciator" ]]; then
    EXECUTABLE="dist/raspberry-pi-32/tarr-annunciator"
    print_status "Found: dist/raspberry-pi-32/tarr-annunciator"
elif [[ -f "dist/raspberry-pi-zero/tarr-annunciator" ]]; then
    EXECUTABLE="dist/raspberry-pi-zero/tarr-annunciator"
    print_status "Found: dist/raspberry-pi-zero/tarr-annunciator"
else
    print_error "TARR Annunciator executable not found!"
    print_info "Please run install_raspberry_pi.sh first or:"
    echo "  make build-raspberry-pi      # For Pi 4/5"
    echo "  make build-raspberry-pi-32   # For Pi 2/3"  
    echo "  make build-raspberry-pi-zero # For Pi Zero/1"
    exit 1
fi

echo ""
echo "=== Audio System Check ==="

# Check audio systems
AUDIO_SYSTEMS=()

# Check ALSA
if command -v aplay >/dev/null 2>&1; then
    print_status "ALSA utilities available"
    AUDIO_SYSTEMS+=("ALSA")
    
    # List available audio devices
    echo ""
    print_info "Available ALSA audio devices:"
    aplay -l 2>/dev/null | grep "^card" | while read -r line; do
        echo "  $line"
    done
    
    # Check current audio output setting
    if command -v amixer >/dev/null 2>&1; then
        CURRENT_OUTPUT=$(amixer cget numid=3 2>/dev/null | grep -o "values=[0-9]" | cut -d= -f2)
        case "$CURRENT_OUTPUT" in
            0) print_info "Current output: Auto" ;;
            1) print_info "Current output: Headphone/Analog" ;;
            2) print_info "Current output: HDMI" ;;
            *) print_info "Current output: Unknown ($CURRENT_OUTPUT)" ;;
        esac
    fi
else
    print_warning "ALSA utilities not available"
fi

# Check PulseAudio
if command -v pactl >/dev/null 2>&1; then
    if pactl info >/dev/null 2>&1; then
        print_status "PulseAudio running"
        AUDIO_SYSTEMS+=("PulseAudio")
        
        echo ""
        print_info "Available PulseAudio sinks:"
        pactl list short sinks 2>/dev/null | while read -r line; do
            echo "  $line"
        done
        
        # Get default sink
        DEFAULT_SINK=$(pactl info 2>/dev/null | grep "Default Sink:" | cut -d: -f2 | xargs)
        if [[ -n "$DEFAULT_SINK" ]]; then
            print_info "Default sink: $DEFAULT_SINK"
        fi
    else
        print_warning "PulseAudio installed but not running"
        print_info "Start with: pulseaudio --start"
    fi
else
    print_warning "PulseAudio not available"
fi

if [[ ${#AUDIO_SYSTEMS[@]} -eq 0 ]]; then
    print_error "No audio systems detected!"
    print_info "Install audio support with:"
    echo "  sudo apt install alsa-utils pulseaudio-utils"
    echo ""
    print_info "Continuing anyway, but audio may not work..."
else
    print_status "Audio systems available: ${AUDIO_SYSTEMS[*]}"
fi

echo ""
echo "=== Raspberry Pi Audio Configuration ==="

# Check if Raspberry Pi audio is enabled
if [[ -f "/boot/config.txt" ]]; then
    if grep -q "dtparam=audio=on" /boot/config.txt; then
        print_status "Audio enabled in /boot/config.txt"
    else
        print_warning "Audio may not be enabled in /boot/config.txt"
        print_info "Enable with: echo 'dtparam=audio=on' | sudo tee -a /boot/config.txt"
    fi
fi

# Check if audio kernel module is loaded
if lsmod | grep -q snd_bcm2835; then
    print_status "BCM2835 audio module loaded"
else
    print_warning "BCM2835 audio module not loaded"
    print_info "Load with: sudo modprobe snd_bcm2835"
fi

echo ""
echo "=== Network Configuration ==="

# Show network interfaces and IP addresses
print_info "Network interfaces:"
ip addr show | grep -E "^[0-9]|inet " | grep -v "127.0.0.1" | while read -r line; do
    if [[ $line =~ ^[0-9] ]]; then
        echo "  $line"
    else
        echo "    $line"
    fi
done

echo ""
echo "=== Starting Application ==="

SCREEN_NAME="tarr-annunciator"

print_info "Executable: $EXECUTABLE"

# Check if screen is available
if ! command -v screen >/dev/null 2>&1; then
    print_warning "GNU Screen not installed, starting directly..."
    print_info "Install screen for session management: sudo apt install screen"
    print_info "Starting TARR Annunciator directly..."
    
    # Set environment for better audio support
    export PULSE_SERVER="unix:${XDG_RUNTIME_DIR}/pulse/native"
    
    # Start the application directly
    exec ./"$EXECUTABLE"
else
    print_status "GNU Screen available"
    
    # Check if already running in screen
    if screen -list | grep -q "$SCREEN_NAME"; then
        print_warning "TARR Annunciator is already running in screen session '$SCREEN_NAME'"
        echo ""
        echo "Session Management Options:"
        echo "• View session: screen -r $SCREEN_NAME"
        echo "• Stop session: screen -S $SCREEN_NAME -X quit"
        echo "• List sessions: screen -list"
        echo ""
        echo "Restart options (if helper scripts exist):"
        if [[ -f "$HOME/tarr-restart.sh" ]]; then
            echo "• Quick restart: ~/tarr-restart.sh"
        fi
        if [[ -f "$HOME/tarr-view.sh" ]]; then
            echo "• Quick view: ~/tarr-view.sh"
        fi
        echo ""
        exit 0
    fi
    
    print_info "Starting TARR Annunciator in screen session '$SCREEN_NAME'..."
    
    # Set environment for better audio support
    export PULSE_SERVER="unix:${XDG_RUNTIME_DIR}/pulse/native"
    
    # Start in screen session
    screen -dmS "$SCREEN_NAME" bash -c "
        export PULSE_SERVER='unix:${XDG_RUNTIME_DIR}/pulse/native'
        cd '$(pwd)'
        echo '==============================================='
        echo '🍓 TARR Annunciator - Raspberry Pi'
        echo '📺 Running in GNU Screen Session'
        echo '==============================================='
        echo 'Working directory: \$(pwd)'
        echo 'Screen session: $SCREEN_NAME'
        echo 'Started: \$(date)'
        echo 'Architecture: $ARCH'
        echo 'Audio systems: ${AUDIO_SYSTEMS[*]}'
        echo ''
        echo '📱 Web Interface: http://localhost:8080'
        echo '⚙️  Admin Panel: http://localhost:8080/admin'
        echo ''
        echo 'Press Ctrl+A then D to detach from session'
        echo 'Use \"screen -r $SCREEN_NAME\" to reattach'
        echo 'Use \"screen -list\" to see all sessions'
        echo ''
        echo 'Starting application...'
        echo '==============================================='
        echo ''
        ./$EXECUTABLE
    "
    
    # Wait and verify
    sleep 2
    if screen -list | grep -q "$SCREEN_NAME"; then
        print_status "TARR Annunciator started successfully in screen session!"
        echo ""
        echo "🌐 Access Points:"
        echo "• Web Interface: http://localhost:8080"
        echo "• Admin Panel: http://localhost:8080/admin"
        echo ""
        echo "📺 Screen Session Management:"
        echo "• View application: screen -r $SCREEN_NAME"
        echo "• Detach from session: Ctrl+A then D"
        echo "• List all sessions: screen -list"
        echo "• Stop application: screen -S $SCREEN_NAME -X quit"
        echo ""
        if [[ -f "$HOME/tarr-view.sh" ]]; then
            echo "🔧 Helper Scripts (if available):"
            echo "• ~/tarr-view.sh    - View application session"
            echo "• ~/tarr-stop.sh    - Stop application"
            echo "• ~/tarr-restart.sh - Restart application"
            echo ""
        fi
        print_info "Application is running in background screen session"
        print_info "Use 'screen -r $SCREEN_NAME' to view the application console"
    else
        print_error "Failed to start screen session"
        print_info "Falling back to direct execution..."
        exec ./"$EXECUTABLE"
    fi
fi