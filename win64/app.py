#!/usr/bin/env python3
"""
TARR Annunciator - Windows Version (Pygame Audio Only)
This version uses only pygame for audio to avoid pydub compatibility issues
"""
from flask import Flask, render_template, request, redirect, url_for, flash, session
import os
import json
import argparse
import sys
import threading
import time
from datetime import datetime
from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger
import atexit
from functools import wraps

# Initialize pygame mixer for audio
try:
    import pygame
    pygame.mixer.init(frequency=22050, size=-16, channels=2, buffer=4096)
    AUDIO_AVAILABLE = True
    CURRENT_VOLUME = 0.7  # Default volume (0.0 to 1.0)
    print("✓ Pygame audio initialized successfully")
except ImportError:
    print("✗ Pygame not available - audio will not work")
    AUDIO_AVAILABLE = False
    CURRENT_VOLUME = 0.0
except Exception as e:
    print(f"⚠ Pygame audio initialization failed: {e}")
    AUDIO_AVAILABLE = False
    CURRENT_VOLUME = 0.0

app = Flask(__name__)
app.secret_key = '2932d8c03fb85143293c803ff3f7f1c27923787e520ce335'

# Admin Configuration
ADMIN_USERNAME = "admin"
ADMIN_PASSWORD = "tarr2025"  # Change this to something secure!

# API Configuration
API_KEY = "tarr-api-2025"  # Change this for security!
API_ENABLED = True

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
JSON_DIR = os.path.join(BASE_DIR, 'json')
MP3_DIR = os.path.join(BASE_DIR, 'static', 'mp3')

# Mapping types to subfolders
MP3_PATHS = {
    'train': os.path.join(MP3_DIR, 'train'),
    'direction': os.path.join(MP3_DIR, 'direction'),
    'destination': os.path.join(MP3_DIR, 'destination'),
    'track': os.path.join(MP3_DIR, 'track'),
    'promo': os.path.join(MP3_DIR, 'promo'),
    'safety': os.path.join(MP3_DIR, 'safety'),
    'chime': os.path.join(MP3_DIR, 'chime.mp3')
}

# JSON files
JSON_FILES = {
    'trains': os.path.join(JSON_DIR, 'trains.json'),
    'directions': os.path.join(JSON_DIR, 'directions.json'),
    'destinations': os.path.join(JSON_DIR, 'destinations.json'),
    'tracks': os.path.join(JSON_DIR, 'tracks.json'),
    'promo': os.path.join(JSON_DIR, 'promo.json'),
    'safety': os.path.join(JSON_DIR, 'safety.json'),
    'cron': os.path.join(JSON_DIR, 'cron.json')
}

current_announcement = None
scheduler = BackgroundScheduler()

# ----------------- Authentication Functions -----------------

def login_required(f):
    """Decorator to require login for admin routes"""
    @wraps(f)
    def decorated_function(*args, **kwargs):
        if not session.get('admin_logged_in'):
            return redirect(url_for('admin_login'))
        return f(*args, **kwargs)
    return decorated_function

def check_admin_credentials(username, password):
    """Check if admin credentials are correct"""
    return username == ADMIN_USERNAME and password == ADMIN_PASSWORD

def api_key_required(f):
    """Decorator to require API key for API routes"""
    @wraps(f)
    def decorated_function(*args, **kwargs):
        if not API_ENABLED:
            return {'error': 'API is disabled'}, 503
        
        # Check for API key in headers or query params
        api_key = request.headers.get('X-API-Key') or request.args.get('api_key') or request.form.get('api_key')
        
        if not api_key:
            return {'error': 'API key required. Use X-API-Key header or api_key parameter.'}, 401
        
        if api_key != API_KEY:
            return {'error': 'Invalid API key'}, 401
            
        return f(*args, **kwargs)
    return decorated_function

def validate_announcement_params(params, required_fields):
    """Validate announcement parameters"""
    missing_fields = [field for field in required_fields if not params.get(field)]
    if missing_fields:
        return False, f"Missing required fields: {', '.join(missing_fields)}"
    return True, None

# ----------------- Audio Functions (Pygame Only) -----------------

def get_audio_devices():
    """Get list of available audio devices (Windows specific)"""
    devices = []
    try:
        # For Windows, we can use the Windows Audio Session API via subprocess
        import subprocess
        import json
        
        # Try to get audio devices using PowerShell
        cmd = ['powershell', '-Command', 
               'Get-WmiObject -Class Win32_SoundDevice | Select-Object Name, DeviceID | ConvertTo-Json']
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=5)
        
        if result.returncode == 0 and result.stdout.strip():
            try:
                device_data = json.loads(result.stdout)
                if isinstance(device_data, list):
                    for device in device_data:
                        if device.get('Name'):
                            devices.append({
                                'id': device.get('DeviceID', 'unknown'),
                                'name': device.get('Name', 'Unknown Device')
                            })
                elif isinstance(device_data, dict) and device_data.get('Name'):
                    devices.append({
                        'id': device_data.get('DeviceID', 'unknown'),
                        'name': device_data.get('Name', 'Unknown Device')
                    })
            except json.JSONDecodeError:
                pass
    except Exception as e:
        print(f"Error getting audio devices: {e}")
    
    # Always include a default option
    if not devices:
        devices = [{'id': 'default', 'name': 'Default Audio Device'}]
    
    return devices

def set_volume(volume):
    """Set the audio volume (0.0 to 1.0)"""
    global CURRENT_VOLUME
    if AUDIO_AVAILABLE:
        try:
            # Clamp volume to valid range
            volume = max(0.0, min(1.0, float(volume)))
            CURRENT_VOLUME = volume
            # Note: pygame.mixer doesn't have global volume control
            # Volume is applied per-sound when playing
            return True
        except Exception as e:
            print(f"Error setting volume: {e}")
            return False
    return False

def get_current_volume():
    """Get current volume setting"""
    return CURRENT_VOLUME

def reinitialize_audio(frequency=22050, buffer_size=4096):
    """Reinitialize pygame audio with new settings"""
    global AUDIO_AVAILABLE
    try:
        pygame.mixer.quit()
        pygame.mixer.init(frequency=frequency, size=-16, channels=2, buffer=buffer_size)
        AUDIO_AVAILABLE = True
        return True, "Audio reinitialized successfully"
    except Exception as e:
        AUDIO_AVAILABLE = False
        return False, f"Failed to reinitialize audio: {e}"

def play_audio(file_path):
    """Play audio file using pygame"""
    if not AUDIO_AVAILABLE:
        print(f"Audio not available - would play: {file_path}")
        return False
    
    if not os.path.exists(file_path):
        print(f"Audio file not found: {file_path}")
        return False
    
    try:
        print(f"Playing audio: {file_path} (Volume: {int(CURRENT_VOLUME * 100)}%)")
        
        # Load and play the audio
        pygame.mixer.music.load(file_path)
        pygame.mixer.music.set_volume(CURRENT_VOLUME)
        pygame.mixer.music.play()
        
        # Wait for playback to finish
        while pygame.mixer.music.get_busy():
            pygame.time.wait(100)
        
        return True
    except Exception as e:
        print(f"Audio playback error: {e}")
        return False

def play_audio_sequence(file_list):
    """Play a sequence of audio files with small gaps"""
    for file_path in file_list:
        if os.path.exists(file_path):
            print(f"Playing: {os.path.basename(file_path)}")
            play_audio(file_path)
            time.sleep(0.3)  # Small gap between announcements
        else:
            print(f"Missing audio file: {file_path}")

# ----------------- Utility Functions -----------------

def load_json(name):
    path = JSON_FILES[name]
    if os.path.exists(path):
        with open(path, 'r') as f:
            data = json.load(f)
            # Handle nested structure - extract the array from the wrapper object
            if isinstance(data, dict) and name in data:
                return data[name]
            elif isinstance(data, dict) and len(data) == 1:
                # If there's only one key, use its value
                return list(data.values())[0]
            else:
                return data
    return []

def save_json(name, data):
    path = JSON_FILES[name]
    with open(path, 'w') as f:
        json.dump(data, f, indent=4)

def stop_safety_announcement():
    global current_announcement
    current_announcement = None

def play_station_announcement(train_number, direction, destination, track_number):
    """Play station announcement sequence"""
    print(f"Station announcement: Train {train_number}, {direction} to {destination}, Track {track_number}")
    
    # Build the sequence of audio files
    audio_sequence = [
        MP3_PATHS['chime'],
        os.path.join(MP3_PATHS['train'], f"{train_number}.mp3"),
        os.path.join(MP3_PATHS['direction'], f"{direction}.mp3"),
        os.path.join(MP3_PATHS['destination'], f"{destination}.mp3"),
        os.path.join(MP3_PATHS['track'], f"{track_number}.mp3")
    ]
    
    # Play in background thread to avoid blocking
    threading.Thread(target=lambda: play_audio_sequence(audio_sequence), daemon=True).start()

def play_promo(file):
    """Play promotional announcement"""
    promo_file = os.path.join(MP3_PATHS['promo'], file)
    threading.Thread(target=lambda: play_audio(promo_file), daemon=True).start()

def play_safety(language):
    """Play safety announcement"""
    safety_file = os.path.join(MP3_PATHS['safety'], f"safety_{language}.mp3")
    threading.Thread(target=lambda: play_audio(safety_file), daemon=True).start()

def update_scheduler():
    """Update scheduler with cron jobs"""
    cron_file = JSON_FILES['cron']
    if not os.path.exists(cron_file):
        return

    with open(cron_file, 'r') as f:
        cron_data = json.load(f)

    # Clear existing scheduled jobs
    scheduler.remove_all_jobs()

    # Station announcements
    for i, item in enumerate(cron_data.get('station_announcements', [])):
        if item.get('enabled'):
            try:
                cron_parts = item['cron'].split()
                if len(cron_parts) == 5:
                    minute, hour, day, month, day_of_week = cron_parts
                    
                    scheduler.add_job(
                        func=play_station_announcement,
                        trigger=CronTrigger(
                            minute=minute, 
                            hour=hour, 
                            day=day if day != '*' else None,
                            month=month if month != '*' else None,
                            day_of_week=day_of_week if day_of_week != '*' else None
                        ),
                        args=[item['train_number'], item['direction'], item['destination'], item['track_number']],
                        id=f'station_{i}',
                        name=f"Station Announcement {i+1}"
                    )
                    print(f"Scheduled: {item['cron']} - Train {item['train_number']}")
            except Exception as e:
                print(f"Error scheduling station announcement {i}: {e}")

    # Promo announcements
    for i, item in enumerate(cron_data.get('promo_announcements', [])):
        if item.get('enabled'):
            try:
                cron_parts = item['cron'].split()
                if len(cron_parts) == 5:
                    minute, hour, day, month, day_of_week = cron_parts
                    
                    scheduler.add_job(
                        func=play_promo,
                        trigger=CronTrigger(
                            minute=minute, 
                            hour=hour, 
                            day=day if day != '*' else None,
                            month=month if month != '*' else None,
                            day_of_week=day_of_week if day_of_week != '*' else None
                        ),
                        args=[item['file']],
                        id=f'promo_{i}',
                        name=f"Promo Announcement {i+1}"
                    )
                    print(f"Scheduled: {item['cron']} - {item['file']}")
            except Exception as e:
                print(f"Error scheduling promo announcement {i}: {e}")

    # Safety announcements
    for i, item in enumerate(cron_data.get('safety_announcements', [])):
        if item.get('enabled'):
            try:
                cron_parts = item['cron'].split()
                if len(cron_parts) == 5:
                    minute, hour, day, month, day_of_week = cron_parts
                    
                    scheduler.add_job(
                        func=play_safety,
                        trigger=CronTrigger(
                            minute=minute, 
                            hour=hour, 
                            day=day if day != '*' else None,
                            month=month if month != '*' else None,
                            day_of_week=day_of_week if day_of_week != '*' else None
                        ),
                        args=[item['language']],
                        id=f'safety_{i}',
                        name=f"Safety Announcement {i+1}"
                    )
                    print(f"Scheduled: {item['cron']} - {item['language']}")
            except Exception as e:
                print(f"Error scheduling safety announcement {i}: {e}")

    print("Scheduler updated.")

# ----------------- Flask Routes -----------------

@app.route('/')
def index():
    trains = load_json('trains')
    directions = load_json('directions')
    destinations = load_json('destinations')
    tracks = load_json('tracks')
    promo_announcements = load_json('promo')
    safety_languages = load_json('safety')
    return render_template('index.html', trains=trains, directions=directions, destinations=destinations,
                           tracks=tracks, promo_announcements=promo_announcements, safety_languages=safety_languages)

@app.route('/play_announcement', methods=['POST'])
def play_announcement_route():
    train_number = request.form.get('train_number')
    direction = request.form.get('direction')
    destination = request.form.get('destination')
    track_number = request.form.get('track_number')
    stop_safety_announcement()
    play_station_announcement(train_number, direction, destination, track_number)
    return 'Station announcement played.'

@app.route('/play_promo', methods=['POST'])
def play_promo_route():
    file = request.form.get('file')
    play_promo(file)
    return 'Promo announcement played.'

@app.route('/play_safety_announcement', methods=['POST'])
def play_safety_route():
    language = request.form.get('language')
    stop_safety_announcement()
    play_safety(language)
    return f"Safety announcement in {language} played."

@app.route('/scheduler_status')
def scheduler_status():
    """Get current scheduler status and jobs"""
    jobs = []
    for job in scheduler.get_jobs():
        jobs.append({
            'id': job.id,
            'name': job.name,
            'next_run': job.next_run_time.strftime('%Y-%m-%d %H:%M:%S') if job.next_run_time else 'Not scheduled'
        })
    return {'scheduler_running': scheduler.running, 'jobs': jobs, 'audio_available': AUDIO_AVAILABLE}

@app.route('/audio_status')
def audio_status():
    """Get audio system status"""
    return {
        'audio_available': AUDIO_AVAILABLE,
        'audio_backend': 'pygame',
        'current_volume': CURRENT_VOLUME,
        'volume_percent': int(CURRENT_VOLUME * 100),
        'chime_exists': os.path.exists(MP3_PATHS['chime']),
        'mp3_directory_exists': os.path.exists(MP3_DIR),
        'audio_devices': get_audio_devices()
    }

@app.route('/audio/devices')
@login_required
def get_audio_devices_route():
    """Get available audio devices"""
    return {'devices': get_audio_devices()}

@app.route('/audio/volume', methods=['POST'])
@login_required
def set_volume_route():
    """Set audio volume"""
    try:
        volume = float(request.form.get('volume', 0.7))
        success = set_volume(volume)
        if success:
            return {'success': True, 'volume': CURRENT_VOLUME, 'volume_percent': int(CURRENT_VOLUME * 100)}
        else:
            return {'success': False, 'error': 'Failed to set volume'}, 400
    except ValueError:
        return {'success': False, 'error': 'Invalid volume value'}, 400

@app.route('/audio/test', methods=['POST'])
@login_required
def test_audio_route():
    """Test audio with current settings"""
    chime_file = MP3_PATHS['chime']
    if os.path.exists(chime_file):
        success = play_audio(chime_file)
        if success:
            return {'success': True, 'message': 'Audio test played successfully'}
        else:
            return {'success': False, 'error': 'Audio test failed'}, 400
    else:
        return {'success': False, 'error': 'Test audio file not found'}, 400

@app.route('/audio/reinitialize', methods=['POST'])
@login_required
def reinitialize_audio_route():
    """Reinitialize audio system"""
    try:
        frequency = int(request.form.get('frequency', 22050))
        buffer_size = int(request.form.get('buffer_size', 4096))
        
        success, message = reinitialize_audio(frequency, buffer_size)
        if success:
            return {'success': True, 'message': message}
        else:
            return {'success': False, 'error': message}, 400
    except ValueError:
        return {'success': False, 'error': 'Invalid audio parameters'}, 400

# ----------------- Public API Routes -----------------

@app.route('/api/status')
def api_status():
    """Get system status (no authentication required)"""
    return {
        'status': 'online',
        'audio_available': AUDIO_AVAILABLE,
        'audio_backend': 'pygame',
        'api_enabled': API_ENABLED,
        'scheduler_running': scheduler.running,
        'volume': int(CURRENT_VOLUME * 100),
        'timestamp': datetime.now().isoformat()
    }

@app.route('/api/docs')
def api_docs():
    """API Documentation"""
    return render_template('api_docs.html')

# ----------------- Authenticated API Routes -----------------

@app.route('/api/announce/station', methods=['POST'])
@api_key_required
def api_station_announcement():
    """Trigger a station announcement via API"""
    data = request.get_json() or request.form.to_dict()
    
    required_fields = ['train_number', 'direction', 'destination', 'track_number']
    valid, error_msg = validate_announcement_params(data, required_fields)
    
    if not valid:
        return {'error': error_msg}, 400
    
    try:
        play_station_announcement(
            data['train_number'], 
            data['direction'], 
            data['destination'], 
            data['track_number']
        )
        
        return {
            'success': True,
            'message': 'Station announcement triggered',
            'announcement': {
                'type': 'station',
                'train_number': data['train_number'],
                'direction': data['direction'],
                'destination': data['destination'],
                'track_number': data['track_number']
            },
            'timestamp': datetime.now().isoformat()
        }
    except Exception as e:
        return {'error': f'Failed to play announcement: {str(e)}'}, 500

@app.route('/api/announce/safety', methods=['POST'])
@api_key_required
def api_safety_announcement():
    """Trigger a safety announcement via API"""
    data = request.get_json() or request.form.to_dict()
    
    required_fields = ['language']
    valid, error_msg = validate_announcement_params(data, required_fields)
    
    if not valid:
        return {'error': error_msg}, 400
    
    # Validate language exists
    available_languages = [lang.get('id') for lang in load_json('safety')]
    if data['language'] not in available_languages:
        return {
            'error': f"Invalid language '{data['language']}'. Available: {', '.join(available_languages)}"
        }, 400
    
    try:
        play_safety(data['language'])
        
        return {
            'success': True,
            'message': 'Safety announcement triggered',
            'announcement': {
                'type': 'safety',
                'language': data['language']
            },
            'timestamp': datetime.now().isoformat()
        }
    except Exception as e:
        return {'error': f'Failed to play announcement: {str(e)}'}, 500

@app.route('/api/announce/promo', methods=['POST'])
@api_key_required
def api_promo_announcement():
    """Trigger a promotional announcement via API"""
    data = request.get_json() or request.form.to_dict()
    
    required_fields = ['file']
    valid, error_msg = validate_announcement_params(data, required_fields)
    
    if not valid:
        return {'error': error_msg}, 400
    
    # Validate promo file exists
    available_promos = [promo.get('id') for promo in load_json('promo')]
    if data['file'] not in available_promos:
        return {
            'error': f"Invalid promo file '{data['file']}'. Available: {', '.join(available_promos)}"
        }, 400
    
    try:
        play_promo(data['file'])
        
        return {
            'success': True,
            'message': 'Promo announcement triggered',
            'announcement': {
                'type': 'promo',
                'file': data['file']
            },
            'timestamp': datetime.now().isoformat()
        }
    except Exception as e:
        return {'error': f'Failed to play announcement: {str(e)}'}, 500

@app.route('/api/audio/volume', methods=['GET', 'POST'])
@api_key_required
def api_audio_volume():
    """Get or set audio volume via API"""
    if request.method == 'GET':
        return {
            'volume': CURRENT_VOLUME,
            'volume_percent': int(CURRENT_VOLUME * 100)
        }
    
    # POST - set volume
    data = request.get_json() or request.form.to_dict()
    
    if 'volume' not in data:
        return {'error': 'Volume parameter required (0.0 to 1.0 or 0 to 100)'}, 400
    
    try:
        volume = float(data['volume'])
        
        # Handle both 0-1 and 0-100 ranges
        if volume > 1.0:
            volume = volume / 100.0
        
        success = set_volume(volume)
        if success:
            return {
                'success': True,
                'volume': CURRENT_VOLUME,
                'volume_percent': int(CURRENT_VOLUME * 100)
            }
        else:
            return {'error': 'Failed to set volume'}, 500
    except ValueError:
        return {'error': 'Invalid volume value'}, 400

@app.route('/api/config', methods=['GET'])
@api_key_required
def api_get_config():
    """Get configuration data via API"""
    return {
        'trains': load_json('trains'),
        'directions': load_json('directions'),
        'destinations': load_json('destinations'),
        'tracks': load_json('tracks'),
        'promo_announcements': load_json('promo'),
        'safety_languages': load_json('safety')
    }

@app.route('/api/schedule', methods=['GET', 'POST'])
@api_key_required
def api_schedule():
    """Get or update schedule via API"""
    if request.method == 'GET':
        return {'schedule': load_json('cron')}
    
    # POST - update schedule
    data = request.get_json()
    
    if not data or 'schedule' not in data:
        return {'error': 'Schedule data required'}, 400
    
    try:
        save_json('cron', data['schedule'])
        update_scheduler()
        return {
            'success': True,
            'message': 'Schedule updated successfully',
            'active_jobs': len(scheduler.get_jobs())
        }
    except Exception as e:
        return {'error': f'Failed to update schedule: {str(e)}'}, 500

# ----------------- Admin Authentication Routes -----------------

@app.route('/admin/login', methods=['GET', 'POST'])
def admin_login():
    """Admin login page"""
    if request.method == 'POST':
        username = request.form.get('username')
        password = request.form.get('password')
        
        if check_admin_credentials(username, password):
            session['admin_logged_in'] = True
            flash('Successfully logged in!', 'success')
            return redirect(url_for('admin'))
        else:
            flash('Invalid username or password!', 'danger')
    
    return render_template('admin_login.html')

@app.route('/admin/logout')
def admin_logout():
    """Admin logout"""
    session.pop('admin_logged_in', None)
    flash('Successfully logged out!', 'success')
    return redirect(url_for('index'))

# ----------------- Admin Interface -----------------

@app.route('/admin', methods=['GET', 'POST'])
@login_required
def admin():
    cron_data = load_json('cron')
    trains = load_json('trains')
    directions = load_json('directions')
    destinations = load_json('destinations')
    tracks = load_json('tracks')
    promo_announcements = load_json('promo')
    safety_languages = load_json('safety')
    current_volume = get_current_volume()

    if request.method == 'POST':
        cron_json = request.form.get('cron_json')
        try:
            cron_data = json.loads(cron_json)
            save_json('cron', cron_data)
            update_scheduler()
            flash("Schedule updated successfully.", "success")
        except Exception as e:
            flash(f"Error saving schedule: {e}", "danger")

    return render_template('admin.html', cron_data=cron_data, trains=trains, directions=directions,
                           destinations=destinations, tracks=tracks, promo_announcements=promo_announcements,
                           safety_languages=safety_languages, current_volume=current_volume)

# ----------------- CLI Handling -----------------

def cli_args():
    parser = argparse.ArgumentParser(description="TARR Annunciator - Windows (Pygame Audio)")
    parser.add_argument('--station', action='store_true', help='Play a station announcement')
    parser.add_argument('--train', help='Train number')
    parser.add_argument('--direction', help='Direction (eastbound/westbound)')
    parser.add_argument('--destination', help='Destination')
    parser.add_argument('--track', help='Track number')
    parser.add_argument('--promo', action='store_true', help='Play promo announcement')
    parser.add_argument('--file', help='Promo file name')
    parser.add_argument('--safety', action='store_true', help='Play safety announcement')
    parser.add_argument('--language', help='Safety announcement language')
    parser.add_argument('--test-audio', action='store_true', help='Test audio playback')
    return parser.parse_args()

def test_audio_system():
    """Test if audio system is working"""
    print("Testing audio system...")
    print(f"Audio available: {AUDIO_AVAILABLE}")
    
    if not AUDIO_AVAILABLE:
        print("✗ Audio system not available")
        return False
    
    chime_file = MP3_PATHS['chime']
    if os.path.exists(chime_file):
        print(f"Playing test chime: {chime_file}")
        success = play_audio(chime_file)
        if success:
            print("✓ Audio test successful")
        else:
            print("✗ Audio test failed")
        return success
    else:
        print(f"✗ Test chime file not found: {chime_file}")
        return False

def handle_cli():
    args = cli_args()
    if args.test_audio:
        test_audio_system()
    elif args.station:
        if args.train and args.direction and args.destination and args.track:
            play_station_announcement(args.train, args.direction, args.destination, args.track)
        else:
            print("Station announcement requires --train, --direction, --destination, and --track")
    elif args.promo:
        if args.file:
            play_promo(args.file)
        else:
            print("Promo announcement requires --file")
    elif args.safety:
        if args.language:
            play_safety(args.language)
        else:
            print("Safety announcement requires --language")
    else:
        # Start Flask web server
        print("Starting TARR Annunciator Web Interface...")
        print(f"Audio system: {'Available' if AUDIO_AVAILABLE else 'Not Available'}")
        print("Access the application at: http://localhost:8080")
        print("Admin interface at: http://localhost:8080/admin")
        
        # Start the scheduler
        if not scheduler.running:
            scheduler.start()
            update_scheduler()
            print("Background scheduler started.")
        
        # Register cleanup function
        atexit.register(lambda: scheduler.shutdown())
        
        app.run(debug=False, host='0.0.0.0', port=8080, threaded=True)

# ----------------- Main -----------------

if __name__ == '__main__':
    print(f"TARR Annunciator starting...")
    print(f"Audio system: {'Available (pygame)' if AUDIO_AVAILABLE else 'Not Available'}")
    
    if len(sys.argv) > 1:
        handle_cli()
    else:
        handle_cli()
