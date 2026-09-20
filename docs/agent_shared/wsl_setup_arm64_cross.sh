#!/usr/bin/env bash
# Setup latest Go (user-local) + ARM64 Linux CGO cross toolchain in WSL Debian.
set -euo pipefail

echo "==============================================="
echo "WSL Debian: Go + ARM64 CGO cross toolchain"
echo "==============================================="

GO_INSTALL_DIR="${HOME}/.local"
GO_ROOT="${GO_INSTALL_DIR}/go"
PROFILE_SNIPPET="${HOME}/.profile_tarr_cross"
BASHRC="${HOME}/.bashrc"

need_sudo_packages=1
if [[ "${1:-}" == "--go-only" ]]; then
  need_sudo_packages=0
fi

download() {
  # Prefer curl/wget when present; fall back to python3 (Debian minimal often lacks curl).
  local url="$1"
  local out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fL --progress-bar -o "${out}" "${url}"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "${out}" "${url}"
  elif command -v python3 >/dev/null 2>&1; then
    python3 - "${url}" "${out}" <<'PY'
import sys, urllib.request
url, out = sys.argv[1], sys.argv[2]
print(f"Downloading {url} -> {out}", flush=True)
urllib.request.urlretrieve(url, out)
print("Download complete", flush=True)
PY
  else
    echo "ERROR: need curl, wget, or python3 to download"
    exit 1
  fi
}

fetch_text() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "${url}"
  elif command -v python3 >/dev/null 2>&1; then
    python3 - "${url}" <<'PY'
import sys, urllib.request
print(urllib.request.urlopen(sys.argv[1]).read().decode(), end="")
PY
  else
    echo "ERROR: need curl, wget, or python3 to fetch ${url}" >&2
    exit 1
  fi
}

echo ""
echo "==> Resolving latest stable Go"
VERSION_LINE="$(fetch_text 'https://go.dev/VERSION?m=text' | head -n 1)"
if [[ -z "${VERSION_LINE}" || "${VERSION_LINE}" != go* ]]; then
  echo "ERROR: could not resolve latest Go version from go.dev"
  exit 1
fi
GO_VERSION="${VERSION_LINE}"
GO_TARBALL="${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"
echo "Latest: ${GO_VERSION}"
echo "URL: ${GO_URL}"

TMPDIR_DL="$(mktemp -d)"
cleanup() { rm -rf "${TMPDIR_DL}"; }
trap cleanup EXIT

echo ""
echo "==> Downloading ${GO_TARBALL}"
download "${GO_URL}" "${TMPDIR_DL}/${GO_TARBALL}"

echo ""
echo "==> Installing Go to ${GO_ROOT}"
mkdir -p "${GO_INSTALL_DIR}"
rm -rf "${GO_ROOT}"
tar -C "${GO_INSTALL_DIR}" -xzf "${TMPDIR_DL}/${GO_TARBALL}"

# Prefer user Go over apt /usr/bin/go
export PATH="${GO_ROOT}/bin:${PATH}"
hash -r || true

echo "Installed: $(go version)"
echo "GOROOT: $(go env GOROOT)"

echo ""
echo "==> Writing PATH snippet: ${PROFILE_SNIPPET}"
cat > "${PROFILE_SNIPPET}" <<'EOF'
# TARR Annunciator WSL cross-compile environment
export PATH="$HOME/.local/go/bin:$PATH"

# Default cross-compile helpers (opt-in via function)
tarr_arm64_env() {
  export CGO_ENABLED=1
  export GOOS=linux
  export GOARCH=arm64
  export CC=aarch64-linux-gnu-gcc
  export CXX=aarch64-linux-gnu-g++
  # Prefer arm64 pkg-config data when present
  if [[ -d /usr/lib/aarch64-linux-gnu/pkgconfig ]]; then
    export PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig
    export PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig
  fi
  # Common include/lib paths for cross builds
  export CGO_CFLAGS="${CGO_CFLAGS:--I/usr/aarch64-linux-gnu/include -I/usr/include/aarch64-linux-gnu}"
  export CGO_LDFLAGS="${CGO_LDFLAGS:--L/usr/aarch64-linux-gnu/lib -L/usr/lib/aarch64-linux-gnu -lasound}"
  echo "ARM64 CGO env set: GOOS=$GOOS GOARCH=$GOARCH CC=$CC"
}

tarr_arm64_build() {
  tarr_arm64_env
  local out="${1:-tarr-annunciator-raspberry-pi-arm64}"
  shift || true
  go build -o "$out" "$@"
  echo "Built: $out"
  file "$out" || true
}
EOF

if [[ -f "${BASHRC}" ]] && ! grep -q 'profile_tarr_cross' "${BASHRC}"; then
  {
    echo ''
    echo '# TARR Annunciator cross-compile helpers'
    echo '[[ -f "$HOME/.profile_tarr_cross" ]] && . "$HOME/.profile_tarr_cross"'
  } >> "${BASHRC}"
  echo "Appended source line to ${BASHRC}"
elif [[ -f "${BASHRC}" ]]; then
  echo "${BASHRC} already sources profile_tarr_cross"
else
  echo "NOTE: ${BASHRC} missing; create it or source ${PROFILE_SNIPPET} manually"
fi

# Also ensure login shells pick it up via .profile if present
if [[ -f "${HOME}/.profile" ]] && ! grep -q 'profile_tarr_cross' "${HOME}/.profile"; then
  {
    echo ''
    echo '# TARR Annunciator cross-compile helpers'
    echo '[[ -f "$HOME/.profile_tarr_cross" ]] && . "$HOME/.profile_tarr_cross"'
  } >> "${HOME}/.profile"
fi

. "${PROFILE_SNIPPET}"

if [[ "${need_sudo_packages}" -eq 0 ]]; then
  echo ""
  echo "Go-only mode complete. Re-run without --go-only to install packages."
  exit 0
fi

echo ""
echo "==> Installing ARM64 cross packages (sudo required once)"
if ! sudo -n true 2>/dev/null; then
  echo ""
  echo "Sudo needs your password for apt installs."
  echo "If this is non-interactive, run in a WSL terminal:"
  echo "  bash '/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_setup_arm64_cross.sh'"
  echo ""
fi

sudo apt-get update
sudo dpkg --add-architecture arm64
sudo apt-get update

# Cross compiler + libc headers + ALSA for aarch64 + helpers
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
  build-essential \
  pkg-config \
  gcc-aarch64-linux-gnu \
  g++-aarch64-linux-gnu \
  libc6-dev-arm64-cross \
  linux-libc-dev-arm64-cross \
  libasound2-dev:arm64 \
  file

echo ""
echo "==> Verifying toolchain"
echo "Go:     $(go version)"
echo "CC:     $(aarch64-linux-gnu-gcc --version | head -n 1)"
echo "CXX:    $(aarch64-linux-gnu-g++ --version | head -n 1)"
echo "pkg-config: $(pkg-config --version)"

if pkg-config --exists alsa; then
  echo "ALSA:   $(pkg-config --modversion alsa) (host pkg-config)"
else
  echo "ALSA:   host pkg-config does not see alsa (expected until PKG_CONFIG_LIBDIR is set)"
fi

if [[ -f /usr/lib/aarch64-linux-gnu/libasound.so ]] || ls /usr/lib/aarch64-linux-gnu/libasound.so* >/dev/null 2>&1; then
  echo "ALSA arm64 libs present under /usr/lib/aarch64-linux-gnu/"
  ls -1 /usr/lib/aarch64-linux-gnu/libasound.so* 2>/dev/null | head -n 5
else
  echo "WARNING: libasound arm64 not found where expected"
fi

# Quick compile smoke test (no Go project required)
SMOKE="${TMPDIR_DL}/smoke.c"
cat > "${SMOKE}" <<'EOF'
int main(void) { return 0; }
EOF
aarch64-linux-gnu-gcc -o "${TMPDIR_DL}/smoke" "${SMOKE}"
file "${TMPDIR_DL}/smoke"

echo ""
echo "==============================================="
echo "Setup complete"
echo "==============================================="
echo "New shells will load helpers from ~/.profile_tarr_cross"
echo "In a shell:"
echo "  source ~/.profile_tarr_cross"
echo "  tarr_arm64_env"
echo "  cd '/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/source'"
echo "  tarr_arm64_build"
echo ""
