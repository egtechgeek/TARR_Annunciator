#!/bin/bash

#################################################################################
# TARR Annunciator - PulseAudio Troubleshooting Script
# This script helps diagnose and fix common PulseAudio issues on Raspberry Pi
#################################################################################

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

check_pulseaudio_status() {
    print_status "Checking PulseAudio status..."
    
    if command_exists pactl; then
        if pactl info >/dev/null 2>&1; then
            print_success "PulseAudio is running"
            
            # Get PulseAudio version
            PA_VERSION=$(pulseaudio --version 2>/dev/null | head -n1)
            print_status "Version: $PA_VERSION"
            
            # Get default sink
            DEFAULT_SINK=$(pactl get-default-sink 2>/dev/null)
            print_status "Default sink: $DEFAULT_SINK"
            
            return 0
        else
            print_error "PulseAudio is not responding"
            return 1
        fi
    else
        print_error "PulseAudio (pactl) not found"
        return 1
    fi
}

list_audio_devices() {
    print_status "Available PulseAudio sinks:"
    
    if pactl list short sinks 2>/dev/null; then
        echo
        print_status "Detailed sink information:"
        pactl list sinks | grep -E "(Name:|Description:|State:)" | head -20
    else
        print_warning "Could not list PulseAudio sinks"
    fi
    
    echo
    print_status "Available ALSA devices (fallback):"
    if command_exists aplay; then
        aplay -l 2>/dev/null || print_warning "No ALSA devices found"
    else
        print_warning "ALSA tools not available"
    fi
}

check_volume_levels() {
    print_status "Checking volume levels..."
    
    # PulseAudio volume
    if pactl get-sink-volume @DEFAULT_SINK@ >/dev/null 2>&1; then
        VOLUME_INFO=$(pactl get-sink-volume @DEFAULT_SINK@ 2>/dev/null)
        print_status "PulseAudio volume: $VOLUME_INFO"
    else
        print_warning "Could not get PulseAudio volume"
    fi
    
    # ALSA volume (fallback)
    if command_exists amixer; then
        PCM_VOLUME=$(amixer get PCM 2>/dev/null | grep -o '[0-9]*%' | head -1)
        MASTER_VOLUME=$(amixer get Master 2>/dev/null | grep -o '[0-9]*%' | head -1)
        if [ -n "$PCM_VOLUME" ]; then
            print_status "ALSA PCM volume: $PCM_VOLUME"
        fi
        if [ -n "$MASTER_VOLUME" ]; then
            print_status "ALSA Master volume: $MASTER_VOLUME"
        fi
    fi
}

restart_pulseaudio() {
    print_status "Restarting PulseAudio..."
    
    # Kill existing PulseAudio processes
    pulseaudio --kill >/dev/null 2>&1
    sleep 2
    
    # Start PulseAudio
    pulseaudio --start >/dev/null 2>&1
    sleep 3
    
    if check_pulseaudio_status; then
        print_success "PulseAudio restarted successfully"
        return 0
    else
        print_error "Failed to restart PulseAudio"
        return 1
    fi
}

test_audio_playback() {
    print_status "Testing audio playback..."
    
    # Test with PulseAudio
    if command_exists paplay; then
        print_status "Testing with paplay..."
        if [ -f "/usr/share/sounds/alsa/Front_Left.wav" ]; then
            paplay /usr/share/sounds/alsa/Front_Left.wav 2>/dev/null && print_success "PulseAudio test successful" || print_warning "PulseAudio test failed"
        else
            print_warning "No test sound file found for PulseAudio"
        fi
    fi
    
    # Test with ALSA (fallback)
    if command_exists aplay; then
        print_status "Testing with aplay (fallback)..."
        if [ -f "/usr/share/sounds/alsa/Front_Left.wav" ]; then
            aplay /usr/share/sounds/alsa/Front_Left.wav 2>/dev/null && print_success "ALSA test successful" || print_warning "ALSA test failed"
        else
            print_warning "No test sound file found for ALSA"
        fi
    fi
    
    # Test with application if available
    if [ -f "app.py" ]; then
        print_status "Testing with TARR Annunciator..."
        python3 app.py --test-audio
    fi
}

fix_permissions() {
    print_status "Checking and fixing audio permissions..."
    
    # Add user to audio group
    USER=$(whoami)
    if groups $USER | grep -q audio; then
        print_success "User $USER is in audio group"
    else
        print_status "Adding user $USER to audio group..."
        sudo usermod -a -G audio $USER
        print_warning "You may need to log out and back in for group changes to take effect"
    fi
    
    # Check PulseAudio directory permissions
    if [ -d "$HOME/.config/pulse" ]; then
        chmod -R u+rw "$HOME/.config/pulse" 2>/dev/null
        print_status "Fixed PulseAudio config directory permissions"
    fi
}

show_troubleshooting_tips() {
    echo
    print_status "Common troubleshooting tips:"
    echo "1. Restart PulseAudio: pulseaudio --kill && pulseaudio --start"
    echo "2. Check if user is in audio group: groups \$(whoami)"
    echo "3. Set default sink: pactl set-default-sink SINK_NAME"
    echo "4. Set volume: pactl set-sink-volume @DEFAULT_SINK@ 70%"
    echo "5. List available sinks: pactl list short sinks"
    echo "6. Check ALSA devices: aplay -l"
    echo "7. Test with speaker-test: speaker-test -t sine -f 1000 -l 1"
    echo "8. If all else fails, try ALSA directly in app.py"
    echo
}

main_menu() {
    echo "=========================================="
    echo "TARR Annunciator - PulseAudio Diagnostics"
    echo "=========================================="
    echo
    echo "1) Check PulseAudio Status"
    echo "2) List Audio Devices"
    echo "3) Check Volume Levels"
    echo "4) Test Audio Playback"
    echo "5) Restart PulseAudio"
    echo "6) Fix Permissions"
    echo "7) Run Full Diagnostic"
    echo "8) Show Troubleshooting Tips"
    echo "9) Exit"
    echo
    read -p "Select an option (1-9): " choice
    
    case $choice in
        1) check_pulseaudio_status ;;
        2) list_audio_devices ;;
        3) check_volume_levels ;;
        4) test_audio_playback ;;
        5) restart_pulseaudio ;;
        6) fix_permissions ;;
        7) run_full_diagnostic ;;
        8) show_troubleshooting_tips ;;
        9) exit 0 ;;
        *) print_error "Invalid option"; main_menu ;;
    esac
    
    echo
    read -p "Press Enter to continue..."
    main_menu
}

run_full_diagnostic() {
    print_status "Running full PulseAudio diagnostic..."
    echo
    
    check_pulseaudio_status
    echo
    
    list_audio_devices
    echo
    
    check_volume_levels
    echo
    
    fix_permissions
    echo
    
    test_audio_playback
    echo
    
    show_troubleshooting_tips
}

# Check if script is run directly or sourced
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    if [ "$1" = "--auto" ] || [ "$1" = "-a" ]; then
        run_full_diagnostic
    else
        main_menu
    fi
fi
