@echo off
REM TARR Annunciator Windows Run Script

echo Starting TARR Annunciator...

REM Check if virtual environment exists
if not exist "tarr_env\Scripts\activate.bat" (
    echo Virtual environment not found. Please run install_windows.bat first.
    pause
    exit /b 1
)

REM Activate virtual environment
call tarr_env\Scripts\activate.bat

REM Run the application
python app.py

pause
