#!/usr/bin/env bash
set -euo pipefail
export PATH="$HOME/.local/go/bin:$PATH"
. "$HOME/.profile_tarr_cross"
tarr_arm64_env

echo "Go:     $(go version)"
echo "GOROOT: $(go env GOROOT)"
echo "CC:     $CC"
echo "CXX:    $CXX"
echo "PKG_CONFIG_LIBDIR: ${PKG_CONFIG_LIBDIR:-unset}"

echo ""
echo "=== pkg-config alsa (arm64) ==="
pkg-config --modversion alsa || true
pkg-config --cflags --libs alsa || true

echo ""
echo "=== smoke: empty main aarch64 ==="
SMOKE="$(mktemp --suffix=.c)"
OUT="$(mktemp)"
printf 'int main(void){return 0;}\n' > "$SMOKE"
aarch64-linux-gnu-gcc -o "$OUT" "$SMOKE"
file "$OUT"
rm -f "$SMOKE" "$OUT"

echo ""
echo "=== smoke: link alsa ==="
SMOKE="$(mktemp --suffix=.c)"
OUT="$(mktemp)"
cat > "$SMOKE" <<'EOF'
#include <alsa/asoundlib.h>
int main(void) {
  return (int)sizeof(snd_pcm_t*);
}
EOF
aarch64-linux-gnu-gcc $CGO_CFLAGS -o "$OUT" "$SMOKE" $CGO_LDFLAGS
file "$OUT"
rm -f "$SMOKE" "$OUT"

echo ""
echo "TOOLCHAIN_OK"
