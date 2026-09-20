#!/bin/bash

echo "=========================================="
echo "TARR Annunciator - Raspberry Pi Installer"
echo "Enhanced with Modern Audio System Support"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Function to print colored output
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

print_feature() {
    echo -e "${PURPLE}★${NC} $1"
}

# Global variables
REBOOT_REQUIRED=false
ROOT_PRIVILEGES=false
PI_MODEL=""
ARCH=""
SERVICE_NAME="tarr-annunciator"
AUDIO_SYSTEM=""
PIPEWIRE_INSTALLED=false
BLUETOOTH_CONFIGURED=false

# Check if running on Raspberry Pi
print_info "Detecting Raspberry Pi..."
if [[ -f "/sys/firmware/devicetree/base/model" ]] && grep -q "Raspberry Pi" "/sys/firmware/devicetree/base/model" 2>/dev/null; then
    PI_MODEL=$(cat /sys/firmware/devicetree/base/model | tr -d '\0')
    print_status "Detected: $PI_MODEL"
elif grep -q "BCM" /proc/cpuinfo; then
    PI_MODEL="Raspberry Pi (detected via /proc/cpuinfo)"
    print_status "Detected: $PI_MODEL"
else
    print_warning "Raspberry Pi not detected, checking for OrangePi..."
    if grep -qE "allwinner|sun8i|sun50i" /proc/cpuinfo 2>/dev/null; then
        PI_MODEL="OrangePi (detected via /proc/cpuinfo)"
        print_status "Detected: $PI_MODEL"
    else
        print_warning "ARM board not detected, but installation will continue..."
        PI_MODEL="Unknown (non-Pi system)"
    fi
fi

# Detect architecture
ARCH=$(uname -m)
print_info "Architecture: $ARCH"

# Detect OS version
if command -v lsb_release &> /dev/null; then
    OS_VERSION=$(lsb_release -d | cut -f2-)
    print_info "OS: $OS_VERSION"
fi

echo ""
echo "=== Enhanced Features Available ==="
print_feature "✨ Modern Audio System Support (PipeWire, PulseAudio, ALSA)"
print_feature "✨ Cross-Platform Hardware Detection (Raspberry Pi, OrangePi)"
print_feature "✨ Bluetooth Device Discovery and Pairing"
print_feature "✨ Audio System Override Controls"
print_feature "✨ Enhanced Web-Based Administration"
print_feature "✨ Comprehensive Audio Device Management"
print_feature "✨ Platform-Specific Optimizations"

echo ""
echo "=== System Requirements Check ==="

# Check if the pre-compiled executable exists
if [[ ! -f "tarr-annunciator" ]]; then
    print_error "Pre-compiled executable 'tarr-annunciator' not found!"
    print_error "This installation package should include the pre-compiled executable."
    exit 1
else
    print_status "Pre-compiled executable found"
    chmod +x tarr-annunciator
fi

# Check root privileges
if [ "$EUID" -eq 0 ]; then
    print_status "Running as root - full system configuration available"
    ROOT_PRIVILEGES=true
else
    print_info "Running as regular user - some operations will require sudo"
    ROOT_PRIVILEGES=false
fi

# Update system packages
print_info "Updating system packages..."
sudo apt update

echo ""
echo "=== Modern Audio System Setup ==="

# Detect current audio system
print_info "Detecting current audio system..."

if pgrep -f pipewire > /dev/null 2>&1; then
    AUDIO_SYSTEM="PipeWire"
    print_status "PipeWire is currently running"
elif pgrep -f pulseaudio > /dev/null 2>&1; then
    AUDIO_SYSTEM="PulseAudio"
    print_status "PulseAudio is currently running"
else
    AUDIO_SYSTEM="ALSA"
    print_info "No high-level audio system detected, defaulting to ALSA"
fi

# Install ALSA utilities (essential for all systems)
print_info "Installing ALSA utilities and development libraries..."
if ! command -v aplay >/dev/null 2>&1; then
    sudo apt install -y alsa-utils libasound2-dev pkg-config build-essential
    print_status "ALSA utilities and development libraries installed"
else
    # Check if dev libraries are installed
    if ! pkg-config --exists alsa; then
        print_info "Installing ALSA development libraries..."
        sudo apt install -y libasound2-dev pkg-config build-essential
        print_status "ALSA development libraries installed"
    fi
    print_status "ALSA utilities already installed"
fi

# Install PipeWire (recommended for modern systems)
echo ""
print_info "PipeWire Installation Options..."
echo "PipeWire is the modern audio system that provides:"
echo "- Better Bluetooth audio support"  
echo "- Lower latency audio processing"
echo "- Compatibility with PulseAudio applications"
echo "- Professional audio capabilities"
echo ""

if command -v pipewire >/dev/null 2>&1; then
    print_status "PipeWire already installed"
    PIPEWIRE_INSTALLED=true
else
    echo -n "Install PipeWire audio system? (recommended) (Y/n): "
    read -r INSTALL_PIPEWIRE
    if [[ ! "$INSTALL_PIPEWIRE" =~ ^[Nn]$ ]]; then
        print_info "Installing PipeWire with PulseAudio compatibility..."
        
        # Install PipeWire packages
        sudo apt install -y \
            pipewire \
            pipewire-pulse \
            pipewire-alsa \
            pipewire-audio-client-libraries \
            wireplumber \
            pipewire-media-session- 2>/dev/null || true
            
        # Install PipeWire utilities if available
        sudo apt install -y \
            pipewire-bin \
            libspa-0.2-modules \
            libspa-0.2-bluetooth 2>/dev/null || true
        
        print_status "PipeWire installed"
        PIPEWIRE_INSTALLED=true
        
        # Enable and start PipeWire services
        print_info "Configuring PipeWire services..."
        systemctl --user --now enable pipewire pipewire-pulse 2>/dev/null || true
        systemctl --user --now enable wireplumber 2>/dev/null || true
        
        print_status "PipeWire services configured"
        print_warning "You may need to reboot for PipeWire to fully take effect"
        REBOOT_REQUIRED=true
    fi
fi

# Install/Configure PulseAudio (for compatibility)
if ! command -v pactl >/dev/null 2>&1; then
    print_info "Installing PulseAudio utilities (compatibility)..."
    sudo apt install -y pulseaudio-utils
    print_status "PulseAudio utilities installed"
else
    print_status "PulseAudio utilities already available"
fi

# Install essential utilities
echo ""
print_info "Installing essential utilities..."

# Install mc (Midnight Commander) for file management
if ! command -v mc >/dev/null 2>&1; then
    print_info "Installing Midnight Commander (mc)..."
    sudo apt install -y mc
    print_status "Midnight Commander installed"
else
    print_status "Midnight Commander already installed"
fi

# Install screen for session management
if ! command -v screen >/dev/null 2>&1; then
    print_info "Installing GNU Screen..."
    sudo apt install -y screen
    print_status "GNU Screen installed"
else
    print_status "GNU Screen already installed"
fi

# Install Bluetooth support
echo ""
print_info "Bluetooth Audio Support..."
if ! command -v bluetoothctl >/dev/null 2>&1; then
    echo -n "Install Bluetooth support? (Y/n): "
    read -r INSTALL_BLUETOOTH
    if [[ ! "$INSTALL_BLUETOOTH" =~ ^[Nn]$ ]]; then
        print_info "Installing Bluetooth support..."
        sudo apt install -y \
            bluetooth \
            bluez \
            bluez-tools \
            bluez-alsa-utils 2>/dev/null || true
            
        # Install Bluetooth audio codecs
        sudo apt install -y \
            pulseaudio-module-bluetooth \
            libspa-0.2-bluetooth 2>/dev/null || true
        
        # Enable Bluetooth service
        sudo systemctl enable bluetooth
        sudo systemctl start bluetooth
        
        print_status "Bluetooth support installed"
        BLUETOOTH_CONFIGURED=true
    fi
else
    print_status "Bluetooth support already available"
    BLUETOOTH_CONFIGURED=true
fi

echo ""
echo "=== Hardware-Specific Audio Configuration ==="

# Raspberry Pi specific configuration
if [[ "$PI_MODEL" == *"Raspberry Pi"* ]]; then
    print_info "Configuring Raspberry Pi audio..."
    
    # Ensure audio is enabled in config.txt
    if ! grep -q "dtparam=audio=on" /boot/config.txt 2>/dev/null && ! grep -q "dtparam=audio=on" /boot/firmware/config.txt 2>/dev/null; then
        print_info "Enabling audio in boot configuration..."
        
        # Try both possible locations
        if [ -f "/boot/config.txt" ]; then
            echo "dtparam=audio=on" | sudo tee -a /boot/config.txt
        elif [ -f "/boot/firmware/config.txt" ]; then
            echo "dtparam=audio=on" | sudo tee -a /boot/firmware/config.txt
        fi
        
        print_status "Audio enabled in boot configuration"
        print_warning "System reboot required for audio changes to take effect"
        REBOOT_REQUIRED=true
    else
        print_status "Audio already enabled in boot configuration"
    fi
    
    # Load audio module if not already loaded
    if ! lsmod | grep -q snd_bcm2835; then
        print_info "Loading BCM2835 audio module..."
        sudo modprobe snd_bcm2835 2>/dev/null || print_warning "Could not load audio module (may require reboot)"
    else
        print_status "BCM2835 audio module loaded"
    fi
    
    # Set default audio output to auto
    print_info "Configuring default audio output..."
    amixer cset numid=3 0 2>/dev/null || print_warning "Could not set audio output (may require reboot)"
    print_status "Audio output configured to auto-detect"

# OrangePi specific configuration  
elif [[ "$PI_MODEL" == *"OrangePi"* ]]; then
    print_info "Configuring OrangePi audio..."
    
    # OrangePi typically uses different audio configuration
    print_info "Checking OrangePi audio modules..."
    sudo modprobe snd-soc-sunxi 2>/dev/null || print_info "OrangePi audio module not available"
    print_status "OrangePi audio configuration completed"
fi

echo ""
echo "=== Audio System Testing & Diagnostics ==="

print_info "Testing audio systems..."

# Test ALSA
print_info "Testing ALSA..."
if command -v aplay >/dev/null 2>&1; then
    if aplay -l 2>/dev/null | grep -q "card"; then
        CARD_COUNT=$(aplay -l 2>/dev/null | grep -c "card")
        print_status "ALSA: $CARD_COUNT audio card(s) detected"
    else
        print_warning "ALSA: No audio cards detected"
    fi
else
    print_error "ALSA utilities not available"
fi

# Test PulseAudio/PipeWire compatibility
print_info "Testing PulseAudio compatibility layer..."
if command -v pactl >/dev/null 2>&1; then
    if pactl info >/dev/null 2>&1; then
        SERVER_INFO=$(pactl info 2>/dev/null | grep "Server Name" | cut -d: -f2 | xargs)
        if [[ "$SERVER_INFO" == *"PipeWire"* ]]; then
            print_status "PipeWire running with PulseAudio compatibility"
        else
            print_status "PulseAudio server running"
        fi
        
        SINK_COUNT=$(pactl list short sinks 2>/dev/null | wc -l)
        print_status "Audio sinks available: $SINK_COUNT"
    else
        print_warning "PulseAudio compatibility not responding"
    fi
fi

# Test PipeWire native tools
if command -v wpctl >/dev/null 2>&1; then
    print_info "Testing PipeWire native tools..."
    if wpctl status >/dev/null 2>&1; then
        print_status "PipeWire native tools working"
    else
        print_warning "PipeWire native tools not responding"
    fi
elif command -v pw-cli >/dev/null 2>&1; then
    if pw-cli info >/dev/null 2>&1; then
        print_status "PipeWire pw-cli working"
    else
        print_warning "PipeWire pw-cli not responding"
    fi
fi

# Test Bluetooth if configured
if [[ "$BLUETOOTH_CONFIGURED" == "true" ]]; then
    print_info "Testing Bluetooth..."
    if systemctl is-active --quiet bluetooth; then
        print_status "Bluetooth service active"
    else
        print_warning "Bluetooth service not active"
    fi
fi

echo ""
echo "=== Application Setup ==="

print_status "TARR Annunciator executable ready for $ARCH architecture"
print_info "Executable: $(pwd)/tarr-annunciator"

# Configure Auto-start with GNU Screen
echo ""
echo "==============================================="
echo "Auto-Start Configuration (User Session)"
echo "==============================================="

print_info "Configuring TARR Annunciator to start automatically using GNU Screen..."
print_info "This avoids audio permission issues by running in the user session."

INSTALL_DIR="$(pwd)"
CURRENT_USER=$(whoami)
HOME_DIR="/home/$CURRENT_USER"

# Check if autologin is enabled
print_info "Checking autologin configuration..."

AUTOLOGIN_ENABLED=false
if [ -f "/etc/systemd/system/getty@tty1.service.d/autologin.conf" ]; then
    if grep -q "ExecStart.*--autologin $CURRENT_USER" "/etc/systemd/system/getty@tty1.service.d/autologin.conf"; then
        AUTOLOGIN_ENABLED=true
        print_status "Autologin already enabled for user: $CURRENT_USER"
    fi
elif systemctl is-enabled autologin@$CURRENT_USER.service >/dev/null 2>&1; then
    AUTOLOGIN_ENABLED=true
    print_status "Autologin service already enabled for user: $CURRENT_USER"
fi

if [ "$AUTOLOGIN_ENABLED" = false ]; then
    echo ""
    print_warning "Autologin is not currently enabled."
    echo "For automatic startup, the system needs to auto-login to your user account."
    echo "This is safe for dedicated Raspberry Pi systems like train announcement boards."
    echo ""
    echo -n "Enable autologin for user '$CURRENT_USER'? (Y/n): "
    read -r ENABLE_AUTOLOGIN
    
    if [[ ! "$ENABLE_AUTOLOGIN" =~ ^[Nn]$ ]]; then
        print_info "Enabling autologin for user: $CURRENT_USER"
        
        # Create autologin configuration
        sudo mkdir -p /etc/systemd/system/getty@tty1.service.d/
        sudo tee /etc/systemd/system/getty@tty1.service.d/autologin.conf > /dev/null << EOF
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin $CURRENT_USER --noclear %I \$TERM
EOF
        
        if [ $? -eq 0 ]; then
            print_status "Autologin configuration created"
            AUTOLOGIN_ENABLED=true
            REBOOT_REQUIRED=true
        else
            print_error "Failed to enable autologin"
        fi
    else
        print_warning "Autologin not enabled - manual startup will be required"
    fi
fi

# Create startup script and directories
print_info "Creating startup script and directories..."

# Create logs directory if it doesn't exist
if [[ ! -d "logs" ]]; then
    mkdir -p logs
    print_status "Created logs directory"
fi

STARTUP_SCRIPT="$HOME_DIR/start_tarr_annunciator.sh"

cat > "$STARTUP_SCRIPT" << 'EOF'
#!/bin/bash

# TARR Annunciator Startup Script
# This script launches the TARR Annunciator in a GNU Screen session

TARR_DIR="__INSTALL_DIR__"
SCREEN_NAME="tarr-annunciator"
LOG_FILE="$HOME/tarr-annunciator-startup.log"

# Function to log with timestamp
log_message() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

log_message "=== TARR Annunciator Startup Script ==="
log_message "Starting TARR Annunciator in Screen session: $SCREEN_NAME"

# Change to the application directory
cd "$TARR_DIR" || {
    log_message "ERROR: Could not change to directory: $TARR_DIR"
    exit 1
}

# Check if screen session already exists
if screen -list | grep -q "$SCREEN_NAME"; then
    log_message "WARNING: Screen session '$SCREEN_NAME' already exists"
    log_message "Attempting to terminate existing session..."
    screen -S "$SCREEN_NAME" -X quit 2>/dev/null || true
    sleep 2
fi

# Wait a moment for audio system to be ready
log_message "Waiting for audio system to initialize..."
sleep 5

# Start the application in a new screen session
log_message "Creating new screen session: $SCREEN_NAME"
log_message "Working directory: $(pwd)"

# Create screen session and start the application
screen -dmS "$SCREEN_NAME" bash -c "
    cd '$TARR_DIR'
    echo 'TARR Annunciator starting in Screen session...'
    echo 'Working directory: $(pwd)'
    echo 'Session: $SCREEN_NAME'
    echo 'Log file: $LOG_FILE'
    echo '============================================'
    ./tarr-annunciator
"

# Verify the screen session was created
sleep 2
if screen -list | grep -q "$SCREEN_NAME"; then
    log_message "SUCCESS: Screen session '$SCREEN_NAME' created successfully"
    log_message "Use 'screen -r $SCREEN_NAME' to attach to the session"
else
    log_message "ERROR: Failed to create screen session '$SCREEN_NAME'"
    exit 1
fi

log_message "TARR Annunciator startup completed"
EOF

# Replace the placeholder with actual install directory
sed -i "s|__INSTALL_DIR__|$INSTALL_DIR|g" "$STARTUP_SCRIPT"
chmod +x "$STARTUP_SCRIPT"

print_status "Startup script created: $STARTUP_SCRIPT"

# Create or update .bashrc for automatic startup
print_info "Configuring automatic startup in .bashrc..."

# Remove any existing TARR startup lines
if [ -f "$HOME_DIR/.bashrc" ]; then
    grep -v "# TARR Annunciator Auto-Start" "$HOME_DIR/.bashrc" > "$HOME_DIR/.bashrc.tmp" || true
    grep -v "start_tarr_annunciator.sh" "$HOME_DIR/.bashrc.tmp" > "$HOME_DIR/.bashrc" || true
    rm -f "$HOME_DIR/.bashrc.tmp"
fi

# Add startup logic to .bashrc
cat >> "$HOME_DIR/.bashrc" << 'EOF'

# TARR Annunciator Auto-Start
if [[ -z "$SSH_CLIENT" && -z "$SSH_TTY" && "$TERM" != "screen"* ]]; then
    # Only auto-start on local console login (not SSH or within screen)
    if [ -f "$HOME/start_tarr_annunciator.sh" ]; then
        echo "Starting TARR Annunciator..."
        "$HOME/start_tarr_annunciator.sh"
        echo "TARR Annunciator started in screen session 'tarr-annunciator'"
        echo "Use 'screen -r tarr-annunciator' to view the application"
        echo "Use 'mc' to manage files or 'screen -list' to see active sessions"
    fi
fi
EOF

print_status "Auto-start configuration added to .bashrc"

# Create helper scripts
print_info "Creating helper scripts..."

# Script to manually start TARR
cat > "$HOME_DIR/tarr-start.sh" << EOF
#!/bin/bash
echo "Starting TARR Annunciator..."
"$HOME_DIR/start_tarr_annunciator.sh"
echo "TARR Annunciator started in screen session 'tarr-annunciator'"
echo "Use 'screen -r tarr-annunciator' to view the application"
EOF
chmod +x "$HOME_DIR/tarr-start.sh"

# Script to stop TARR
cat > "$HOME_DIR/tarr-stop.sh" << 'EOF'
#!/bin/bash
echo "Stopping TARR Annunciator..."
if screen -list | grep -q "tarr-annunciator"; then
    screen -S "tarr-annunciator" -X quit
    echo "TARR Annunciator stopped"
else
    echo "TARR Annunciator is not running"
fi
EOF
chmod +x "$HOME_DIR/tarr-stop.sh"

# Script to restart TARR
cat > "$HOME_DIR/tarr-restart.sh" << EOF
#!/bin/bash
echo "Restarting TARR Annunciator..."
"$HOME_DIR/tarr-stop.sh"
sleep 2
"$HOME_DIR/tarr-start.sh"
EOF
chmod +x "$HOME_DIR/tarr-restart.sh"

# Script to view TARR session
cat > "$HOME_DIR/tarr-view.sh" << 'EOF'
#!/bin/bash
echo "Connecting to TARR Annunciator session..."
if screen -list | grep -q "tarr-annunciator"; then
    echo "Press Ctrl+A then D to detach from the session"
    screen -r tarr-annunciator
else
    echo "TARR Annunciator session is not running"
    echo "Start it with: ./tarr-start.sh"
fi
EOF
chmod +x "$HOME_DIR/tarr-view.sh"

print_status "Helper scripts created:"
print_info "• tarr-start.sh   - Start TARR Annunciator"
print_info "• tarr-stop.sh    - Stop TARR Annunciator"
print_info "• tarr-restart.sh - Restart TARR Annunciator"
print_info "• tarr-view.sh    - View TARR Annunciator session"

echo ""
echo "==============================================="
echo "Enhanced Installation Complete!"
echo "==============================================="
echo ""

# Installation summary
print_status "TARR Annunciator Enhanced Installation Summary"
echo ""
echo "System Configuration:"
echo "• Platform: $PI_MODEL"
echo "• Architecture: $ARCH"
echo "• Audio System: $AUDIO_SYSTEM"
echo "• PipeWire: $([ "$PIPEWIRE_INSTALLED" = true ] && echo "✓ Installed" || echo "✗ Not installed")"
echo "• Bluetooth: $([ "$BLUETOOTH_CONFIGURED" = true ] && echo "✓ Available" || echo "✗ Not configured")"
echo ""

if [[ "$AUTOLOGIN_ENABLED" == "true" ]]; then
    print_status "Screen-based auto-start configured"
    echo ""
    echo "🌐 Access Points (after reboot/login):"
    echo "• Web Interface: http://localhost:8080"
    echo "• Admin Panel: http://localhost:8080/admin"
    echo "• API Documentation: http://localhost:8080/api/docs"
    echo "• Platform Info: http://localhost:8080/api/platform"
    echo ""
    echo "🔧 Enhanced Features Available:"
    echo "• Audio System Override (Admin → Audio Controls)"
    echo "• Bluetooth Device Management (Admin → Audio Controls)"
    echo "• Screen-Based Restart (Admin → System Status - Pi Only)"
    echo "• Cross-Platform Hardware Detection"
    echo "• Advanced Audio Device Selection"
    echo ""
else
    print_status "Manual screen-based installation completed"
    print_info "Start with: ./tarr-start.sh"
fi

echo "📱 Admin Panel Features:"
echo "• 🔊 Enhanced Audio Controls with System Override"
echo "• 📶 Bluetooth Device Discovery and Pairing"  
echo "• ⚙️  System Information and Platform Detection"
echo "• 🔄 Application Restart and Shutdown Controls"
echo "• 🎵 Advanced Audio Device Management"
echo "• 📋 Queue Management with Priority Controls"
echo ""

# Audio configuration help
if [[ "$PI_MODEL" == *"Raspberry Pi"* ]]; then
    echo "🔊 Raspberry Pi Audio Configuration:"
    echo "• Auto-detect: amixer cset numid=3 0"
    echo "• Headphone (3.5mm): amixer cset numid=3 1"
    echo "• HDMI: amixer cset numid=3 2"
    echo "• Test: speaker-test -t sine -f 1000 -c 2"
    echo ""
fi

# Audio system troubleshooting
echo "🔍 Audio System Troubleshooting:"
echo "• Check devices: Use Admin Panel → Audio Controls → Audio System Override"
echo "• Force ALSA: Select 'Force ALSA' in Audio System Override"
echo "• Force PipeWire: Select 'Force PipeWire' in Audio System Override"  
echo "• Redetect: Use 'Redetect Audio Devices' button in Admin Panel"
echo "• Platform info available at: /admin/system/platform-info"
echo ""

# Bluetooth help
if [[ "$BLUETOOTH_CONFIGURED" == "true" ]]; then
    echo "📶 Bluetooth Management:"
    echo "• Scan: Admin Panel → Audio Controls → Bluetooth Functions"
    echo "• Pair devices: Use web interface for easy pairing"
    echo "• Command line: bluetoothctl (for advanced users)"
    echo ""
fi

# Test audio if available
echo -n "Test audio system now? (y/N): "
read -r TEST_AUDIO
if [[ "$TEST_AUDIO" =~ ^[Yy]$ ]]; then
    print_info "Testing audio system..."
    echo ""
    
    # Test with speaker-test if available
    if command -v speaker-test &> /dev/null; then
        print_info "Running speaker test (3 seconds, Ctrl+C to stop)..."
        timeout 3 speaker-test -t sine -f 1000 -c 2 2>/dev/null || true
        echo ""
    fi
    
    # Show detected devices
    print_info "Detected Audio Devices:"
    if command -v aplay &> /dev/null; then
        echo "ALSA Devices:"
        aplay -l 2>/dev/null | grep -E "(card|device)" || echo "  No ALSA devices found"
        echo ""
    fi
    
    if command -v pactl &> /dev/null && pactl info >/dev/null 2>&1; then
        echo "PulseAudio/PipeWire Sinks:"
        pactl list short sinks 2>/dev/null || echo "  No sinks found"
        echo ""
    fi
    
    print_info "For detailed testing, use the web interface after starting the application"
fi

echo ""

# Reboot check
if [[ "$REBOOT_REQUIRED" == "true" ]]; then
    print_warning "🔄 REBOOT RECOMMENDED for optimal audio system configuration!"
    echo "The following changes require a reboot to take full effect:"
    echo "• Audio system configuration changes"
    echo "• PipeWire installation and setup"
    echo "• Hardware-specific driver loading"
    echo ""
    echo -n "Reboot now? (y/n): "
    read -r REBOOT_NOW
    if [[ "$REBOOT_NOW" =~ ^[Yy]$ ]]; then
        print_info "Rebooting system..."
        sudo reboot
    else
        print_warning "Remember to reboot later for optimal performance"
        echo "After reboot, verify installation with:"
        echo "sudo systemctl status $SERVICE_NAME"
    fi
fi

print_status "🎉 Enhanced Raspberry Pi installation completed successfully!"
echo ""

# Final status and instructions
echo "🎛️  Screen Session Management:"
echo "• Start: ./tarr-start.sh"
echo "• Stop: ./tarr-stop.sh"
echo "• Restart: ./tarr-restart.sh"
echo "• View: ./tarr-view.sh"
echo "• List sessions: screen -list"
echo "• Attach manually: screen -r tarr-annunciator"
echo "• Detach from session: Ctrl+A then D"
echo ""

if [[ "$AUTOLOGIN_ENABLED" == "true" ]]; then
    print_info "🚀 Auto-Start: Enabled (will start automatically after reboot/login)"
    print_info "📊 View Logs: ~/tarr-annunciator-startup.log"
    print_info "🌐 Access Web Interface: http://localhost:8080"
else
    print_info "▶️  Manual Start: ./tarr-start.sh"
    print_info "🔗 Or direct: screen -dmS tarr-annunciator ./tarr-annunciator"
fi

echo ""
echo "For support and documentation, visit the admin panel or check the README files."
echo "Happy announcing! 🚂"
echo ""