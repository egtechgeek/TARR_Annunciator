# TARR Annunciator - Raspberry Pi Guide

Complete guide for running TARR Annunciator on Raspberry Pi devices with full audio system compatibility.

## 🍓 Raspberry Pi Compatibility

### ✅ Supported Models
- **Raspberry Pi 5** (ARM64) - Recommended
- **Raspberry Pi 4** (ARM64) - Recommended  
- **Raspberry Pi 3B/3B+** (ARM32) - Supported
- **Raspberry Pi 2** (ARM32) - Supported
- **Raspberry Pi Zero 2W** (ARM64) - Supported
- **Raspberry Pi Zero/Zero W** (ARMv6) - Supported
- **Raspberry Pi 1** (ARMv6) - Supported

### 📊 Performance Recommendations
- **Best**: Pi 5, Pi 4 (4GB+ RAM recommended)
- **Good**: Pi 3B+, Pi Zero 2W
- **Basic**: Pi 3B, Pi 2, Pi Zero/1 (adequate for basic usage)

## 🎵 Audio System Support

### Raspberry Pi Audio Hardware
- **3.5mm Headphone Jack** - Analog stereo output
- **HDMI Audio** - Digital audio via HDMI
- **USB Audio Devices** - External USB sound cards
- **I2S HAT Devices** - Advanced audio HATs (HiFiBerry, etc.)

### Software Audio Systems
1. **ALSA** (Always available)
   - Direct hardware access
   - Low latency
   - Manual configuration required

2. **PulseAudio** (Recommended)
   - Device switching support
   - Better application compatibility
   - Automatic device management

## 🚀 Quick Installation

### One-Line Install
```bash
# Download and run the installer
curl -fsSL https://raw.githubusercontent.com/your-repo/tarr-annunciator/main/install_raspberry_pi.sh | bash
```

### Manual Installation
```bash
# 1. Clone or download the project
git clone <repository-url>
cd tarr-annunciator

# 2. Make scripts executable
chmod +x install_raspberry_pi.sh run_raspberry_pi.sh

# 3. Run the installer
./install_raspberry_pi.sh

# 4. Start the application
./run_raspberry_pi.sh
```

## 🔧 Detailed Setup Guide

### Step 1: System Preparation

```bash
# Update your Raspberry Pi
sudo apt update && sudo apt upgrade -y

# Install required packages
sudo apt install -y git curl wget build-essential
```

### Step 2: Audio System Setup

#### Enable Raspberry Pi Audio
```bash
# Edit config file
sudo nano /boot/config.txt

# Add this line if not present:
dtparam=audio=on

# Reboot to apply changes
sudo reboot
```

#### Install Audio Utilities
```bash
# ALSA utilities (essential)
sudo apt install -y alsa-utils

# PulseAudio (recommended)
sudo apt install -y pulseaudio pulseaudio-utils
```

#### Configure Audio Output
```bash
# Set audio output mode
amixer cset numid=3 0   # Auto (default)
amixer cset numid=3 1   # Headphone/Analog
amixer cset numid=3 2   # HDMI

# Test audio
speaker-test -t sine -f 1000 -c 2
```

### Step 3: Go Installation

The installer script will automatically install Go, but for manual installation:

```bash
# For Raspberry Pi 4/5 (ARM64)
wget https://golang.org/dl/go1.21.6.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-arm64.tar.gz

# For older Pi models (ARM32)
wget https://golang.org/dl/go1.21.6.linux-armv6l.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-armv6l.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### Step 4: Build Application

```bash
# Clone project
git clone <repository-url>
cd tarr-annunciator

# Build for your Pi model
make build-raspberry-pi        # Pi 4/5 (ARM64)
make build-raspberry-pi-32     # Pi 2/3 (ARM32)  
make build-raspberry-pi-zero   # Pi Zero/1 (ARMv6)

# Or use automatic detection
./install_raspberry_pi.sh
```

## 🎛️ Audio Configuration

### Raspberry Pi Audio Output Selection

The application automatically detects and configures Raspberry Pi audio devices:

#### Via Web Interface
1. Go to `http://your-pi-ip:8080/admin`
2. Navigate to **Audio Controls** section
3. Select from available devices:
   - Raspberry Pi Headphone/Analog Audio
   - Raspberry Pi HDMI Audio
   - External USB devices

#### Via Command Line
```bash
# List available devices
aplay -l

# Set output via amixer
amixer cset numid=3 0   # Auto-select
amixer cset numid=3 1   # Force analog/headphone
amixer cset numid=3 2   # Force HDMI

# PulseAudio device switching
pactl list short sinks
pactl set-default-sink <sink-name>
```

### Audio Troubleshooting

#### No Audio Devices
```bash
# Check if audio is enabled
grep "dtparam=audio" /boot/config.txt

# Load audio module manually
sudo modprobe snd_bcm2835

# Check loaded modules
lsmod | grep snd
```

#### HDMI Audio Issues
```bash
# Force HDMI audio in config.txt
echo "hdmi_force_audio=1" | sudo tee -a /boot/config.txt

# Set HDMI as default
amixer cset numid=3 2
```

#### PulseAudio Issues
```bash
# Restart PulseAudio
pulseaudio --kill
pulseaudio --start

# Check PulseAudio status
pactl info
```

## 🌐 Network Configuration

### Access Points
- **Web Interface**: `http://raspberry-pi-ip:8080`
- **Admin Panel**: `http://raspberry-pi-ip:8080/admin`
- **API**: `http://raspberry-pi-ip:8080/api/status`

### Find Your Pi's IP Address
```bash
hostname -I
# or
ip addr show wlan0
```

### Remote Access Setup
```bash
# Enable SSH (if needed)
sudo systemctl enable ssh
sudo systemctl start ssh

# Connect from another computer
ssh pi@raspberry-pi-ip
```

## 🔄 System Service Setup

### Create Systemd Service
```bash
sudo nano /etc/systemd/system/tarr-annunciator.service
```

```ini
[Unit]
Description=TARR Annunciator Train Announcement System
After=network.target sound.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/tarr-annunciator
ExecStart=/home/pi/tarr-annunciator/tarr-annunciator
Restart=always
RestartSec=5
Environment=PULSE_SERVER=unix:/run/user/1000/pulse/native

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable tarr-annunciator
sudo systemctl start tarr-annunciator

# Check status
sudo systemctl status tarr-annunciator

# View logs
journalctl -u tarr-annunciator -f
```

## 📱 Platform-Specific Features

### Raspberry Pi Detection
The application automatically detects Raspberry Pi hardware and provides:

- **Model Detection**: Identifies specific Pi model
- **Audio Configuration**: Pi-specific audio setup
- **Performance Optimization**: Tailored for ARM architecture
- **Hardware Integration**: Direct BCM2835 audio support

### API Endpoints for Pi Information
```bash
# Get platform information
curl http://localhost:8080/api/platform

# Response includes:
{
  "platform_info": {
    "platform": "linux",
    "arch": "arm64",
    "is_arm": true,
    "is_raspberry_pi": true,
    "pi_model": "Raspberry Pi 4 Model B Rev 1.4",
    "pi_audio_config": {
      "output": "auto",
      "config_enabled": true
    },
    "pi_audio_enabled": true,
    "pi_hdmi_audio": true,
    "pi_headphone_audio": true
  }
}
```

## ⚡ Performance Optimization

### Memory Configuration
```bash
# For Pi with 1GB RAM or less, increase GPU memory split
echo "gpu_mem=128" | sudo tee -a /boot/config.txt

# For audio-focused usage
echo "gpu_mem=64" | sudo tee -a /boot/config.txt
```

### CPU Governor
```bash
# Set performance mode for better audio
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor
```

### Audio Buffer Optimization
The Go audio backend (faiface/beep) is optimized for Raspberry Pi with:
- **Low latency buffers** for responsive audio
- **ARM-optimized compilation** for better performance
- **Minimal CPU usage** for background operation

## 🔍 Monitoring and Logs

### Application Logs
```bash
# If running as service
journalctl -u tarr-annunciator -f

# If running manually
./run_raspberry_pi.sh 2>&1 | tee logs/app.log
```

### System Monitoring
```bash
# CPU and memory usage
htop

# Audio system status
cat /proc/asound/cards

# Temperature monitoring
vcgencmd measure_temp
```

## 🛠️ Advanced Configuration

### Custom Audio HATs
For users with advanced audio HATs (HiFiBerry, etc.):

```bash
# Edit config.txt for your HAT
sudo nano /boot/config.txt

# Example for HiFiBerry DAC+
dtoverlay=hifiberry-dacplus

# Reboot and verify
sudo reboot
aplay -l
```

### Headless Operation
```bash
# Auto-start on boot
sudo systemctl enable tarr-annunciator

# Remote management via SSH
ssh pi@raspberry-pi-ip

# Web-based management
# Access admin panel remotely at http://pi-ip:8080/admin
```

### Multiple Pi Network
For multiple Raspberry Pi installations:

```bash
# Change port on additional Pi units
export PORT=8081  # or modify main.go
./tarr-annunciator

# Access at http://pi-ip:8081
```

## 📋 Troubleshooting Guide

### Common Issues

#### Build Failures
```bash
# Check Go installation
go version

# Verify architecture detection
uname -m

# Clean build
make clean && make build-raspberry-pi
```

#### Audio Not Working
```bash
# Check audio devices
aplay -l

# Verify audio enabled
cat /boot/config.txt | grep audio

# Test with system sounds
sudo apt install sound-theme-freedesktop
paplay /usr/share/sounds/alsa/Front_Left.wav
```

#### Network Issues
```bash
# Check if service is listening
netstat -tlnp | grep 8080

# Check firewall (if enabled)
sudo ufw status

# Test local connectivity
curl http://localhost:8080/api/status
```

#### Performance Issues
```bash
# Check system resources
free -h
df -h

# Monitor CPU usage
top -p $(pgrep tarr-annunciator)

# Check audio buffer underruns
grep underrun /var/log/syslog
```

## 🎯 Use Cases

### Home Automation
- **Smart Home Integration**: Control via web API
- **Voice Assistant Integration**: Trigger announcements
- **Scheduled Announcements**: Automated daily messages

### Model Railroads
- **Scale Model Control**: Perfect for HO/N/O scale layouts
- **Realistic Sound Effects**: Authentic train station atmosphere
- **Remote Operation**: Control from smartphone/tablet

### Public Spaces
- **Small Venues**: Museums, visitor centers
- **Educational Settings**: Schools, libraries
- **Event Spaces**: Temporary announcements

## 💡 Tips and Best Practices

1. **Use a quality SD card** (Class 10 or better)
2. **Ensure stable power supply** (official Pi power adapter)
3. **Keep system updated** regularly
4. **Monitor temperature** in enclosed installations
5. **Backup your configuration** files
6. **Use wired Ethernet** for most reliable network connection
7. **Test audio setup** thoroughly before deployment

## 📞 Support

### Getting Help
- Check the logs: `journalctl -u tarr-annunciator -f`
- Test platform detection: `curl localhost:8080/api/platform`
- Verify audio: `./run_raspberry_pi.sh` (shows detailed audio check)

### Reporting Issues
Include the following information:
- Raspberry Pi model and OS version
- Output of `/api/platform` endpoint
- Audio system configuration (`aplay -l`, `pactl list sinks`)
- Error logs and symptoms

The TARR Annunciator is optimized to work seamlessly on Raspberry Pi hardware, providing a professional train announcement system that's perfect for model railroads, museums, and educational installations.