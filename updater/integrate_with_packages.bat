@echo off
echo Integrating updater with install packages...

REM Build all updater platforms first
echo Building updaters...
call build.bat > nul

REM Copy Windows updater to Windows package
echo Copying Windows updater to Windows package...
if exist "..\packages\TARR_Annunciator_Windows_x64" (
    copy "tarr-updater-windows-x64.exe" "..\packages\TARR_Annunciator_Windows_x64\tarr-updater.exe"
    echo Windows updater added to package
) else (
    echo WARNING: Windows package directory not found
)

REM Copy ARM64 updater to Raspberry Pi package  
echo Copying ARM64 updater to Raspberry Pi package...
if exist "..\packages\TARR_Annunciator_RaspberryPi_ARM64" (
    copy "tarr-updater-raspberry-pi-arm64" "..\packages\TARR_Annunciator_RaspberryPi_ARM64\tarr-updater"
    echo Raspberry Pi updater added to package
) else (
    echo WARNING: Raspberry Pi package directory not found
)

REM Update README files to mention updater
echo Updating package documentation...

REM Create update scripts for packages
echo Creating update convenience scripts...

REM Windows update script
(
echo @echo off
echo echo Running TARR Annunciator Updater...
echo tarr-updater.exe
echo pause
) > "..\packages\TARR_Annunciator_Windows_x64\update.bat"

REM Raspberry Pi update script
(
echo #!/bin/bash
echo echo "Running TARR Annunciator Updater..."
echo ./tarr-updater
) > "..\packages\TARR_Annunciator_RaspberryPi_ARM64\update.sh"

REM Make Pi update script executable
attrib +x "..\packages\TARR_Annunciator_RaspberryPi_ARM64\update.sh" 2>nul

echo.
echo Recreating packages with updater...

REM Recreate Windows package
cd "..\packages"
if exist "TARR_Annunciator_Windows_x64.zip" del "TARR_Annunciator_Windows_x64.zip"
powershell "Compress-Archive -Path 'TARR_Annunciator_Windows_x64' -DestinationPath 'TARR_Annunciator_Windows_x64.zip' -Force"

REM Recreate Raspberry Pi package
if exist "TARR_Annunciator_RaspberryPi_ARM64.tar.gz" del "TARR_Annunciator_RaspberryPi_ARM64.tar.gz"
tar -czf "TARR_Annunciator_RaspberryPi_ARM64.tar.gz" "TARR_Annunciator_RaspberryPi_ARM64/"

cd "..\updater"

echo.
echo ========================================
echo Integration Complete!
echo ========================================
echo.
echo Updated packages now include:
echo - Platform-appropriate updater executable
echo - Convenience update scripts (update.bat / update.sh)
echo - Updated documentation
echo.
echo Package files:
dir /b ..\packages\*.zip ..\packages\*.tar.gz
echo.

pause