#!/usr/bin/env bash
# Build a slim Raspberry Pi arm64 GitHub Release package.
# Intended to run inside WSL2 Debian with the aarch64 cross toolchain configured
# (see fixit_scripts/wsl_build_pi_arm64.sh / ~/.profile_tarr_cross).
#
# Usage:
#   ./scripts/package_pi_release.sh 1.1.0
#   ./scripts/package_pi_release.sh 1.1.0 --skip-build   # reuse existing binary
#
# Output:
#   releases/TARR_Annunciator_Pi_arm64_vX.Y.Z.tar.gz
#   releases/TARR_Annunciator_Pi_arm64_vX.Y.Z/UPDATE_PACKAGE.json
set -euo pipefail

VERSION="${1:-}"
SKIP_BUILD=0
if [[ "${2:-}" == "--skip-build" ]]; then
  SKIP_BUILD=1
fi

if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version> [--skip-build]"
  echo "Example: $0 1.1.0"
  exit 1
fi

VERSION="${VERSION#v}"
TAG="v${VERSION}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# scripts/ -> repo root (works for both /mnt/c/... and native Linux paths)
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SRC_DIR="$REPO_ROOT/source"
DATA_DIR="$REPO_ROOT/data"
OUT_DIR="$REPO_ROOT/releases"
STAGE_NAME="TARR_Annunciator_Pi_arm64_${TAG}"
STAGE_DIR="$OUT_DIR/staging/$STAGE_NAME"
TARBALL="$OUT_DIR/${STAGE_NAME}.tar.gz"
BIN_NAME="tarr-annunciator"

echo "==> Packaging $TAG"
echo "    Repo: $REPO_ROOT"

mkdir -p "$OUT_DIR" "$STAGE_DIR"

if [[ "$SKIP_BUILD" -eq 0 ]]; then
  if [[ -f "$HOME/.profile_tarr_cross" ]]; then
    # shellcheck disable=SC1090
    . "$HOME/.profile_tarr_cross"
    if declare -F tarr_arm64_env >/dev/null 2>&1; then
      tarr_arm64_env
    fi
  fi
  export PATH="${HOME}/.local/go/bin:${PATH:-}"
  export GOOS=linux
  export GOARCH=arm64
  export CGO_ENABLED=1

  echo "==> go build (linux/arm64 CGO) AppVersion=$VERSION"
  cd "$SRC_DIR"
  go mod download
  WSL_OUT="${HOME}/.local/share/tarr/${BIN_NAME}"
  mkdir -p "$(dirname "$WSL_OUT")"
  go build -ldflags "-X main.AppVersion=${VERSION}" -o "$WSL_OUT" .
  cp -f "$WSL_OUT" "$OUT_DIR/${BIN_NAME}"
  chmod +x "$OUT_DIR/${BIN_NAME}"
  file "$OUT_DIR/${BIN_NAME}" || true
else
  echo "==> Skipping build; using $OUT_DIR/${BIN_NAME}"
  if [[ ! -f "$OUT_DIR/${BIN_NAME}" ]]; then
    echo "ERROR: binary not found at $OUT_DIR/${BIN_NAME}"
    exit 1
  fi
fi

SHA256="$(sha256sum "$OUT_DIR/${BIN_NAME}" | awk '{print $1}')"
echo "    sha256: $SHA256"

echo "==> Assembling staging tree"
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"
cp -f "$OUT_DIR/${BIN_NAME}" "$STAGE_DIR/${BIN_NAME}"
chmod +x "$STAGE_DIR/${BIN_NAME}"

mkdir -p "$STAGE_DIR/templates" "$STAGE_DIR/static" "$STAGE_DIR/json"
cp -a "$DATA_DIR/templates/." "$STAGE_DIR/templates/"
cp -a "$DATA_DIR/static/." "$STAGE_DIR/static/"
# Seeds only — updater migrations never bulk-overwrite live operator JSON
cp -a "$DATA_DIR/json/." "$STAGE_DIR/json/"
# Never ship live runtime secrets/state as "defaults"
rm -f "$STAGE_DIR/json/audio_settings.json" \
      "$STAGE_DIR/json/install_version.json" \
      "$STAGE_DIR/json/admin_config.json" 2>/dev/null || true

NOTES="${RELEASE_NOTES:-TARR Annunciator Pi arm64 ${TAG}}"
CREATED="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

# List known schema migrations up through this package version (informational for UPDATE_PACKAGE.json)
MIGRATIONS_JSON="$(python3 - "$VERSION" <<'PY'
import json, sys
ver = sys.argv[1].lstrip("v")
known = ["1.1.0", "1.1.1"]
out = [m for m in known if tuple(int(x) for x in m.split(".")) <= tuple(int(x) for x in ver.split(".")[:3])]
if ver not in out:
    out.append(ver)
print(json.dumps(out))
PY
)"

cat > "$STAGE_DIR/UPDATE_PACKAGE.json" <<EOF
{
  "schema_version": 1,
  "app_version": "${VERSION}",
  "platform": "linux",
  "arch": "arm64",
  "min_app_version": "1.0.0",
  "binary": {
    "path": "tarr-annunciator",
    "sha256": "${SHA256}"
  },
  "created_at": "${CREATED}",
  "release_notes": $(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$NOTES"),
  "migrations": ${MIGRATIONS_JSON}
}
EOF

echo "==> Creating tarball"
mkdir -p "$OUT_DIR"
tar -C "$OUT_DIR/staging" -czf "$TARBALL" "$STAGE_NAME"

echo ""
echo "PACKAGE_OK"
echo "  $TARBALL"
ls -lh "$TARBALL"
echo ""
echo "Upload with:"
echo "  gh release create ${TAG} \"$TARBALL\" --repo egtechgeek/TARR_Annunciator --title \"${TAG}\" --notes \"$NOTES\""
echo "Or use the GitHub website Releases UI. See docs/PI_RELEASE_PACKAGING.md"
