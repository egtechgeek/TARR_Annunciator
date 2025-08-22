#!/bin/bash

# Install additional audio tools for cron compatibility
echo "📦 Installing audio command-line tools for TARR Annunciator..."

# Update package list
sudo apt update

# Install audio playback tools
echo "Installing audio tools..."
sudo apt install -y mpg123 sox pulseaudio-utils

# Test installations
echo -e "\n✅ Testing installed tools:"

if command -v paplay &> /dev/null; then
    echo "✓ paplay available (PulseAudio)"
else
    echo "❌ paplay not available"
fi

if command -v mpg123 &> /dev/null; then
    echo "✓ mpg123 available (MP3 player)"
    mpg123 --version | head -1
else
    echo "❌ mpg123 not available"
fi

if command -v aplay &> /dev/null; then
    echo "✓ aplay available (ALSA player)"
else
    echo "❌ aplay not available"
fi

if command -v play &> /dev/null; then
    echo "✓ play available (SoX)"
else
    echo "❌ play not available"
fi

echo -e "\n🔧 Testing audio tools with a short test..."

# Create a short test tone
if command -v sox &> /dev/null; then
    echo "Generating test tone..."
    sox -n test_tone.wav synth 1 sine 440 vol 0.1
    
    if [ -f "test_tone.wav" ]; then
        echo "✓ Test tone created"
        
        # Test paplay
        if command -v paplay &> /dev/null; then
            echo "Testing paplay..."
            paplay test_tone.wav && echo "✓ paplay works" || echo "❌ paplay failed"
        fi
        
        # Test aplay
        if command -v aplay &> /dev/null; then
            echo "Testing aplay..."
            aplay test_tone.wav && echo "✓ aplay works" || echo "❌ aplay failed"
        fi
        
        # Clean up
        rm -f test_tone.wav
    fi
fi

echo -e "\n📋 Audio tools installation complete!"
echo "Now test the TARR audio system with: python3 app.py --safety --language english"
