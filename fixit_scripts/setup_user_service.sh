#!/bin/bash

# Setup TARR Annunciator as systemd user service instead of screen
# This preserves audio permissions while providing automatic startup and robust service management

set -e

echo "🔧 Setting up TARR Annunciator as systemd user service..."

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

# Determine current user and paths
CURRENT_USER=$(whoami)
USER_HOME="/home/$CURRENT_USER"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARR_DIR="$(dirname "$SCRIPT_DIR")"

print_status "Current user: $CURRENT_USER"
print_status "TARR directory: $TARR_DIR"

# Check if TARR Annunciator executable exists
if [ ! -f "$TARR_DIR/TARR_Annunciator_RaspberryPi_ARM64" ]; then
    print_error "TARR Annunciator binary not found at $TARR_DIR/TARR_Annunciator_RaspberryPi_ARM64"
    print_error "Please ensure you're running this script from the TARR Annunciator directory structure"
    exit 1
fi

# Stop existing screen session if running
print_status "Stopping any existing TARR Annunciator screen sessions..."
if screen -list 2>/dev/null | grep -q tarr-annunciator; then
    screen -S tarr-annunciator -X quit 2>/dev/null
    print_success "TARR Annunciator screen session stopped"
else
    print_status "No TARR Annunciator screen session found"
fi

# Remove TARR Annunciator screen autostart from .bashrc
print_status "Removing TARR Annunciator screen autostart from .bashrc..."

# Check for TARR Annunciator autostart patterns in .bashrc
if grep -q "TARR.*Annunciator\|tarr.*annunciator\|start_tarr_annunciator\|screen.*tarr-annunciator" "$USER_HOME/.bashrc" 2>/dev/null; then
    # Create backup of .bashrc
    BACKUP_FILE="$USER_HOME/.bashrc.backup.$(date +%Y%m%d_%H%M%S)"
    cp "$USER_HOME/.bashrc" "$BACKUP_FILE"
    print_status "Created .bashrc backup: $BACKUP_FILE"

    # Create a temporary file to rebuild .bashrc properly
    TEMP_BASHRC=$(mktemp)

    # Process .bashrc line by line to remove the TARR Annunciator autostart block
    awk '
    BEGIN { in_tarr_block = 0; skip_empty = 0 }

    # Start of TARR block
    /# TARR Annunciator Auto-Start/ { in_tarr_block = 1; skip_empty = 1; next }

    # End conditions for TARR block
    in_tarr_block && (/^[[:space:]]*$/ && skip_empty) { next }  # Skip empty lines at start of block
    in_tarr_block && /^[[:space:]]*$/ && !skip_empty { in_tarr_block = 0; next }  # End block on empty line
    in_tarr_block && /^[[:space:]]*#/ && !/TARR|tarr/ { in_tarr_block = 0; print; next }  # End on non-TARR comment
    in_tarr_block && /^[^[:space:]#]/ && !/TARR|tarr|start_tarr|screen.*tarr/ { in_tarr_block = 0; print; next }  # End on non-TARR code

    # Skip lines within TARR block
    in_tarr_block {
        if (!/^[[:space:]]*$/) skip_empty = 0  # Found non-empty line, stop skipping empties
        next
    }

    # Keep all other lines
    { print }
    ' "$USER_HOME/.bashrc" > "$TEMP_BASHRC"

    # Replace original .bashrc with cleaned version
    mv "$TEMP_BASHRC" "$USER_HOME/.bashrc"

    print_success "TARR Annunciator screen autostart removed from .bashrc (backup created)"
else
    print_status "No TARR Annunciator screen autostart found in .bashrc"
fi

# Remove any existing system-wide TARR Annunciator service
print_status "Removing any existing system-wide TARR Annunciator service..."
SERVICE_NAMES=("tarr-annunciator.service" "annunciator.service")

for service in "${SERVICE_NAMES[@]}"; do
    if systemctl is-active --quiet "$service" 2>/dev/null; then
        print_status "Stopping system-wide service: $service"
        sudo systemctl stop "$service"
    fi
    if systemctl is-enabled --quiet "$service" 2>/dev/null; then
        print_status "Disabling system-wide service: $service"
        sudo systemctl disable "$service"
    fi
    if [ -f "/etc/systemd/system/$service" ]; then
        print_status "Removing system-wide service file: $service"
        sudo rm "/etc/systemd/system/$service"
    fi
done

# Also check for existing user service
if systemctl --user is-active --quiet tarr-annunciator.service 2>/dev/null; then
    print_status "Stopping existing user service: tarr-annunciator.service"
    systemctl --user stop tarr-annunciator.service
fi

sudo systemctl daemon-reload 2>/dev/null || true
systemctl --user daemon-reload 2>/dev/null || true
print_success "Existing services cleaned up"

# Create systemd user service directory
print_status "Creating TARR Annunciator systemd user service..."
mkdir -p "$USER_HOME/.config/systemd/user"

# Create the user service file
cat > "$USER_HOME/.config/systemd/user/tarr-annunciator.service" << EOF
[Unit]
Description=TARR Annunciator Train Announcement System
After=graphical-session.target pulseaudio.service pipewire.service
Wants=pulseaudio.service pipewire.service

[Service]
Type=simple
ExecStart=$TARR_DIR/TARR_Annunciator_RaspberryPi_ARM64
WorkingDirectory=$TARR_DIR
Restart=always
RestartSec=5

# Preserve user environment for audio access
Environment=HOME=$USER_HOME
Environment=USER=$CURRENT_USER
Environment=LOGNAME=$CURRENT_USER
Environment=XDG_RUNTIME_DIR=/run/user/%i

# Audio environment variables for PipeWire and PulseAudio
Environment=PULSE_SERVER=unix:/run/user/%i/pulse/native
Environment=PIPEWIRE_RUNTIME_DIR=/run/user/%i/pipewire

# Logging to both journal and files
StandardOutput=append:$USER_HOME/tarr-annunciator.log
StandardError=append:$USER_HOME/tarr-annunciator-error.log
SyslogIdentifier=tarr-annunciator

[Install]
WantedBy=default.target
EOF

print_success "User service file created"

# Enable lingering for current user (allows user services to start at boot)
print_status "Enabling user service lingering..."
sudo loginctl enable-linger "$CURRENT_USER"
print_success "User lingering enabled for $CURRENT_USER - services will start at boot"

# Reload user systemd daemon
print_status "Reloading user systemd daemon..."
systemctl --user daemon-reload

# Enable the service
print_status "Enabling TARR Annunciator user service..."
systemctl --user enable tarr-annunciator.service
print_success "TARR Annunciator service enabled for automatic startup"

# Start the service
print_status "Starting TARR Annunciator user service..."
systemctl --user start tarr-annunciator.service
sleep 3

# Check service status
print_status "Checking service status..."
if systemctl --user is-active --quiet tarr-annunciator.service; then
    print_success "✅ TARR Annunciator user service is running!"
else
    print_warning "⚠️  Service may not be running properly"
fi

# Show service status
echo ""
echo "=== TARR Annunciator Service Status ==="
systemctl --user status tarr-annunciator.service --no-pager -l || true

# Create management scripts in the TARR directory
print_status "Creating TARR Annunciator service management scripts..."

# Start script
cat > "$TARR_DIR/tarr-start.sh" << 'EOF'
#!/bin/bash
echo "Starting TARR Annunciator user service..."
systemctl --user start tarr-annunciator.service
if systemctl --user is-active --quiet tarr-annunciator.service; then
    echo "TARR Annunciator service started successfully"
    echo "Check status: systemctl --user status tarr-annunciator.service"
    echo "View logs: journalctl --user -u tarr-annunciator.service -f"
else
    echo "Failed to start TARR Annunciator service"
    echo "Check status: systemctl --user status tarr-annunciator.service"
fi
EOF

# Stop script
cat > "$TARR_DIR/tarr-stop.sh" << 'EOF'
#!/bin/bash
echo "Stopping TARR Annunciator user service..."
systemctl --user stop tarr-annunciator.service
if systemctl --user is-active --quiet tarr-annunciator.service; then
    echo "Failed to stop TARR Annunciator service"
else
    echo "TARR Annunciator service stopped"
fi
EOF

# Restart script
cat > "$TARR_DIR/tarr-restart.sh" << 'EOF'
#!/bin/bash
echo "Restarting TARR Annunciator user service..."
systemctl --user restart tarr-annunciator.service
if systemctl --user is-active --quiet tarr-annunciator.service; then
    echo "TARR Annunciator service restarted successfully"
    echo "Check status: systemctl --user status tarr-annunciator.service"
else
    echo "Failed to restart TARR Annunciator service"
    echo "Check status: systemctl --user status tarr-annunciator.service"
fi
EOF

# Status script
cat > "$TARR_DIR/tarr-status.sh" << 'EOF'
#!/bin/bash
echo "TARR Annunciator systemd service status:"
echo "========================================"
systemctl --user status tarr-annunciator.service
echo ""
echo "Recent logs:"
echo "============"
journalctl --user -u tarr-annunciator.service --no-pager -l -n 20
echo ""
echo "Use 'journalctl --user -u tarr-annunciator.service -f' to follow logs in real-time"
EOF

# Logs script
cat > "$TARR_DIR/tarr-logs.sh" << 'EOF'
#!/bin/bash
echo "TARR Annunciator logs (press Ctrl+C to exit):"
echo "=============================================="
journalctl --user -u tarr-annunciator.service -f
EOF

chmod +x "$TARR_DIR/tarr-start.sh" "$TARR_DIR/tarr-stop.sh" "$TARR_DIR/tarr-restart.sh" "$TARR_DIR/tarr-status.sh" "$TARR_DIR/tarr-logs.sh"

print_success "TARR Annunciator management scripts created"

echo ""
print_success "🎉 TARR Annunciator systemd user service setup completed!"
echo ""
echo "=== Service Management Scripts ==="
echo "📋 Status:   ./tarr-status.sh"
echo "▶️  Start:    ./tarr-start.sh"
echo "⏹️  Stop:     ./tarr-stop.sh"
echo "🔄 Restart:  ./tarr-restart.sh"
echo "📝 Logs:     ./tarr-logs.sh"
echo ""
echo "=== Manual systemctl Commands ==="
echo "Status:   systemctl --user status tarr-annunciator.service"
echo "Start:    systemctl --user start tarr-annunciator.service"
echo "Stop:     systemctl --user stop tarr-annunciator.service"
echo "Restart:  systemctl --user restart tarr-annunciator.service"
echo "Logs:     journalctl --user -u tarr-annunciator.service -f"
echo ""
echo "=== Auto-start Configuration ==="
if systemctl --user is-enabled --quiet tarr-annunciator.service; then
    echo "✅ TARR Annunciator service will start automatically at boot"
else
    echo "❌ TARR Annunciator service not enabled for auto-start"
fi

if sudo loginctl show-user "$CURRENT_USER" | grep -q "Linger=yes"; then
    echo "✅ User lingering enabled for $CURRENT_USER - service starts without login"
else
    echo "⚠️  User lingering not enabled - may need login to start"
fi

echo ""
echo "🎵 The systemd user service preserves audio permissions!"
echo "🌐 Web interface: http://$(hostname -I | awk '{print $1}' 2>/dev/null):8080"
echo "🌐 Local access: http://localhost:8080"
echo ""
echo "📁 Service logs are written to:"
echo "   • systemd journal: journalctl --user -u tarr-annunciator.service"
echo "   • Application log: $USER_HOME/tarr-annunciator.log"
echo "   • Error log: $USER_HOME/tarr-annunciator-error.log"
echo ""
echo "🔧 If audio still doesn't work after setup:"
echo "   1. Check service logs: ./tarr-logs.sh"
echo "   2. Verify user is in audio group: groups \$USER"
echo "   3. Test audio manually: speaker-test -t wav"
echo "   4. Use Admin Panel → Audio Controls for audio system override"