#!/usr/bin/env bash
set -euo pipefail

export PATH="$HOME/.local/go/bin:$PATH"
. "$HOME/.profile_tarr_cross"
tarr_arm64_env

SRC="/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/source"
OUT_DIR="/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/releases"
OUT="${OUT_DIR}/tarr-annunciator-raspberry-pi-arm64-wsl-test"

mkdir -p "$OUT_DIR"
cd "$SRC"

echo "Working directory: $(pwd)"
echo "Go: $(go version)"
echo "GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=$CGO_ENABLED"
echo "CC=$CC"
echo "CGO_CFLAGS=$CGO_CFLAGS"
echo "CGO_LDFLAGS=$CGO_LDFLAGS"
echo ""

echo "==> go mod download"
go mod download

echo ""
echo "==> go build (linux/arm64 CGO)"
# Build into WSL filesystem first (faster / fewer Windows FS quirks), then copy out
WSL_OUT="$HOME/.local/share/tarr/tarr-annunciator-raspberry-pi-arm64"
mkdir -p "$(dirname "$WSL_OUT")"

set +e
go build -o "$WSL_OUT" .
BUILD_RC=$?
set -e

if [[ $BUILD_RC -ne 0 ]]; then
  echo ""
  echo "BUILD_FAILED rc=$BUILD_RC"
  exit $BUILD_RC
fi

cp -f "$WSL_OUT" "$OUT"
chmod +x "$OUT" || true

echo ""
echo "BUILD_OK"
ls -lh "$WSL_OUT" "$OUT"
file "$WSL_OUT"
echo "sha256: $(sha256sum "$WSL_OUT" | awk '{print $1}')"
