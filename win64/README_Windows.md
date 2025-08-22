# TARR Annunciator - Windows Port

This is a Windows-compatible version of the TARR Annunciator system, originally designed for Raspberry Pi 4.

## What Changed from the Original

### ✅ Replaced Linux-specific components:
- **Cron scheduling** → **APScheduler** (Python-based background scheduler)
- **PulseAudio** → **Windows default audio system** (via pydub)
- **System cron commands** → **Background threading**
- **apt-get packages** → **pip packages**
- **Linux paths** → **Windows-compatible paths**

### ✅ Added Windows-specific features:
- Virtual environment setup
- FFmpeg detection and installation guidance
- Windows batch and PowerShell installation scripts
- Threading for non-blocking audio playback
- Scheduler status endpoint (`/scheduler_status`)
- Audio system testing (`--test-audio` CLI flag)

## Prerequisites

### Required Software:
1. **Python 3.8 or higher** - Download from [python.org](https://python.org)
   - ⚠️ **Important**: Check "Add Python to PATH" during installation
2. **FFmpeg** (for audio processing) - Choose one method:
   - **Chocolatey**: `choco install ffmpeg`
   - **Winget**: `winget install FFmpeg.FFmpeg`
   - **Manual**: Download from [FFmpeg.org](https://ffmpeg.org/download.html)

### Audio Requirements:
- Functional Windows audio system
- Speakers or headphones connected
- Audio drivers properly installed

## Installation

### Option 1: Batch Script (Recommended for beginners)
```batch
# Run as Administrator (optional but recommended)
install_windows.bat
```

### Option 2: PowerShell Script
```powershell
# Run in PowerShell
PowerShell -ExecutionPolicy Bypass -File install_windows.ps1
```

### Option 3: Manual Installation
```batch
# Create virtual environment
python -m venv tarr_env

# Activate virtual environment
tarr_env\Scripts\activate.bat

# Install dependencies
pip install -r requirements_windows.txt
```

## Running the Application

### Option 1: Using the batch script
```batch
run_windows.bat
```

### Option 2: Manual execution
```batch
# Activate virtual environment
tarr_env\Scripts\activate.bat

# Run the application
python app_windows.py
```

### Option 3: Command line with arguments
```batch
# Test audio system
python app_windows.py --test-audio

# Play station announcement
python app_windows.py --station --train "1" --direction "westbound" --destination "goodwin_station" --track "1"

# Play promo
python app_windows.py --promo --file "promo_english.mp3"

# Play safety announcement
python app_windows.py --safety --language "english"
```

## Accessing the Interface

Once running, the application will be available at:
- **Main Interface**: http://localhost:8080
- **Admin Interface**: http://localhost:8080/admin
- **Scheduler Status**: http://localhost:8080/scheduler_status

## Key Differences from Raspberry Pi Version

### Scheduling System
- **Original**: Uses Linux cron jobs
- **Windows**: Uses APScheduler with the same cron syntax
- **Benefit**: Integrated with the application, no separate system configuration needed

### Audio System
- **Original**: PulseAudio with specific Raspberry Pi audio configuration
- **Windows**: Uses Windows default audio system through pydub
- **Benefit**: Works with any Windows audio setup

### Installation
- **Original**: System-wide installation with nginx and systemd services
- **Windows**: Self-contained virtual environment
- **Benefit**: No system modifications, easy to uninstall

### File Paths
- **Original**: Linux paths (`/opt/TARR_Annunciator/`)
- **Windows**: Relative paths from application directory
- **Benefit**: Portable, can run from any directory

## Troubleshooting

### "Python not found" error
- Reinstall Python from [python.org](https://python.org)
- Make sure to check "Add Python to PATH" during installation
- Restart Command Prompt/PowerShell after installation

### "FFmpeg not found" warning
- Install FFmpeg using one of the methods in Prerequisites
- Add FFmpeg to your system PATH
- Restart the application after installing FFmpeg

### Audio not playing
- Check Windows audio settings and volume
- Ensure speakers/headphones are connected and working
- Test with: `python app_windows.py --test-audio`
- Verify MP3 files exist in the `static/mp3/` directory

### Scheduler not working
- Check the `/scheduler_status` endpoint to see active jobs
- Verify cron expressions in the admin interface
- Check console output for scheduling errors

### Port 8080 already in use
- Close other applications using port 8080
- Or modify the port in `app_windows.py`: change `port=8080` to another port

### Virtual environment issues
- Delete the `tarr_env` folder and run installation again
- Make sure you're running from the correct directory

## Configuration

### Audio Files
Place your MP3 files in the following structure:
```
static/mp3/
├── chime.mp3
├── destination/
│   ├── goodwin_station.mp3
│   ├── hialeah.mp3
│   └── ...
├── direction/
│   ├── eastbound.mp3
│   └── westbound.mp3
├── promo/
│   └── promo_english.mp3
├── safety/
│   ├── safety_english.mp3
│   ├── safety_spanish.mp3
│   └── ...
├── track/
│   ├── 1.mp3
│   ├── 2.mp3
│   └── express.mp3
└── train/
    ├── 1.mp3
    ├── 9.mp3
    └── ...
```

### JSON Configuration Files
Located in the `json/` directory:
- `cron.json` - Scheduled announcements
- `trains.json` - Available trains
- `directions.json` - Available directions
- `destinations.json` - Available destinations
- `tracks.json` - Available tracks
- `promo.json` - Promotional announcements
- `safety.json` - Safety announcement languages

### Scheduling Format
The cron format is: `minute hour day month day_of_week`
- `*` means "any value"
- Examples:
  - `0 8 * * *` = Every day at 8:00 AM
  - `0 12 * * 1-5` = Weekdays at 12:00 PM
  - `*/15 * * * *` = Every 15 minutes

## Running as a Windows Service (Optional)

To run the application as a Windows service that starts automatically:

1. Install `pywin32`:
   ```batch
   pip install pywin32
   ```

2. Create a service wrapper script (advanced users)

3. Or use Task Scheduler:
   - Open Task Scheduler
   - Create Basic Task
   - Set trigger (e.g., "At startup")
   - Set action to run `run_windows.bat`

## Uninstalling

To remove the application:
1. Stop the application (Ctrl+C in the console)
2. Delete the application folder
3. No system changes were made, so no additional cleanup is needed

## Development

### Adding New Features
The Windows version maintains the same Flask structure as the original, so most modifications should work in both versions.

### Key Files
- `app_windows.py` - Main application (Windows-specific version)
- `app.py` - Original Raspberry Pi version
- `requirements_windows.txt` - Python dependencies
- `install_windows.bat` - Installation script
- `run_windows.bat` - Run script

### API Endpoints
- `GET /` - Main interface
- `POST /play_announcement` - Play station announcement
- `POST /play_promo` - Play promotional announcement
- `POST /play_safety_announcement` - Play safety announcement
- `GET /admin` - Admin interface
- `POST /admin` - Update cron schedule
- `GET /scheduler_status` - View scheduler status (Windows only)

## Support

For issues specific to the Windows port:
1. Check the troubleshooting section above
2. Verify all prerequisites are installed
3. Check console output for error messages
4. Ensure all MP3 files are present and valid

For issues with the original functionality:
- Refer to the original README.md
- Check the JSON configuration files
- Verify audio file paths and formats

## License

Same as the original TARR Annunciator project.
