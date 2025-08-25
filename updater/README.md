# TARR Annunciator Updater

A standalone updater application for TARR Annunciator that automatically detects the current system and downloads updates from the GitHub repository.

## Features

- **Automatic OS/Architecture Detection**: Detects Windows, Linux, macOS, and various ARM architectures
- **Executable Updates**: Downloads and replaces the main TARR Annunciator executable
- **Data File Synchronization**: Downloads missing or outdated configuration files, templates, and audio files
- **Safe Updates**: Creates backups before replacing files
- **GitHub Integration**: Uses GitHub API to check for updates

## How It Works

1. **System Detection**: Automatically detects your operating system and CPU architecture
2. **Executable Check**: Compares your local executable with the latest version in `compiled_packages/`
3. **Data Sync**: Checks `data/` directory for missing or updated configuration files
4. **Safe Download**: Downloads to temporary files before replacing originals
5. **Backup & Replace**: Creates backups and safely replaces files

## Supported Platforms

- **Windows**: x64 (`tarr-updater-windows-x64.exe`)
- **Linux**: x64 (`tarr-updater-linux-x64`) 
- **Raspberry Pi**: ARM64 (`tarr-updater-raspberry-pi-arm64`)
- **Raspberry Pi**: ARM32 (`tarr-updater-raspberry-pi-arm32`)
- **macOS**: x64 and ARM64 (Apple Silicon)

## Usage

### Basic Usage
```bash
# Windows
tarr-updater-windows-x64.exe

# Linux/Raspberry Pi/macOS
./tarr-updater-linux-x64
./tarr-updater-raspberry-pi-arm64
```

### Configuration

The updater creates a configuration file `updater_config.json`:

```json
{
  "current_version": "1.0.0",
  "last_check": "2024-08-25T10:30:00Z",
  "auto_update": false
}
```

### Expected GitHub Repository Structure

The updater expects this structure in the GitHub repository:

```
compiled_packages/
├── tarr-annunciator-windows-x64.exe
├── tarr-annunciator-linux-x64
├── tarr-annunciator-raspberry-pi-arm64
├── tarr-annunciator-raspberry-pi-arm32
├── tarr-annunciator-macos-x64
└── tarr-annunciator-macos-arm64

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
        ├── destination/
        ├── direction/
        ├── emergency/
        ├── promo/
        ├── safety/
        ├── track/
        └── train/
```

## File Mapping

The updater maps remote files to local paths:

- `data/json/*.json` → `json/*.json`
- `data/templates/*.html` → `templates/*.html` 
- `data/static/mp3/*.mp3` → `static/mp3/*.mp3`

## Building

### Prerequisites
- Go 1.21 or higher

### Build All Platforms
```bash
# Windows
build.bat

# Linux/macOS
chmod +x build.sh
./build.sh
```

### Manual Build
```bash
# Current platform
go build -o tarr-updater .

# Cross-compile for specific platform
GOOS=linux GOARCH=amd64 go build -o tarr-updater-linux-x64 .
GOOS=windows GOARCH=amd64 go build -o tarr-updater-windows-x64.exe .
GOOS=linux GOARCH=arm64 go build -o tarr-updater-raspberry-pi-arm64 .
```

## Installation

1. Download the appropriate updater for your platform from the releases
2. Place it in your TARR Annunciator directory
3. Run the updater to check for and install updates
4. The updater will automatically detect your system and download the correct files

## Error Handling

The updater includes comprehensive error handling:
- Network connectivity issues
- File permission problems
- Backup and restore on failure
- Detailed logging of all operations

## Security

- Uses HTTPS for all downloads
- Verifies file sizes before replacement
- Creates backups before any changes
- Uses temporary files to prevent corruption

## Integration

The updater can be integrated into the main TARR Annunciator application:
- Called from admin interface
- Scheduled automatic checks
- Status reporting back to main application

## Troubleshooting

### Common Issues

**Permission Denied**: 
- On Linux/macOS: Make sure updater is executable (`chmod +x tarr-updater-*`)
- On Windows: Run as Administrator if updating system-level installations

**Network Errors**:
- Check internet connectivity
- Verify GitHub is accessible
- Check firewall settings

**File Not Found**:
- Ensure you're running from the TARR Annunciator directory
- Verify directory structure matches expected layout

### Debug Mode

Set environment variable for detailed logging:
```bash
TARR_DEBUG=1 ./tarr-updater-linux-x64
```

## API Rate Limits

The updater respects GitHub API rate limits:
- 60 requests per hour for unauthenticated requests
- Uses conditional requests when possible
- Includes User-Agent header for proper identification

## Future Enhancements

- [ ] Cryptographic signature verification
- [ ] Delta updates for large files
- [ ] Configuration file updates
- [ ] Rollback functionality
- [ ] Update scheduling
- [ ] GUI interface