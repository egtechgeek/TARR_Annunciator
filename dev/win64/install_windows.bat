@echo off
REM TARR Annunciator Windows Installation Script
REM Run this as Administrator for best results

echo ===============================================
echo TARR Annunciator Windows Installation
echo ===============================================

REM Check if Python is installed
python --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Python is not installed or not in PATH
    echo Please install Python 3.8+ from https://python.org
    echo Make sure to check "Add Python to PATH" during installation
    pause
    exit /b 1
)

echo Python found. Checking version...
python -c "import sys; print(f'Python {sys.version}')"

REM Check if pip is available
pip --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: pip is not available
    echo Please reinstall Python with pip included
    pause
    exit /b 1
)

echo Creating virtual environment...
python -m venv tarr_env

echo Activating virtual environment...
call tarr_env\Scripts\activate.bat

echo Upgrading pip...
python -m pip install --upgrade pip

echo Installing required Python packages...
pip install flask
pip install pydub
pip install apscheduler
pip install requests

echo Checking for FFmpeg (required for pydub audio processing)...
ffmpeg -version >nul 2>&1
if errorlevel 1 (
    echo WARNING: FFmpeg not found in PATH
    echo pydub may not work correctly without FFmpeg
    echo.
    echo To install FFmpeg:
    echo 1. Download from https://ffmpeg.org/download.html
    echo 2. Extract to a folder like C:\ffmpeg
    echo 3. Add C:\ffmpeg\bin to your system PATH
    echo.
    echo Alternatively, you can install via chocolatey: choco install ffmpeg
    echo Or via winget: winget install FFmpeg.FFmpeg
    echo.
)

echo.
echo ===============================================
echo Installation Complete!
echo ===============================================
echo.
echo To run the application:
echo 1. Open Command Prompt in the application directory
echo 2. Run: tarr_env\Scripts\activate.bat
echo 3. Run: python app_windows.py
echo.
echo Or use the provided run_windows.bat script
echo.
echo The web interface will be available at:
echo http://localhost:8080
echo.
echo Admin interface at:
echo http://localhost:8080/admin
echo.
pause
