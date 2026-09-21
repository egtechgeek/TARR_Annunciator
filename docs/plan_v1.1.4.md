# Plan — v1.1.4

Status: **deferred** (notes only — implement later)  
Prior release: **v1.1.3**  
Do not block packaging or shipping of v1.1.3 on these items.

---

## Goals

Organizational and Admin UX follow-ups that were consciously left out of v1.1.3.

---

## In scope (later)

### 1. Reorganize horns / chimes / tones under `static/mp3/horns-chimes-tones/`

**Intent:** Improve audio layout by creating:

```
static/mp3/horns-chimes-tones/
```

to house:

- Alert horns currently under `static/mp3/lightning/` (e.g. `Horn_RedAlert.mp3`, `Horn_AllClear.mp3`)
- Station announcement pre-chime currently at `static/mp3/chime.mp3`

Leave **voice** clips in `static/mp3/lightning/` (`Voice_*.mp3`).

**Complexity (from v1.1.3 assessment):** Moderate — doable, not a free move.

| Piece | Effort | Notes |
|-------|--------|--------|
| Chime path only | Low | Hardcoded in a few places (`main.go`, `audio.go`, `announcement_queue.go`) as `MP3Dir/chime.mp3`, plus package manifests / MANUAL |
| Horns out of `lightning/` | Moderate | JSON stores basenames only; Admin MP3 selects list everything in `lightning/`; path resolve + select split needed |
| Field upgrades | Main risk | Devices already have `Horn_*.mp3` beside voices — prefer new dir, fall back to `lightning/` (same idea as 1.1.2 rename migration) |

**Suggested shape when implemented:**

- New dir for `chime.mp3` + `Horn_*.mp3`
- Voices stay in `lightning/`
- Keep basename config with dual-path lookup for upgrades
- Split Admin horn vs voice dropdowns (or scan both dirs cleanly)
- Docs / package contents / staging copies

Rough scope: ~half day with migration + Admin select split + docs; chime-only would be under an hour.

---

### 2. Schedule UI — keep raw JSON for now

**Decision for v1.1.3 / until 1.1.4 work:** Schedule Admin UI remains **raw JSON**.

**Later (v1.1.4+):** Replace or augment raw JSON with a better form UI that can **dynamically / automatically** pick up new JSON entries and MP3 assets (similar spirit to Lightning’s filesystem-backed MP3 selects), so operators do not hand-edit schedule JSON when adding trains, destinations, audio, etc.

Until that dynamic form system exists, do not invest in a half-baked Schedule form rewrite.

---

## Out of scope for this note

- Any life-safety Lightning behavior changes
- Packaging / shipping of v1.1.3

---

## When ready to implement

1. Land `horns-chimes-tones` move + upgrade fallback + Admin select split.
2. Design Schedule dynamic forms (JSON schema + MP3 discovery), then replace raw JSON editor.
3. Version bump, CHANGELOG, MANUAL, Pi package.
