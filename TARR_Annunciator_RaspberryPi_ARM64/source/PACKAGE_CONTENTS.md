# TARR Annunciator - Package Contents Guide

This document details exactly which files need to be included in each platform-specific installation package.

## 📁 Core Application Files (Required for ALL packages)

### Executable Files
- **Executable Binary** (platform-specific):
  - Windows: `tarr-annunciator.exe`
  - Linux/Pi: `tarr-annunciator` (no extension)

### Configuration & Data Files
```
json/
├── admin_config.json       # Admin panel configuration
├── cron.json              # Scheduler configuration
├── destinations.json      # Station destinations
├── directions.json        # Direction options (eastbound/westbound)
├── promo.json            # Promotional announcements
├── safety.json           # Safety announcements
├── tracks.json           # Track number options
└── trains.json           # Train number options
```

### Web Interface Templates
```
templates/
├── admin.html            # Admin panel interface
├── admin_login.html      # Admin login page
├── api_docs.html         # API documentation page
└── index.html            # Main public interface
```

### Audio Files
```
static/mp3/
├── chime.mp3             # Station chime sound
├── destination/          # Destination announcements
│   ├── goodwin_station.mp3
│   ├── hialeah.mp3
│   ├── picnic_station.mp3
│   ├── tradewinds_central_station.mp3
│   └── yard.mp3
├── direction/            # Direction announcements
│   ├── eastbound.mp3
│   └── westbound.mp3
├── promo/               # Promotional announcements
│   └── promo_english.mp3
├── safety/              # Safety announcements
│   ├── safety_announcement.mp3
│   ├── safety_english.mp3
│   ├── safety_english2.mp3
│   ├── safety_portuguese.mp3
│   ├── safety_russian.mp3
│   ├── safety_spanish.mp3
│   └── safety_tts.txt
├── track/               # Track number announcements
│   ├── 1.mp3
│   ├── 2.mp3
│   └── express.mp3
└── train/               # Train number announcements
    ├── 1.mp3
    ├── 2006.mp3
    ├── 2199.mp3
    ├── 27.mp3
    ├── 347.mp3
    ├── 415.mp3
    ├── 428.mp3
    ├── 4900.mp3
    ├── 573.mp3
    ├── 774.mp3
    ├── 815.mp3
    ├── 9.mp3
    └── 944.mp3
```

### Directory Structure (create empty directories)
```
logs/                     # Application logs directory (create empty)
```

## 📦 Windows Package Contents

### Required Files
**Executable:**
- `tarr-annunciator.exe` (Windows x64 build)

**Scripts & Tools:**
- `build_windows.bat`     # Build script
- `run_windows_go.bat`    # Run script
- `install_windows.bat`   # Installation script  
- `install_windows.ps1`   # PowerShell installation script
- `cleanup.bat`           # Cleanup utility

**Documentation:**
- `README_Windows.md`     # Windows-specific documentation
- `README_CrossPlatform.md` # Cross-platform guide

**All Core Files** (listed above)

### Optional Files (for development/building)
- `requirements_windows.txt` # Python dependencies (if including Python tools)
- `validate_cron.py`        # Cron validation utility
- `api_test.py`             # API testing script

### Package Structure
```
TARR_Annunciator_Windows_x64/
├── tarr-annunciator.exe
├── run_windows_go.bat
├── install_windows.bat
├── install_windows.ps1
├── cleanup.bat
├── README_Windows.md
├── README_CrossPlatform.md
├── json/                 # All JSON files
├── templates/            # All template files  
├── static/mp3/           # All audio files
└── logs/                 # Empty directory
```

## 📦 Linux x64 Package Contents

### Required Files
**Executable:**
- `tarr-annunciator` (Linux x64 build)

**Scripts:**
- `build_linux.sh`        # Build script
- `run_linux.sh`          # Run script
- `test_crossplatform.sh` # Cross-platform test script
- `Makefile`              # Build system

**Documentation:**
- `README_CrossPlatform.md` # Primary documentation
- `README_Go.md`          # Go-specific documentation

**All Core Files** (listed above)

### Package Structure  
```
TARR_Annunciator_Linux_x64/
├── tarr-annunciator
├── run_linux.sh
├── build_linux.sh
├── test_crossplatform.sh
├── Makefile
├── README_CrossPlatform.md
├── README_Go.md
├── json/                 # All JSON files
├── templates/            # All template files
├── static/mp3/           # All audio files
└── logs/                 # Empty directory
```

## 📦 Raspberry Pi Package Contents

### Raspberry Pi ARM64 (Pi 4/5)
**Executable:**
- `tarr-annunciator` (Linux ARM64 build)

**Pi-Specific Scripts:**
- `install_raspberry_pi.sh` # Complete Pi installer
- `run_raspberry_pi.sh`     # Pi-optimized launcher

**Standard Scripts:**
- `build_linux.sh`          # Build script  
- `test_crossplatform.sh`   # Test script
- `Makefile`                # Build system

**Documentation:**
- `README_RaspberryPi.md`   # Primary Pi documentation
- `README_CrossPlatform.md` # Cross-platform guide

**All Core Files** (listed above)

### Package Structure
```
TARR_Annunciator_RaspberryPi_ARM64/
├── tarr-annunciator
├── install_raspberry_pi.sh
├── run_raspberry_pi.sh
├── build_linux.sh
├── test_crossplatform.sh
├── Makefile
├── README_RaspberryPi.md
├── README_CrossPlatform.md
├── json/                 # All JSON files
├── templates/            # All template files
├── static/mp3/           # All audio files
└── logs/                 # Empty directory
```

### Raspberry Pi ARM32 (Pi 2/3)
Same as ARM64 package but with:
- `tarr-annunciator` (Linux ARM32 build)

### Raspberry Pi ARMv6 (Pi Zero/1)  
Same as ARM64 package but with:
- `tarr-annunciator` (Linux ARMv6 build)

## 📦 macOS Package Contents (Future)

### Required Files
**Executable:**
- `tarr-annunciator` (Darwin x64 or ARM64 build)

**Scripts:**
- `run_macos.sh`           # macOS launcher
- `Makefile`               # Build system

**Documentation:**
- `README_CrossPlatform.md` # Primary documentation

**All Core Files** (listed above)

## 🔧 Package Creation Guidelines

### File Permissions
**Linux/Pi Packages:**
```bash
# Executable permissions
chmod +x tarr-annunciator
chmod +x *.sh
chmod +x Makefile

# Directory permissions  
chmod 755 json/ templates/ static/ logs/
chmod 644 json/* templates/* static/mp3/*/*
```

**Windows Packages:**
- All `.exe` and `.bat` files should be executable
- No special permissions needed

### Package Sizes (Approximate)
- **Windows x64**: ~45MB (with audio files)
- **Linux x64**: ~44MB (with audio files)  
- **Raspberry Pi**: ~44MB (with audio files)
- **Audio files only**: ~38MB
- **Application only**: ~6MB

### Compression Recommendations
- **Windows**: ZIP format (.zip)
- **Linux**: TAR.GZ format (.tar.gz)
- **Raspberry Pi**: TAR.GZ format (.tar.gz)
- **macOS**: TAR.GZ or DMG format

## ⚠️ Important Notes

### Files NOT to Include in Packages
- `go.mod`, `go.sum` (Go build files)
- `*.go` source files (main.go, api.go, etc.)
- `api_test_go.go.bak` (backup file)
- `app.py` (old Python version)
- `rename_files.bat` (development utility)
- Development directories like `.git/`

### Platform-Specific Considerations

**Windows:**
- Include both `.bat` and `.ps1` scripts for flexibility
- Consider including Visual C++ redistributables if needed

**Linux:**
- Ensure scripts have proper line endings (LF, not CRLF)
- Include systemd service examples in documentation

**Raspberry Pi:**
- Consider creating separate packages for each ARM variant
- Include audio configuration helpers
- Document GPIO expansion possibilities

### Version Management
Each package should include:
- Version information in README files
- Build date/commit in executable (if possible)
- Clear architecture identification in package names

### Dependencies Documentation
Each package should clearly document:
- **Windows**: PowerShell modules, audio drivers
- **Linux**: ALSA/PulseAudio requirements
- **Raspberry Pi**: Audio configuration, boot config requirements

This structure ensures each package contains exactly what's needed for that platform while avoiding bloat from unnecessary files.