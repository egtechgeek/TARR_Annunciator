#!/usr/bin/env bash
set -euo pipefail
ROOT="/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator"
BIN="$ROOT/releases/TARR_Annunciator_Pi_arm64_v1.1.1/tarr-annunciator"
META="$ROOT/releases/TARR_Annunciator_Pi_arm64_v1.1.1/UPDATE_PACKAGE.json"

echo "=== file ==="
file "$BIN"
echo

echo "=== sha256 vs UPDATE_PACKAGE.json ==="
GOT=$(sha256sum "$BIN" | awk '{print $1}')
WANT=$(python3 -c "import json; print(json.load(open('$META'))['binary']['sha256'])")
echo "got:  $GOT"
echo "want: $WANT"
if [[ "$GOT" == "$WANT" ]]; then echo "SHA256 OK"; else echo "SHA256 MISMATCH"; exit 1; fi
echo

echo "=== NEEDED shared libraries (should include libasound) ==="
if command -v aarch64-linux-gnu-readelf >/dev/null; then
  READELF=aarch64-linux-gnu-readelf
else
  READELF=readelf
fi
$READELF -d "$BIN" | grep NEEDED || true
echo

echo "=== INTERP (Pi dynamic linker) ==="
$READELF -l "$BIN" | grep -A1 INTERP || true
echo

echo "=== AppVersion string ==="
strings "$BIN" | grep -E '^1\.1\.1$' | head -3 || true
echo "=== Thor URL hardcoded in binary? ==="
if strings "$BIN" | grep -qi 'thormobile\|FL0115'; then
  strings "$BIN" | grep -i 'thormobile\|FL0115' | head -5
  echo "(note: may appear only if linked from templates/json embedded — check)"
else
  echo "(none found — good)"
fi
echo

echo "=== About the earlier WSL ALSA error ==="
echo "That failure was a native WSL 'go test' without arm64 cross env / host alsa.pc."
echo "This artifact was built with aarch64-linux-gnu-gcc + CGO via package_pi_release.sh."
echo "On the Pi it needs Raspberry Pi OS libasound2 (already required by the installer)."
echo
echo "CHECK_OK"
