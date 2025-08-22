#!/bin/bash

# This script will update the TARR Annunciator Raspberry Pi version to the latest version.
# It should be run as the user who installed the application (not root).

# Exit on any error
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$(id -u)" -eq 0 ]; then
    echo -e "${RED}This script should not be run as root!${NC}"
    echo "Run as the same user who installed TARR Annunciator."
    exit 1
fi

# Get current directory
CURRENT_DIR=$(pwd)
APP_DIR="$CURRENT_DIR"

echo -e "${BLUE}=================================="
echo "TARR Annunciator - Raspberry Pi Update"
echo -e "==================================${NC}"

# Check if this is a valid TARR Annunciator directory
if [ ! -f "app.py" ] || [ ! -f "requirements.txt" ]; then
    echo -e "${RED}Error: This doesn't appear to be a TARR Annunciator directory${NC}"
    echo "Please run this script from the TARR Annunciator installation directory"
    exit 1
fi

# Stop the service if it's running
echo -e "${YELLOW}Stopping TARR Annunciator service...${NC}"
sudo systemctl stop tarr-announcer.service 2>/dev/null || echo "Service was not running"

# Backup current configuration
echo -e "${YELLOW}Backing up current configuration...${NC}"
BACKUP_DIR="/tmp/tarr-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

if [ -d "json" ]; then
    cp -r json "$BACKUP_DIR/"
    echo "Configuration backed up to: $BACKUP_DIR"
fi

if [ -d "static/mp3" ]; then
    cp -r static/mp3 "$BACKUP_DIR/"
    echo "Audio files backed up to: $BACKUP_DIR"
fi

# Clone the latest version from GitHub
echo -e "${YELLOW}Downloading latest version...${NC}"
TEMP_DIR="/tmp/TARR_Annunciator_update"
rm -rf "$TEMP_DIR"
git clone https://github.com/egtechgeek/TARR_Annunciator.git "$TEMP_DIR"

# Copy updated files from the deb-arm directory
echo -e "${YELLOW}Updating application files...${NC}"
SOURCE_DIR="$TEMP_DIR/dev/deb-arm"

if [ ! -d "$SOURCE_DIR" ]; then
    echo -e "${RED}Error: Could not find Raspberry Pi version in repository${NC}"
    echo "Expected directory: $SOURCE_DIR"
    rm -rf "$TEMP_DIR"
    exit 1
fi

# Copy core application files
cp "$SOURCE_DIR/app.py" "$APP_DIR/"
cp "$SOURCE_DIR/validate_cron.py" "$APP_DIR/"
cp "$SOURCE_DIR/api_test.py" "$APP_DIR/"
cp "$SOURCE_DIR/requirements.txt" "$APP_DIR/"
cp "$SOURCE_DIR/install-rpi.sh" "$APP_DIR/"

# Copy templates if they exist
if [ -d "$SOURCE_DIR/templates" ]; then
    cp -r "$SOURCE_DIR/templates" "$APP_DIR/"
else
    echo -e "${YELLOW}Warning: No templates directory found in source${NC}"
fi

# Copy static files (but preserve existing mp3 files)
if [ -d "$SOURCE_DIR/static" ]; then
    # Copy static files but skip mp3 directory to preserve user audio files
    rsync -av --exclude='mp3' "$SOURCE_DIR/static/" "$APP_DIR/static/" 2>/dev/null || {
        echo -e "${YELLOW}Note: rsync not available, using cp (mp3 files may be overwritten)${NC}"
        cp -r "$SOURCE_DIR/static" "$APP_DIR/"
    }
else
    echo -e "${YELLOW}Warning: No static directory found in source${NC}"
fi

# Set executable permissions
chmod +x "$APP_DIR/app.py"
chmod +x "$APP_DIR/validate_cron.py" 
chmod +x "$APP_DIR/api_test.py"
chmod +x "$APP_DIR/install-rpi.sh"

# Activate virtual environment if it exists
if [ -d "venv" ]; then
    echo -e "${YELLOW}Updating Python dependencies...${NC}"
    source venv/bin/activate
    pip install --upgrade pip
    pip install -r requirements.txt
    deactivate
else
    echo -e "${RED}Warning: Virtual environment not found${NC}"
    echo "You may need to reinstall dependencies manually"
fi

# Restore configuration files
echo -e "${YELLOW}Restoring configuration...${NC}"
if [ -d "$BACKUP_DIR/json" ]; then
    cp -r "$BACKUP_DIR/json" "$APP_DIR/"
    echo "Configuration restored"
fi

# Update crontab with any new schedule
echo -e "${YELLOW}Updating crontab...${NC}"
if [ -d "venv" ]; then
    source venv/bin/activate
    python3 app.py --update-cron
    deactivate
else
    python3 app.py --update-cron
fi

# Update systemd service file
echo -e "${YELLOW}Updating systemd service...${NC}"
sudo tee /etc/systemd/system/tarr-announcer.service > /dev/null <<EOF
[Unit]
Description=TARR Annunciator Web Service
After=network.target sound.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$APP_DIR
Environment=PATH=$APP_DIR/venv/bin
ExecStart=$APP_DIR/venv/bin/python $APP_DIR/app.py
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and restart service
sudo systemctl daemon-reload

# Clean up temporary files
rm -rf "$TEMP_DIR"

# Start the service
echo -e "${YELLOW}Starting TARR Annunciator service...${NC}"
sudo systemctl start tarr-announcer.service

# Check service status
sleep 2
if sudo systemctl is-active --quiet tarr-announcer.service; then
    echo -e "${GREEN}✓ Service started successfully${NC}"
else
    echo -e "${RED}⚠ Service may have failed to start${NC}"
    echo "Check status with: sudo systemctl status tarr-announcer.service"
fi

echo ""
echo -e "${GREEN}=================================="
echo "Update Complete!"
echo -e "==================================${NC}"
echo ""
echo -e "${BLUE}Service status:${NC}"
sudo systemctl status tarr-announcer.service --no-pager -l

echo ""
echo -e "${BLUE}Backup location:${NC} $BACKUP_DIR"
echo -e "${BLUE}Web interface:${NC} http://$(hostname -I | awk '{print $1}'):8080"
echo -e "${BLUE}Logs:${NC} /var/log/tarr-announcer.log"
echo ""
echo -e "${YELLOW}To view logs:${NC} tail -f /var/log/tarr-announcer.log"
echo -e "${YELLOW}To restart service:${NC} sudo systemctl restart tarr-announcer.service"
echo ""