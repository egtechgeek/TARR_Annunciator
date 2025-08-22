#!/bin/bash

echo "🔧 TARR Pydub Installation & Fix"
echo "================================"

# Check current Python and pip
echo -e "\n1. 📋 Python Environment:"
echo "Python version: $(python3 --version)"
echo "Pip version: $(pip3 --version)"
echo "Current user: $(whoami)"
echo "Current directory: $(pwd)"

# Check if pydub is installed
echo -e "\n2. 📦 Checking pydub installation:"
python3 -c "import pydub; print('✅ pydub is available')" 2>/dev/null || echo "❌ pydub not available"

# Check dependencies
echo -e "\n3. 🔍 Checking pydub dependencies:"

# Check ffmpeg
if command -v ffmpeg &> /dev/null; then
    echo "✅ ffmpeg is available"
    ffmpeg -version | head -1
else
    echo "❌ ffmpeg not available (required for pydub)"
fi

# Check simpleaudio (pydub playback dependency)
python3 -c "import simpleaudio; print('✅ simpleaudio is available')" 2>/dev/null || echo "❌ simpleaudio not available"

# Install missing components
echo -e "\n4. 📦 Installing missing components:"

# Install system dependencies
echo "Installing system dependencies..."
sudo apt update
sudo apt install -y ffmpeg python3-dev python3-pip build-essential libasound2-dev portaudio19-dev

# Install Python packages
echo -e "\nInstalling Python packages..."
pip3 install --user pydub
pip3 install --user simpleaudio

# Alternative: install with sudo if user install fails
echo -e "\nTrying system-wide installation as backup..."
sudo pip3 install pydub simpleaudio 2>/dev/null || echo "System install skipped"

# Test installation
echo -e "\n5. 🧪 Testing pydub installation:"

python3 -c "
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

try:
    import simpleaudio
    print('✅ simpleaudio import successful')
except ImportError as e:
    print(f'❌ simpleaudio import failed: {e}')
"

# Test with actual file
if [ -f "static/mp3/safety/safety_english.mp3" ]; then
    echo -e "\n6. 🎵 Testing audio file loading:"
    python3 -c "
from pydub import AudioSegment
try:
    audio = AudioSegment.from_mp3('static/mp3/safety/safety_english.mp3')
    print(f'✅ Audio file loaded successfully: {len(audio)}ms duration')
except Exception as e:
    print(f'❌ Audio file loading failed: {e}')
"
else
    echo -e "\n⚠️  Safety audio file not found for testing"
fi

echo -e "\n7. 🔧 Alternative solutions if pydub still doesn't work:"
echo "Option A: Use system audio tools (paplay, mpg123)"
echo "Option B: Convert MP3 files to WAV (better compatibility)"
echo "Option C: Use pygame with proper headless setup"

echo -e "\n✅ Installation complete!"
echo "Now test with: python3 app.py --safety --language english"
