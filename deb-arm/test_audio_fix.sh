#!/bin/bash

# Quick audio system test and fix script
echo "🔧 TARR Audio System Test & Fix"
echo "==============================="

# Check if audio tools are installed
echo -e "\n1. 📦 Checking audio tools..."

tools_to_check=("paplay" "mpg123" "aplay" "pulseaudio")
missing_tools=()

for tool in "${tools_to_check[@]}"; do
    if command -v "$tool" &> /dev/null; then
        echo "✅ $tool is available"
    else
        echo "❌ $tool is missing"
        missing_tools+=("$tool")
    fi
done

# Install missing tools
if [ ${#missing_tools[@]} -gt 0 ]; then
    echo -e "\n📦 Installing missing audio tools..."
    sudo apt update
    
    # Install based on what's missing
    packages_to_install=()
    
    for tool in "${missing_tools[@]}"; do
        case $tool in
            "paplay")
                packages_to_install+=("pulseaudio-utils")
                ;;
            "mpg123")
                packages_to_install+=("mpg123")
                ;;
            "aplay")
                packages_to_install+=("alsa-utils")
                ;;
            "pulseaudio")
                packages_to_install+=("pulseaudio")
                ;;
        esac
    done
    
    # Remove duplicates
    packages_to_install=($(printf "%s\n" "${packages_to_install[@]}" | sort -u))
    
    echo "Installing: ${packages_to_install[*]}"
    sudo apt install -y "${packages_to_install[@]}"
fi

# Test PulseAudio status
echo -e "\n2. 🔊 Testing PulseAudio..."

if pulseaudio --check; then
    echo "✅ PulseAudio is running"
else
    echo "⚠️  PulseAudio not running, attempting to start..."
    pulseaudio --start
    sleep 2
    if pulseaudio --check; then
        echo "✅ PulseAudio started successfully"
    else
        echo "❌ Failed to start PulseAudio"
    fi
fi

# List audio devices
echo -e "\n3. 🎵 Available audio devices..."
if command -v pactl &> /dev/null; then
    echo "PulseAudio sinks:"
    pactl list short sinks
    
    echo -e "\nDefault sink:"
    pactl get-default-sink 2>/dev/null || echo "No default sink set"
else
    echo "❌ pactl not available"
fi

# Test the TARR safety file specifically
safety_file="/home/pi/TARR_Annunciator/deb-arm/static/mp3/safety/safety_english.mp3"

echo -e "\n4. 🧪 Testing audio playback with different methods..."

if [ -f "$safety_file" ]; then
    echo "✅ Safety file exists: $safety_file"
    
    # Test paplay
    if command -v paplay &> /dev/null; then
        echo -e "\n🔊 Testing paplay..."
        timeout 10 paplay --volume 32768 "$safety_file" 2>&1
        if [ $? -eq 0 ]; then
            echo "✅ paplay succeeded"
        else
            echo "❌ paplay failed"
        fi
    fi
    
    # Test mpg123
    if command -v mpg123 &> /dev/null; then
        echo -e "\n🔊 Testing mpg123..."
        timeout 10 mpg123 -q --gain 70 "$safety_file" 2>&1
        if [ $? -eq 0 ]; then
            echo "✅ mpg123 succeeded"
        else
            echo "❌ mpg123 failed"
        fi
    fi
    
    # Test with plain paplay (no volume)
    if command -v paplay &> /dev/null; then
        echo -e "\n🔊 Testing simple paplay..."
        timeout 10 paplay "$safety_file" 2>&1
        if [ $? -eq 0 ]; then
            echo "✅ simple paplay succeeded"
        else
            echo "❌ simple paplay failed"
        fi
    fi
    
else
    echo "❌ Safety file not found: $safety_file"
fi

# Test manual TARR command
echo -e "\n5. 🎯 Testing TARR app..."
cd "/home/pi/TARR_Annunciator/deb-arm" || exit 1

echo "Running: python3 app.py --safety --language english"
timeout 30 python3 app.py --safety --language english

echo -e "\n✅ Audio test complete!"
echo -e "\n🔧 If audio still doesn't work, try:"
echo "1. Check volume: amixer sget PCM"
echo "2. Set volume: amixer sset PCM 80%"
echo "3. Check default sink: pactl info"
echo "4. List audio devices: aplay -l"
echo "5. Test system audio: speaker-test -c2 -t sine -f 440 -l 1"
