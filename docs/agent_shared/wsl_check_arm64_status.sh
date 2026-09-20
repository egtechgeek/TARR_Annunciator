#!/usr/bin/env bash
set -euo pipefail
echo "=== apt/dpkg processes ==="
ps aux | grep -E 'apt|dpkg' | grep -v grep || echo "none"
echo "=== ready marker ==="
if [[ -f /home/ariellenbogen/.local/share/tarr/arm64_cross_ready ]]; then
  echo READY
  cat /home/ariellenbogen/.local/share/tarr/arm64_cross_ready
else
  echo NOT_READY
fi
echo "=== gcc ==="
command -v aarch64-linux-gnu-gcc && aarch64-linux-gnu-gcc --version | head -1 || echo missing
echo "=== packages ==="
dpkg -l 'gcc-aarch64-linux-gnu' 'g++-aarch64-linux-gnu' 'libasound2-dev:arm64' 'libc6-dev-arm64-cross' 'pkg-config' 'build-essential' 2>/dev/null | awk 'NR==1 || /^[hi]/'
