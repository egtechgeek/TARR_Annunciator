# Plan — v1.1.4 (merged)

Status: **ready for packaging** (implemented in tree)  
Prior release: **v1.1.3** (field-deployed)  
Merged from: Plan A (updater + deferred items) + Plan B (sensor telemetry collapse).

Single release: one version bump, one Pi package, one Admin/docs pass.

---

## Release goals

1. **Updater:** Forward-safe in-app updates (fix `browardtg.json` and future seeds across version jumps).
2. **Life safety:** Detect Thor silent sensor death (valid XML + metric cliff); hold Red Alert lock on poll 1; failover recovery.
3. **Audio layout:** `static/mp3/horns-chimes-tones/` for chime + horns; voices stay in `lightning/`.
4. **Schedule:** Admin remains raw JSON (dynamic forms deferred to a later release).

---

## Part 1 — Updater contract redesign

### Problem

The **running** binary applies updates ([source/update.go](source/update.go)). On 1.1.1→1.1.3, `browardtg.json` was in the tarball but never landed in live `json/`.

Root causes:

- `migrateFromPackageSeeds` only special-cases a few files (not generic “copy if missing”).
- `runSchemaMigrations` uses **running** `AppVersion`, not package target.
- `install_version.json` is stamped to target **before** restart → new binary skips migrations (`from == to`).
- One-click installer copies missing `json/*` seeds; in-app updater does not.

Static MP3 merge via `mergeCopyDir` is already OK.

### Layer A — Pending update handoff (1.1.4+ applying packages)

In [source/update.go](source/update.go):

1. Extract to durable `BaseDir/update_pending/`.
2. Replace `templates/`; merge `static/`.
3. **Generic JSON seeds:** for each file in package `json/`, if live missing and not protected → copy. Use `protectedJSONBasenames()` in [source/migrate.go](source/migrate.go).
4. Write `update_pending/meta.json`: `{ from_version, target_version, seeds_relpath }` with `from_version` **before** any stamp.
5. Swap binary.
6. **Do not** finalize `install_version.json` until new binary completes migrations.

Startup ([source/main.go](source/main.go), [source/migrate.go](source/migrate.go)):

1. If pending meta exists: `migrateFromPackageSeeds(pending/json)` → `runSchemaMigrations(from → AppVersion)`.
2. Success → write `install_version.json`, remove pending.
3. Failure → keep pending, log + Admin error state.

### Layer B — Embed safety net (1.1.3→1.1.4 and repaired units)

Upgrades **into** 1.1.4 may still be applied by **1.1.3** (no pending handoff). Therefore:

1. `go:embed` critical seeds (at least `browardtg.json`).
2. Every startup: idempotent `ensureEmbeddedJSONSeeds()` — missing file → write; catalogs → optional merge; never overwrite protected JSON.
3. Idempotent repairs run **outside** `runSchemaMigrations` early return when `from == to`.

### Updater verification

- Dumb path: `install_version` already 1.1.4, no `browardtg` on disk → startup embed creates catalog; Admin dropdown works.
- Smart path: 1.1.4 applies package with pending meta → migrations `from → target` → version finalized; protected JSON untouched.
- One-click install unchanged; operator `lightning.json` / `operating_hours` edits survive updates.

---

## Part 2 — Sensor telemetry collapse (Thor silent-fail)

### Problem

Feed “healthy” when HTTP OK + `<lightningalert>` + non-stale `<localtime>`. When the **sensor** dies, Thor server may keep publishing XML with **AllClear** and zeros — TARR never fails over and may **unlock** Red Alert.

Metrics: **LHL, DI, AD only** (no FCC/FCCRate). Vendor scales documented in MANUAL.

### Critical safety ordering

**Lock safety is poll 1. Failover is polls 1…N.**

On every active-feed poll, after parse:

1. `stale_localtime` / `telemetry_collapse` checks.
2. If failed: classify `telemetry_collapse` (or stale); **do not** `processConditionChange`; **no** All Clear announce; **no** unlock.
3. `recordFeedFailure` → consecutive failures → failover (Layer B).
4. Healthy path only → `processConditionChange` / unlock.

`authorizeAllClearRelease` / `failover_vote`: collapsed feed never counts as AllClear.

### Detection — immediate floor cliff only

**Floor:** `LHL <= 1` AND `DI == 0` AND `AD == 0` (`lhl` may be 0 or 1; `di` may be `0` or `0.0`).

**Trip when all:**

1. Prior activity (last N samples, default N=3): `LHL >= 3` OR `DI >= 2.3` OR `AD >= 1`.
2. Current sample is full floor.
3. One-step cliff from elevated to floor (no slope/rate rule).

**Do not trip:** calm day with no elevated history (e.g. [FL0108.xml](https://broward.thormobile4.net/mp/FL0108.xml) — working sensor, AllClear + zeros); cold start; gradual multi-poll decline; partial drops without floor.

**Alert independence:** trip on **metric cliff alone** — not tied to `<lightningalert>AllClear</lightningalert>` (future Thor fail-safe into RedAlert still covered).

### Two layers

| Layer | Behavior | Admin |
|-------|----------|--------|
| **A — Condition gate** | Discard feed alert for lock/announce; deny release authority | Always on |
| **B — Failover counter** | Count toward feed switch | `failover.triggers.telemetry_collapse` (default true) |

### Telemetry verification

- FL0108-style steady floor, no history → no trip.
- Red Alert lock + cliff on active feed → lock held, no All Clear audio on failure #1 (even if consecutive threshold is 5).
- After N failures → switch to healthy failover; fake primary AllClear never unlocked in between.
- Disabled Layer B trigger still denies unlock from collapsed feed.
- Live Status: LHL/DI/AD + failure class per feed.

### Primary files

- [source/lightning_trigger.go](source/lightning_trigger.go) — parse, cliff, fetch path, unlock gate
- [source/lightning_monitor.go](source/lightning_monitor.go) — triggers, health, ring buffer
- [source/migrate.go](source/migrate.go) — 1.1.4 migration gates (updater + `telemetry_collapse` default)
- [data/templates/admin.html](data/templates/admin.html) — **one coordinated pass:** failover trigger + Live Status metrics + horn/voice selects (Part 3)
- [data/json/lightning.json](data/json/lightning.json) seed

---

## Part 3 — Audio: `horns-chimes-tones/`

- `static/mp3/horns-chimes-tones/`: `chime.mp3` + `Horn_*.mp3`.
- `static/mp3/lightning/`: `Voice_*.mp3` only (after move).
- Chime paths: [source/main.go](source/main.go), [source/audio.go](source/audio.go), [source/announcement_queue.go](source/announcement_queue.go).
- Horn resolve: prefer new dir, fall back to `lightning/` for upgrades.
- Admin: separate horn vs voice MP3 dropdowns (scan appropriate dirs).
- Package script + MANUAL tree updated.

---

## Part 4 — Schedule UI

- **This release:** raw JSON only in Admin.
- **Later:** dynamic JSON + MP3 discovery forms (not v1.1.4).

---

## Shared touchpoints (edit once)

| Area | Parts |
|------|--------|
| [source/migrate.go](source/migrate.go) | Updater pending repair, embed ensure, 1.1.4 lightning trigger default |
| [data/templates/admin.html](data/templates/admin.html) | Lightning failover triggers, Live Status metrics, MP3 select split |
| [CHANGELOG.md](CHANGELOG.md) / [MANUAL.md](MANUAL.md) / [docs/PI_RELEASE_PACKAGING.md](docs/PI_RELEASE_PACKAGING.md) | End of cycle |

---

## Implementation order

1. **Version** → `1.1.4` in [source/version.go](source/version.go); `migrateLightning114` / updater hooks in migrate.go.
2. **Updater Layer A + B** — update.go, main startup, embed seeds.
3. **Telemetry collapse** — lightning_trigger + lightning_monitor + migrate default + Admin (Lightning section).
4. **Horns-chimes-tones** — static move, path resolve, Admin MP3 selects, listLightningAudioFiles split.
5. **Docs + package** — CHANGELOG, MANUAL, plan status; `scripts/package_pi_release.sh 1.1.4`; GitHub release asset.

Do not ship partial slices without life-safety + updater baseline.

---

## Out of scope

- Thor vendor fixing fail-open AllClear.
- LHL/DI/AD-driven announce (metrics = health only).
- Schedule dynamic forms.
- FCC / FCCRate parsing.

---

## Pre-ship checklist

- [ ] Updater dumb + smart paths verified
- [ ] browardtg on disk after update/start without SCP
- [ ] Telemetry cliff + calm-day non-trip + same-poll lock hold
- [ ] horns-chimes-tones play on Pi + upgrade fallback from old paths
- [ ] Protected JSON not overwritten on update
- [ ] Pi arm64 tarball + UPDATE_PACKAGE.json
