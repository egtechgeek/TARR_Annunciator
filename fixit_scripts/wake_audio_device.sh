#!/bin/bash

# Wake up the suspended USB audio device
echo "🔧 Waking up suspended USB Sound Blaster..."

# Try to wake up the audio device
echo "Current PipeWire status:"
wpctl status | grep -A 3 -B 3 "Sound Blaster"

echo ""
echo "Attempting to wake up the audio device..."

# Method 1: Play a brief silent audio to wake it up
if command -v wpctl >/dev/null 2>&1; then
    echo "Sending wake-up audio signal..."
    # Create a very brief silent audio file
    timeout 2 speaker-test -D pipewire -t sine -f 440 -l 1 -s 1 >/dev/null 2>&1 || echo "Wake-up signal sent"
fi

# Method 2: Set volume to wake device
echo "Adjusting volume to wake device..."
wpctl set-volume 64 0.8 2>/dev/null || echo "Volume adjustment attempted"

# Method 3: Test direct ALSA access
echo "Testing direct ALSA access..."
timeout 1 aplay -D hw:S3 /dev/zero 2>/dev/null || echo "ALSA wake-up attempted"

echo ""
echo "Checking device status after wake-up attempts:"
wpctl status | grep -A 3 -B 3 "Sound Blaster"

echo ""
echo "Now try the TTS test from the web interface!"
echo "The device should wake up when audio is actually played."