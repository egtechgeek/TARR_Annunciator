#!/usr/bin/env bash
set -euo pipefail

REPO="/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator"
DATA="$REPO/data"
BIN="$DATA/tarr-annunciator"
ALSA_ROOT="${HOME}/.local/share/tarr-alsa-amd64"

if [[ ! -x "$BIN" ]]; then
  echo "Binary missing: $BIN"
  exit 1
fi

echo "==> Stopping any previous test instance"
pkill -f "$BIN" 2>/dev/null || true
sleep 1

echo "==> Starting from data/ (cwd=$DATA)"
cd "$DATA"
mkdir -p logs xml
export LD_LIBRARY_PATH="$ALSA_ROOT/root/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

: > "$DATA/logs/wsl_console.log"
nohup "$BIN" > "$DATA/logs/wsl_console.log" 2>&1 &
echo $! > "$DATA/logs/wsl_pid.txt"
sleep 2
PID=$(cat "$DATA/logs/wsl_pid.txt")
if kill -0 "$PID" 2>/dev/null; then
  echo "STARTED pid=$PID"
  tail -n 25 "$DATA/logs/wsl_console.log"
else
  echo "FAILED to stay up — console log:"
  tail -n 50 "$DATA/logs/wsl_console.log" || true
  exit 1
fi

echo ""
echo "Open Admin UI at: http://127.0.0.1:8080/admin"
echo "Hard-refresh the browser (Ctrl+F5) to pick up admin.html changes."
