@echo off
echo Building TARR Annunciator Updater...

REM Build for Windows
echo Building Windows x64...
set GOOS=windows
set GOARCH=amd64
go build -o tarr-updater-windows-x64.exe .
if %errorlevel% neq 0 (
    echo Error: Windows build failed
    pause
    exit /b 1
)

REM Build for Linux x64
echo Building Linux x64...
set GOOS=linux
set GOARCH=amd64
go build -o tarr-updater-linux-x64 .
if %errorlevel% neq 0 (
    echo Error: Linux build failed
    pause
    exit /b 1
)

REM Build for Raspberry Pi ARM64
echo Building Raspberry Pi ARM64...
set GOOS=linux
set GOARCH=arm64
go build -o tarr-updater-raspberry-pi-arm64 .
if %errorlevel% neq 0 (
    echo Error: Raspberry Pi ARM64 build failed
    pause
    exit /b 1
)

REM Build for Raspberry Pi ARM32
echo Building Raspberry Pi ARM32...
set GOOS=linux
set GOARCH=arm
set GOARM=7
go build -o tarr-updater-raspberry-pi-arm32 .
if %errorlevel% neq 0 (
    echo Error: Raspberry Pi ARM32 build failed
    pause
    exit /b 1
)

echo.
echo All updater builds completed successfully!
echo.
echo Built files:
dir /b tarr-updater-*
echo.

pause