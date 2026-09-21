# TARR Annunciator

Train announcement system for **Raspberry Pi** (64-bit / `linux/arm64`). Built in Go with a web Admin UI, ALSA/PipeWire audio, Screen-based auto-start, and in-app updates from GitHub Releases.

**Target hardware:** Raspberry Pi 4 (or similar) running 64-bit Raspberry Pi OS.

---

## One-click install

On the Pi, as user `pi` (**not root**):

```bash
curl -fsSL https://github.com/egtechgeek/TARR_Annunciator/releases/download/one-click-installer/install_raspberry_pi.sh | bash
```

If `curl` is unavailable:

```bash
wget -qO- https://github.com/egtechgeek/TARR_Annunciator/releases/download/one-click-installer/install_raspberry_pi.sh | bash
```

The installer will:

1. Install system dependencies (ALSA, Screen, optional PipeWire/Bluetooth)
2. Download the latest thin Pi package from GitHub Releases
3. Install into `~/TARR_Annunciator_RaspberryPi_ARM64`
4. Create Screen auto-start and helper scripts in `~`

After install:

| Action | Command |
|--------|---------|
| Start | `~/tarr-start.sh` |
| Stop | `~/tarr-stop.sh` |
| Restart | `~/tarr-restart.sh` |
| Attach to session | `~/tarr-view.sh` or `screen -r tarr-annunciator` |

**Web UI:** [http://localhost:8080](http://localhost:8080) · **Admin:** [http://localhost:8080/admin](http://localhost:8080/admin)

**Operator manual:** see [`MANUAL.md`](MANUAL.md) for a full plain-language guide to every screen and setting.

> Review the installer script before piping to `bash` if your environment requires it. Private repos: set `GITHUB_TOKEN` before running.

---

## Features

- Station, safety, promo, emergency, and lightning announcements
- Priority announcement queue
- Web Admin for trains, destinations, audio devices, volume, operating hours
- ALSA volume / device persistence
- Log rotation
- In-app **Check for Updates** / install from GitHub Releases (after first deploy)

---

## Updates (after first install)

1. Open **Admin → Updates**
2. **Check for Updates**
3. **Download and Install** when a newer release is available

The updater downloads a slim tarball, verifies the binary checksum, applies additive JSON migrations (does not overwrite operator settings), swaps `tarr-annunciator`, and restarts the Screen session.

Install path stays `~/TARR_Annunciator_RaspberryPi_ARM64` so start/stop scripts keep working.

---

## Requirements

- Raspberry Pi OS **64-bit** (`aarch64`)
- Network access to GitHub Releases (for install and updates)
- Audio output configured (3.5mm, HDMI, or USB as preferred)
- Port **8080** available for the web UI

---

## Manual helpers (installed by the one-click script)

Scripts live in the home directory:

```bash
~/tarr-start.sh
~/tarr-stop.sh
~/tarr-restart.sh
~/tarr-view.sh
~/start_tarr_annunciator.sh   # used by auto-start / .bashrc
```

Application directory:

```text
~/TARR_Annunciator_RaspberryPi_ARM64/
  tarr-annunciator
  templates/
  static/
  json/
  logs/
```

---

## Packaging & releases (maintainers)

How to build thin Pi packages, publish semver releases, and refresh the one-click installer asset:

See [`docs/PI_RELEASE_PACKAGING.md`](docs/PI_RELEASE_PACKAGING.md).

---

## API

With the app running and an Admin session (or API key):

- Docs: [http://localhost:8080/admin/api-docs](http://localhost:8080/admin/api-docs) (login required)
- Platform: `GET /api/platform`

Authenticated Admin/API calls use the API key or session configured in Admin.

---

## License

Part of the TARR (Tradewinds and Atlantic Railroad) system.
