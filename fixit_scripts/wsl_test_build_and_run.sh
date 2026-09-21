#!/usr/bin/env bash
set -euo pipefail

REPO="/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator"
SRC="$REPO/source"
DATA="$REPO/data"
BIN="$DATA/tarr-annunciator"
ALSA_ROOT="${HOME}/.local/share/tarr-alsa-amd64"

echo "==> Preparing user-local ALSA (amd64) for CGO"
rm -rf "$ALSA_ROOT"
mkdir -p "$ALSA_ROOT/debs" "$ALSA_ROOT/root"
cd "$ALSA_ROOT/debs"
apt-get download libasound2-dev:amd64 libasound2t64:amd64 libasound2-data
for f in *.deb; do
  dpkg-deb -x "$f" "$ALSA_ROOT/root"
done

export PKG_CONFIG_PATH="$ALSA_ROOT/root/usr/lib/x86_64-linux-gnu/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
export CGO_CFLAGS="-I$ALSA_ROOT/root/usr/include"
export CGO_LDFLAGS="-L$ALSA_ROOT/root/usr/lib/x86_64-linux-gnu -lasound"
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64

pkg-config --exists alsa
echo "    alsa cflags: $(pkg-config --cflags alsa)"
echo "    alsa libs:   $(pkg-config --libs alsa)"

echo "==> Building native WSL binary → $BIN"
cd "$SRC"
go mod download
# AppVersion comes from source/version.go (currently 1.1.2). Do not append -wsl-test.
go build -o "$BIN" .
chmod +x "$BIN"
file "$BIN"
ls -la "$BIN"

echo "==> Stopping any previous test instance"
pkill -f "$BIN" 2>/dev/null || true
sleep 1

echo "==> Starting from data/ (cwd=$DATA)"
cd "$DATA"
mkdir -p logs xml
# Prefer Pulse if present; ALSA still needed at link time
export LD_LIBRARY_PATH="$ALSA_ROOT/root/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

nohup "$BIN" > "$DATA/logs/wsl_console.log" 2>&1 &
echo $! > "$DATA/logs/wsl_pid.txt"
sleep 2
PID=$(cat "$DATA/logs/wsl_pid.txt")
if kill -0 "$PID" 2>/dev/null; then
  echo "STARTED pid=$PID"
else
  echo "FAILED to stay up — console log:"
  tail -50 "$DATA/logs/wsl_console.log" || true
  exit 1
fi

echo "==> Recent console:"
tail -40 "$DATA/logs/wsl_console.log"
echo ""
echo "Open Admin UI at: http://127.0.0.1:8080/admin  (or the port logged above)"
echo "Console log: $DATA/logs/wsl_console.log"
