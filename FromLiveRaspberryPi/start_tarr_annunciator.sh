#!/bin/bash

# TARR Annunciator Startup Script
# This script launches the TARR Annunciator in a GNU Screen session

TARR_DIR="/home/pi/TARR_Annunciator_RaspberryPi_ARM64"
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
