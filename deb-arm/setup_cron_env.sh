#!/bin/bash

# TARR Cron Environment Setup Script
# This ensures cron jobs can access audio properly

echo "🔧 Setting up TARR cron environment..."

# 1. Create a wrapper script that sets up the environment
cat > /tmp/tarr_cron_wrapper.sh << 'EOF'
#!/bin/bash

# TARR Cron Wrapper Script
# Sets up proper environment for audio playback from cron

# Set environment variables for audio
export PULSE_RUNTIME_PATH="/run/user/$(id -u)/pulse"
export XDG_RUNTIME_DIR="/run/user/$(id -u)"

# Ensure PulseAudio is available
if ! pulseaudio --check; then
    # Try to connect to user's pulse session
    export PULSE_SERVER="unix:/run/user/$(id -u)/pulse/native"
fi

# Set up display for SDL (even though we use dummy)
export DISPLAY=:0
export SDL_VIDEODRIVER=dummy

# Change to TARR directory
cd "$(dirname "$0")"

# Run the actual TARR command
exec python3 app.py "$@"
EOF

# Make wrapper executable and move to project directory
chmod +x /tmp/tarr_cron_wrapper.sh
mv /tmp/tarr_cron_wrapper.sh ./tarr_cron_wrapper.sh

echo "✅ Created tarr_cron_wrapper.sh"

# 2. Update the crontab generation to use the wrapper
echo "🔧 The wrapper script is ready."
echo "Next, we need to update the cron command generation..."

# Show what the new cron commands should look like
echo -e "\n📋 New cron commands will use:"
echo "   */2 * * * * cd $(pwd) && ./tarr_cron_wrapper.sh --safety --language english >> /var/log/tarr-announcer.log 2>&1"

echo -e "\n🔧 To complete setup:"
echo "1. Run: chmod +x tarr_cron_wrapper.sh"
echo "2. Test: ./tarr_cron_wrapper.sh --safety --language english"
echo "3. Update cron: python3 app.py --update-cron"
