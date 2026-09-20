#!/bin/bash

# Convert Annunciator System from systemd to screen-based autostart
# Run this script on the Raspberry Pi as the pi user: ./convert_to_screen.sh

set -e  # Exit on any error

echo "🔧 Converting Annunciator System from systemd to screen-based autostart..."

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

# Check if running as pi user
if [ "$USER" != "pi" ]; then
    print_error "This script must be run as the 'pi' user"
    print_error "Current user: $USER"
    exit 1
fi

# Check if we're in the right directory
if [ ! -f "/home/pi/annunciator/annunciator" ]; then
    print_error "Annunciator binary not found at /home/pi/annunciator/annunciator"
    print_error "Please run this script from /home/pi/annunciator/ or ensure the installation is correct"
    exit 1
fi

print_status "Current user: $USER"
print_status "Annunciator directory: $(pwd)"

# Step 1: Stop and disable systemd service
print_status "Step 1: Removing systemd service..."

# Stop the service if it's running
if systemctl is-active --quiet annunciator.service 2>/dev/null; then
    print_status "Stopping annunciator service..."
    sudo systemctl stop annunciator.service
    print_success "Service stopped"
else
    print_status "Service is not currently running"
fi

# Disable the service if it exists
if systemctl is-enabled --quiet annunciator.service 2>/dev/null; then
    print_status "Disabling annunciator service..."
    sudo systemctl disable annunciator.service
    print_success "Service disabled"
else
    print_status "Service is not enabled"
fi

# Remove the service file
if [ -f "/etc/systemd/system/annunciator.service" ]; then
    print_status "Removing service file..."
    sudo rm /etc/systemd/system/annunciator.service
    sudo systemctl daemon-reload
    print_success "Service file removed"
else
    print_status "Service file does not exist"
fi

# Step 2: Create screen startup script
print_status "Step 2: Creating screen startup script..."

cd /home/pi/annunciator

cat > screen_start_annunciator.sh << 'EOF'
#!/bin/bash
cd /home/pi/annunciator
echo "Starting Annunciator System in GNU Screen session..."

# Kill any existing screen sessions
screen -ls | grep annunciator-system && screen -S annunciator-system -X quit 2>/dev/null
sleep 1

# Start new screen session
screen -dmS annunciator-system bash -c 'cd /home/pi/annunciator && echo "=== Annunciator System Started ===" && echo "Web interface: http://$(hostname -I | awk '"'"'{print $1}'"'"'):8080" && echo "================================" && ./annunciator'

echo "Screen session 'annunciator-system' started"
echo "To attach to session: screen -r annunciator-system"
echo "To detach from session: Ctrl+A then D"
echo "Web interface: http://$(hostname -I | awk '{print $1}'):8080"
EOF

chmod +x screen_start_annunciator.sh
print_success "Screen startup script created"

# Step 3: Create stop script for convenience
print_status "Creating stop script..."

cat > screen_stop_annunciator.sh << 'EOF'
#!/bin/bash
echo "Stopping Annunciator System screen session..."

# Check if session exists
if screen -list | grep -q annunciator-system; then
    screen -S annunciator-system -X quit
    echo "Screen session 'annunciator-system' stopped"
else
    echo "No annunciator-system screen session found"
fi
EOF

chmod +x screen_stop_annunciator.sh
print_success "Screen stop script created"

# Step 4: Check if autologin is enabled
print_status "Step 3: Checking autologin configuration..."

check_autologin() {
    if grep -q "autologin-user=pi" /etc/lightdm/lightdm.conf 2>/dev/null; then
        return 0  # autologin enabled
    elif [ -f "/etc/systemd/system/getty@tty1.service.d/autologin.conf" ]; then
        return 0  # autologin enabled via systemd
    else
        return 1  # autologin not enabled
    fi
}

AUTOLOGIN_ENABLED=false
if check_autologin; then
    AUTOLOGIN_ENABLED=true
    print_success "Autologin is already enabled for pi user"
else
    print_warning "Autologin is not enabled"
    echo
    echo "To enable autologin, you have two options:"
    echo "1. Use raspi-config: sudo raspi-config -> Boot Options -> Console Autologin"
    echo "2. Manual setup (this script can do it for you)"
    echo
    read -p "Would you like this script to enable autologin? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_status "Enabling autologin for pi user..."
        
        # Create autologin configuration
        sudo mkdir -p /etc/systemd/system/getty@tty1.service.d
        sudo tee /etc/systemd/system/getty@tty1.service.d/autologin.conf > /dev/null << 'EOF'
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin pi --noclear %I $TERM
EOF
        
        sudo systemctl daemon-reload
        AUTOLOGIN_ENABLED=true
        print_success "Autologin enabled - will take effect after reboot"
    fi
fi

# Step 5: Add autostart to .bashrc
print_status "Step 4: Setting up autostart in .bashrc..."

# Remove any existing annunciator autostart entries
sed -i '/# Auto-start Annunciator System/d' ~/.bashrc
sed -i '/annunciator.*screen/d' ~/.bashrc
sed -i '/screen.*annunciator/d' ~/.bashrc

if $AUTOLOGIN_ENABLED; then
    # Add autostart to .bashrc
    echo >> ~/.bashrc
    echo "# Auto-start Annunciator System in screen session" >> ~/.bashrc
    echo "if [ \$(screen -list | grep -c annunciator-system) -eq 0 ]; then" >> ~/.bashrc
    echo "    echo 'Starting Annunciator System...'" >> ~/.bashrc
    echo "    /home/pi/annunciator/screen_start_annunciator.sh" >> ~/.bashrc
    echo "    echo 'Annunciator System started in screen session. Use: screen -r annunciator-system'" >> ~/.bashrc
    echo "fi" >> ~/.bashrc
    
    print_success "Autostart added to .bashrc"
else
    print_warning "Autostart not added to .bashrc because autologin is not enabled"
    print_status "You can manually start with: ./screen_start_annunciator.sh"
fi

# Step 6: Create status script
print_status "Creating status script..."

cat > annunciator_status.sh << 'EOF'
#!/bin/bash
echo "=== Annunciator System Status ==="
echo

# Check if screen session exists
if screen -list | grep -q annunciator-system; then
    echo "Status: ✅ RUNNING in screen session"
    echo "Session: annunciator-system"
    echo
    echo "Commands:"
    echo "  Attach to session: screen -r annunciator-system"
    echo "  Detach from session: Ctrl+A then D"
    echo "  Stop system: ./screen_stop_annunciator.sh"
    echo
    echo "Web interface: http://$(hostname -I | awk '{print $1}'):8080"
else
    echo "Status: ❌ NOT RUNNING"
    echo
    echo "Commands:"
    echo "  Start system: ./screen_start_annunciator.sh"
fi

echo
echo "Recent screen sessions:"
screen -list
EOF

chmod +x annunciator_status.sh
print_success "Status script created"

print_success "Conversion completed successfully!"
echo
echo "=================================================="
echo "🎉 Annunciator System converted to screen-based startup!"
echo "=================================================="
echo
print_status "Summary of changes:"
echo "  ❌ Removed systemd service (annunciator.service)"
echo "  ✅ Created screen startup script (screen_start_annunciator.sh)"
echo "  ✅ Created screen stop script (screen_stop_annunciator.sh)"
echo "  ✅ Created status script (annunciator_status.sh)"
if $AUTOLOGIN_ENABLED; then
    echo "  ✅ Added autostart to .bashrc (will start on login)"
else
    echo "  ⚠️  Autologin not enabled - manual start required"
fi
echo
print_status "Available scripts:"
echo "  ./screen_start_annunciator.sh  - Start the system"
echo "  ./screen_stop_annunciator.sh   - Stop the system"  
echo "  ./annunciator_status.sh        - Check status"
echo "  ./start_annunciator.sh         - Manual start (no screen)"
echo
if $AUTOLOGIN_ENABLED; then
    print_warning "Next steps:"
    echo "  1. Reboot the Pi to enable autologin and autostart"
    echo "  2. After reboot, the system should start automatically"
    echo "  3. Use 'screen -r annunciator-system' to attach to the session"
else
    print_warning "Next steps:"
    echo "  1. Enable autologin with: sudo raspi-config"
    echo "  2. Reboot the Pi"
    echo "  3. Or start manually with: ./screen_start_annunciator.sh"
fi
echo
print_success "Conversion script completed!"