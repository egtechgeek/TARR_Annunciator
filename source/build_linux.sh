#!/bin/bash
# TARR Annunciator Linux Installation Script
# Builds the application and optionally installs as systemd service

echo "==============================================="
echo "TARR Annunciator Linux Installation"
echo "==============================================="

# Check for root privileges for service installation
if [ "$EUID" -eq 0 ]; then
    echo "Running as root - systemd service installation available"
    ROOT_PRIVILEGES=true
else
    echo "Running as regular user - service installation will require sudo"
    ROOT_PRIVILEGES=false
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

echo "Go version: $(go version)"

# Check for required directories
mkdir -p json templates static logs

echo "Directory structure verified"

# Check audio system
echo "Checking Linux audio system..."
if command -v pactl &> /dev/null; then
    echo "PulseAudio detected - full audio device control available"
    PULSE_AVAILABLE=true
else
    PULSE_AVAILABLE=false
fi

if command -v aplay &> /dev/null; then
    echo "ALSA utilities detected - basic audio support available"
    ALSA_AVAILABLE=true
else
    ALSA_AVAILABLE=false
fi

if [ "$PULSE_AVAILABLE" = false ] && [ "$ALSA_AVAILABLE" = false ]; then
    echo "WARNING: No audio system detected"
    echo "Install audio utilities:"
    echo "  sudo apt install pulseaudio-utils alsa-utils    # Ubuntu/Debian"
    echo "  sudo yum install pulseaudio-utils alsa-utils     # RHEL/CentOS"
fi

# Download dependencies
echo ""
echo "Downloading dependencies..."
go mod download
if [ $? -ne 0 ]; then
    echo "Error: Failed to download dependencies"
    exit 1
fi

# Build the application
echo "Building executable..."
go build -o tarr-annunciator .
if [ $? -ne 0 ]; then
    echo "Error: Build failed"
    exit 1
fi

echo "Build completed successfully!"
echo "Executable created: tarr-annunciator"

echo ""
echo "==============================================="
echo "Service Installation Options"  
echo "==============================================="

# Service installation prompt
if [ "$ROOT_PRIVILEGES" = true ] || command -v sudo &> /dev/null; then
    echo "Would you like to install TARR Annunciator as a systemd service?"
    echo "This will allow the application to start automatically at boot."
    echo ""
    read -p "Install as systemd service? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo ""
        echo "Installing TARR Annunciator as systemd service..."
        
        INSTALL_DIR="$(pwd)"
        SERVICE_NAME="tarr-annunciator"
        SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
        
        # Create service file
        echo "Creating systemd service file..."
        SERVICE_CONTENT="[Unit]
Description=TARR Annunciator Train Announcement System
After=network.target sound.target

[Service]
Type=simple
User=$USER
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
            echo "Service file created successfully!"
            
            # Reload systemd and enable service
            if [ "$ROOT_PRIVILEGES" = true ]; then
                systemctl daemon-reload
                systemctl enable "$SERVICE_NAME"
            else
                sudo systemctl daemon-reload
                sudo systemctl enable "$SERVICE_NAME"
            fi
            
            if [ $? -eq 0 ]; then
                echo "Service enabled successfully!"
                echo ""
                echo "Service Details:"
                echo "- Name: $SERVICE_NAME"
                echo "- Description: TARR Annunciator Train Announcement System"
                echo "- Start Type: Enabled (starts at boot)"
                echo "- Working Directory: $INSTALL_DIR"
                echo ""
                
                read -p "Start the service now? (Y/n): " -n 1 -r
                echo
                if [[ ! $REPLY =~ ^[Nn]$ ]]; then
                    echo "Starting TARR Annunciator service..."
                    if [ "$ROOT_PRIVILEGES" = true ]; then
                        systemctl start "$SERVICE_NAME"
                    else
                        sudo systemctl start "$SERVICE_NAME"
                    fi
                    
                    if [ $? -eq 0 ]; then
                        echo "Service started successfully!"
                        
                        # Show service status
                        sleep 2
                        if [ "$ROOT_PRIVILEGES" = true ]; then
                            systemctl --no-pager status "$SERVICE_NAME"
                        else
                            sudo systemctl --no-pager status "$SERVICE_NAME"
                        fi
                    else
                        echo "WARNING: Service failed to start"
                        echo "Check logs with: journalctl -u $SERVICE_NAME -f"
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
                echo "ERROR: Failed to enable service"
                echo "You may need to run with sudo or check systemctl permissions"
            fi
        else
            echo "ERROR: Failed to create service file"
            echo "Make sure you have write permissions to /etc/systemd/system/"
        fi
    else
        echo "Skipping service installation"
        echo "You can run the application manually using: ./tarr-annunciator"
    fi
else
    echo "Service installation not available - sudo not found"
    echo "Install sudo or run as root to enable systemd service installation"
fi

echo ""
echo "==============================================="
echo "Installation Complete!"
echo "==============================================="
echo ""

if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
    echo "TARR Annunciator installed as systemd service"
    echo "The service will start automatically on boot"
    echo ""
    echo "Access Points:"
    echo "- Web Interface: http://localhost:8080"
    echo "- Admin Panel: http://localhost:8080/admin"
    echo "- API Documentation: http://localhost:8080/api/docs"
    echo ""
    echo "Service Management:"
    echo "- View status: sudo systemctl status $SERVICE_NAME"
    echo "- View logs: journalctl -u $SERVICE_NAME -f"
    echo ""
else
    echo "TARR Annunciator ready for manual operation"
    echo ""
    echo "Manual Operation:"
    echo "- Run: ./tarr-annunciator"
    echo "- Or: ./run_linux.sh"
    echo ""
fi

echo "Configuration:"
echo "- Edit JSON files in json/ directory"
echo "- Access admin panel for web-based configuration"
echo ""

echo "Audio System Notes:"
if [ "$PULSE_AVAILABLE" = true ]; then
    echo "- PulseAudio detected: Full device switching available"
else
    echo "- PulseAudio not found: Install with 'sudo apt install pulseaudio-utils'"
fi

if [ "$ALSA_AVAILABLE" = true ]; then
    echo "- ALSA utilities detected: Basic audio support available"
else
    echo "- ALSA not found: Install with 'sudo apt install alsa-utils'"
fi

echo ""

# Test audio if available
read -p "Test audio system now? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Testing audio system..."
    if command -v speaker-test &> /dev/null; then
        echo "Running speaker test (Ctrl+C to stop)..."
        timeout 3 speaker-test -t sine -f 1000 -c 2 2>/dev/null || true
    elif [ -f "./tarr-annunciator" ]; then
        echo "Testing with TARR Annunciator audio test..."
        ./tarr-annunciator --test-audio 2>/dev/null || echo "Note: Start application to test audio via web interface"
    else
        echo "No audio test available - test via web interface after starting"
    fi
fi

echo ""
echo "Installation completed successfully!"

if [ ! -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
    echo "To start manually: ./tarr-annunciator"
else
    echo "Service installed: $SERVICE_NAME"
    echo "Check status: sudo systemctl status $SERVICE_NAME"
fi

echo ""