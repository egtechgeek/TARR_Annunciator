#!/usr/bin/env bash
# Package-only half of ARM64 cross toolchain setup (assumes Go already installed).
set -euo pipefail

echo "==============================================="
echo "Installing ARM64 cross-compile packages"
echo "==============================================="
echo "You will be prompted for your Debian/WSL sudo password."
echo ""

sudo apt-get update
sudo dpkg --add-architecture arm64
sudo apt-get update

sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
  build-essential \
  pkg-config \
  curl \
  wget \
  gcc-aarch64-linux-gnu \
  g++-aarch64-linux-gnu \
  libc6-dev-arm64-cross \
  linux-libc-dev-arm64-cross \
  libasound2-dev:arm64 \
  file

echo ""
echo "==> Verification"
export PATH="$HOME/.local/go/bin:$PATH"
[[ -f "$HOME/.profile_tarr_cross" ]] && . "$HOME/.profile_tarr_cross"

echo "Go:  $(go version 2>/dev/null || echo missing)"
echo "CC:  $(aarch64-linux-gnu-gcc --version | head -n 1)"
echo "CXX: $(aarch64-linux-gnu-g++ --version | head -n 1)"

if ls /usr/lib/aarch64-linux-gnu/libasound.so* >/dev/null 2>&1; then
  echo "ALSA arm64:"
  ls -1 /usr/lib/aarch64-linux-gnu/libasound.so* | head -n 5
else
  echo "WARNING: libasound arm64 not found"
fi

SMOKE="$(mktemp --suffix=.c)"
OUT="$(mktemp)"
cat > "${SMOKE}" <<'EOF'
int main(void) { return 0; }
EOF
aarch64-linux-gnu-gcc -o "${OUT}" "${SMOKE}"
file "${OUT}"
rm -f "${SMOKE}" "${OUT}"

# Persist marker so the agent can detect success non-interactively
mkdir -p "$HOME/.local/share/tarr"
date -Is > "$HOME/.local/share/tarr/arm64_cross_ready"
echo "OK" >> "$HOME/.local/share/tarr/arm64_cross_ready"

echo ""
echo "DONE. You can close this window."
read -r -p "Press Enter to exit..."
