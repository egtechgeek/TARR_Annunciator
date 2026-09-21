# Changelog

All notable changes to TARR Annunciator are documented here.

Product versions match `AppVersion` in `source/version.go` and GitHub Release tags (`vX.Y.Z`).
Target platform going forward: **Raspberry Pi OS 64-bit (`linux/arm64`)**.

Versioning note: older internal milestones were labeled “2.0” / “2.1” in prior docs. Those features shipped on live Pi units **before** in-app GitHub Releases updates existed and are treated as the **`1.0.0` baseline**. Formal semver: **`1.0.0`** → **`1.1.0`** → **`1.1.1`** → **`1.1.2`** (current).

---

## [1.1.2] - 2026-09-20

Thor Guard multi-feed failover with full Admin control (life-safety).

### Added

- Up to **three XML feeds** (primary + failover_1 + failover_2) with ordered failover/failback
- Configurable failover **triggers**, failback mode, dwell, and Red Alert interaction flags
- **All Clear release authority** (`allclear_release_mode`): `primary_only` (default, 1.1.1-compatible on primary) or `failover_vote` (primary clears alone; otherwise both failovers must return All Clear — Unknown/error does not count)
- Separate **feed-switch announcement matrix** (per from→to / reason / optional sensor) — quiet by default
- Per-feed announce inherit/custom + optional per-condition MP3 overrides
- **Global Condition Audio** (horn + announce MP3 pairs) for Red Alert / All Clear / Warning / Caution / Unknown — Admin-owned, not hardcoded in the player
- Per-feed **horn overrides** for Red Alert and All Clear (inherit global or custom enable + file)
- Reminder policy exposes editable **reminder horn MP3** (was previously hardcoded on save)
- Admin Live Status **Last Status** column (last successfully parsed Thor condition per feed)
- Public main UI **tab navigation** (Dashboard / Station / Promo / Safety); Dashboard shows simplified lightning status + announcement controls
- Public index **dynamic** audio/scheduler status (same operating-hours labels as Admin, e.g. “Paused (outside operating hours)”)
- `browardtg.json` catalog loader + Admin sensor dropdown (correct path prefixes)
- Live status: active feed, displayname, feed health table, pin for drills
- Scoped Admin saves: `monitor` | `condition_announce` | `condition_audio` | `red_alert_policy`
- Additive migration from 1.1.1 single `monitor.url`; in-app update installs/merges catalog
- **Lightning MP3 rename migration** on upgrade: remaps known legacy `thor_*.mp3` / `redalert.mp3` paths in `lightning.json` to `Voice_*` / `Horn_*` (custom filenames left alone). Updater merges new static assets; old files on disk are left in place. No manual file copy required for standard 1.1.1 → 1.1.2 updates.

### Changed

- Lightning play sequence is assembled from `condition_audio` (+ per-feed overrides): optional horn then announce
- Announcement queue no longer hardcodes `thor_red_alert.mp3`+`redalert.mp3` / All Clear pairs
- Replaced legacy `require_allclear_from_same_feed` with `allclear_release_mode` (unlock-only; does not change Red Alert enter)
- API Docs moved behind Admin session (`/admin/api-docs`); removed from public index
- Public `/lightning_status` includes `active_displayname`, `on_failover`, feed health for Dashboard
- `AppVersion` **1.1.2**

### Safety

- No Thor URL invented in Go; never auto-All-Clear on total outage; announce-off ≠ monitor/lock-off
- All Clear release modes are unlock-only; v1.1.1 `shouldAcceptAllClear` / Unknown-ignore / Red Alert enter rules preserved

---

## [1.1.1] - 2026-09-20

Thor Guard lightning announce controls and persisted monitor config (post–field-test follow-up).

### Investigation (Caution / Warning / AllClear chatter)

- Observed sequences like Caution → AllClear → Caution / Warning during feed changes.
- **All Clear** lock release + audio already require a prior **Red Alert**; Caution/Warning never take that lock.
- Chatter still occurs when those conditions flip in XML while their announcements are enabled.
- **Mitigation:** Admin can enable/disable automatic announce per condition; monitoring continues either way. Manual Test buttons still force-play for verification.

### Added

- Admin → Lightning Alerts → **Condition Announcements (auto-play)** toggles for Red Alert, Warning, Caution, All Clear, Unknown
- Default seed: **Caution** and **Warning** auto-play **disabled** (reduces XML flap chatter); Red Alert / All Clear remain enabled by default
- Default lightning monitor `fetch_interval`: **60** seconds
- Default operating hours window: **09:00–16:00**
- Default `trains_selected.json`: all trains from `trains_available.json`
- Persists via `lightning_announcements[].enabled` in `json/lightning.json`
- Status API includes `condition_announce` snapshot
- `lightning.json` **`monitor`** block (`enabled`, `url`, `fetch_interval`, `timeout`) — XML URL no longer only in-memory/hardcoded
- Additive migration `migrateLightningMonitorBlock` for upgrades to 1.1.1
- Planning doc: [`docs/plan_v1.1.2.md`](docs/plan_v1.1.2.md) (multi-feed failover, `<displayname>`, per-sensor MP3 overrides)

### Changed

- Automatic lightning announces respect per-condition enable flags; Red Alert **lock** enter/exit still applies when All Clear is accepted after Red Alert even if All Clear audio is disabled
- Thor Guard XML URL is **not hardcoded in Go** — only `lightning.json` → `monitor.url` (seed still defaults to Tradewinds FL0115)
- Admin **Software Updates**: list GitHub thin Pi releases and install a **selected** tag (not forced to newest)
- `AppVersion` → **1.1.1**

### Config location note

- Thor Guard settings: `json/lightning.json` (announcements, red alert policy, monitor URL)
- Cached XML: `xml/*.xml` beside the binary (not under `json/`)
- Default / historical feed URL: `https://broward.thormobile4.net/tp/FL0115.xml` (`UNIQUEID` = `FL0115`, host = Broward Thor Mobile)

---

## [1.1.0] - 2026-09-20

First updater-capable release. Units missing `json/install_version.json` are treated as `1.0.0` until this binary runs and writes the install record.

### Added

#### In-app updates (Admin UI)
- **Check for Updates** / **Download and Install** against GitHub Releases (`/releases/latest`)
- Thin package contract: `UPDATE_PACKAGE.json` (version, platform/arch, binary sha256, migrations)
- Download → extract to temp → verify sha256 → additive JSON migrations → swap `tarr-annunciator` → Screen restart
- Install path remains `~/TARR_Annunciator_RaspberryPi_ARM64` (versioned tarball folder name is never used as the live path)
- Temp download/extract cleaned up after install; optional `update_backup_*` keeps prior binary + json

#### One-click installer
- Evergreen Release tag `one-click-installer` hosts `install_raspberry_pi.sh`
- README one-liner:
  `curl -fsSL https://github.com/egtechgeek/TARR_Annunciator/releases/download/one-click-installer/install_raspberry_pi.sh | bash`
- Installer bootstraps deps, fetches latest thin Pi package, installs to fixed path, wires Screen helpers (`~/tarr-start.sh`, etc.)
- Additive JSON seeds only; never overwrites `admin_config.json` / `audio_settings.json`

#### Versioning & migrations
- Embedded `AppVersion` (`1.1.0`) and `json/install_version.json`
- Additive schema migrations (`source/migrate.go`) — create missing files / keys only; never bulk-overwrite operator settings
- `1.1.0` migrations: operating hours defaults, lightning schema additions

#### Operating hours & time
- Operating-hours configuration and enforcement
- NTP / time awareness for schedule correctness on the Pi

#### Audio
- ALSA volume control (`alsa_volume.go`) with persistence
- Audio device + volume persistence across restarts
- Lightning / THOR assets and trigger support (Caution, Warning, Red Alert, All Clear, repeats)

#### Logging
- Log rotation / size management for long-running Pi deployments

#### Packaging & docs
- Slim Pi release packaging: `scripts/package_pi_release.sh` → `releases/TARR_Annunciator_Pi_arm64_vX.Y.Z.tar.gz`
- Maintainer guide: `docs/PI_RELEASE_PACKAGING.md`
- Pi-focused `README.md` (one-click install front and center)

### Changed
- Distribution channel moved from repo-bloat (`compiled_packages/`, full clones) to **GitHub Release assets**
- Raspberry Pi is the supported deployment target; Windows / macOS packaging is no longer a product goal

### Removed
- Standalone CLI updater (`updater/` at repo root) — replaced by in-app Admin updater
- `legacy_py/` — original Python annunciator
- `compiled_packages/` — obsolete binary drop folder (use `releases/` + GitHub Releases)

### Fixed
- Various Go vet / build issues for linux/arm64 CGO + ALSA cross-compile (WSL2 Debian toolchain)
- Installer messaging: Screen helpers live in `~`, not relative `./` under the app dir
- Packaging excludes live operator secrets from seed JSON

---

## [1.0.0] - Pi baseline (pre–in-app updater)

**Status:** Deployed on live Raspberry Pi units before `1.1.0`. No slim GitHub Release package was required for this tag; it is the assumed version when `install_version.json` is absent.

This baseline includes the Go rewrite and the former internal milestones documented as “2.0” (2025-08-25) and “2.1” (2025-08-29).

### Included from former “2.0” milestone (2025-08-25)

- Go rewrite with Gin web UI / Admin / API
- Multi-user Admin authentication and API key management
- Priority announcement queue (Emergency / High / Normal / Low)
- Project layout with `source/` and runtime assets
- Cross-platform packaging experiments (Windows / Linux / Pi) and early CLI updater designs (superseded in `1.1.0`)
- Raspberry Pi–oriented install and audio notes

### Included from former “2.1” milestone (2025-08-29)

- Multi-language safety announcements (sequential languages + delay)
- File logging with date-stamped logs and retention cleanup
- Track layout / available trains & destinations Admin fixes
- Bluetooth discovery & pairing improvements on Pi
- PipeWire / PulseAudio / ALSA audio system options
- **GNU Screen** session management replacing systemd for Pi audio permissions
  - `~/start_tarr_annunciator.sh`, `~/tarr-start.sh`, `~/tarr-stop.sh`, `~/tarr-restart.sh`, `~/tarr-view.sh`
  - Autologin + `.bashrc` console auto-start
- Enhanced `install_raspberry_pi.sh` (deps, audio, Screen helpers) — later evolved into the `1.1.0` one-click GitHub installer

### Runtime layout (live Pi)

```text
/home/pi/TARR_Annunciator_RaspberryPi_ARM64/
  tarr-annunciator
  templates/  static/  json/  logs/
```

---

## [Unreleased]

- Publish additional semver thin packages (`v1.1.3+` / `v1.2.0+`) for Admin in-app updates after `1.1.2` is on the Pi
- Optional: prune accumulated `update_backup_*` folders on long-lived units

---

## Release artifact map

| Channel | Tag / name | Asset | Consumers |
|---------|------------|--------|-----------|
| One-click installer | `one-click-installer` | `install_raspberry_pi.sh` | Fresh Pi install (curl\|bash) |
| App package | `vX.Y.Z` (GitHub “latest”) | `TARR_Annunciator_Pi_arm64_vX.Y.Z.tar.gz` | One-click installer + Admin updater |
| Version baseline | *(implicit)* | — | Missing `install_version.json` ⇒ `1.0.0` |

See [`docs/PI_RELEASE_PACKAGING.md`](docs/PI_RELEASE_PACKAGING.md) for build and upload steps.

---

## Links

- [1.1.0]: https://github.com/egtechgeek/TARR_Annunciator/releases/tag/v1.1.0
- [one-click-installer]: https://github.com/egtechgeek/TARR_Annunciator/releases/tag/one-click-installer
