# TARR Annunciator Windows Installation Script (PowerShell)
# Run with: PowerShell -ExecutionPolicy Bypass -File install_windows.ps1

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "TARR Annunciator Windows Installation" -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan

# Check if Python is installed
try {
    $pythonVersion = python --version 2>$null
    Write-Host "✓ $pythonVersion found" -ForegroundColor Green
} catch {
    Write-Host "✗ Python is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Python 3.8+ from https://python.org" -ForegroundColor Yellow
    Write-Host "Make sure to check 'Add Python to PATH' during installation" -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

# Check Python version
$pythonVersionInfo = python -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}')"
$version = [version]$pythonVersionInfo
if ($version -lt [version]"3.8.0") {
    Write-Host "✗ Python version $pythonVersionInfo is too old. Please install Python 3.8 or newer." -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Check if pip is available
try {
    $pipVersion = pip --version 2>$null
    Write-Host "✓ pip is available" -ForegroundColor Green
} catch {
    Write-Host "✗ pip is not available" -ForegroundColor Red
    Write-Host "Please reinstall Python with pip included" -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host "`nCreating virtual environment..." -ForegroundColor Yellow
python -m venv tarr_env

Write-Host "Activating virtual environment..." -ForegroundColor Yellow
& ".\tarr_env\Scripts\Activate.ps1"

Write-Host "Upgrading pip..." -ForegroundColor Yellow
python -m pip install --upgrade pip

Write-Host "Installing required Python packages..." -ForegroundColor Yellow
pip install -r requirements_windows.txt

# Check for FFmpeg
try {
    $ffmpegVersion = ffmpeg -version 2>$null
    Write-Host "✓ FFmpeg is available" -ForegroundColor Green
} catch {
    Write-Host "⚠ WARNING: FFmpeg not found in PATH" -ForegroundColor Yellow
    Write-Host "pydub may not work correctly without FFmpeg" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "To install FFmpeg:" -ForegroundColor Cyan
    Write-Host "• Via Chocolatey: choco install ffmpeg" -ForegroundColor White
    Write-Host "• Via Winget: winget install FFmpeg.FFmpeg" -ForegroundColor White
    Write-Host "• Manual: Download from https://ffmpeg.org/download.html" -ForegroundColor White
    Write-Host ""
}

Write-Host ""
Write-Host "===============================================" -ForegroundColor Green
Write-Host "Installation Complete!" -ForegroundColor Green
Write-Host "===============================================" -ForegroundColor Green
Write-Host ""
Write-Host "To run the application:" -ForegroundColor Cyan
Write-Host "1. Open PowerShell in the application directory" -ForegroundColor White
Write-Host "2. Run: .\tarr_env\Scripts\Activate.ps1" -ForegroundColor White
Write-Host "3. Run: python app_windows.py" -ForegroundColor White
Write-Host ""
Write-Host "Or use the provided run_windows.bat script" -ForegroundColor White
Write-Host ""
Write-Host "The web interface will be available at:" -ForegroundColor Cyan
Write-Host "http://localhost:8080" -ForegroundColor White
Write-Host ""
Write-Host "Admin interface at:" -ForegroundColor Cyan
Write-Host "http://localhost:8080/admin" -ForegroundColor White
Write-Host ""

# Test audio system
Write-Host "Testing audio system..." -ForegroundColor Yellow
if (Test-Path "static\mp3\chime.mp3") {
    Write-Host "✓ Chime file found. You can test audio with: python app_windows.py --test-audio" -ForegroundColor Green
} else {
    Write-Host "⚠ Chime file not found. Please ensure all MP3 files are in place." -ForegroundColor Yellow
}

Read-Host "`nPress Enter to exit"
