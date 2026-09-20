# TARR Annunciator Updater - Usage Examples

## Basic Update Check

The simplest way to use the updater:

```bash
# Windows
tarr-updater.exe

# Linux/Raspberry Pi/macOS
./tarr-updater
```

## Expected Output

```
🔄 TARR Annunciator Updater v1.0
=====================================
📱 Detected System: windows/amd64
🎯 Target Executable: tarr-annunciator.exe
📅 Last Check: never

🔍 Checking for updates...

🔍 Checking for executable updates...
📦 Found executable: tarr-annunciator-windows-x64.exe (18834944 bytes)
⬇️  Downloading updated executable...
✅ Executable updated successfully

🔍 Checking for data file updates...
⬇️  Updating: emergencies.json
⬇️  Updating: admin.html
✅ Updated 2 data files

✅ Update check complete!
```

## Repository Setup Required

For the updater to work, the GitHub repository must have this structure:

### 1. Compiled Packages Directory

Create: `compiled_packages/` in your repository root with these files:

```
compiled_packages/
├── tarr-annunciator-windows-x64.exe
├── tarr-annunciator-linux-x64
├── tarr-annunciator-raspberry-pi-arm64
├── tarr-annunciator-raspberry-pi-arm32
├── tarr-annunciator-macos-x64
└── tarr-annunciator-macos-arm64
```

### 2. Data Directory

Create: `data/` in your repository root mirroring the local structure:

```
data/
├── json/
│   ├── trains.json
│   ├── directions.json
│   ├── destinations.json
│   ├── tracks.json
│   ├── promo.json
│   ├── safety.json
│   ├── emergencies.json
│   └── cron.json
├── templates/
│   ├── admin.html
│   ├── admin_login.html
│   ├── api_docs.html
│   └── index.html
└── static/
    └── mp3/
        ├── chime.mp3
        └── [other audio files...]
```

## Installation in Packages

The integration script automatically adds the updater to install packages:

### Windows Package Contents
```
TARR_Annunciator_Windows_x64/
├── tarr-annunciator.exe          # Main application
├── tarr-updater.exe              # Updater (NEW)
├── update.bat                    # Convenience script (NEW)
├── install_windows.bat
├── run.bat
├── json/
├── templates/
├── static/
└── logs/
```

### Raspberry Pi Package Contents
```
TARR_Annunciator_RaspberryPi_ARM64/
├── tarr-annunciator              # Main application
├── tarr-updater                  # Updater (NEW)
├── update.sh                     # Convenience script (NEW)
├── install_raspberry_pi.sh
├── run.sh
├── json/
├── templates/
├── static/
└── logs/
```

## User Instructions

### Windows Users
1. Extract `TARR_Annunciator_Windows_x64.zip`
2. Run `install_windows.bat` for initial setup
3. Use `update.bat` to check for updates anytime
4. Or run `tarr-updater.exe` directly

### Raspberry Pi Users
1. Extract `TARR_Annunciator_RaspberryPi_ARM64.tar.gz`
2. Run `./install_raspberry_pi.sh` for initial setup
3. Use `./update.sh` to check for updates anytime
4. Or run `./tarr-updater` directly

## Configuration File

The updater creates `updater_config.json`:

```json
{
  "current_version": "unknown",
  "last_check": "2024-08-25T11:30:00Z", 
  "auto_update": false
}
```

## Error Scenarios

### Network Issues
```
🔄 TARR Annunciator Updater v1.0
=====================================
📱 Detected System: linux/amd64
🎯 Target Executable: tarr-annunciator

🔍 Checking for updates...
❌ Error checking executable updates: failed to get compiled packages directory: Get "https://api.github.com/repos/egtechgeek/TARR_Annunciator/contents/compiled_packages": dial tcp: lookup api.github.com: no such host

✅ Update check complete!
```

### No Updates Available
```
🔄 TARR Annunciator Updater v1.0
=====================================
📱 Detected System: windows/amd64
🎯 Target Executable: tarr-annunciator.exe

🔍 Checking for updates...

🔍 Checking for executable updates...
📦 Found executable: tarr-annunciator-windows-x64.exe (18834944 bytes)
✅ Executable is up to date

🔍 Checking for data file updates...
✅ All data files are up to date

✅ Update check complete!
```

## Integration with Main Application

The updater can be called from the main TARR Annunciator application:

### Admin Interface Integration
Add an "Update" button that calls:
```go
cmd := exec.Command("./tarr-updater")
output, err := cmd.CombinedOutput()
```

### Startup Check
Check for updates on application startup:
```go
// Optional: Check for updates silently on startup
go func() {
    cmd := exec.Command("./tarr-updater")
    cmd.Run()
}()
```

## Platform-Specific Notes

### Windows
- Updater executable: `tarr-updater.exe`
- May require firewall approval for internet access
- Run as Administrator if updating system-level installations

### Linux/Raspberry Pi
- Updater executable: `tarr-updater`
- Must be executable: `chmod +x tarr-updater`
- May need sudo for system-level updates

### macOS
- Same as Linux but with macOS-specific executable names
- May require Gatekeeper approval for first run

## GitHub API Rate Limits

- 60 requests/hour for unauthenticated access
- Should be sufficient for normal update checking
- Add authentication token if higher limits needed

## Future Enhancements

Planned improvements:
- Automatic update scheduling
- Delta updates for large files
- Cryptographic signature verification
- GUI interface for updates
- Integration with systemd/Windows services for automatic updates