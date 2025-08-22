#!/usr/bin/env python3
"""
TARR Cron Test Script - Debug cron scheduling issues
"""
import json
import os
import subprocess
import sys
from datetime import datetime

def test_cron_update():
    """Test the cron update process"""
    print("🔧 Testing TARR Cron Update Process")
    print("=" * 50)
    
    # Check if we're in the right directory
    if not os.path.exists('json/cron.json'):
        print("❌ Error: json/cron.json not found")
        print("   Make sure you're running this from the TARR project directory")
        return False
    
    # Load cron configuration
    with open('json/cron.json', 'r') as f:
        cron_data = json.load(f)
    
    print("📄 Current cron configuration:")
    
    # Check enabled schedules
    enabled_count = 0
    for schedule_type in ['station_announcements', 'promo_announcements', 'safety_announcements']:
        schedules = cron_data.get(schedule_type, [])
        enabled = [s for s in schedules if s.get('enabled', False)]
        print(f"   {schedule_type}: {len(enabled)} enabled out of {len(schedules)}")
        enabled_count += len(enabled)
        
        for i, schedule in enumerate(enabled):
            print(f"     → {i+1}: {schedule.get('cron')} - {schedule}")
    
    if enabled_count == 0:
        print("⚠️  WARNING: No schedules are enabled!")
        return False
    
    print(f"\n✅ Found {enabled_count} enabled schedule(s)")
    
    # Test cron generation
    print("\n🔧 Testing cron entry generation...")
    
    BASE_DIR = os.path.dirname(os.path.abspath(__file__))
    script_path = os.path.abspath(__file__).replace('test_cron.py', 'app.py')
    
    print(f"   Base directory: {BASE_DIR}")
    print(f"   Script path: {script_path}")
    
    cron_entries = []
    cron_entries.append("# TARR Annunciator scheduled announcements")
    
    # Generate safety announcement entries
    for i, item in enumerate(cron_data.get('safety_announcements', [])):
        if item.get('enabled'):
            cron_expr = item['cron']
            language = item['language']
            command = f"cd {BASE_DIR} && python3 {script_path} --safety --language '{language}' >> /var/log/tarr-announcer.log 2>&1"
            cron_entries.append(f"{cron_expr} {command}")
            print(f"   ✅ Safety: {cron_expr} - {language}")
    
    # Generate station announcement entries  
    for i, item in enumerate(cron_data.get('station_announcements', [])):
        if item.get('enabled'):
            cron_expr = item['cron']
            command = f"cd {BASE_DIR} && python3 {script_path} --station --train '{item['train_number']}' --direction '{item['direction']}' --destination '{item['destination']}' --track '{item['track_number']}' >> /var/log/tarr-announcer.log 2>&1"
            cron_entries.append(f"{cron_expr} {command}")
            print(f"   ✅ Station: {cron_expr} - Train {item['train_number']}")
    
    # Generate promo announcement entries
    for i, item in enumerate(cron_data.get('promo_announcements', [])):
        if item.get('enabled'):
            cron_expr = item['cron']
            command = f"cd {BASE_DIR} && python3 {script_path} --promo --file '{item['file']}' >> /var/log/tarr-announcer.log 2>&1"
            cron_entries.append(f"{cron_expr} {command}")
            print(f"   ✅ Promo: {cron_expr} - {item['file']}")
    
    print(f"\n📝 Generated {len(cron_entries) - 1} cron entries")
    
    # Show what would be installed
    print("\n📋 Cron entries that would be installed:")
    for entry in cron_entries:
        print(f"   {entry}")
    
    # Test crontab update
    print("\n🔧 Testing crontab update...")
    
    temp_cron_file = '/tmp/tarr-crontab-test'
    try:
        # Get existing crontab
        result = subprocess.run(['crontab', '-l'], capture_output=True, text=True)
        existing_cron = ""
        if result.returncode == 0:
            print("   ✅ Current crontab read successfully")
            # Filter out existing TARR entries
            lines = result.stdout.strip().split('\n') if result.stdout.strip() else []
            filtered_lines = []
            skip_next = False
            for line in lines:
                if '# TARR Annunciator' in line:
                    skip_next = True
                    continue
                if skip_next and (line.strip() == '' or line.startswith('#')):
                    continue
                if skip_next and not line.strip().startswith('#'):
                    skip_next = False
                if not skip_next and 'tarr-announcer.log' not in line:
                    filtered_lines.append(line)
            existing_cron = '\n'.join(filtered_lines)
            print(f"   📄 Existing non-TARR cron entries: {len(filtered_lines)}")
        else:
            print("   ℹ️  No existing crontab (this is normal)")
        
        # Write new crontab
        with open(temp_cron_file, 'w') as f:
            if existing_cron.strip():
                f.write(existing_cron + '\n')
            f.write('\n'.join(cron_entries) + '\n')
        
        print(f"   ✅ Test crontab written to {temp_cron_file}")
        
        # Show what would be installed
        print("\n📄 Complete crontab that would be installed:")
        with open(temp_cron_file, 'r') as f:
            content = f.read()
            print("   " + content.replace('\n', '\n   '))
        
        # Ask if user wants to install
        response = input("\n❓ Install this crontab? (y/N): ").strip().lower()
        if response == 'y':
            result = subprocess.run(['crontab', temp_cron_file], capture_output=True)
            if result.returncode == 0:
                print("   ✅ Crontab installed successfully!")
                
                # Verify installation
                result = subprocess.run(['crontab', '-l'], capture_output=True, text=True)
                if result.returncode == 0:
                    tarr_jobs = [line for line in result.stdout.split('\n') if 'tarr-announcer.log' in line and not line.strip().startswith('#')]
                    print(f"   ✅ Verification: {len(tarr_jobs)} TARR jobs now in crontab")
                    return True
                else:
                    print("   ❌ Could not verify crontab installation")
                    return False
            else:
                print(f"   ❌ Crontab installation failed: {result.stderr.decode()}")
                return False
        else:
            print("   ⏸️  Crontab installation skipped")
            return False
        
    except Exception as e:
        print(f"   ❌ Crontab update error: {e}")
        return False
    finally:
        # Clean up
        if os.path.exists(temp_cron_file):
            os.remove(temp_cron_file)
    
    return True

def test_manual_execution():
    """Test manual execution of safety announcement"""
    print("\n🧪 Testing Manual Safety Announcement")
    print("=" * 50)
    
    # Check if audio file exists
    safety_file = 'static/mp3/safety/safety_english.mp3'
    if os.path.exists(safety_file):
        print(f"✅ Safety audio file exists: {safety_file}")
    else:
        print(f"❌ Safety audio file missing: {safety_file}")
        print("Available safety files:")
        safety_dir = 'static/mp3/safety'
        if os.path.exists(safety_dir):
            files = os.listdir(safety_dir)
            for f in files:
                print(f"   - {f}")
        else:
            print("   Safety directory not found!")
        return False
    
    # Test the command that cron would run
    print("\n🔧 Testing command execution...")
    script_path = 'app.py'
    if not os.path.exists(script_path):
        print(f"❌ app.py not found in current directory")
        return False
    
    print("Running: python3 app.py --safety --language english")
    try:
        result = subprocess.run([
            'python3', script_path, '--safety', '--language', 'english'
        ], capture_output=True, text=True, timeout=30)
        
        print(f"Exit code: {result.returncode}")
        if result.stdout:
            print("STDOUT:")
            print("   " + result.stdout.replace('\n', '\n   '))
        if result.stderr:
            print("STDERR:")
            print("   " + result.stderr.replace('\n', '\n   '))
        
        return result.returncode == 0
        
    except subprocess.TimeoutExpired:
        print("❌ Command timed out (30 seconds)")
        return False
    except Exception as e:
        print(f"❌ Command execution error: {e}")
        return False

def check_cron_status():
    """Check current cron status"""
    print("\n📊 Current Cron Status")
    print("=" * 50)
    
    # Check if cron service is running
    try:
        result = subprocess.run(['systemctl', 'is-active', 'cron'], capture_output=True, text=True)
        if result.returncode == 0:
            print("✅ Cron service is running")
        else:
            # Try crond
            result = subprocess.run(['systemctl', 'is-active', 'crond'], capture_output=True, text=True)
            if result.returncode == 0:
                print("✅ Crond service is running")
            else:
                print("❌ Cron service is not running")
                print("   Try: sudo systemctl start cron")
                return False
    except FileNotFoundError:
        print("⚠️  systemctl not available, checking processes...")
        result = subprocess.run(['ps', 'aux'], capture_output=True, text=True)
        if 'cron' in result.stdout:
            print("✅ Cron process found in process list")
        else:
            print("❌ No cron process found")
            return False
    
    # Check current crontab
    try:
        result = subprocess.run(['crontab', '-l'], capture_output=True, text=True)
        if result.returncode == 0:
            lines = result.stdout.strip().split('\n')
            tarr_jobs = [line for line in lines if 'tarr-announcer.log' in line and not line.strip().startswith('#')]
            print(f"✅ Current crontab has {len(lines)} total lines")
            print(f"📋 TARR jobs in crontab: {len(tarr_jobs)}")
            
            if tarr_jobs:
                print("TARR cron jobs:")
                for job in tarr_jobs:
                    print(f"   {job}")
            else:
                print("⚠️  No TARR jobs found in crontab")
        else:
            print("ℹ️  No crontab set for current user")
    except Exception as e:
        print(f"❌ Error checking crontab: {e}")
        return False
    
    return True

if __name__ == '__main__':
    print(f"🕒 TARR Cron Test - {datetime.now()}")
    print("=" * 60)
    
    print(f"Current directory: {os.getcwd()}")
    print(f"Current user: {os.getenv('USER', 'unknown')}")
    print()
    
    # Run all tests
    status_ok = check_cron_status()
    config_ok = test_cron_update()
    manual_ok = test_manual_execution()
    
    print("\n" + "=" * 60)
    print("📋 SUMMARY:")
    print(f"   Cron Status: {'✅' if status_ok else '❌'}")
    print(f"   Configuration: {'✅' if config_ok else '❌'}")
    print(f"   Manual Test: {'✅' if manual_ok else '❌'}")
    
    if not (status_ok and config_ok and manual_ok):
        print("\n🔧 TROUBLESHOOTING STEPS:")
        if not status_ok:
            print("   1. Start cron service: sudo systemctl start cron")
        if not config_ok:
            print("   2. Enable schedules in admin interface or json/cron.json")
        if not manual_ok:
            print("   3. Fix audio/Python issues before enabling cron")
        print("   4. Check log file: tail -f /var/log/tarr-announcer.log")
        print("   5. Run diagnostic script: ./diagnose_cron.sh")
