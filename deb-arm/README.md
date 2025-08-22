# TARR Annunciator - Raspberry Pi 4B Enhanced Version

This is the enhanced Raspberry Pi version of the TARR Annunciator system with features ported from the Windows version.

## 🆕 What's New in This Version

### ✅ Enhanced Features Added:
- **🔐 Admin Authentication** → Secure login system for admin interface
- **🔌 REST API** → Full API for external control and integration
- **🔊 Audio Controls** → Volume control and audio device management
- **📊 System Monitoring** → Audio status, scheduler status, device info
- **🔧 Enhanced CLI** → Audio testing and improved command-line interface
- **📱 Modern UI** → Updated templates with better styling and functionality
- **🛠️ Utility Scripts** → API testing and cron validation tools

### ✅ Maintained Pi-Specific Features:
- **🕐 Cron Scheduling** → Native Linux cron integration (as in original)
- **🎵 PulseAudio System** → Enhanced Raspberry Pi audio with PulseAudio
- **🔧 Audio Integration** → PulseAudio + ALSA fallback support
- **⚙️ System Integration** → Native Pi audio controls with pactl/amixer

## Prerequisites

### Required Software:
1. **Raspberry Pi OS** (Bullseye or newer recommended)
2. **Python 3.8 or higher** (usually pre-installed)
3. **Audio System** configured properly:
   - Pi audio output set to 3.5mm or HDMI as needed
   - Audio drivers working correctly

### Required Python Packages:
- Flask (web framework)
- pydub (audio processing)
- requests (for API testing)

## Installation

### Quick Installation:
```bash
# Navigate to the directory
cd /path/to/TARR_Annunciator/dev/deb-arm

# Install Python dependencies
pip3 install -r requirements.txt

# Make scripts executable
chmod +x *.py
chmod +x *.sh

# Test audio system
python3 app.py --test-audio
```

### Manual Installation:
```bash
# Install system dependencies (if needed)
sudo apt update
sudo apt install python3-pip python3-venv ffmpeg pulseaudio pulseaudio-utils alsa-utils

# Create virtual environment (optional but recommended)
python3 -m venv tarr_env
source tarr_env/bin/activate

# Install Python requirements
pip install -r requirements.txt
```

## Running the Application

### Option 1: Web Interface (Recommended)
```bash
python3 app.py
```
Then access:
- **Main Interface**: http://localhost:8080
- **Admin Interface**: http://localhost:8080/admin (login: admin/tarr2025)
- **API Documentation**: http://localhost:8080/api/docs

### Option 2: Command Line Usage
```bash
# Test audio system
python3 app.py --test-audio

# Play station announcement
python3 app.py --station --train "1" --direction "westbound" --destination "goodwin_station" --track "1"

# Play safety announcement
python3 app.py --safety --language "english"

# Play promotional announcement
python3 app.py --promo --file "promo_english.mp3"
```

## 🔧 Configuration

### Audio Setup:
1. **PulseAudio will be configured automatically** during installation
2. **Test audio** with the built-in test:
   ```bash
   python3 app.py --test-audio
   ```
3. **Check audio devices**:
   ```bash
   pactl list short sinks
   ```
4. **Adjust volume** via the web interface or API

### Admin Access:
- **Configuration**: `json/admin_config.json`
- **Default Username**: `admin`
- **Default Password**: `tarr2025`
- **⚠️ Change these** in `admin_config.json` before production use!
- **Session timeout**: 60 minutes (configurable)

### API Access:
- **API Key**: `tarr-api-2025`
- **⚠️ Change this** in `app.py` for security!

## 🔌 API Usage

### Quick API Examples:

**Trigger Station Announcement:**
```bash
curl -X POST http://localhost:8080/api/announce/station \
  -H "X-API-Key: tarr-api-2025" \
  -H "Content-Type: application/json" \
  -d '{
    "train_number": "1",
    "direction": "westbound",
    "destination": "goodwin_station",
    "track_number": "1"
  }'
```

**Check System Status:**
```bash
curl http://localhost:8080/api/status
```

**Set Volume:**
```bash
curl -X POST http://localhost:8080/api/audio/volume \
  -H "X-API-Key: tarr-api-2025" \
  -d "volume=75"
```

See `/api/docs` for complete API documentation.

## 🕐 Scheduling

The system uses native Linux cron for scheduling announcements:

1. **Configure schedules** in the admin interface
2. **Schedules are automatically synced** to system crontab
3. **View active jobs** with:
   ```bash
   crontab -l
   ```

### Cron Format:
```
minute hour day month day_of_week
```

**Examples:**
- `0 8 * * *` - Daily at 8:00 AM
- `*/15 * * * *` - Every 15 minutes
- `0 12 * * 1-5` - Weekdays at noon

## 🛠️ Utility Scripts

### Test API Functionality:
```bash
python3 api_test.py
```

### Validate Cron Expressions:
```bash
python3 validate_cron.py
```

## 📁 File Structure

```
deb-arm/
├── app.py                 # Main application (enhanced)
├── requirements.txt       # Python dependencies
├── api_test.py           # API testing script
├── validate_cron.py      # Cron validation utility
├── README.md             # This file
├── json/                 # Configuration files
│   ├── admin_config.json # Admin credentials and security settings
│   ├── cron.json         # Scheduled announcements
│   ├── trains.json       # Available trains
│   ├── directions.json   # Available directions
│   ├── destinations.json # Available destinations
│   ├── tracks.json       # Available tracks
│   ├── promo.json        # Promotional announcements
│   └── safety.json       # Safety announcement languages
├── static/mp3/           # Audio files
│   ├── chime.mp3         # Announcement chime
│   ├── train/            # Train number announcements
│   ├── direction/        # Direction announcements
│   ├── destination/      # Destination announcements
│   ├── track/            # Track announcements
│   ├── promo/            # Promotional announcements
│   └── safety/           # Safety announcements
└── templates/            # Web interface templates
    ├── index.html        # Main interface
    ├── admin.html        # Admin interface
    ├── admin_login.html  # Login page
    └── api_docs.html     # API documentation
```

## 🔊 Audio Configuration

### Supported Audio Formats:
- **MP3** files in the `static/mp3/` directory structure
- **Organized by type**: train numbers, directions, destinations, etc.

### Audio Device Selection:
- Uses **PulseAudio** for primary audio control
- **ALSA fallback** for compatibility
- **Automatic device detection** via pactl
- **Volume control** via pactl and amixer fallback
- **Device info** available in admin interface

### Testing Audio:
```bash
# Test with the application
python3 app.py --test-audio

# Test PulseAudio directly
pactl info
pactl list short sinks

# Test with system tools (fallback)
aplay /usr/share/sounds/alsa/Front_Left.wav
speaker-test -t sine -f 1000 -l 1
```

## 🔒 Security Considerations

1. **Change default credentials** in `json/admin_config.json`:
   - Admin username and password
   - Session timeout settings
   - Security options

2. **Change API key** in `app.py`:
   - `API_KEY` variable

3. **Network security**:
   - Application runs on `0.0.0.0:8080` (all interfaces)
   - Consider firewall rules for production
   - Use HTTPS in production environments

3. **File permissions**:
   - Ensure proper permissions on audio files
   - Protect configuration files (especially admin_config.json)

## 🐛 Troubleshooting

### Audio Issues:
```bash
# Check PulseAudio status
pactl info
pactl list short sinks

# Restart PulseAudio if needed
pulseaudio --kill
pulseaudio --start

# Check ALSA devices (fallback)
aplay -l

# Test system audio
speaker-test -c2

# Check volume controls
pactl get-sink-volume @DEFAULT_SINK@
alsamixer

# Test application audio
python3 app.py --test-audio
```

### Cron Issues:
```bash
# Check cron service
sudo systemctl status cron

# View system logs
sudo journalctl -u cron

# Validate cron expressions
python3 validate_cron.py
```

### API Issues:
```bash
# Test API connectivity
python3 api_test.py

# Check application logs
python3 app.py  # Look for error output
```

### General Troubleshooting:
1. **Check Python version**: `python3 --version` (should be 3.8+)
2. **Check dependencies**: `pip3 show flask pydub requests`
3. **Check file permissions**: Ensure scripts are executable
4. **Check audio setup**: Use `raspi-config` to configure audio
5. **Check system resources**: `htop` or `free -m`

## 🚀 Running as a Service (Optional)

To run automatically on boot:

1. **Create systemd service file**:
   ```bash
   sudo nano /etc/systemd/system/tarr-annunciator.service
   ```

2. **Add service configuration**:
   ```ini
   [Unit]
   Description=TARR Annunciator
   After=network.target sound.target

   [Service]
   Type=simple
   User=pi
   WorkingDirectory=/path/to/TARR_Annunciator/dev/deb-arm
   ExecStart=/usr/bin/python3 app.py
   Restart=always
   RestartSec=10

   [Install]
   WantedBy=multi-user.target
   ```

3. **Enable and start service**:
   ```bash
   sudo systemctl enable tarr-annunciator.service
   sudo systemctl start tarr-annunciator.service
   sudo systemctl status tarr-annunciator.service
   ```

## 📊 Monitoring

### System Status:
- **Web interface**: http://localhost:8080/scheduler_status
- **API endpoint**: `/api/status`
- **Audio status**: `/audio_status`

### Logs:
- **Application logs**: Console output or service logs
- **Cron logs**: `/var/log/cron.log` or `journalctl -u cron`
- **System logs**: `journalctl -u tarr-annunciator.service`

## 🔄 Differences from Windows Version

| Feature | Windows Version | Raspberry Pi Version |
|---------|----------------|---------------------|
| **Scheduling** | APScheduler | Native Linux cron |
| **Audio Backend** | pygame | pydub + PulseAudio |
| **Volume Control** | Windows API | pactl + amixer fallback |
| **Device Detection** | Windows WMI | PulseAudio pactl list |
| **Installation** | Virtual env + batch scripts | pip + bash |
| **Service** | Task Scheduler / Windows Service | systemd |

## 📈 Future Enhancements

Potential improvements for future versions:
- **Web-based audio file upload**
- **Real-time audio level monitoring**
- **Multiple audio output support**
- **Integration with external train management systems**
- **Enhanced security features (OAuth, rate limiting)**
- **Mobile-responsive interface improvements**

## 📄 License

Same as the original TARR Annunciator project.

## 🤝 Support

1. **Check this README** for common solutions
2. **Run diagnostic scripts**: `api_test.py`, `validate_cron.py`
3. **Check system logs** for detailed error information
4. **Verify audio setup** with system tools

## 🎯 Quick Start Checklist

- [ ] Raspberry Pi OS installed and updated
- [ ] Python 3.8+ available
- [ ] Audio system configured and tested
- [ ] Dependencies installed (`pip3 install -r requirements.txt`)
- [ ] Audio files present in `static/mp3/` directory structure
- [ ] Application tested (`python3 app.py --test-audio`)
- [ ] Admin credentials changed in `json/admin_config.json`
- [ ] API key changed in `app.py`
- [ ] Web interface accessible at http://localhost:8080

**🎉 You're ready to use the enhanced TARR Annunciator!**
