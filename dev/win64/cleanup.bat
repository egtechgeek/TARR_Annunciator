@echo off
REM TARR Annunciator Directory Cleanup Script
REM This removes files that are no longer needed after successful setup

echo ===============================================
echo TARR Annunciator Directory Cleanup
echo ===============================================
echo.
echo This will remove the following unnecessary files:
echo - Old application versions (app_windows.py, app_windows_fixed.py)
echo - Diagnostic tools (check_config.py, test_*.py)
echo - Old fix scripts (fix_audio*.bat)
echo - Debug scripts (run_windows_debug.bat)
echo - Old templates (admin.html, index.html, admin_fixed.html)
echo - Duplicate requirements file
echo.
echo The following will be KEPT:
echo - app_pygame.py (main working application)
echo - install_windows.bat/ps1 (installation scripts)
echo - run_windows.bat (run script)
echo - requirements_windows.txt (dependencies)
echo - README_Windows.md (documentation)
echo - templates/index_fixed.html and admin_fixed2.html (working templates)
echo - json/, static/, tarr_env/, logs/ directories
echo.

set /p confirm="Continue with cleanup? (y/N): "
if /i "%confirm%" NEQ "y" (
    echo Cleanup cancelled.
    pause
    exit /b 0
)

echo.
echo Starting cleanup...

REM Remove old application files
if exist "app_windows.py" (
    del "app_windows.py"
    echo ✓ Removed app_windows.py
)

if exist "app_windows_fixed.py" (
    del "app_windows_fixed.py" 
    echo ✓ Removed app_windows_fixed.py
)

REM Remove diagnostic tools
if exist "check_config.py" (
    del "check_config.py"
    echo ✓ Removed check_config.py
)

if exist "test_setup.py" (
    del "test_setup.py"
    echo ✓ Removed test_setup.py
)

if exist "test_venv.py" (
    del "test_venv.py"
    echo ✓ Removed test_venv.py
)

REM Remove old fix scripts
if exist "fix_audio.bat" (
    del "fix_audio.bat"
    echo ✓ Removed fix_audio.bat
)

if exist "fix_audio_v2.bat" (
    del "fix_audio_v2.bat"
    echo ✓ Removed fix_audio_v2.bat
)

REM Remove duplicate requirements
if exist "requirements_windows_fixed.txt" (
    del "requirements_windows_fixed.txt"
    echo ✓ Removed requirements_windows_fixed.txt
)

REM Remove debug script
if exist "run_windows_debug.bat" (
    del "run_windows_debug.bat"
    echo ✓ Removed run_windows_debug.bat
)

REM Remove old templates
if exist "templates\admin.html" (
    del "templates\admin.html"
    echo ✓ Removed templates\admin.html
)

if exist "templates\admin_fixed.html" (
    del "templates\admin_fixed.html"
    echo ✓ Removed templates\admin_fixed.html
)

if exist "templates\index.html" (
    del "templates\index.html"
    echo ✓ Removed templates\index.html
)

echo.
echo ===============================================
echo Cleanup Complete!
echo ===============================================
echo.
echo Remaining files:
dir /b *.py *.bat *.ps1 *.txt *.md 2>nul
echo.
echo Remaining templates:
dir /b templates\*.html 2>nul
echo.
echo Your working TARR Annunciator is ready to use!
echo Run: run_windows.bat
echo.
pause
