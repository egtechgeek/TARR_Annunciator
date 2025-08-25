# TARR Annunciator Legacy Version

-Designed to be run on a Raspberry Pi 4b	<br>
-ALWAYS review through shell scripts before running them!	<br>
-You will encounter errors if you attempt to run install.sh more than once	<br>
-Update.sh will poll the repository for updated files, and copy them into the install directory.	<br>

## Prerequisites 

-Pi audio output set to 3.5mm	<br>
-Pi advanced audio set to pulseaudio
<br>

# TARR Annunciator Enhanced Versions (win64 & deb-arm)

# TARR Annunciator - Cross-Platform Go Version

A cross-platform train announcement system built with Go, supporting Windows, Linux, and macOS.

## 🌍 Platform Support

### ✅ Windows
- **Audio Backend**: faiface/beep with native Windows speaker support
- **Device Detection**: AudioDeviceCmdlets PowerShell module (with WMI fallback)
- **Device Switching**: Full support via AudioDeviceCmdlets
- **Requirements**: Windows 7+ (PowerShell recommended)

### ✅ Linux
- **Audio Backend**: faiface/beep with ALSA/PulseAudio support
- **Device Detection**: PulseAudio (`pactl`) and ALSA (`aplay`) support
- **Device Switching**: PulseAudio full support, ALSA manual configuration
- **Requirements**: PulseAudio or ALSA

### ⚠️ macOS (Basic Support)
- **Audio Backend**: faiface/beep with Core Audio
- **Device Detection**: Basic system detection
- **Device Switching**: Not yet implemented
- **Requirements**: macOS 10.12+

## 🚀 Quick Start

### Build for Current Platform
```bash
# Using make (recommended)
make build

# Or using go directly
go build -o tarr-annunciator .
```

### Cross-Platform Builds
```bash
# Build for all platforms
make build-all

# Or specific platforms
make build-windows    # Creates dist/windows/tarr-annunciator.exe
make build-linux      # Creates dist/linux/tarr-annunciator  
make build-darwin     # Creates dist/darwin/tarr-annunciator
```

### Platform-Specific Scripts

#### Windows
```cmd
REM Build
build_windows.bat

REM Run
run_windows_go.bat
```

#### Linux
```bash
# Build
chmod +x build_linux.sh
./build_linux.sh

# Run
chmod +x run_linux.sh
./run_linux.sh
```

## 📱 Features

### 🔊 Cross-Platform Audio
- **Volume Control**: Real-time volume adjustment (0-100%)
- **Device Selection**: Platform-appropriate audio device enumeration
- **Audio Testing**: Built-in audio test functionality

### 🌐 Web Interface
- **Admin Panel**: `/admin` - Full configuration interface
- **Main Interface**: `/` - Public announcement interface  
- **API Documentation**: `/api/docs` - Complete API reference

### 📡 REST API
- **Cross-Platform Status**: `GET /api/platform` - Platform and audio system info
- **Device Management**: `GET/POST /api/audio/devices` - List and set audio devices
- **Volume Control**: `GET/POST /api/audio/volume` - Get and set volume
- **Announcements**: Station, safety, and promo announcement triggers

## 🔧 Platform-Specific Setup

### Windows Setup

1. **Optional**: Install AudioDeviceCmdlets for advanced audio device control:
   ```powershell
   Install-Module -Name AudioDeviceCmdlets -Force
   ```

2. **Build and run**:
   ```cmd
   build_windows.bat
   run_windows_go.bat
   ```

### Linux Setup

1. **Ensure audio system is available**:
   ```bash
   # For PulseAudio (recommended)
   sudo apt install pulseaudio-utils  # Ubuntu/Debian
   sudo yum install pulseaudio-utils   # RHEL/CentOS
   
   # For ALSA (fallback)  
   sudo apt install alsa-utils         # Ubuntu/Debian
   sudo yum install alsa-utils         # RHEL/CentOS
   ```

2. **Build and run**:
   ```bash
   chmod +x build_linux.sh run_linux.sh
   ./build_linux.sh
   ./run_linux.sh
   ```

### macOS Setup

1. **Build and run**:
   ```bash
   make build
   ./tarr-annunciator
   ```

## 🎛️ Audio System Details

### Windows Audio
- **Primary**: AudioDeviceCmdlets PowerShell module
  - Full device enumeration and switching
  - Requires: `Install-Module AudioDeviceCmdlets`
- **Fallback**: WMI (Windows Management Instrumentation)
  - Basic device detection
  - No device switching capability

### Linux Audio
- **Primary**: PulseAudio
  - Full device enumeration via `pactl list sinks`
  - Device switching via `pactl set-default-sink`
  - Automatic default device detection
- **Fallback**: ALSA
  - Device enumeration via `aplay -l`
  - Manual configuration required for device switching
  - Edit `~/.asoundrc` or `/etc/asound.conf`

### macOS Audio
- **Current**: Basic Core Audio support
- **Planned**: Enhanced device enumeration and switching

## 🌐 API Endpoints

### Platform Information
```bash
# Get platform and audio system info
curl http://localhost:8080/api/platform

# Response includes:
{
  "platform_info": {
    "platform": "linux",
    "arch": "amd64", 
    "pulse_available": true,
    "alsa_available": true
  },
  "audio_devices": [...],
  "current_device": "device_id",
  "cross_platform": true
}
```

### Audio Device Management
```bash
# List available audio devices
curl http://localhost:8080/api/audio/devices \
  -H "X-API-Key: #########"

# Set audio device
curl -X POST http://localhost:8080/api/audio/devices \
  -H "X-API-Key: #########" \
  -H "Content-Type: application/json" \
  -d '{"device_id": "pulse_sink_name"}'
```

## 🛠️ Development

### Project Structure
```
tarr-annunciator/
├── main.go              # Main application entry
├── audio_devices.go     # Cross-platform audio device management  
├── audio.go             # Audio playback using faiface/beep
├── api.go               # REST API handlers
├── utils.go             # Utility functions
├── Makefile             # Cross-platform build system
├── build_windows.bat    # Windows build script
├── build_linux.sh       # Linux build script
├── run_linux.sh         # Linux run script
└── templates/           # HTML templates
    ├── admin.html       # Admin interface with platform info
    ├── index.html       # Main interface
    └── api_docs.html    # API documentation
```

### Adding Platform Support

1. **Add platform detection** in `audio_devices.go`:
   ```go
   case "your_platform":
       return getYourPlatformAudioDevices()
   ```

2. **Implement device functions**:
   ```go
   func getYourPlatformAudioDevices() []AudioDevice { ... }
   func setYourPlatformAudioDevice(deviceID string) error { ... }
   ```

3. **Update platform info** in `getPlatformInfo()`:
   ```go
   case "your_platform":
       // Add platform-specific capability detection
   ```

## 🐛 Troubleshooting

### Windows Issues
- **AudioDeviceCmdlets not found**: Install with `Install-Module AudioDeviceCmdlets`
- **PowerShell execution policy**: Run `Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser`

### Linux Issues  
- **No audio devices found**: Install `pulseaudio-utils` or `alsa-utils`
- **PulseAudio not running**: Start with `pulseaudio --start`
- **Permission issues**: Add user to `audio` group: `sudo usermod -a -G audio $USER`

### Cross-Platform Issues
- **Build failures**: Ensure Go 1.21+ is installed
- **Audio not working**: Check platform-specific audio system is running
- **Device switching not working**: See platform-specific requirements above

## 📋 System Requirements

- **Go**: 1.21 or higher
- **Memory**: 50MB RAM
- **Disk**: 20MB for executable + audio files
- **Network**: Port 8080 (configurable)

## 🔗 Useful Commands

```bash
# Development
make deps          # Download dependencies
make fmt           # Format code  
make vet           # Vet code
make clean         # Clean build artifacts

# Platform detection
./tarr-annunciator --platform-info  # (if implemented)

# Audio testing
curl -X POST http://localhost:8080/audio/test \
  -H "Cookie: session=admin_session"
```

## 📄 License

This project is part of the TARR (Train Announcement Railroad Radio) system and follows the same licensing terms as the main project.