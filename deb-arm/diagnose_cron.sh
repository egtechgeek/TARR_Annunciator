#!/bin/bash

# TARR Annunciator Cron Diagnostic Script
# This script helps diagnose cron scheduler issues

echo "🔍 TARR Annunciator Cron Diagnostics"
echo "====================================="

# Check if cron service is running
echo -e "\n1. 📋 Cron Service Status:"
if command -v systemctl &> /dev/null; then
    echo "Cron service active: $(systemctl is-active cron 2>/dev/null || systemctl is-active crond 2>/dev/null || echo 'INACTIVE')"
    systemctl status cron --no-pager -l 2>/dev/null || systemctl status crond --no-pager -l 2>/dev/null
else
    echo "systemctl not available, checking process list..."
    ps aux | grep -E '[c]ron|[c]rond' || echo "❌ No cron process found"
fi

# Check current user's crontab
echo -e "\n2. 📅 Current User Crontab:"
echo "Current user: $(whoami)"
if crontab -l 2>/dev/null; then
    echo "✅ Crontab exists and is readable"
    
    # Count TARR jobs
    tarr_jobs=$(crontab -l 2>/dev/null | grep -c "tarr-announcer.log" || echo "0")
    echo "TARR jobs in crontab: $tarr_jobs"
    
    if [ "$tarr_jobs" -gt 0 ]; then
        echo -e "\n📋 TARR Cron Jobs:"
        crontab -l 2>/dev/null | grep "tarr-announcer.log"
    fi
else
    echo "❌ No crontab found for current user"
fi

# Check cron.json file
echo -e "\n3. 📄 TARR Configuration:"
if [ -f "json/cron.json" ]; then
    echo "✅ cron.json exists"
    echo "Enabled safety announcements:"
    python3 -c "
import json
with open('json/cron.json') as f:
    data = json.load(f)
for i, item in enumerate(data.get('safety_announcements', [])):
    status = 'ENABLED' if item.get('enabled') else 'disabled'
    print(f'  Safety {i+1}: {item.get(\"cron\", \"N/A\")} - {item.get(\"language\", \"N/A\")} ({status})')
"
else
    echo "❌ json/cron.json not found"
fi

# Check log file
echo -e "\n4. 📜 Log File Status:"
log_file="/var/log/tarr-announcer.log"
if [ -f "$log_file" ]; then
    echo "✅ Log file exists: $log_file"
    echo "File permissions: $(ls -la $log_file)"
    echo "File size: $(du -h $log_file | cut -f1)"
    echo -e "\nLast 10 lines of log:"
    tail -10 "$log_file" 2>/dev/null || echo "Cannot read log file"
else
    echo "❌ Log file doesn't exist: $log_file"
    echo "Checking if directory is writable..."
    if [ -w "/var/log" ]; then
        echo "✅ /var/log is writable"
        echo "Creating log file..."
        touch "$log_file" 2>/dev/null && echo "✅ Created log file" || echo "❌ Cannot create log file"
    else
        echo "❌ /var/log is not writable"
    fi
fi

# Check Python and audio files
echo -e "\n5. 🐍 Python and Audio Check:"
echo "Python3 path: $(which python3)"
echo "Current directory: $(pwd)"
echo "app.py exists: $([ -f app.py ] && echo 'YES' || echo 'NO')"

if [ -f "static/mp3/safety/safety_english.mp3" ]; then
    echo "✅ Safety audio file exists"
else
    echo "❌ Safety audio file missing: static/mp3/safety/safety_english.mp3"
    echo "Available safety files:"
    ls -la static/mp3/safety/ 2>/dev/null || echo "Safety directory not found"
fi

# Test manual execution
echo -e "\n6. 🧪 Manual Test:"
echo "Testing manual execution of safety announcement..."
if [ -f app.py ]; then
    echo "Running: python3 app.py --safety --language english"
    timeout 30 python3 app.py --safety --language english 2>&1
    echo "Manual test completed (exit code: $?)"
else
    echo "❌ app.py not found in current directory"
fi

# Check cron logs
echo -e "\n7. 📊 System Cron Logs:"
if [ -f "/var/log/cron.log" ]; then
    echo "Recent cron activity (last 20 lines):"
    tail -20 /var/log/cron.log | grep -E "(CRON|cron)" || echo "No recent cron activity"
elif [ -f "/var/log/syslog" ]; then
    echo "Checking syslog for cron activity:"
    tail -50 /var/log/syslog | grep -i cron | tail -10 || echo "No cron activity in syslog"
else
    echo "No standard cron logs found"
fi

# Check environment issues
echo -e "\n8. 🌍 Environment Check:"
echo "PATH: $PATH"
echo "User: $(whoami)"
echo "Home: $HOME"
echo "Shell: $SHELL"

# Test cron execution simulation
echo -e "\n9. 🔬 Cron Environment Simulation:"
echo "Testing command that would run from cron..."
cd "$(dirname "$0")" || exit 1
current_dir=$(pwd)
echo "Working directory: $current_dir"

# Simulate the exact command cron would run
test_command="cd $current_dir && python3 $current_dir/app.py --safety --language english"
echo "Simulated cron command: $test_command"
echo "Executing with minimal environment..."

# Run with minimal PATH like cron would
env -i PATH="/usr/local/bin:/usr/bin:/bin" HOME="$HOME" USER="$(whoami)" bash -c "$test_command" 2>&1

echo -e "\n✅ Diagnostic complete!"
echo -e "\n🔧 Next steps:"
echo "1. Check if cron service is running"
echo "2. Verify log file permissions"
echo "3. Test manual command execution"
echo "4. Check if audio files exist"
echo "5. Ensure TARR app can run without GUI/display"
