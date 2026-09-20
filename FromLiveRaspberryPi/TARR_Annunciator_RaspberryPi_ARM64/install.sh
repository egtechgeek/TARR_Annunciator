#!/bin/bash

# NOTE: This script installs an optional systemd unit. The current Raspberry Pi
# deployment method is GNU Screen + autologin + ~/start_tarr_annunciator.sh.
# For a new Pi matching the live units, run: ./install_raspberry_pi.sh instead.
# Admin restart / in-app updates expect the screen-based setup.

echo "=========================================="
echo "TARR Annunciator - Raspberry Pi Installer"
echo "(legacy systemd path — prefer install_raspberry_pi.sh)"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

# Check if running on Raspberry Pi
print_info "Detecting Raspberry Pi..."
if [[ -f "/sys/firmware/devicetree/base/model" ]] && grep -q "Raspberry Pi" "/sys/firmware/devicetree/base/model" 2>/dev/null; then
    PI_MODEL=$(cat /sys/firmware/devicetree/base/model | tr -d '\0')
    print_status "Detected: $PI_MODEL"
elif grep -q "BCM" /proc/cpuinfo; then
    PI_MODEL="Raspberry Pi (detected via /proc/cpuinfo)"
    print_status "Detected: $PI_MODEL"
else
    print_warning "Raspberry Pi not detected, but installation will continue..."
    PI_MODEL="Unknown (non-Pi system)"
fi

# Detect architecture
ARCH=$(uname -m)
print_info "Architecture: $ARCH"

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

# Update system packages
print_info "Updating system packages..."
sudo apt update

# Install audio system requirements
echo ""
echo "=== Audio System Setup ==="

print_info "Installing audio system components..."

# Install ALSA utilities (essential for Raspberry Pi)
if ! command -v aplay >/dev/null 2>&1; then
    print_info "Installing ALSA utilities..."
    sudo apt install -y alsa-utils
    print_status "ALSA utilities installed"
else
    print_status "ALSA utilities already installed"
fi

# Install PulseAudio (optional but recommended)
if ! command -v pactl >/dev/null 2>&1; then
    print_info "Installing PulseAudio..."
    sudo apt install -y pulseaudio pulseaudio-utils
    print_status "PulseAudio installed"
else
    print_status "PulseAudio already installed"
fi

# Check and configure Raspberry Pi audio
print_info "Configuring Raspberry Pi audio..."

# Ensure audio is enabled in config.txt
if ! grep -q "dtparam=audio=on" /boot/config.txt; then
    print_info "Enabling audio in /boot/config.txt..."
    echo "dtparam=audio=on" | sudo tee -a /boot/config.txt
    print_warning "System reboot will be required for audio changes to take effect"
    REBOOT_REQUIRED=true
else
    print_status "Audio already enabled in /boot/config.txt"
fi

# Load audio module if not already loaded
if ! lsmod | grep -q snd_bcm2835; then
    print_info "Loading BCM2835 audio module..."
    sudo modprobe snd_bcm2835
fi

# Set default audio output to auto
print_info "Setting default audio output to auto..."
amixer cset numid=3 0 2>/dev/null || print_warning "Could not set audio output (may require reboot)"

echo ""
echo "=== Application Setup ==="

print_status "Pre-compiled TARR Annunciator executable ready for $ARCH architecture"
print_info "Executable: $(pwd)/tarr-annunciator"

echo ""
echo "=== Audio System Test ==="

print_info "Testing audio system..."

# Test ALSA
if command -v aplay >/dev/null 2>&1; then
    print_status "ALSA available"
    if aplay -l | grep -q "card"; then
        print_status "Audio devices detected via ALSA"
    else
        print_warning "No audio devices detected via ALSA"
    fi
else
    print_error "ALSA not available"
fi

# Test PulseAudio
if command -v pactl >/dev/null 2>&1; then
    if pactl info >/dev/null 2>&1; then
        print_status "PulseAudio running"
        if pactl list short sinks | grep -q "sink"; then
            print_status "Audio sinks available via PulseAudio"
        else
            print_warning "No audio sinks detected via PulseAudio"
        fi
    else
        print_warning "PulseAudio installed but not running"
        print_info "You can start PulseAudio with: pulseaudio --start"
    fi
fi

echo ""
echo "==============================================="
echo "Service Installation Options"
echo "==============================================="

# Check for root privileges for service installation
if [ "$EUID" -eq 0 ]; then
    print_status "Running as root - systemd service installation available"
    ROOT_PRIVILEGES=true
else
    print_info "Running as regular user - service installation will require sudo"
    ROOT_PRIVILEGES=false
fi

# Service installation prompt
if [ "$ROOT_PRIVILEGES" = true ] || command -v sudo &> /dev/null; then
    echo "Would you like to install TARR Annunciator as a systemd service?"
    echo "This will allow the application to start automatically at boot."
    echo ""
    echo -n "Install as systemd service? (y/N): "
    read -r CREATE_SERVICE
    
    if [[ "$CREATE_SERVICE" =~ ^[Yy]$ ]]; then
        echo ""
        print_info "Installing TARR Annunciator as systemd service..."
        
        INSTALL_DIR="$(pwd)"
        SERVICE_NAME="tarr-annunciator"
        SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
        SERVICE_USER=$(whoami)
        
        # Create service file
        print_info "Creating systemd service file..."
        SERVICE_CONTENT="[Unit]
Description=TARR Annunciator Train Announcement System (Raspberry Pi)
After=network.target sound.target

[Service]
Type=simple
User=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/tarr-annunciator
Restart=always
RestartSec=5
Environment=PULSE_SERVER=unix:/run/user/$(id -u)/pulse/native

[Install]
WantedBy=multi-user.target"

        if [ "$ROOT_PRIVILEGES" = true ]; then
            echo "$SERVICE_CONTENT" > "$SERVICE_FILE"
        else
            echo "$SERVICE_CONTENT" | sudo tee "$SERVICE_FILE" > /dev/null
        fi
        
        if [ $? -eq 0 ]; then
            print_status "Service file created successfully!"
            
            # Reload systemd and enable service
            if [ "$ROOT_PRIVILEGES" = true ]; then
                systemctl daemon-reload
                systemctl enable "$SERVICE_NAME"
            else
                sudo systemctl daemon-reload
                sudo systemctl enable "$SERVICE_NAME"
            fi
            
            if [ $? -eq 0 ]; then
                print_status "Service enabled successfully!"
                echo ""
                echo "Service Details:"
                echo "- Name: $SERVICE_NAME"
                echo "- Description: TARR Annunciator Train Announcement System (Raspberry Pi)"
                echo "- Start Type: Enabled (starts at boot)"
                echo "- Working Directory: $INSTALL_DIR"
                echo "- Pi Model: $PI_MODEL"
                echo ""
                
                echo -n "Start the service now? (Y/n): "
                read -r START_SERVICE
                if [[ ! "$START_SERVICE" =~ ^[Nn]$ ]]; then
                    print_info "Starting TARR Annunciator service..."
                    if [ "$ROOT_PRIVILEGES" = true ]; then
                        systemctl start "$SERVICE_NAME"
                    else
                        sudo systemctl start "$SERVICE_NAME"
                    fi
                    
                    if [ $? -eq 0 ]; then
                        print_status "Service started successfully!"
                        
                        # Show service status
                        sleep 2
                        if [ "$ROOT_PRIVILEGES" = true ]; then
                            systemctl --no-pager status "$SERVICE_NAME"
                        else
                            sudo systemctl --no-pager status "$SERVICE_NAME"
                        fi
                    else
                        print_warning "Service failed to start"
                        print_info "Check logs with: journalctl -u $SERVICE_NAME -f"
                    fi
                fi
                
                echo ""
                echo "Service Management Commands:"
                echo "- Start:   sudo systemctl start $SERVICE_NAME"
                echo "- Stop:    sudo systemctl stop $SERVICE_NAME"
                echo "- Restart: sudo systemctl restart $SERVICE_NAME"
                echo "- Status:  sudo systemctl status $SERVICE_NAME"
                echo "- Logs:    journalctl -u $SERVICE_NAME -f"
                echo "- Disable: sudo systemctl disable $SERVICE_NAME"
                echo "- Remove:  sudo systemctl disable $SERVICE_NAME && sudo rm $SERVICE_FILE"
                echo ""
            else
                print_error "Failed to enable service"
                print_info "You may need to run with sudo or check systemctl permissions"
            fi
        else
            print_error "Failed to create service file"
            print_info "Make sure you have write permissions to /etc/systemd/system/"
        fi
    else
        print_info "Skipping service installation"
        print_info "You can run the application manually using: ./tarr-annunciator"
    fi
else
    print_warning "Service installation not available - sudo not found"
    print_info "Install sudo or run as root to enable systemd service installation"
fi

echo ""
echo "==============================================="
echo "Installation Complete!"
echo "==============================================="
echo ""

if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
    print_status "TARR Annunciator installed as systemd service"
    echo "The service will start automatically on boot"
    echo ""
    echo "Access Points:"
    echo "- Web Interface: http://localhost:8080"
    echo "- Admin Panel: http://localhost:8080/admin"
    echo "- API Documentation: http://localhost:8080/api/docs"
    echo "- Platform Info: http://localhost:8080/api/platform"
    echo ""
    echo "Service Management:"
    echo "- View status: sudo systemctl status $SERVICE_NAME"
    echo "- View logs: journalctl -u $SERVICE_NAME -f"
    echo ""
else
    print_status "TARR Annunciator ready for manual operation"
    echo ""
    echo "Manual Operation:"
    echo "- Run: ./tarr-annunciator"
    echo "- Or: ./run_raspberry_pi.sh"
    echo ""
fi

print_info "System: $PI_MODEL"
print_info "Architecture: $ARCH"
echo ""

echo "Configuration:"
echo "- Edit JSON files in json/ directory"
echo "- Access admin panel for web-based configuration"
echo ""

echo "Raspberry Pi Audio Configuration:"
echo "- Auto output: amixer cset numid=3 0"
echo "- Headphone: amixer cset numid=3 1"
echo "- HDMI: amixer cset numid=3 2"
echo ""

# Test audio if available
echo -n "Test Raspberry Pi audio system now? (y/N): "
read -r TEST_AUDIO
if [[ "$TEST_AUDIO" =~ ^[Yy]$ ]]; then
    print_info "Testing Raspberry Pi audio system..."
    if command -v speaker-test &> /dev/null; then
        print_info "Running speaker test (Ctrl+C to stop)..."
        timeout 3 speaker-test -t sine -f 1000 -c 2 2>/dev/null || true
    elif [ -f "./tarr-annunciator" ]; then
        print_info "Testing with TARR Annunciator audio test..."
        ./tarr-annunciator --test-audio 2>/dev/null || print_info "Note: Start application to test audio via web interface"
    else
        print_info "No audio test available - test via web interface after starting"
    fi
fi

echo ""

if [[ "$REBOOT_REQUIRED" == "true" ]]; then
    print_warning "REBOOT REQUIRED for audio configuration changes!"
    echo -n "Reboot now? (y/n): "
    read -r REBOOT_NOW
    if [[ "$REBOOT_NOW" =~ ^[Yy]$ ]]; then
        print_info "Rebooting system..."
        sudo reboot
    else
        print_warning "Remember to reboot later for audio changes to take effect"
    fi
fi

print_status "Raspberry Pi installation completed successfully!"

if [ ! -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
    print_info "To start manually: ./tarr-annunciator"
else
    print_info "Service installed: $SERVICE_NAME"
    print_info "Check status: sudo systemctl status $SERVICE_NAME"
fi

echo ""