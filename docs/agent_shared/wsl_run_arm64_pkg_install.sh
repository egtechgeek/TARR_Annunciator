#!/usr/bin/env bash
# Wrapper with no spaces in its own invocation path issues:
# copies package installer into ~/ then runs it.
set -euo pipefail
SRC="/mnt/c/Users/Ari Ellenbogen/Documents/GitHub/TARR_Annunciator/docs/agent_shared/wsl_install_arm64_packages.sh"
DEST="$HOME/.local/bin/wsl_install_arm64_packages.sh"
mkdir -p "$HOME/.local/bin"
cp "$SRC" "$DEST"
chmod +x "$DEST"
exec bash "$DEST"
