#!/bin/bash
#
# TARR Annunciator - one-click Raspberry Pi installer
#
# README (on the Pi, as user pi - not root):
#   curl -fsSL https://github.com/egtechgeek/TARR_Annunciator/releases/download/one-click-installer/install_raspberry_pi.sh | bash
#
# This script is hosted on the evergreen GitHub Release tag "one-click-installer".
# It then downloads the newest semver thin package from /releases/latest
# (TARR_Annunciator_Pi_arm64_*.tar.gz) and installs into:
#   ~/TARR_Annunciator_RaspberryPi_ARM64
#
# Optional:
#   GITHUB_TOKEN=ghp_...   # private repo / higher API rate limits
#   TARR_INSTALL_DIR=...   # override install path
#   --skip-download        # only configure Screen helpers using existing files
#
set -euo pipefail

echo "=========================================="
echo "TARR Annunciator - Raspberry Pi Installer"
echo "One-click: deps + latest GitHub thin package"
echo "=========================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

print_status()  { echo -e "${GREEN}✓${NC} $1"; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
print_error()   { echo -e "${RED}✗${NC} $1"; }
print_info()    { echo -e "${BLUE}ℹ${NC} $1"; }
print_feature() { echo -e "${PURPLE}★${NC} $1"; }

# --- Config ---
GITHUB_OWNER="${GITHUB_OWNER:-egtechgeek}"
GITHUB_REPO="${GITHUB_REPO:-TARR_Annunciator}"
ASSET_PREFIX="TARR_Annunciator_Pi_arm64_"
DEFAULT_INSTALL_NAME="TARR_Annunciator_RaspberryPi_ARM64"

REBOOT_REQUIRED=false
PI_MODEL=""
ARCH=""
AUDIO_SYSTEM=""
PIPEWIRE_INSTALLED=false
BLUETOOTH_CONFIGURED=false
SKIP_DOWNLOAD=false
RELEASE_TAG=""
RELEASE_ASSET=""
INSTALLED_APP_VERSION=""

for arg in "$@"; do
    case "$arg" in
        --skip-download) SKIP_DOWNLOAD=true ;;
        -h|--help)
            sed -n '2,16p' "$0"
            exit 0
            ;;
    esac
done

# Never install as root — Screen/audio must run as the console user (usually pi)
if [ "${EUID}" -eq 0 ]; then
    print_error "Do not run this installer as root/sudo."
    print_info "Run as the Pi user, e.g.:  ./install_raspberry_pi.sh"
    print_info "The script will prompt for sudo only when needed."
    exit 1
fi

CURRENT_USER="$(whoami)"
HOME_DIR="${HOME:-/home/$CURRENT_USER}"
INSTALL_DIR="${TARR_INSTALL_DIR:-$HOME_DIR/$DEFAULT_INSTALL_NAME}"

# --- Platform checks ---
print_info "Detecting Raspberry Pi..."
if [[ -f "/sys/firmware/devicetree/base/model" ]] && grep -q "Raspberry Pi" "/sys/firmware/devicetree/base/model" 2>/dev/null; then
    PI_MODEL=$(tr -d '\0' < /sys/firmware/devicetree/base/model)
    print_status "Detected: $PI_MODEL"
elif grep -q "BCM" /proc/cpuinfo 2>/dev/null; then
    PI_MODEL="Raspberry Pi (detected via /proc/cpuinfo)"
    print_status "Detected: $PI_MODEL"
else
    print_warning "Raspberry Pi not detected, checking for OrangePi..."
    if grep -qE "allwinner|sun8i|sun50i" /proc/cpuinfo 2>/dev/null; then
        PI_MODEL="OrangePi (detected via /proc/cpuinfo)"
        print_status "Detected: $PI_MODEL"
    else
        print_warning "ARM board not detected; installation will continue..."
        PI_MODEL="Unknown (non-Pi system)"
    fi
fi

ARCH="$(uname -m)"
print_info "Architecture: $ARCH"
case "$ARCH" in
    aarch64|arm64) ;;
    *)
        print_error "This installer expects linux/arm64 (aarch64). Found: $ARCH"
        print_info "Thin GitHub releases are built for Raspberry Pi 64-bit OS only."
        exit 1
        ;;
esac

if command -v lsb_release &>/dev/null; then
    print_info "OS: $(lsb_release -d | cut -f2-)"
fi

print_info "Install directory: $INSTALL_DIR"
print_info "GitHub: ${GITHUB_OWNER}/${GITHUB_REPO}"

echo ""
echo "=== Features ==="
print_feature "Downloads latest TARR_Annunciator_Pi_arm64_*.tar.gz from GitHub Releases"
print_feature "Installs to fixed path (keeps start/stop script paths stable)"
print_feature "GNU Screen auto-start + helper scripts in ~"
print_feature "Modern audio (ALSA / PipeWire / PulseAudio)"

# --- Bootstrap packages needed before download ---
echo ""
echo "=== System package bootstrap ==="
print_info "Updating apt package lists..."
sudo apt update

print_info "Installing curl, ca-certificates, python3, tar, gzip..."
sudo apt install -y curl ca-certificates python3 tar gzip

# Essential runtime deps early (screen needed for helpers)
if ! command -v screen >/dev/null 2>&1; then
    print_info "Installing GNU Screen..."
    sudo apt install -y screen
fi
if ! command -v mc >/dev/null 2>&1; then
    print_info "Installing Midnight Commander..."
    sudo apt install -y mc
fi

# --- Download + place latest thin release ---
download_and_install_release() {
    local api_url tmp_json tmp_dir archive_path extract_dir pkg_root meta_path got_sha

    api_url="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest"
    tmp_dir="$(mktemp -d /tmp/tarr-install-XXXXXX)"
    tmp_json="$tmp_dir/latest.json"
    trap 'rm -rf "$tmp_dir"' RETURN

    print_info "Querying GitHub Releases: $api_url"
    local curl_args=(-fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: TARR-Annunciator-Installer/1.1")
    if [[ -n "${GITHUB_TOKEN:-}" ]]; then
        curl_args+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
        print_info "Using GITHUB_TOKEN for authenticated API access"
    fi
    if ! curl "${curl_args[@]}" "$api_url" -o "$tmp_json"; then
        print_error "Failed to fetch latest release from GitHub"
        print_info "If the repo is private, set GITHUB_TOKEN and retry."
        exit 1
    fi

    # Parse tag + browser_download_url for Pi arm64 asset
    eval "$(python3 - "$tmp_json" "$ASSET_PREFIX" <<'PY'
import json, sys, shlex
path, prefix = sys.argv[1], sys.argv[2]
with open(path, encoding="utf-8") as f:
    data = json.load(f)
tag = data.get("tag_name") or ""
asset_name = ""
url = ""
for a in data.get("assets") or []:
    name = a.get("name") or ""
    if name.startswith(prefix) and name.endswith(".tar.gz"):
        asset_name = name
        url = a.get("browser_download_url") or ""
        break
if not tag or not url:
    sys.stderr.write("No matching release asset found\n")
    sys.exit(2)
print("RELEASE_TAG=" + shlex.quote(tag))
print("RELEASE_ASSET=" + shlex.quote(asset_name))
print("RELEASE_URL=" + shlex.quote(url))
PY
)"

    print_status "Latest release: $RELEASE_TAG"
    print_status "Asset: $RELEASE_ASSET"

    archive_path="$tmp_dir/$RELEASE_ASSET"
    print_info "Downloading package..."
    local dl_args=(-fL --progress-bar -H "User-Agent: TARR-Annunciator-Installer/1.1")
    if [[ -n "${GITHUB_TOKEN:-}" ]]; then
        dl_args+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
    fi
    curl "${dl_args[@]}" "$RELEASE_URL" -o "$archive_path"
    print_status "Download complete ($(du -h "$archive_path" | cut -f1))"

    extract_dir="$tmp_dir/extracted"
    mkdir -p "$extract_dir"
    print_info "Extracting archive..."
    tar -xzf "$archive_path" -C "$extract_dir"

    # Prefer single top-level directory as package root
    pkg_root="$(find "$extract_dir" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
    if [[ -z "$pkg_root" ]]; then
        pkg_root="$extract_dir"
    fi
    meta_path="$pkg_root/UPDATE_PACKAGE.json"
    if [[ ! -f "$meta_path" ]]; then
        print_error "UPDATE_PACKAGE.json missing in package"
        exit 1
    fi

    eval "$(python3 - "$meta_path" <<'PY'
import json, sys, shlex
with open(sys.argv[1], encoding="utf-8") as f:
    m = json.load(f)
bin_path = (m.get("binary") or {}).get("path") or "tarr-annunciator"
sha = (m.get("binary") or {}).get("sha256") or ""
ver = m.get("app_version") or ""
plat = m.get("platform") or ""
arch = m.get("arch") or ""
print("PKG_BIN_REL=" + shlex.quote(bin_path))
print("PKG_SHA256=" + shlex.quote(sha))
print("PKG_APP_VERSION=" + shlex.quote(ver))
print("PKG_PLATFORM=" + shlex.quote(plat))
print("PKG_ARCH=" + shlex.quote(arch))
PY
)"

    if [[ -n "$PKG_PLATFORM" && "$PKG_PLATFORM" != "linux" ]]; then
        print_error "Package platform is '$PKG_PLATFORM' (expected linux)"
        exit 1
    fi
    if [[ -n "$PKG_ARCH" && "$PKG_ARCH" != "arm64" ]]; then
        print_error "Package arch is '$PKG_ARCH' (expected arm64)"
        exit 1
    fi

    local bin_src="$pkg_root/$PKG_BIN_REL"
    if [[ ! -f "$bin_src" ]]; then
        print_error "Binary not found in package: $PKG_BIN_REL"
        exit 1
    fi

    if [[ -n "$PKG_SHA256" ]]; then
        print_info "Verifying binary sha256..."
        got_sha="$(sha256sum "$bin_src" | awk '{print $1}')"
        if [[ "${got_sha,,}" != "${PKG_SHA256,,}" ]]; then
            print_error "Binary sha256 mismatch"
            print_error "  got:  $got_sha"
            print_error "  want: $PKG_SHA256"
            exit 1
        fi
        print_status "sha256 OK"
    fi

    # Stop running instance before replacing files
    if screen -list 2>/dev/null | grep -q "tarr-annunciator"; then
        print_warning "Stopping existing screen session 'tarr-annunciator'..."
        screen -S "tarr-annunciator" -X quit 2>/dev/null || true
        sleep 2
    fi

    mkdir -p "$INSTALL_DIR"/{json,templates,static,logs}

    print_info "Installing binary..."
    cp -f "$bin_src" "$INSTALL_DIR/tarr-annunciator"
    chmod +x "$INSTALL_DIR/tarr-annunciator"

    print_info "Installing templates..."
    if [[ -d "$pkg_root/templates" ]]; then
        # Replace templates tree (HTML is versioned with the app)
        rm -rf "$INSTALL_DIR/templates"
        cp -a "$pkg_root/templates" "$INSTALL_DIR/templates"
    fi

    print_info "Installing static assets (merge)..."
    if [[ -d "$pkg_root/static" ]]; then
        cp -a "$pkg_root/static/." "$INSTALL_DIR/static/"
    fi

    print_info "Installing JSON seeds (additive — never overwrite operator secrets)..."
    if [[ -d "$pkg_root/json" ]]; then
        local src f base
        shopt -s nullglob
        for src in "$pkg_root/json"/*; do
            [[ -f "$src" ]] || continue
            base="$(basename "$src")"
            case "$base" in
                admin_config.json|audio_settings.json)
                    # Never ship/overwrite operator credentials or device prefs from seeds
                    if [[ ! -f "$INSTALL_DIR/json/$base" ]]; then
                        print_warning "Skipping seed $base (create via Admin UI after first start)"
                    else
                        print_info "Keeping existing $base"
                    fi
                    continue
                    ;;
            esac
            f="$INSTALL_DIR/json/$base"
            if [[ -f "$f" ]]; then
                print_info "Keeping existing json/$base"
            else
                cp -f "$src" "$f"
                print_status "Created json/$base"
            fi
        done
        shopt -u nullglob
    fi

    # Record installed version for Admin / updater
    local ver_file="$INSTALL_DIR/json/install_version.json"
    INSTALLED_APP_VERSION="${PKG_APP_VERSION:-${RELEASE_TAG#v}}"
    python3 - "$ver_file" "$INSTALLED_APP_VERSION" "$RELEASE_TAG" <<'PY'
import json, sys, datetime
path, ver, tag = sys.argv[1], sys.argv[2], sys.argv[3]
doc = {
    "app_version": ver.lstrip("v"),
    "installed_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "source": "install_raspberry_pi.sh:github-release:" + tag,
}
with open(path, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=4)
    f.write("\n")
PY
    print_status "Wrote install_version.json ($INSTALLED_APP_VERSION)"

    # Keep a copy of the installer next to the app for reference
    if [[ -f "$0" ]]; then
        cp -f "$0" "$INSTALL_DIR/install_raspberry_pi.sh" 2>/dev/null || true
        chmod +x "$INSTALL_DIR/install_raspberry_pi.sh" 2>/dev/null || true
    fi

    print_status "Application files installed to $INSTALL_DIR"
}

echo ""
echo "=== Application package ==="
if [[ "$SKIP_DOWNLOAD" == true ]]; then
    if [[ ! -x "$INSTALL_DIR/tarr-annunciator" ]]; then
        print_error "--skip-download set but $INSTALL_DIR/tarr-annunciator is missing"
        exit 1
    fi
    print_warning "Skipping download; using existing install at $INSTALL_DIR"
else
    download_and_install_release
fi

if [[ ! -x "$INSTALL_DIR/tarr-annunciator" ]]; then
    print_error "Executable missing after install: $INSTALL_DIR/tarr-annunciator"
    exit 1
fi
print_status "Executable ready: $INSTALL_DIR/tarr-annunciator"

# --- Audio stack ---
echo ""
echo "=== Modern Audio System Setup ==="
print_info "Detecting current audio system..."
if pgrep -f pipewire >/dev/null 2>&1; then
    AUDIO_SYSTEM="PipeWire"
    print_status "PipeWire is currently running"
elif pgrep -f pulseaudio >/dev/null 2>&1; then
    AUDIO_SYSTEM="PulseAudio"
    print_status "PulseAudio is currently running"
else
    AUDIO_SYSTEM="ALSA"
    print_info "No high-level audio system detected, defaulting to ALSA"
fi

print_info "Installing ALSA utilities and development libraries..."
sudo apt install -y alsa-utils libasound2-dev pkg-config build-essential

echo ""
print_info "PipeWire Installation Options..."
echo "PipeWire is the modern audio system (Bluetooth, low latency, Pulse compatibility)."
if command -v pipewire >/dev/null 2>&1; then
    print_status "PipeWire already installed"
    PIPEWIRE_INSTALLED=true
else
    echo -n "Install PipeWire audio system? (recommended) (Y/n): "
    read -r INSTALL_PIPEWIRE
    if [[ ! "$INSTALL_PIPEWIRE" =~ ^[Nn]$ ]]; then
        print_info "Installing PipeWire with PulseAudio compatibility..."
        sudo apt install -y \
            pipewire \
            pipewire-pulse \
            pipewire-alsa \
            pipewire-audio-client-libraries \
            wireplumber \
            pipewire-media-session- 2>/dev/null || true
        sudo apt install -y \
            pipewire-bin \
            libspa-0.2-modules \
            libspa-0.2-bluetooth 2>/dev/null || true
        print_status "PipeWire installed"
        PIPEWIRE_INSTALLED=true
        REBOOT_REQUIRED=true
        AUDIO_SYSTEM="PipeWire"
    fi
fi

echo ""
print_info "Bluetooth Audio Support..."
if ! command -v bluetoothctl >/dev/null 2>&1; then
    echo -n "Install Bluetooth support? (Y/n): "
    read -r INSTALL_BLUETOOTH
    if [[ ! "$INSTALL_BLUETOOTH" =~ ^[Nn]$ ]]; then
        print_info "Installing Bluetooth support..."
        sudo apt install -y bluetooth bluez bluez-tools bluez-alsa-utils 2>/dev/null || true
        sudo apt install -y pulseaudio-module-bluetooth libspa-0.2-bluetooth 2>/dev/null || true
        sudo systemctl enable bluetooth
        sudo systemctl start bluetooth || true
        print_status "Bluetooth support installed"
        BLUETOOTH_CONFIGURED=true
    fi
else
    print_status "Bluetooth support already available"
    BLUETOOTH_CONFIGURED=true
fi

echo ""
echo "=== Hardware-Specific Audio Configuration ==="
if [[ "$PI_MODEL" == *"Raspberry Pi"* ]]; then
    print_info "Configuring Raspberry Pi audio..."
    if ! grep -q "dtparam=audio=on" /boot/config.txt 2>/dev/null && ! grep -q "dtparam=audio=on" /boot/firmware/config.txt 2>/dev/null; then
        print_info "Enabling audio in boot configuration..."
        if [ -f "/boot/config.txt" ]; then
            echo "dtparam=audio=on" | sudo tee -a /boot/config.txt
        elif [ -f "/boot/firmware/config.txt" ]; then
            echo "dtparam=audio=on" | sudo tee -a /boot/firmware/config.txt
        fi
        print_status "Audio enabled in boot configuration"
        print_warning "System reboot required for audio changes to take effect"
        REBOOT_REQUIRED=true
    else
        print_status "Audio already enabled in boot configuration"
    fi
    if ! lsmod | grep -q snd_bcm2835; then
        print_info "Loading BCM2835 audio module..."
        sudo modprobe snd_bcm2835 2>/dev/null || print_warning "Could not load audio module (may require reboot)"
    else
        print_status "BCM2835 audio module loaded"
    fi
    print_info "Configuring default audio output..."
    amixer cset numid=3 0 2>/dev/null || print_warning "Could not set audio output (may require reboot)"
elif [[ "$PI_MODEL" == *"OrangePi"* ]]; then
    print_info "Configuring OrangePi audio..."
    sudo modprobe snd-soc-sunxi 2>/dev/null || print_info "OrangePi audio module not available"
fi

echo ""
echo "=== Audio System Testing ==="
if command -v aplay >/dev/null 2>&1; then
    if aplay -l 2>/dev/null | grep -q "card"; then
        print_status "ALSA: $(aplay -l 2>/dev/null | grep -c card) audio card(s) detected"
    else
        print_warning "ALSA: No audio cards detected"
    fi
fi
if command -v pactl >/dev/null 2>&1 && pactl info >/dev/null 2>&1; then
    SERVER_INFO=$(pactl info 2>/dev/null | grep "Server Name" | cut -d: -f2 | xargs || true)
    print_status "Pulse layer: ${SERVER_INFO:-running}"
    print_status "Audio sinks: $(pactl list short sinks 2>/dev/null | wc -l)"
fi

# --- Screen auto-start + helpers ---
echo ""
echo "==============================================="
echo "Auto-Start Configuration (GNU Screen)"
echo "==============================================="
print_info "Configuring auto-start in the user session (avoids audio permission issues)."
print_info "Fixed install path: $INSTALL_DIR"

AUTOLOGIN_ENABLED=false
if [ -f "/etc/systemd/system/getty@tty1.service.d/autologin.conf" ]; then
    if grep -q "ExecStart.*--autologin $CURRENT_USER" "/etc/systemd/system/getty@tty1.service.d/autologin.conf"; then
        AUTOLOGIN_ENABLED=true
        print_status "Autologin already enabled for user: $CURRENT_USER"
    fi
elif systemctl is-enabled "autologin@$CURRENT_USER.service" >/dev/null 2>&1; then
    AUTOLOGIN_ENABLED=true
    print_status "Autologin service already enabled for user: $CURRENT_USER"
fi

if [ "$AUTOLOGIN_ENABLED" = false ]; then
    echo ""
    print_warning "Autologin is not currently enabled."
    echo "For automatic startup after reboot, the console should auto-login as '$CURRENT_USER'."
    echo -n "Enable autologin for user '$CURRENT_USER'? (Y/n): "
    read -r ENABLE_AUTOLOGIN
    if [[ ! "$ENABLE_AUTOLOGIN" =~ ^[Nn]$ ]]; then
        print_info "Enabling autologin for user: $CURRENT_USER"
        sudo mkdir -p /etc/systemd/system/getty@tty1.service.d/
        sudo tee /etc/systemd/system/getty@tty1.service.d/autologin.conf >/dev/null << EOF
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin $CURRENT_USER --noclear %I \$TERM
EOF
        AUTOLOGIN_ENABLED=true
        REBOOT_REQUIRED=true
        print_status "Autologin configuration created"
    else
        print_warning "Autologin not enabled — use ~/tarr-start.sh after reboot"
    fi
fi

mkdir -p "$INSTALL_DIR/logs"

STARTUP_SCRIPT="$HOME_DIR/start_tarr_annunciator.sh"
cat > "$STARTUP_SCRIPT" << EOF
#!/bin/bash
# TARR Annunciator Startup Script (GNU Screen)

TARR_DIR="$INSTALL_DIR"
SCREEN_NAME="tarr-annunciator"
LOG_FILE="\$HOME/tarr-annunciator-startup.log"

log_message() {
    echo "[\$(date '+%Y-%m-%d %H:%M:%S')] \$1" >> "\$LOG_FILE"
}

log_message "=== TARR Annunciator Startup Script ==="
log_message "Starting TARR Annunciator in Screen session: \$SCREEN_NAME"

cd "\$TARR_DIR" || {
    log_message "ERROR: Could not change to directory: \$TARR_DIR"
    exit 1
}

if screen -list | grep -q "\$SCREEN_NAME"; then
    log_message "WARNING: Screen session '\$SCREEN_NAME' already exists — terminating"
    screen -S "\$SCREEN_NAME" -X quit 2>/dev/null || true
    sleep 2
fi

log_message "Waiting for audio system to initialize..."
sleep 5

log_message "Creating new screen session: \$SCREEN_NAME"
screen -dmS "\$SCREEN_NAME" bash -c "
    cd '\$TARR_DIR'
    echo 'TARR Annunciator starting in Screen session...'
    echo 'Working directory: \$(pwd)'
    echo 'Session: \$SCREEN_NAME'
    ./tarr-annunciator
"

sleep 2
if screen -list | grep -q "\$SCREEN_NAME"; then
    log_message "SUCCESS: Screen session '\$SCREEN_NAME' created"
else
    log_message "ERROR: Failed to create screen session '\$SCREEN_NAME'"
    exit 1
fi
log_message "TARR Annunciator startup completed"
EOF
chmod +x "$STARTUP_SCRIPT"
print_status "Startup script: $STARTUP_SCRIPT"

# .bashrc auto-start (console login only)
print_info "Configuring automatic startup in .bashrc..."
if [ -f "$HOME_DIR/.bashrc" ]; then
    grep -v "# TARR Annunciator Auto-Start" "$HOME_DIR/.bashrc" > "$HOME_DIR/.bashrc.tmp" || true
    grep -v "start_tarr_annunciator.sh" "$HOME_DIR/.bashrc.tmp" > "$HOME_DIR/.bashrc" || true
    rm -f "$HOME_DIR/.bashrc.tmp"
fi
cat >> "$HOME_DIR/.bashrc" << 'EOF'

# TARR Annunciator Auto-Start
if [[ -z "$SSH_CLIENT" && -z "$SSH_TTY" && "$TERM" != "screen"* ]]; then
    if [ -f "$HOME/start_tarr_annunciator.sh" ]; then
        echo "Starting TARR Annunciator..."
        "$HOME/start_tarr_annunciator.sh"
        echo "TARR Annunciator started in screen session 'tarr-annunciator'"
        echo "Use 'screen -r tarr-annunciator' to view the application"
    fi
fi
EOF
print_status "Auto-start configuration added to .bashrc"

cat > "$HOME_DIR/tarr-start.sh" << EOF
#!/bin/bash
echo "Starting TARR Annunciator..."
"$HOME_DIR/start_tarr_annunciator.sh"
echo "TARR Annunciator started in screen session 'tarr-annunciator'"
echo "Use 'screen -r tarr-annunciator' to view the application"
EOF
chmod +x "$HOME_DIR/tarr-start.sh"

cat > "$HOME_DIR/tarr-stop.sh" << 'EOF'
#!/bin/bash
echo "Stopping TARR Annunciator..."
if screen -list | grep -q "tarr-annunciator"; then
    screen -S "tarr-annunciator" -X quit
    echo "TARR Annunciator stopped"
else
    echo "TARR Annunciator is not running"
fi
EOF
chmod +x "$HOME_DIR/tarr-stop.sh"

cat > "$HOME_DIR/tarr-restart.sh" << EOF
#!/bin/bash
echo "Restarting TARR Annunciator..."
"$HOME_DIR/tarr-stop.sh"
sleep 2
"$HOME_DIR/tarr-start.sh"
EOF
chmod +x "$HOME_DIR/tarr-restart.sh"

cat > "$HOME_DIR/tarr-view.sh" << 'EOF'
#!/bin/bash
echo "Connecting to TARR Annunciator session..."
if screen -list | grep -q "tarr-annunciator"; then
    echo "Press Ctrl+A then D to detach from the session"
    screen -r tarr-annunciator
else
    echo "TARR Annunciator session is not running"
    echo "Start it with: ~/tarr-start.sh"
fi
EOF
chmod +x "$HOME_DIR/tarr-view.sh"

print_status "Helper scripts created in $HOME_DIR:"
print_info "• ~/tarr-start.sh"
print_info "• ~/tarr-stop.sh"
print_info "• ~/tarr-restart.sh"
print_info "• ~/tarr-view.sh"

# Offer to start now
echo ""
echo -n "Start TARR Annunciator now in screen? (Y/n): "
read -r START_NOW
if [[ ! "$START_NOW" =~ ^[Nn]$ ]]; then
    "$HOME_DIR/tarr-start.sh" || print_warning "Start failed — try again after reboot if audio is not ready"
fi

echo ""
echo "==============================================="
echo "Installation Complete!"
echo "==============================================="
print_status "Install path: $INSTALL_DIR"
[[ -n "$INSTALLED_APP_VERSION" ]] && print_status "App version: $INSTALLED_APP_VERSION ($RELEASE_TAG)"
print_status "Audio system: $AUDIO_SYSTEM"
print_status "PipeWire: $([ "$PIPEWIRE_INSTALLED" = true ] && echo installed || echo not installed)"
print_status "Bluetooth: $([ "$BLUETOOTH_CONFIGURED" = true ] && echo available || echo not configured)"

echo ""
echo "Web UI: http://localhost:8080"
echo "Admin:  http://localhost:8080/admin"
echo ""
echo "Screen helpers:"
echo "  ~/tarr-start.sh | ~/tarr-stop.sh | ~/tarr-restart.sh | ~/tarr-view.sh"

if [[ "$AUTOLOGIN_ENABLED" == "true" ]]; then
    print_info "Auto-start: enabled after console login/reboot"
else
    print_info "Manual start: ~/tarr-start.sh"
fi

if [[ "$REBOOT_REQUIRED" == "true" ]]; then
    echo ""
    print_warning "A reboot is recommended (audio / autologin changes)."
    echo -n "Reboot now? (y/N): "
    read -r REBOOT_NOW
    if [[ "$REBOOT_NOW" =~ ^[Yy]$ ]]; then
        print_info "Rebooting..."
        sudo reboot
    else
        print_warning "Remember to reboot later, then verify with: screen -list"
    fi
fi

echo ""
print_status "Done. Happy announcing!"
