#!/usr/bin/env bash
set -euo pipefail
echo "wget: $(command -v wget || echo missing)"
echo "curl: $(command -v curl || echo missing)"
echo "python3: $(command -v python3 || echo missing)"
echo "apt-get: $(command -v apt-get || echo missing)"
if command -v python3 >/dev/null; then
  python3 - <<'PY'
import urllib.request
print("go_latest:", urllib.request.urlopen("https://go.dev/VERSION?m=text").read().decode().splitlines()[0])
PY
fi
