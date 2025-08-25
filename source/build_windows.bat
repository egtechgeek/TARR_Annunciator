@echo off
echo TARR Annunciator Windows Package Verification...

REM Check if pre-compiled executable exists
if not exist "tarr-annunciator.exe" (
    echo Error: tarr-annunciator.exe not found
    echo This package should include the pre-compiled Windows executable
    echo Please ensure you have the complete installation package
    pause
    exit /b 1
)

echo Found: tarr-annunciator.exe
echo.

REM Check required directories
echo Verifying package structure...

if not exist "json" (
    echo Error: json directory not found
    echo Please ensure the complete installation package is extracted
    pause
    exit /b 1
)
echo Found: json directory

if not exist "templates" (
    echo Error: templates directory not found  
    echo Please ensure the complete installation package is extracted
    pause
    exit /b 1
)
echo Found: templates directory

if not exist "static" (
    echo Error: static directory not found
    echo Please ensure the complete installation package is extracted
    pause
    exit /b 1
)
echo Found: static directory

if not exist "logs" mkdir logs
echo Verified: logs directory

echo.
echo Package verification completed successfully!
echo All required files and directories are present.
echo.
echo The TARR Annunciator is ready to run.
echo Use run_windows_go.bat to start the application.
echo Or run install_windows.bat for full installation with service setup.

pause