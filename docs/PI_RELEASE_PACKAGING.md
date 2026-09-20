# Raspberry Pi slim release packaging (GitHub Releases)

How to build, package, and upload thin Pi updates for the in-app Admin updater.

**Version baseline:** units currently deployed on the Pi are **`v1.0.0`**. The first updater-capable release from this work is **`v1.1.0`**.

These packages are **not** a full git clone and should **not** be committed into `main` as bulk binaries. Distribute them as **GitHub Release assets**.

---

## Why this exists

- The old CLI updater looked under `compiled_packages/` and `data/` on the default branch. That bloats the repo and is a poor version channel.
- The Admin UI updater calls GitHub **Releases** (`/releases/latest`), downloads one slim tarball, verifies it, runs additive JSON migrations, swaps the prebuilt binary, and restarts.
- Fresh Pi units use a **one-click installer** (not the Admin updater). Host the script as a dedicated Release asset under tag `one-click-installer`; the script then downloads the newest **semver** thin package (`/releases/latest`).

---

## One-click Pi install (first deploy)

README one-liner (64-bit Raspberry Pi OS, user `pi`, not root):

```bash
curl -fsSL https://github.com/egtechgeek/TARR_Annunciator/releases/download/one-click-installer/install_raspberry_pi.sh | bash
```

Fallback if `curl` is missing:

```bash
wget -qO- https://github.com/egtechgeek/TARR_Annunciator/releases/download/one-click-installer/install_raspberry_pi.sh | bash
```

Private repo: `GITHUB_TOKEN=… bash <(curl -fsSL …/install_raspberry_pi.sh)`

### Two GitHub Release artifacts (do not conflate)

| Artifact | Tag / channel | Asset | Purpose |
|----------|---------------|--------|---------|
| Installer script | **`one-click-installer`** (evergreen, re-upload anytime) | `install_raspberry_pi.sh` | What the README curl\|bash fetches |
| App package | **`vX.Y.Z`** (and whichever is GitHub “latest”) | `TARR_Annunciator_Pi_arm64_vX.Y.Z.tar.gz` | What the installer downloads and extracts |

The installer is **not** the Admin updater. It still needs a published thin **app** release so there is something to install. Publish/update the installer script on the `one-click-installer` tag independently whenever the script changes.

Create or refresh the installer release:

```bash
# first time
gh release create one-click-installer scripts/install_raspberry_pi.sh \
  --repo egtechgeek/TARR_Annunciator \
  --title "One-click Pi installer" \
  --notes "curl|bash installer only — app binaries ship on semver releases (vX.Y.Z)."

# later: replace the script asset
gh release upload one-click-installer scripts/install_raspberry_pi.sh \
  --repo egtechgeek/TARR_Annunciator --clobber
```

Install path is always `~/TARR_Annunciator_RaspberryPi_ARM64` (override with `TARR_INSTALL_DIR`) so `~/start_tarr_annunciator.sh` paths stay stable. The versioned tarball folder name is never used as the live path.

---

## Conventions

| Item | Value |
|------|--------|
| Git tag / Release name | `vX.Y.Z` (semver with leading `v`), e.g. `v1.1.0` |
| Primary asset filename | `TARR_Annunciator_Pi_arm64_vX.Y.Z.tar.gz` |
| Binary build host | WSL2 Debian + aarch64 cross toolchain (CGO + ALSA) |
| Local output folder | `releases/` at repo root (gitignored) |
| Packaging script | [`scripts/package_pi_release.sh`](../scripts/package_pi_release.sh) |
| WSL build helper | [`fixit_scripts/wsl_build_pi_arm64.sh`](../fixit_scripts/wsl_build_pi_arm64.sh) |

---

## One-command package (WSL)

```bash
cd /mnt/c/Users/Ari\ Ellenbogen/Documents/GitHub/TARR_Annunciator
chmod +x scripts/package_pi_release.sh
./scripts/package_pi_release.sh 1.1.0
```

Produces:

- `releases/TARR_Annunciator_Pi_arm64_v1.1.0.tar.gz`
- Staging tree under `releases/staging/…` including `UPDATE_PACKAGE.json`

Reuse an already-built binary:

```bash
./scripts/package_pi_release.sh 1.1.0 --skip-build
```

---

## Slim tarball layout

Inside `TARR_Annunciator_Pi_arm64_v1.1.0.tar.gz`:

```
TARR_Annunciator_Pi_arm64_v1.1.0/
  UPDATE_PACKAGE.json
  tarr-annunciator              # prebuilt linux/arm64 ELF
  templates/                    # from data/templates
  static/                       # from data/static
  json/                         # DEFAULT SEEDS only (for migrations)
```

### Include
- Prebuilt `tarr-annunciator` for `linux/arm64`
- Web templates and MP3/static assets
- JSON **seeds** used only to create missing files or add missing keys/list ids

### Exclude
- `FromLiveRaspberryPi/`
- Live Pi state (`audio_settings.json`, `alsa.state`, `logs/`)
- `admin_config.json` from seeds (operator secrets)
- `legacy_py/`, Windows packages, unrelated repo folders

Install must **never** bulk-overwrite live operator JSON. Migrations are additive only.

---

## `UPDATE_PACKAGE.json` (updater contract)

```json
{
  "schema_version": 1,
  "app_version": "1.1.0",
  "platform": "linux",
  "arch": "arm64",
  "min_app_version": "1.0.0",
  "binary": {
    "path": "tarr-annunciator",
    "sha256": "<hex>"
  },
  "created_at": "2026-09-20T00:00:00Z",
  "release_notes": "Short human summary",
  "migrations": ["1.1.0"]
}
```

---

## How the Pi discovers an update

1. `GET https://api.github.com/repos/egtechgeek/TARR_Annunciator/releases/latest`
2. Compare Release `tag_name` to local `AppVersion`
3. Select asset `TARR_Annunciator_Pi_arm64_*.tar.gz`
4. Download → verify sha256 → migrate → swap binary → restart screen

If the repo is **private**, the Pi will need a read-only token later. If **public**, unauthenticated Releases API works (watch rate limits).

---

## Upload to GitHub Release

**Option A — GitHub website**

1. Repo → **Releases** → **Draft a new release**
2. Tag `v1.1.0` (from the commit you built)
3. Title: `v1.1.0`
4. Upload `releases/TARR_Annunciator_Pi_arm64_v1.1.0.tar.gz`
5. **Publish release**

**Option B — GitHub CLI**

```bash
gh release create v1.1.0 \
  releases/TARR_Annunciator_Pi_arm64_v1.1.0.tar.gz \
  --repo egtechgeek/TARR_Annunciator \
  --title "v1.1.0" \
  --notes "In-app updates, migrations, ALSA volume, operating hours, log rotation"
```

Do **not** commit large binaries/tarballs into `main`.

---

## First release sequence (v1.0.0 → v1.1.0)

1. Tag historical deploy as **`v1.0.0`** on GitHub (optional annotated tag / release notes: “Pi baseline before in-app updater”). No slim package required for v1.0.0 unless you want one for archival.
2. Build and publish **`v1.1.0`** slim package with this codebase (`AppVersion=1.1.0`).
3. Manually deploy `v1.1.0` once to the Pi (copy binary/templates or full package). After that, Admin **Check for Updates** targets later tags (`v1.2.0+`).

Units without `json/install_version.json` are treated as **1.0.0** until the v1.1.0 binary runs migrations and writes the install record.

---

## Per-release checklist

- [ ] `AppVersion` / ldflags match the tag (`v1.1.0` → `1.1.0`)
- [ ] WSL build OK; `file` shows ELF 64-bit ARM aarch64
- [ ] Tarball contains `UPDATE_PACKAGE.json`, binary, `templates/`, `static/`, seed `json/`
- [ ] sha256 in `UPDATE_PACKAGE.json` matches the binary
- [ ] GitHub Release published as the newest non-prerelease if you want it as “latest”
- [ ] Smoke: download the asset URL in a browser or with `curl`

---

## Related repo paths

- App sources: `source/` (`version.go`, `migrate.go`, `update.go`)
- Runtime assets in git: `data/templates`, `data/static`, `data/json`
- Live Pi snapshot: `FromLiveRaspberryPi/` (= `/home/pi/`)
