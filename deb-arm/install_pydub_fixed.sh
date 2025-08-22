#!/bin/bash

echo "🔧 TARR Pydub Installation & Fix (Virtual Environment Compatible)"
echo "================================================================="

# Check current Python and pip
echo -e "\n1. 📋 Python Environment:"
echo "Python version: $(python3 --version)"
echo "Pip version: $(pip3 --version)"
echo "Current user: $(whoami)"
echo "Current directory: $(pwd)"

# Check if we're in a virtual environment
if [[ -n "$VIRTUAL_ENV" ]]; then
    echo "✅ Virtual environment detected: $VIRTUAL_ENV"
    USE_USER=""
elif [[ -n "$CONDA_DEFAULT_ENV" ]]; then
    echo "✅ Conda environment detected: $CONDA_DEFAULT_ENV"
    USE_USER=""
else
    echo "ℹ️  No virtual environment detected, will use --user install"
    USE_USER="--user"
fi

# Install system dependencies first
echo -e "\n2. 📦 Installing system dependencies:"
sudo apt update
sudo apt install -y ffmpeg python3-dev libasound2-dev portaudio19-dev build-essential mpg123 pulseaudio-utils alsa-utils

# Install Python packages
echo -e "\n3. 📦 Installing Python packages:"

# Try installing pydub
echo "Installing pydub..."
if pip3 install $USE_USER pydub; then
    echo "✅ pydub installed successfully"
else
    echo "❌ pydub installation failed"
fi

# Try installing simpleaudio
echo "Installing simpleaudio..."
if pip3 install $USE_USER simpleaudio; then
    echo "✅ simpleaudio installed successfully"
else
    echo "❌ simpleaudio installation failed"
    echo "Trying alternative audio backend..."
    # Try pygame as fallback
    pip3 install $USE_USER pygame || echo "pygame also failed"
fi

# Test installations
echo -e "\n4. 🧪 Testing installations:"

python3 -c "
import sys
print(f'Python executable: {sys.executable}')
print(f'Python path: {sys.path[0]}')

# Test pydub
try:
    from pydub import AudioSegment
    print('✅ AudioSegment import successful')
except ImportError as e:
    print(f'❌ AudioSegment import failed: {e}')

try:
    from pydub.playback import play
    print('✅ pydub.playback import successful')
except ImportError as e:
    print(f'❌ pydub.playback import failed: {e}')

# Test simpleaudio
try:
    import simpleaudio
    print('✅ simpleaudio import successful')
except ImportError as e:
    print(f'❌ simpleaudio import failed: {e}')

# Test system audio tools
import subprocess
tools = ['paplay', 'mpg123', 'aplay', 'ffmpeg']
for tool in tools:
    try:
        result = subprocess.run(['which', tool], capture_output=True)
        if result.returncode == 0:
            print(f'✅ {tool} available')
        else:
            print(f'❌ {tool} not available')
    except:
        print(f'❌ {tool} check failed')
"

# Test with actual audio file
if [ -f "static/mp3/safety/safety_english.mp3" ]; then
    echo -e "\n5. 🎵 Testing audio file loading:"
    python3 -c "
try:
    from pydub import AudioSegment
    audio = AudioSegment.from_mp3('static/mp3/safety/safety_english.mp3')
    print(f'✅ Audio file loaded successfully: {len(audio)}ms duration')
except Exception as e:
    print(f'❌ Audio file loading failed: {e}')
"
else
    echo -e "\n⚠️  Safety audio file not found for testing"
fi

# Test the TARR app directly
echo -e "\n6. 🎯 Testing TARR app audio system:"
python3 -c "
import sys
import os
sys.path.insert(0, '.')

# Try to import the audio functions from app.py
try:
    exec(open('app.py').read())
    print('✅ TARR app loaded successfully')
    print(f'   PulseAudio available: {PULSEAUDIO_AVAILABLE}')
    print(f'   Pydub available: {PYDUB_AVAILABLE}')
    print(f'   Audio available: {AUDIO_AVAILABLE}')
except Exception as e:
    print(f'❌ TARR app loading failed: {e}')
"

echo -e "\n✅ Installation complete!"
echo -e "\n📋 Summary:"
echo "- System audio tools installed (paplay, mpg123, etc.)"
echo "- Python packages installed in current environment"
echo "- Multiple audio backends available for fallback"
echo ""
echo "🧪 Test with: python3 app.py --safety --language english"
