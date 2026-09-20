# Plan — v1.1.5 (Thor Guard multi-feed, catalog & Admin control)

Status: **deep planning** (catalog seed ready; app wiring **not** started).  
Related shipped work: **v1.1.1** condition announce toggles, persisted `monitor`, defaults (60s poll, hours 09:00–16:00).

**Design mandate:** Thor Guard is a **life-safety** subsystem. v1.1.5 is not “add two backup URLs.” It is a full Admin-controlled multi-feed monitor with explicit failover policy, per-feed announcement control, timing, verification, and live operator visibility — every behavior an on-site admin can inspect and change without editing JSON by hand.

---

## 1. Findings from current codebase (baseline)

### Where Thor Guard config lives

| What | Where |
|------|--------|
| Announcements, Red Alert policy, monitor URL | `json/lightning.json` |
| **Known Broward sensors catalog (new)** | `json/browardtg.json` ← converted from `broward_thor_guard_sensors.csv` |
| Live poller state | In-memory `LightningTrigger` (`source/lightning_trigger.go`) |
| Cached XML snapshots | `xml/` next to the binary |
| MP3 assets | `static/mp3/lightning/` |
| Admin UI today | Single URL + global interval/timeout + global condition announce toggles + Red Alert policy |

### How XML URLs work (important correction)

Feeds are **not** all under `/tp/`. Each Broward sensor has its own **path prefix**:

| Example | URL |
|---------|-----|
| Tradewinds Park `FL0115` | `https://broward.thormobile4.net/tp/FL0115.xml` |
| Brian Piccolo Park `FL0101` | `https://broward.thormobile4.net/bpp/FL0101.xml` |
| Markham Park `FL0108` | `https://broward.thormobile4.net/mp/FL0108.xml` |

Authoritative fields come from the CSV / `browardtg.json`: `id`, `path_prefix`, `display_name`, `url`.  
Always prefer the stored **`url`**; do not reconstruct from `id` alone.

Live XML also contains `<displayname>` and `<uniqueid>` for runtime verification.

### Current poll / lock behavior (single feed)

- Default fetch interval **60s** (1.1.1); reads `<lightningalert>`.
- Fetch/parse failures are **logged and ignored** — no failover, no consecutive-failure counter exposed to Admin.
- All Clear audio/lock-release only after Red Alert.
- Caution / Warning announce **off by default** (1.1.1) to reduce flap chatter.
- Condition announce enable/disable is **global** (one set of toggles for the single URL).
- Red Alert policy (preempt / suppress / reminder) is global — keep that; do not silently fork per-feed lock semantics.

### Gaps vs life-safety Admin mandate

| Gap | Why it matters |
|-----|----------------|
| Only one XML URL | Sensor/host outage = blind until manual URL change |
| Failover not configurable | Operator cannot define *what* counts as failure or *when* to switch |
| No failback policy | Stuck on failover after primary recovers, or flapping if too eager |
| Announce toggles are global only | Failover park may need different Caution/Warning/Unknown policy |
| No per-feed audio mapping | Same MP3s always; cannot identify *which* sensor spoke |
| No displayname / uniqueid check | Silent wrong-feed or stale proxy not detectable in Admin |
| Status UI is thin | Operator cannot see feed health, failure streak, active source |
| Test Fetch is single-URL | Cannot validate primary + each failover independently before go-live |
| Timing is only global interval/timeout | No per-feed timeout, no failover cooldown, no failback dwell |

---

## 2. Goals for v1.1.5

1. Ship **`browardtg.json`** catalog (seed done) and load it in Go for Admin/API.
2. Admin: up to **three feed slots** (primary + failover_1 + failover_2), each with catalog dropdown + Custom URL + enable + Test.
3. **Configurable failover & failback policy** fully editable in Admin (not hardcoded magic numbers only).
4. Parse **`<displayname>`** / **`<uniqueid>`**; show which sensor is driving status; optional mismatch = failure.
5. **Per-feed** announcement enable map + optional per-condition MP3 overrides; global defaults remain as fallback.
6. **Timing** controls: global poll interval + per-feed timeout + failover/failback timing + optional announce cooldown.
7. Full Admin + API parity; additive migrations only; no invented Thor URLs in Go.
8. Operator **status dashboard** rich enough to trust during an incident.

---

## 3. Safety invariants (non-negotiable)

These remain true regardless of Admin knobs (document in UI help text):

1. **Red Alert lock** still enters on Red Alert from the *active* feed; All Clear only releases after Red Alert (existing rule).
2. **Disabling announce audio does not disable monitoring or lock** — status tracking continues.
3. **Manual Test buttons** always play (verification), independent of auto-announce toggles.
4. **Empty primary URL** → monitoring stays stopped (same as 1.1.1); do not invent a default URL in code.
5. **Failover never invents URLs** — only uses Admin-enabled feeds with non-empty URLs.
6. **At most one active feed** drives condition/lock at a time (no “worst of three” merge in v1.1.5 — explicit ordered failover).
7. Saving a dangerous config (all feeds disabled, zero announce for Red Alert, etc.) shows an **Admin warning** but is allowed if operator confirms (site may be in maintenance). Prefer soft-warn + confirm, not silent refuse — except empty URL when enabled.

---

## 4. Catalog: `broward_thor_guard_sensors.csv` → `browardtg.json`

### Source

Repo root: `broward_thor_guard_sensors.csv` (19 Broward parks / sensors).

Columns: `ID`, `Path Prefix`, `Display Name`, `Full Endpoint`.

### Target (seed already generated)

`data/json/browardtg.json` → deployed as `json/browardtg.json` beside the binary.

```json
{
  "schema_version": 1,
  "region": "broward",
  "host": "broward.thormobile4.net",
  "source": "broward_thor_guard_sensors.csv",
  "generated_for": "TARR Annunciator v1.1.5+",
  "sensors": [
    {
      "id": "FL0115",
      "path_prefix": "tp",
      "display_name": "Tradewinds Park",
      "url": "https://broward.thormobile4.net/tp/FL0115.xml"
    }
  ]
}
```

### Regeneration (maintainers)

When the CSV is updated:

```bash
python -c "
import csv, json
from pathlib import Path
rows = list(csv.DictReader(Path('broward_thor_guard_sensors.csv').open(encoding='utf-8-sig')))
sensors = [{
  'id': r['ID'].strip(),
  'path_prefix': r['Path Prefix'].strip(),
  'display_name': r['Display Name'].strip(),
  'url': r['Full Endpoint'].strip(),
} for r in rows]
Path('data/json/browardtg.json').write_text(json.dumps({
  'schema_version': 1,
  'region': 'broward',
  'host': 'broward.thormobile4.net',
  'source': 'broward_thor_guard_sensors.csv',
  'generated_for': 'TARR Annunciator v1.1.5+',
  'sensors': sensors,
}, indent=2) + '\n', encoding='utf-8')
"
```

Optional later: `scripts/generate_browardtg.py` wrapping the above.

### Packaging

- Include `browardtg.json` in slim Pi packages (copy with other `data/json` seeds).
- Catalog update policy: merge by `sensors[].id` (add/update known; never delete local-only entries if Admin add-local is added later).

---

## 5. Failover / failback policy (Admin-owned)

Failover is **ordered**: try `feeds` in array order among `enabled && url != ""`. Only one feed is **active**.

### 5.1 What counts as a “failure” (each independently toggleable in Admin)

| Failure class | Default ON? | Description |
|---------------|-------------|-------------|
| `http_error` | yes | Network error, DNS, TLS, timeout |
| `http_status_not_ok` | yes | Non-2xx response |
| `empty_body` | yes | Zero-length body |
| `encoding_error` | yes | UTF-16/UTF-8 conversion failure |
| `missing_lightningalert` | yes | Tag missing / empty (today silently returns) |
| `unknown_condition` | **no** | XML says `Unknown` — often noise; optional treat as failure for failover |
| `displayname_mismatch` | **no** | Live `<displayname>` ≠ feed `expected_displayname` (when expected set) |
| `uniqueid_mismatch` | **no** | Live `<uniqueid>` ≠ feed `expected_uniqueid` (when expected set) |
| `stale_xml` | **no** | Optional: if XML has a timestamp field we can parse later — **defer unless we confirm Thor schema**; placeholder in plan only |

Admin UI: checklist under **Failover triggers**. Saving writes `monitor.failover.triggers`.

### 5.2 When to switch away from active feed

| Setting | Default | Meaning |
|---------|---------|---------|
| `consecutive_failures` | `3` | N classified failures in a row on active feed → promote next enabled feed |
| `failure_window_seconds` | `0` (off) | If >0, failures must occur within this window (else streak resets) — optional advanced |
| `min_dwell_on_feed_seconds` | `0` | Do not leave a feed until it has been active at least this long (anti-flap after switch) |

### 5.3 Failback to a higher-priority feed

| Setting | Default | Meaning |
|---------|---------|---------|
| `failback_mode` | `prefer_primary` | Options: `sticky` (stay on failover until it fails), `prefer_primary` (probe primary and return when healthy), `prefer_highest_priority` (same, for any higher slot) |
| `failback_after_successes` | `2` | Consecutive *successful* polls of the preferred higher feed before switching back |
| `failback_probe_interval_seconds` | `0` | `0` = probe on same poll cadence while on failover; `>0` = probe preferred feed every N seconds |
| `announce_on_feed_switch` | `false` | If true, optional short operator/log event; **do not** play public park audio by default on silent failover |

### 5.4 Behavior during Red Alert lock

| Setting | Default | Meaning |
|---------|---------|---------|
| `allow_failover_during_red_alert` | `true` | Keep monitoring available if primary dies mid-alert |
| `require_allclear_from_same_feed` | `false` | If true, All Clear only accepted from the feed that declared Red Alert; if false, All Clear from current active feed is enough |
| `preserve_condition_across_failover` | `true` | When switching feeds, do not treat first poll as a “condition change” that re-announces the same Red Alert unless condition string differs |

Document these carefully in Admin — wrong defaults cause either missed All Clear or duplicate Red Alert audio.

### 5.5 Total outage (all feeds failed)

| Setting | Default | Meaning |
|---------|---------|---------|
| `on_all_feeds_failed` | `hold_last_condition` | Options: `hold_last_condition`, `force_unknown_status` (status only; announce Unknown only if enabled) |
| `all_failed_status_label` | visible in Admin | Show banner: “All Thor feeds unreachable” |

**Never** auto-All-Clear on outage.

---

## 6. Proposed `lightning.json` shape (additive, comprehensive)

Legacy `monitor.url` migrates into `feeds[0].url`. New keys are additive; old single-URL clients ignored after migration.

```json
{
  "monitor": {
    "enabled": true,
    "fetch_interval": 60,
    "timeout": 30,
    "failover": {
      "consecutive_failures": 3,
      "failure_window_seconds": 0,
      "min_dwell_on_feed_seconds": 0,
      "failback_mode": "prefer_primary",
      "failback_after_successes": 2,
      "failback_probe_interval_seconds": 0,
      "announce_on_feed_switch": false,
      "allow_failover_during_red_alert": true,
      "require_allclear_from_same_feed": false,
      "preserve_condition_across_failover": true,
      "on_all_feeds_failed": "hold_last_condition",
      "triggers": {
        "http_error": true,
        "http_status_not_ok": true,
        "empty_body": true,
        "encoding_error": true,
        "missing_lightningalert": true,
        "unknown_condition": false,
        "displayname_mismatch": false,
        "uniqueid_mismatch": false
      }
    },
    "feeds": [
      {
        "id": "primary",
        "label": "Primary",
        "enabled": true,
        "sensor_id": "FL0115",
        "source": "catalog",
        "expected_displayname": "Tradewinds Park",
        "expected_uniqueid": "",
        "url": "https://broward.thormobile4.net/tp/FL0115.xml",
        "timeout_seconds": 0,
        "announce": {
          "inherit_global": true,
          "RedAlert": true,
          "Warning": false,
          "Caution": false,
          "AllClear": true,
          "Unknown": false
        },
        "audio_by_condition": {
          "RedAlert": "",
          "Warning": "",
          "Caution": "",
          "AllClear": "",
          "Unknown": ""
        }
      },
      {
        "id": "failover_1",
        "label": "Failover 1",
        "enabled": false,
        "sensor_id": "",
        "source": "custom",
        "expected_displayname": "",
        "expected_uniqueid": "",
        "url": "",
        "timeout_seconds": 0,
        "announce": { "inherit_global": true },
        "audio_by_condition": {}
      },
      {
        "id": "failover_2",
        "label": "Failover 2",
        "enabled": false,
        "sensor_id": "",
        "source": "custom",
        "expected_displayname": "",
        "expected_uniqueid": "",
        "url": "",
        "timeout_seconds": 0,
        "announce": { "inherit_global": true },
        "audio_by_condition": {}
      }
    ]
  },
  "condition_announce": {
    "RedAlert": true,
    "Warning": false,
    "Caution": false,
    "AllClear": true,
    "Unknown": false
  },
  "announce_timing": {
    "min_seconds_between_same_condition": 0,
    "min_seconds_between_any_lightning_announce": 0
  },
  "lightning_announcements": [],
  "red_alert_policy": {},
  "displayname_overrides": [
    {
      "match_displayname": "Tradewinds Park",
      "enabled": false,
      "audio_by_condition": {
        "Caution": "thor_caution.mp3",
        "Warning": "thor_warning.mp3",
        "RedAlert": "thor_red_alert.mp3",
        "AllClear": "thor_all_clear.mp3"
      }
    }
  ]
}
```

### Field notes

| Field | Rule |
|-------|------|
| `feeds[].timeout_seconds` | `0` = use `monitor.timeout` |
| `feeds[].announce.inherit_global` | `true` → use top-level `condition_announce`; `false` → use per-feed flags |
| `feeds[].audio_by_condition.*` | Empty string → fall through to `displayname_overrides` then global `lightning_announcements` / built-in paths |
| `feeds[].source` | `catalog` \| `custom`; URL edit after catalog pick → `custom` and clear `sensor_id` (or keep id but mark custom — prefer clear for honest dropdown) |
| `announce_timing` | Global anti-spam; `0` = off (current behavior) |

### Migration from 1.1.1

1. If `monitor.feeds` missing and `monitor.url` present → create `feeds[0]` with that URL, `id: primary`, `enabled: monitor.enabled`.
2. Match URL against `browardtg.json` to set `sensor_id`, `expected_displayname`, `source: catalog`.
3. Copy existing global announce toggles into `condition_announce` if not already present (1.1.1 already uses announcement `enabled` flags — keep that path; add explicit `condition_announce` object only if cleaner for Admin — **prefer one source of truth**: either keep announcement `enabled` as today *or* introduce `condition_announce` and sync both on save; decide in implementation to avoid dual-write bugs. **Recommendation:** keep existing announcement-enabled path as canonical; Admin “global condition announce” continues to write those; per-feed `announce` is an optional override layer evaluated at play time.
4. Insert default `monitor.failover` block if missing.
5. Never overwrite operator URL / enabled flags on update package merge.

---

## 7. Announce resolution order (play time)

When condition changes on the **active** feed and auto-announce is allowed:

1. If `preserve_condition_across_failover` and this is first successful poll after switch with **same** condition string → **skip announce** (still update health).
2. Resolve **whether** to announce:
   - If active feed `announce.inherit_global` → global condition enable (existing).
   - Else → per-feed `announce[Condition]`.
3. Resolve **which MP3**:
   - Active feed `audio_by_condition[Condition]` if non-empty.
   - Else first enabled `displayname_overrides` where `match_displayname` equals live displayname.
   - Else existing global lightning announcement / queue builder paths.
4. Apply `announce_timing` cooldowns if configured.
5. Queue via existing announcement manager (Red Alert preempt/reminder unchanged).

---

## 8. Admin UI — comprehensive Lightning panel layout

Keep existing tab **Lightning Alerts**. Expand into clear sections (scrollable; do not hide safety controls behind modals-only).

### A. Live Status (always first, large)

Show at a glance:

- Monitoring enabled / running
- **Active feed:** label + sensor_id + live `<displayname>` + URL (truncated)
- **Active condition** + time since change
- Red Alert lock on/off + reminder next-due
- **Feed health table** (all 3 slots):

  | Feed | Enabled | Role | Last OK | Failures | Last error | Live displayname | Match? |
  |------|---------|------|---------|----------|------------|------------------|--------|

- Banner if all feeds failed / using failover / displayname mismatch
- Buttons: Refresh status · Reset THOR state (existing)

### B. Global monitor timing

- Enable monitoring
- Fetch interval (seconds) — global poll cadence
- Default request timeout (seconds)
- Announce timing cooldowns (advanced, collapsed by default)

### C. Failover & failback policy (new card, border-warning)

- Consecutive failures before failover
- Failback mode select + successes required + probe interval
- Checkboxes for each **failure trigger**
- Red Alert interaction toggles (`allow_failover_during_red_alert`, `require_allclear_from_same_feed`, `preserve_condition_across_failover`)
- On-all-failed behavior select
- Save Policy button (independent save OK, like Red Alert policy today)
- Short help: “Failover changes *which XML is trusted*, not the meaning of Red Alert.”

### D. Feed slots (×3 cards: Primary / Failover 1 / Failover 2)

Each slot:

```
[ ✓ Enabled ]
[ Sensor: ▼ Tradewinds Park (FL0115) ]
        ├─ — Custom URL… —
        └─ … catalog …

[ XML URL: https://… ]          ← always editable
[ Expected displayname: … ]     ← auto-filled from catalog; editable
[ Expected uniqueid: … ]        ← optional
[ Timeout override (0=global): ]

[ Test Fetch for this feed ]

── Announcements for this feed ──
( ) Inherit global condition toggles
( ) Custom for this feed
    [✓] RedAlert  [ ] Warning  [ ] Caution  [✓] AllClear  [ ] Unknown

── Optional MP3 overrides (filename in static/mp3/lightning/) ──
RedAlert: [____________]  Warning: […]  Caution: […]  AllClear: […]  Unknown: […]
```

Behavior:

| Action | Result |
|--------|--------|
| Pick catalog sensor | Sets `url`, `sensor_id`, `expected_displayname`, `source: catalog` |
| Pick Custom URL… | Clears `sensor_id`; `source: custom`; URL free-typed |
| Edit URL after catalog | Treat as custom for dropdown honesty |
| Test Fetch | POST with that feed’s URL (and show parsed alert + displayname + uniqueid) |
| Disable feed | Skipped in failover order; if active feed disabled on save → re-pick highest enabled |

Default primary: **Tradewinds Park / FL0115**. Failovers start disabled with empty URL.

### E. Global Condition Announcements (existing card)

Keep current global toggles — they apply when feed inherits. Label: “Default for feeds set to Inherit.”

### F. Red Alert Policy (existing card)

Unchanged semantics; still global.

### G. Manual tests (existing sidebar)

Keep Test Red Alert / Warning / Caution / All Clear / Reminder / Reset.  
Add: **Force active feed** dropdown (Admin-only) for drill: temporarily pin primary/failover without waiting for failures — with clear “PINNED — not auto” badge and Unpin button. (Life-safety drills need this.)

### H. Confirmations / warnings

On Save, soft-confirm if:

- Monitoring enabled but fewer than 1 feed with URL
- Red Alert announce disabled globally and on all custom feeds
- Failover enabled (slot 2/3 on) but `consecutive_failures` < 2
- `require_allclear_from_same_feed` true while failovers configured (explain All Clear risk)

---

## 9. API surface

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/admin/lightning/status` | Expand: feeds[], active_feed_id, active_displayname, active_uniqueid, failover state, health, known_sensors, policy |
| POST | `/admin/lightning/config` | Accept full monitor (feeds + failover + timing); validate; persist; hot-apply |
| POST | `/admin/lightning/test` | Body may include `url` **or** `feed_id`; return alert + displayname + uniqueid |
| GET | `/admin/lightning/sensors` | Catalog from `browardtg.json` (or embed in status) |
| POST | `/admin/lightning/active-feed` | Optional pin/unpin for drills |
| existing | test-condition, test-reminder, reset | unchanged |

Public `/lightning_status` should expose **safe** fields: active condition, red alert, active displayname (no need to expose all failover URLs publicly unless already public — prefer minimal).

---

## 10. Runtime poller design (Go)

### State machine (conceptual)

```
SELECT active = first enabled feed with URL
LOOP every fetch_interval:
  IF pinned: use pinned feed
  ELSE IF failback_mode needs probe: probe preferred higher feed; maybe failback
  FETCH active (timeout = feed override or global)
  IF success:
    reset failure streak; update displayname/uniqueid; maybe apply mismatch triggers
    process condition (with preserve_across_failover rules)
  IF failure (classified & trigger enabled):
    streak++
    IF streak >= consecutive_failures AND past min_dwell:
      switch to next enabled feed (or all-failed handler)
```

### Logging

- Every feed switch: `Lightning: active feed changed primary → failover_1 (reason=consecutive_failures:3, last_error=...)`
- Rate-limit repetitive fetch errors (keep existing 15-minute dedupe) **per feed id**
- Persist last error strings in status JSON for Admin (not only logs)

### XML cache

- Save under `xml/{feed_id}_{basename}.xml` so failover snapshots do not overwrite each other

---

## 11. Implementation outline (when approved — not started)

### Go

- `loadBrowardTGCatalog()`; migration `ensureMonitorFeeds()` + `ensureFailoverPolicy()`
- Extend `LightningTrigger` for multi-feed + health + pin
- Announce resolution helper (global / per-feed / override)
- Expand status + config handlers; validate ranges (interval ≥ 30, timeout ≥ 5, failures ≥ 1, etc.)

### Admin (`admin.html`)

- Rebuild Lightning panel per §8
- Status poll includes feed health table
- Independent Save for: Global timing · Failover policy · Each feed (or one “Save all lightning” — prefer **one Save all** plus existing separate Saves for Red Alert / global announce to limit foot-guns; exact UX TBD at implement time but must not strand half-applied policy)

### Packaging / CHANGELOG

- Ship `browardtg.json`; seed `lightning.json` with `feeds` + default failover
- CHANGELOG `[1.1.5]` when shipped

---

## 12. Test / acceptance matrix (life-safety)

### Config & catalog

- [ ] `browardtg.json` loads; Admin dropdown lists all CSV sensors with correct prefixed URLs
- [ ] Custom URL works; catalog pick fills URL + expected displayname
- [ ] Additive migration from 1.1.1 single `monitor.url`

### Failover

- [ ] Primary HTTP fail × N → switches to failover_1 within policy
- [ ] Missing `<lightningalert>` counts when trigger enabled; ignored when disabled
- [ ] `unknown_condition` trigger off by default (no flap on Unknown)
- [ ] Displayname mismatch failover only when trigger + expected set
- [ ] Failback `prefer_primary` returns after M successes; `sticky` does not
- [ ] All feeds down → hold last condition; **never** auto All Clear
- [ ] Failover during Red Alert allowed/denied per toggle; All Clear same-feed rule honored

### Announce

- [ ] Inherit global vs per-feed custom announce matrix
- [ ] Per-feed MP3 override used when set
- [ ] displayname_overrides used when feed audio empty
- [ ] Preserve condition across failover does not re-blast Red Alert audio
- [ ] Manual tests always play

### Admin visibility

- [ ] Status shows active feed, live displayname, per-feed failure streak / last error
- [ ] Test Fetch per feed returns alert + displayname + uniqueid
- [ ] Soft-confirm warnings for dangerous saves
- [ ] Optional pin feed for drill + clear unpin

### Regression

- [ ] Red Alert preempt / suppress / reminder unchanged when single primary only
- [ ] Operating hours still do not block lightning
- [ ] No Thor URL hardcoded in Go binary

---

## 13. Out of scope (unless pulled forward)

- Regions other than Broward (`browardtg.json` name intentional)
- More than three feeds
- Parallel “worst of N” voting across feeds
- Auto-scraping Thor Mobile for new sensors
- Parsing Thor XML wall-clock for `stale_xml` until schema confirmed
- Admin editing of `browardtg.json` catalog itself (CSV regenerate remains path)
- Separate public PA message on silent feed switch

---

## 14. Open decisions (resolve before coding)

1. **Canonical announce enable source:** keep `lightning_announcements[].enabled` only, or add parallel `condition_announce` object? → **Recommend:** keep announcements array as write target for global; per-feed overrides only in `feeds[].announce`.
2. **Pin active feed for drills:** in v1.1.5 or follow-up? → **Recommend: include** (cheap, high drill value).
3. **`failure_window_seconds`:** ship in schema now (default 0) even if UI advanced-collapsed?
4. **Public `/lightning_status`:** expose active displayname + “on_failover” boolean?
5. **Save UX:** one Save All vs section saves — pick one primary path to avoid partial policy.

---

## 15. Done so far (prep)

- [x] Document URL path-prefix reality (not always `/tp/`).
- [x] Convert CSV → [`data/json/browardtg.json`](../data/json/browardtg.json) (19 sensors).
- [x] Expand plan for life-safety Admin: failover triggers, failback, per-feed announce/audio/timing, status, API, invariants, test matrix.
- [ ] Resolve open decisions (§14).
- [ ] Implement (explicitly gated — do not start until approved).
