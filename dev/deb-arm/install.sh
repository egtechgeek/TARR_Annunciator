#!/bin/bash

#################################################################################
# TARR Annunciator - Enhanced Raspberry Pi Installation Script
# This script installs the enhanced version with API, authentication, and
# improved features ported from the Windows version.
#################################################################################

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
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

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check if running on Raspberry Pi
check_raspberry_pi() {
    if [ ! -f /proc/device-tree/model ] || ! grep -q "Raspberry Pi" /proc/device-tree/model 2>/dev/null; then
        print_warning "This script is designed for Raspberry Pi, but will attempt to continue anyway."
        read -p "Continue? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_error "Installation cancelled."
            exit 1
        fi
    else
        print_success "Raspberry Pi detected."
    fi
}

# Function to check Python version
check_python() {
    if command_exists python3; then
        PYTHON_VERSION=$(python3 -c 'import sys; print(".".join(map(str, sys.version_info[:2])))')
        print_status "Python version: $PYTHON_VERSION"
        
        # Check if version is 3.8 or higher
        if python3 -c 'import sys; exit(0 if sys.version_info >= (3, 8) else 1)'; then
            print_success "Python version is compatible."
        else
            print_error "Python 3.8 or higher is required. Current version: $PYTHON_VERSION"
            exit 1
        fi
    else
        print_error "Python 3 is not installed. Please install Python 3.8 or higher."
        exit 1
    fi
}

# Function to install system dependencies
install_system_deps() {
    print_status "Installing system dependencies..."
    
    # Update package list
    sudo apt update
    
    # Install required packages
    PACKAGES="python3-pip python3-venv python3-dev pulseaudio pulseaudio-utils ffmpeg portaudio19-dev alsa-utils"
    
    print_status "Installing packages: $PACKAGES"
    sudo apt install -y $PACKAGES
    
    print_success "System dependencies installed."
}

# Function to install Python dependencies
install_python_deps() {
    print_status "Installing Python dependencies..."
    
    # Check if requirements.txt exists
    if [ ! -f "requirements.txt" ]; then
        print_error "requirements.txt not found in current directory."
        exit 1
    fi
    
    # Install Python packages
    pip3 install --user -r requirements.txt
    
    print_success "Python dependencies installed."
}

# Function to test audio system
test_audio() {
    print_status "Testing audio system..."
    
    # Check if PulseAudio is running
    if command_exists pactl; then
        print_status "Testing PulseAudio..."
        if pactl info >/dev/null 2>&1; then
            print_success "PulseAudio is running"
            print_status "Available PulseAudio sinks:"
            pactl list short sinks || print_warning "No PulseAudio sinks found"
        else
            print_warning "PulseAudio is not running or accessible"
        fi
    fi
    
    # Check if audio devices are available (fallback)
    if command_exists aplay; then
        print_status "Available ALSA devices (fallback):"
        aplay -l | grep -E "card [0-9]:" || print_warning "No ALSA devices found"
    fi
    
    # Test with the application
    if [ -f "app.py" ]; then
        print_status "Testing application audio system..."
        python3 app.py --test-audio
    else
        print_warning "app.py not found, skipping application audio test."
    fi
}

# Function to configure audio
configure_audio() {
    print_status "Configuring audio system..."
    
    # Start PulseAudio if not running
    if command_exists pulseaudio; then
        print_status "Starting PulseAudio..."
        # Kill any existing PulseAudio processes and restart
        pulseaudio --kill >/dev/null 2>&1 || true
        pulseaudio --start >/dev/null 2>&1 || true
        sleep 2
        
        if pactl info >/dev/null 2>&1; then
            print_success "PulseAudio started successfully"
            
            # Set reasonable default volume
            print_status "Setting default volume to 70%..."
            pactl set-sink-volume @DEFAULT_SINK@ 70% 2>/dev/null || print_warning "Could not set PulseAudio volume"
        else
            print_warning "PulseAudio failed to start properly"
        fi
    fi
    
    # Check current audio configuration (fallback)
    if command_exists amixer; then
        print_status "Configuring ALSA fallback..."
        amixer set PCM 70% 2>/dev/null || print_warning "Could not set PCM volume"
        amixer set Master 70% 2>/dev/null || print_warning "Could not set Master volume"
    fi
    
    # Provide audio configuration guidance
    echo
    print_status "Audio Configuration Guide:"
    echo "1. PulseAudio is now the primary audio system"
    echo "2. Use 'pactl list short sinks' to see available audio devices"
    echo "3. Use 'pactl set-default-sink SINK_NAME' to change default output"
    echo "4. For troubleshooting: 'pulseaudio --kill && pulseaudio --start'"
    echo "5. ALSA commands are available as fallback if needed"
    echo
}

# Function to setup file permissions
setup_permissions() {
    print_status "Setting up file permissions..."
    
    # Make Python scripts executable
    chmod +x *.py 2>/dev/null || true
    
    # Make shell scripts executable
    chmod +x *.sh 2>/dev/null || true
    
    # Ensure proper permissions on directories
    chmod 755 . 2>/dev/null || true
    chmod -R 644 json/ 2>/dev/null || true
    chmod -R 644 static/ 2>/dev/null || true
    chmod -R 644 templates/ 2>/dev/null || true
    
    print_success "File permissions configured."
}

# Function to create systemd service
setup_service() {
    read -p "Would you like to set up TARR Annunciator as a system service? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_status "Setting up systemd service..."
        
        CURRENT_DIR=$(pwd)
        USER=$(whoami)
        
        # Create service file
        sudo tee /etc/systemd/system/tarr-annunciator.service > /dev/null <<EOF
[Unit]
Description=TARR Annunciator
After=network.target sound.target
Wants=network.target

[Service]
Type=simple
User=$USER
Group=$USER
WorkingDirectory=$CURRENT_DIR
ExecStart=/usr/bin/python3 app.py
Restart=always
RestartSec=10
Environment=PATH=/usr/local/bin:/usr/bin:/bin
Environment=PYTHONPATH=$CURRENT_DIR

[Install]
WantedBy=multi-user.target
EOF

        # Reload systemd and enable service
        sudo systemctl daemon-reload
        sudo systemctl enable tarr-annunciator.service
        
        print_success "Systemd service created and enabled."
        print_status "Service commands:"
        echo "  Start:   sudo systemctl start tarr-annunciator"
        echo "  Stop:    sudo systemctl stop tarr-annunciator"
        echo "  Status:  sudo systemctl status tarr-annunciator"
        echo "  Logs:    sudo journalctl -u tarr-annunciator -f"
    fi
}

# Function to display security warnings
security_warnings() {
    echo
    print_warning "SECURITY NOTICE:"
    echo "1. Default admin credentials: admin/tarr2025 (change in json/admin_config.json)"
    echo "2. Default API key: tarr-api-2025 (change in app.py)"
    echo "3. CHANGE THESE before production use!"
    echo "4. The application runs on all interfaces (0.0.0.0:8080)"
    echo "5. Consider firewall rules for production environments"
    echo
}

# Function to run validation tests
run_validation() {
    print_status "Running validation tests..."
    
    # Test Python imports
    print_status "Testing Python dependencies..."
    python3 -c "import flask, pydub, requests" && print_success "Python dependencies OK" || print_error "Python dependency test failed"
    
    # Validate cron expressions if script exists
    if [ -f "validate_cron.py" ]; then
        print_status "Validating cron expressions..."
        python3 validate_cron.py
    fi
    
    # Test API if script exists
    if [ -f "api_test.py" ]; then
        print_status "API test script available at: ./api_test.py"
        print_status "Run 'python3 api_test.py' after starting the application"
    fi
}

# Function to display final instructions
show_final_instructions() {
    echo
    print_success "TARR Annunciator installation completed!"
    echo
    print_status "Next steps:"
    echo "1. Start the application:    python3 app.py"
    echo "2. Test audio:              python3 app.py --test-audio"
    echo "3. Access web interface:    http://localhost:8080"
    echo "4. Access admin panel:      http://localhost:8080/admin"
    echo "5. View API docs:           http://localhost:8080/api/docs"
    echo
    print_status "Utility scripts:"
    echo "• Test API:                 python3 api_test.py"
    echo "• Validate cron:            python3 validate_cron.py"
    echo
    print_status "Configuration files:"
    echo "• Main config:              json/*.json"
    echo "• Admin credentials:        json/admin_config.json"
    echo "• Audio files:              static/mp3/"
    echo "• API key:                  Change in app.py"
    echo
}

# Main installation function
main() {
    echo "#######################################################"
    echo "#     TARR Annunciator - Enhanced Pi Installation     #"
    echo "#######################################################"
    echo
    
    # Check if running as root
    if [ "$EUID" -eq 0 ]; then
        print_error "Please do not run this script as root."
        print_status "Run as a regular user - sudo will be used when needed."
        exit 1
    fi
    
    # Pre-installation checks
    check_raspberry_pi
    check_python
    
    # Check if we're in the right directory
    if [ ! -f "app.py" ] || [ ! -f "requirements.txt" ]; then
        print_error "Installation files not found in current directory."
        print_status "Please run this script from the TARR_Annunciator/dev/deb-arm directory."
        exit 1
    fi
    
    print_status "Starting installation..."
    
    # Install dependencies
    install_system_deps
    install_python_deps
    
    # Configure system
    setup_permissions
    configure_audio
    
    # Test installation
    test_audio
    run_validation
    
    # Optional service setup
    setup_service
    
    # Security warnings
    security_warnings
    
    # Final instructions
    show_final_instructions
    
    print_success "Installation script completed successfully!"
}

# Run main function
main "$@"
