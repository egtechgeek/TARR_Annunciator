# TARR Annunciator - Go Version

This is a Go port of the TARR Annunciator train announcement system. The application provides a web interface and REST API for triggering train station announcements, safety announcements, and promotional messages with scheduled automation.

## Features

- **Web Interface**: Easy-to-use web UI for manual announcements
- **REST API**: Full API for external integrations and automation
- **Scheduled Announcements**: Cron-based automatic announcements
- **Audio Playback**: High-quality audio playback using the Beep library
- **Admin Panel**: Protected admin interface for configuration
- **Volume Control**: Adjustable audio volume
- **Multi-language Safety**: Safety announcements in multiple languages

## Quick Start

### Prerequisites

- **Go 1.21 or later**: Download from https://golang.org/dl/
- **Audio files**: MP3 files in the `static/mp3` directory structure
- **Configuration files**: JSON configuration files in the `json` directory

### Installation & Setup

1. **Build the application**:
   ```batch
   build_windows.bat
   ```

2. **Run the server**:
   ```batch
   run_windows_go.bat
   ```

3. **Access the application**:
   - Main Interface: http://localhost:8080
   - Admin Panel: http://localhost:8080/admin (admin/tarr2025)
   - API Documentation: http://localhost:8080/api/docs

### Directory Structure

```
TARR_Annunciator/
├── main.go                 # Main application entry point
├── api.go                  # REST API handlers
├── audio.go                # Audio playback functions
├── utils.go                # Utility functions and scheduler
├── go.mod                  # Go module definition
├── build_windows.bat       # Build script
├── run_windows_go.bat      # Run script
├── api_test_go.go          # API test suite
├── json/                   # Configuration files
│   ├── trains.json
│   ├── directions.json
│   ├── destinations.json
│   ├── tracks.json
│   ├── promo.json
│   ├── safety.json
│   └── cron.json
├── static/mp3/             # Audio files
│   ├── chime.mp3
│   ├── train/              # Train number audio files
│   ├── direction/          # Direction audio files
│   ├── destination/        # Destination audio files
│   ├── track/              # Track number audio files
│   ├── promo/              # Promotional audio files
│   └── safety/             # Safety announcement audio files
└── templates/              # HTML templates
    ├── index.html
    ├── admin.html
    ├── admin_login.html
    └── api_docs.html
```

## Configuration

### JSON Configuration Files

The application uses JSON files in the `json/` directory for configuration:

- **trains.json**: Available train numbers
- **directions.json**: Direction options (eastbound/westbound)
- **destinations.json**: Station destinations
- **tracks.json**: Track numbers
- **promo.json**: Available promotional announcements
- **safety.json**: Safety announcement languages
- **cron.json**: Scheduled announcement configuration

### Scheduled Announcements

Edit `json/cron.json` to configure automatic announcements using cron expressions:

```json
{
  "station_announcements": [
    {
      "enabled": true,
      "cron": "0 8 * * 1-5",
      "train_number": "1",
      "direction": "westbound",
      "destination": "goodwin_station",
      "track_number": "1"
    }
  ],
  "promo_announcements": [
    {
      "enabled": true,
      "cron": "30 */2 * * *",
      "file": "promo_english"
    }
  ],
  "safety_announcements": [
    {
      "enabled": true,
      "cron": "0 */4 * * *",
      "language": "english"
    }
  ]
}
```

## API Usage

### Authentication

Most API endpoints require authentication using the API key `tarr-api-2025`. Include it in:
- Header: `X-API-Key: tarr-api-2025`
- Query parameter: `?api_key=tarr-api-2025`
- Form parameter: `api_key=tarr-api-2025`

### Example API Calls

**Station Announcement**:
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

**Safety Announcement**:
```bash
curl -X POST http://localhost:8080/api/announce/safety \
  -H "X-API-Key: tarr-api-2025" \
  -H "Content-Type: application/json" \
  -d '{"language": "english"}'
```

**Volume Control**:
```bash
curl -X POST http://localhost:8080/api/audio/volume \
  -H "X-API-Key: tarr-api-2025" \
  -H "Content-Type: application/json" \
  -d '{"volume": 75}'
```

## Testing

Run the API test suite to verify functionality:

```bash
go run api_test_go.go
```

Or build and run:
```bash
go build -o api_test.exe api_test_go.go
api_test.exe
```

## Differences from Python Version

### Libraries Used
- **Web Framework**: Gin instead of Flask
- **Audio**: Beep instead of Pygame
- **Cron Scheduler**: robfig/cron instead of APScheduler
- **Sessions**: Gin sessions instead of Flask sessions

### Improvements
- **Better Performance**: Go's compiled nature provides better performance
- **Lower Memory Usage**: More efficient memory utilization
- **Faster Startup**: Quicker application startup time
- **Better Concurrency**: Native goroutine support for audio playback
- **Single Executable**: No need for Python runtime installation

### Audio System
- Uses the Beep audio library for MP3 playback
- Supports volume control and multiple audio formats
- Non-blocking audio playback using goroutines
- Better audio timing and synchronization

## Security Notes

**Important**: Change default credentials before production use:
- Admin username: `admin` 
- Admin password: `tarr2025`
- API key: `tarr-api-2025`

Update these values in `main.go`:
```go
Config: &Config{
    AdminUsername: "your_admin_username",
    AdminPassword: "your_secure_password", 
    APIKey:        "your-secure-api-key",
    // ...
}
```

## Troubleshooting

### Audio Issues
- Ensure MP3 files exist in correct directory structure
- Check file permissions
- Verify audio drivers are installed

### Build Issues
- Ensure Go 1.21+ is installed
- Run `go mod download` to fetch dependencies
- Check network connectivity for dependency downloads

### Runtime Issues
- Verify all required directories exist (json/, static/, templates/)
- Check JSON configuration file syntax
- Ensure port 8080 is not in use

## Development

### Building from Source
```bash
go mod download
go build -o tarr-annunciator.exe .
```

### Running Tests
```bash
go test ./...
```

### Adding New Features
1. Follow Go conventions and existing code patterns
2. Add appropriate error handling
3. Update API documentation if adding new endpoints
4. Test thoroughly before deployment

## License

Same as the original Python version.