#!/usr/bin/env python3
"""
TARR Annunciator - Raspberry Pi Version
This version uses pydub + PulseAudio for audio and crontab for scheduling
"""
from flask import Flask, render_template, request, redirect, url_for, flash, session
import os
import json
import argparse
import sys
import threading
import time
import subprocess
from datetime import datetime
from functools import wraps

# Initialize audio system for Raspberry Pi (pygame + PulseAudio)
PULSEAUDIO_AVAILABLE = False
PYGAME_AVAILABLE = False

try:
    # Test PulseAudio availability
    result = subprocess.run(['pulseaudio', '--check'], capture_output=True)
    if result.returncode == 0:
        PULSEAUDIO_AVAILABLE = True
        print("✓ PulseAudio detected")
    else:
        # Try to start PulseAudio
        subprocess.run(['pulseaudio', '--start'], capture_output=True)
        result = subprocess.run(['pulseaudio', '--check'], capture_output=True)
        if result.returncode == 0:
            PULSEAUDIO_AVAILABLE = True
            print("✓ PulseAudio started")
except Exception as e:
    print(f"⚠ PulseAudio check failed: {e}")

try:
    # Initialize pydub for audio playback (better for headless/cron operation)
    from pydub import AudioSegment
    from pydub.playback import play
    import warnings
    
    # Suppress any pydub warnings
    warnings.filterwarnings("ignore", category=RuntimeWarning)
    
    PYDUB_AVAILABLE = True
    print("✓ Pydub audio system initialized")
except ImportError as e:
    print(f"✗ Pydub not available: {e}")
    print("  Install with: pip3 install pydub simpleaudio")
    print("  System deps: sudo apt install ffmpeg python3-dev libasound2-dev")
    PYDUB_AVAILABLE = False
except Exception as e:
    print(f"⚠ Pydub initialization failed: {e}")
    PYDUB_AVAILABLE = False

AUDIO_AVAILABLE = PYDUB_AVAILABLE or PULSEAUDIO_AVAILABLE  # Audio available if either works
CURRENT_VOLUME = 70  # Default volume (0-100)

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
    """Get list of available PulseAudio devices"""
    devices = []
    if not PULSEAUDIO_AVAILABLE:
        return [{'id': 'default', 'name': 'Default Audio Device'}]
    
    try:
        # Get PulseAudio sinks (output devices)
        result = subprocess.run(['pactl', 'list', 'short', 'sinks'], 
                              capture_output=True, text=True, timeout=5)
        
        if result.returncode == 0 and result.stdout.strip():
            for line in result.stdout.strip().split('\n'):
                parts = line.split('\t')
                if len(parts) >= 2:
                    device_id = parts[0]
                    device_name = parts[1]
                    devices.append({
                        'id': device_id,
                        'name': device_name
                    })
    except Exception as e:
        print(f"Error getting audio devices: {e}")
    
    # Always include a default option
    if not devices:
        devices = [{'id': '@DEFAULT_SINK@', 'name': 'Default Audio Device'}]
    
    return devices

def set_volume(volume):
    """Set the audio volume (0-100)"""
    global CURRENT_VOLUME
    if PULSEAUDIO_AVAILABLE:
        try:
            # Clamp volume to valid range (0-100)
            volume = max(0, min(100, int(volume)))
            CURRENT_VOLUME = volume
            
            # Set PulseAudio volume
            subprocess.run(['pactl', 'set-sink-volume', '@DEFAULT_SINK@', f'{volume}%'], 
                          capture_output=True, timeout=2)
            return True
        except Exception as e:
            print(f"Error setting PulseAudio volume: {e}")
            # Fall back to storing volume for pygame use
            CURRENT_VOLUME = volume
            return False
    else:
        # Store volume for pygame use (0-100)
        CURRENT_VOLUME = max(0, min(100, int(volume)))
        return True

def get_current_volume():
    """Get current volume setting"""
    return CURRENT_VOLUME

def reinitialize_audio(frequency=22050, buffer_size=4096):
    """Reinitialize audio system (pydub doesn't need reinitialization)"""
    global AUDIO_AVAILABLE
    try:
        # For pydub, we just check if it's still available
        from pydub import AudioSegment
        from pydub.playback import play
        AUDIO_AVAILABLE = True
        return True, "Audio system verified (pydub doesn't require reinitialization)"
    except Exception as e:
        AUDIO_AVAILABLE = False
        return False, f"Failed to verify audio system: {e}"

def play_audio(file_path):
    """Play audio file using pydub with system audio fallbacks"""
    if not os.path.exists(file_path):
        print(f"Audio file not found: {file_path}")
        return False
    
    print(f"Playing audio: {file_path} (Volume: {CURRENT_VOLUME}%)")
    
    # Try pydub first (preferred method)
    if PYDUB_AVAILABLE:
        try:
            # Load audio file with pydub
            audio = AudioSegment.from_mp3(file_path)
            
            # Adjust volume (pydub uses dB, convert from 0-100 percentage)
            # 0% = -60dB (almost silent), 100% = 0dB (original volume)
            volume_db = (CURRENT_VOLUME - 100) * 0.6  # Scale to reasonable dB range
            audio = audio + volume_db
            
            # Play the audio
            play(audio)
            
            print("✓ Audio played successfully via pydub")
            return True
            
        except Exception as e:
            print(f"Pydub playback failed: {e}")
            print("Falling back to system audio tools...")
    else:
        print("Pydub not available, using system audio tools...")
    
    # Fallback to system audio tools (works in cron)
    try:
        # Try paplay (PulseAudio) first
        if PULSEAUDIO_AVAILABLE and subprocess.run(['which', 'paplay'], capture_output=True).returncode == 0:
            print("Attempting playback via paplay...")
            
            # Convert volume from 0-100 to 0-65535 (paplay range), but cap it
            pa_volume = min(int(CURRENT_VOLUME * 655.35), 32768)
            
            result = subprocess.run([
                'paplay', '--volume', str(pa_volume), file_path
            ], capture_output=True, text=True, timeout=30)
            
            if result.returncode == 0:
                print("✓ Audio played successfully via paplay")
                return True
            else:
                print(f"paplay with volume failed: {result.stderr}")
                # Try without volume setting
                result = subprocess.run(['paplay', file_path], capture_output=True, text=True, timeout=30)
                if result.returncode == 0:
                    print("✓ Audio played successfully via paplay (no volume control)")
                    return True
        
        # Try mpg123 as fallback
        if subprocess.run(['which', 'mpg123'], capture_output=True).returncode == 0:
            print("Attempting playback via mpg123...")
            result = subprocess.run([
                'mpg123', '-q', '--gain', str(CURRENT_VOLUME), file_path
            ], capture_output=True, text=True, timeout=30)
            
            if result.returncode == 0:
                print("✓ Audio played successfully via mpg123")
                return True
            else:
                # Try simple mpg123 without gain
                result = subprocess.run(['mpg123', '-q', file_path], capture_output=True, text=True, timeout=30)
                if result.returncode == 0:
                    print("✓ Audio played successfully via mpg123 (no volume control)")
                    return True
        
        # Try aplay for WAV files or if file is converted
        if file_path.lower().endswith('.wav') and subprocess.run(['which', 'aplay'], capture_output=True).returncode == 0:
            print("Attempting playback via aplay...")
            result = subprocess.run(['aplay', file_path], capture_output=True, text=True, timeout=30)
            if result.returncode == 0:
                print("✓ Audio played successfully via aplay")
                return True
        
        print("❌ All audio playback methods failed")
        print("Install pydub with: pip3 install pydub simpleaudio")
        print("Or install audio tools: sudo apt install mpg123 pulseaudio-utils")
        return False
        
    except subprocess.TimeoutExpired:
        print("❌ Audio playback timed out")
        return False
    except Exception as e:
        print(f"❌ Audio playback error: {e}")
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

def update_crontab():
    """Update system crontab with scheduled announcements"""
    cron_file = JSON_FILES['cron']
    if not os.path.exists(cron_file):
        return

    with open(cron_file, 'r') as f:
        cron_data = json.load(f)

    script_path = os.path.abspath(__file__)
    cron_entries = []
    
    # Add header comment
    cron_entries.append("# TARR Annunciator scheduled announcements")

    # Station announcements
    for i, item in enumerate(cron_data.get('station_announcements', [])):
        if item.get('enabled'):
            try:
                cron_expr = item['cron']
                command = f"cd {BASE_DIR} && python3 {script_path} --station --train '{item['train_number']}' --direction '{item['direction']}' --destination '{item['destination']}' --track '{item['track_number']}' >> /var/log/tarr-announcer.log 2>&1"
                cron_entries.append(f"{cron_expr} {command}")
                print(f"Scheduled: {cron_expr} - Train {item['train_number']}")
            except Exception as e:
                print(f"Error scheduling station announcement {i}: {e}")

    # Promo announcements
    for i, item in enumerate(cron_data.get('promo_announcements', [])):
        if item.get('enabled'):
            try:
                cron_expr = item['cron']
                command = f"cd {BASE_DIR} && python3 {script_path} --promo --file '{item['file']}' >> /var/log/tarr-announcer.log 2>&1"
                cron_entries.append(f"{cron_expr} {command}")
                print(f"Scheduled: {cron_expr} - {item['file']}")
            except Exception as e:
                print(f"Error scheduling promo announcement {i}: {e}")

    # Safety announcements
    for i, item in enumerate(cron_data.get('safety_announcements', [])):
        if item.get('enabled'):
            try:
                cron_expr = item['cron']
                command = f"cd {BASE_DIR} && python3 {script_path} --safety --language '{item['language']}' >> /var/log/tarr-announcer.log 2>&1"
                cron_entries.append(f"{cron_expr} {command}")
                print(f"Scheduled: {cron_expr} - {item['language']}")
            except Exception as e:
                print(f"Error scheduling safety announcement {i}: {e}")

    # Write to temporary cron file and install
    temp_cron_file = '/tmp/tarr-crontab'
    try:
        # Get existing crontab (excluding TARR entries)
        result = subprocess.run(['crontab', '-l'], capture_output=True, text=True)
        existing_cron = ""
        if result.returncode == 0:
            # Filter out existing TARR entries
            lines = result.stdout.strip().split('\n')
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
        
        # Write new crontab
        with open(temp_cron_file, 'w') as f:
            if existing_cron.strip():
                f.write(existing_cron + '\n')
            f.write('\n'.join(cron_entries) + '\n')
        
        # Install the new crontab
        result = subprocess.run(['crontab', temp_cron_file], capture_output=True)
        if result.returncode == 0:
            print("Crontab updated successfully.")
        else:
            print(f"Error updating crontab: {result.stderr.decode()}")
            
        # Clean up
        os.remove(temp_cron_file)
        
    except Exception as e:
        print(f"Error updating crontab: {e}")

def get_cron_status():
    """Get current crontab status"""
    try:
        result = subprocess.run(['crontab', '-l'], capture_output=True, text=True)
        if result.returncode == 0:
            tarr_jobs = []
            for line in result.stdout.strip().split('\n'):
                if 'tarr-announcer.log' in line and not line.strip().startswith('#'):
                    tarr_jobs.append(line.strip())
            return {'active': True, 'jobs': tarr_jobs, 'count': len(tarr_jobs)}
        else:
            return {'active': False, 'jobs': [], 'count': 0}
    except Exception as e:
        return {'active': False, 'error': str(e), 'jobs': [], 'count': 0}

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
    """Get current crontab status and jobs"""
    cron_status = get_cron_status()
    return {
        'scheduler_running': cron_status['active'], 
        'jobs': [{'id': f'cron_{i}', 'name': job, 'next_run': 'See crontab'} for i, job in enumerate(cron_status['jobs'])], 
        'audio_available': AUDIO_AVAILABLE,
        'cron_jobs_count': cron_status['count']
    }

@app.route('/audio_status')
def audio_status():
    """Get audio system status"""
    return {
        'audio_available': AUDIO_AVAILABLE,
        'audio_backend': 'pydub + pulseaudio',
        'pulseaudio_available': PULSEAUDIO_AVAILABLE,
        'pydub_available': PYDUB_AVAILABLE,
        'current_volume': CURRENT_VOLUME,
        'volume_percent': CURRENT_VOLUME,
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
        volume = float(request.form.get('volume', 70))
        
        # Handle both 0-1 and 0-100 ranges (same logic as API)
        if volume <= 1.0:
            volume = volume * 100.0
        
        success = set_volume(volume)
        if success:
            return {'success': True, 'volume': CURRENT_VOLUME, 'volume_percent': CURRENT_VOLUME}
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
        'audio_backend': 'pydub + pulseaudio',
        'api_enabled': API_ENABLED,
        'scheduler_running': get_cron_status()['active'],
        'volume': CURRENT_VOLUME,
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
            'volume_percent': CURRENT_VOLUME
        }
    
    # POST - set volume
    data = request.get_json() or request.form.to_dict()
    
    if 'volume' not in data:
        return {'error': 'Volume parameter required (0.0 to 1.0 or 0 to 100)'}, 400
    
    try:
        volume = float(data['volume'])
        
        # Handle both 0-1 and 0-100 ranges
        if volume <= 1.0:
            volume = volume * 100.0
        
        success = set_volume(volume)
        if success:
            return {
                'success': True,
                'volume': CURRENT_VOLUME,
                'volume_percent': CURRENT_VOLUME
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
        update_crontab()
        cron_status = get_cron_status()
        return {
            'success': True,
            'message': 'Schedule updated successfully',
            'active_jobs': cron_status['count']
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
            update_crontab()
            flash("Schedule updated successfully.", "success")
        except Exception as e:
            flash(f"Error saving schedule: {e}", "danger")

    return render_template('admin.html', cron_data=cron_data, trains=trains, directions=directions,
                           destinations=destinations, tracks=tracks, promo_announcements=promo_announcements,
                           safety_languages=safety_languages, current_volume=current_volume)

# ----------------- CLI Handling -----------------

def cli_args():
    parser = argparse.ArgumentParser(description="TARR Annunciator - Raspberry Pi (Pydub + PulseAudio)")
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
    parser.add_argument('--update-cron', action='store_true', help='Update crontab from config')
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
    elif args.update_cron:
        print("Updating crontab from configuration...")
        update_crontab()
        print("Crontab updated.")
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
        print(f"PulseAudio: {'Available' if PULSEAUDIO_AVAILABLE else 'Not Available'}")
        print(f"Pydub: {'Available' if PYDUB_AVAILABLE else 'Not Available'}")
        print("Access the application at: http://localhost:8080")
        print("Admin interface at: http://localhost:8080/admin")
        
        # Update crontab on startup
        try:
            update_crontab()
            print("Crontab updated with current schedule.")
        except Exception as e:
            print(f"Warning: Could not update crontab: {e}")
        
        app.run(debug=False, host='0.0.0.0', port=8080, threaded=True)

# ----------------- Main -----------------

if __name__ == '__main__':
    print(f"TARR Annunciator starting...")
    print(f"Audio system: {'Available' if AUDIO_AVAILABLE else 'Not Available'}")
    print(f"  - PulseAudio: {'Available' if PULSEAUDIO_AVAILABLE else 'Not Available'}")
    print(f"  - Pydub: {'Available' if PYDUB_AVAILABLE else 'Not Available'}")
    
    if len(sys.argv) > 1:
        handle_cli()
    else:
        handle_cli()
