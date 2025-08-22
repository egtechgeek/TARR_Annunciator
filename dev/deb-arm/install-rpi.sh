#!/bin/bash
# TARR Annunciator - Raspberry Pi Installation Script

set -e

echo "=================================="
echo "TARR Annunciator - Raspberry Pi Setup"
echo "=================================="

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo "Please do not run this script as root."
    echo "Run as a regular user with sudo privileges."
    exit 1
fi

# Update system packages
echo "Updating system packages..."
sudo apt update && sudo apt upgrade -y

# Install system dependencies
echo "Installing system dependencies..."
sudo apt install -y \
    python3 \
    python3-pip \
    python3-venv \
    pulseaudio \
    pulseaudio-utils \
    alsa-utils \
    cron \
    git

# Install pygame dependencies
echo "Installing pygame dependencies..."
sudo apt install -y \
    libsdl2-dev \
    libsdl2-image-dev \
    libsdl2-mixer-dev \
    libsdl2-ttf-dev \
    libfreetype6-dev \
    python3-dev

# Start and enable PulseAudio
echo "Setting up PulseAudio..."
systemctl --user enable pulseaudio
systemctl --user start pulseaudio || true

# Create virtual environment
echo "Creating Python virtual environment..."
python3 -m venv venv
source venv/bin/activate

# Install Python dependencies
echo "Installing Python dependencies..."
pip install --upgrade pip
pip install -r requirements.txt

# Create necessary directories
echo "Creating directories..."
mkdir -p json static/mp3/{train,direction,destination,track,promo,safety} logs

# Set up logging
echo "Setting up logging..."
sudo mkdir -p /var/log
sudo touch /var/log/tarr-announcer.log
sudo chown $USER:$USER /var/log/tarr-announcer.log

# Create systemd service file
echo "Creating systemd service..."
sudo tee /etc/systemd/system/tarr-announcer.service > /dev/null <<EOF
[Unit]
Description=TARR Annunciator Web Service
After=network.target sound.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$(pwd)
Environment=PATH=$(pwd)/venv/bin
ExecStart=$(pwd)/venv/bin/python $(pwd)/app.py
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start the service
echo "Enabling TARR Annunciator service..."
sudo systemctl daemon-reload
sudo systemctl enable tarr-announcer.service

# Test audio setup
echo "Testing audio system..."
pulseaudio --check || pulseaudio --start
echo "PulseAudio status: $(pulseaudio --check && echo "Running" || echo "Not running")"

# Set appropriate permissions
chmod +x app.py validate_cron.py api_test.py

# Create sample configuration if it doesn't exist
if [ ! -f "json/cron.json" ]; then
    echo "Creating sample configuration..."
    mkdir -p json
    cat > json/cron.json << 'EOF'
{
    "station_announcements": [],
    "promo_announcements": [],
    "safety_announcements": []
}
EOF
fi

echo ""
echo "=================================="
echo "Installation Complete!"
echo "=================================="
echo ""
echo "To start the service:"
echo "  sudo systemctl start tarr-announcer.service"
echo ""
echo "To check service status:"
echo "  sudo systemctl status tarr-announcer.service"
echo ""
echo "To run manually (for testing):"
echo "  source venv/bin/activate"
echo "  python3 app.py"
echo ""
echo "Web interface will be available at:"
echo "  http://$(hostname -I | awk '{print $1}'):8080"
echo "  http://localhost:8080 (local access)"
echo ""
echo "Test audio:"
echo "  python3 app.py --test-audio"
echo ""
echo "Update crontab from config:"
echo "  python3 app.py --update-cron"
echo ""
echo "Log file location: /var/log/tarr-announcer.log"
echo ""