# Plan — v1.1.3

Status: **shipped**  
Prior release: **v1.1.2** (Thor Guard multi-feed, All Clear release authority, public tabs, Admin API docs).  
Package: `releases/TARR_Annunciator_Pi_arm64_v1.1.3.tar.gz`

Do **not** ship piecemeal UI-only commits without the life-safety items below; implement this document end-to-end, then package.

---

## Goals

Critical operator / life-safety fixes after field use of v1.1.2, plus Admin UI clarity so Lightning settings are usable without scrolling through one oversized tab.

---

## In scope

### 1. Admin Thor / lightning state override (all-sensors-failed) — done

**Problem:** If Primary, Failover 1, and Failover 2 are all unreachable or unusable, operators only have Manual Test and Reset today. There is no controlled way to **set** operational lightning state (e.g. force Red Alert lock or force All Clear unlock) when Thor feeds are completely down.

**Intent:** Add Admin controls (alongside existing test / clear-reset) so an authorized admin can **override** Thor/lightning state during total sensor failure.

**Notes:**
- Override must be explicit, logged, and visible in Live Status (e.g. “manual override active”).
- Must not be confused with Manual Test (which plays audio / drills).
- Prefer clear actions such as “Force Red Alert (override)” and “Force All Clear / clear lock (override)” with confirmation.
- Auto-clear or require explicit release when feeds recover (decide in implementation).
- Place controls on the **basic** Lightning Alerts tab (with tests / reset), not buried in Advanced.

### 2. Configurable composite Red Alert triggers — done

**Problem:** Failover sensors may be **1–5 miles** from the physical site. A Warning on two distant failovers may warrant treating the site as Red Alert even though no single feed reports `RedAlert`.

**Intent:** Allow more precise definition of what **enters** Red Alert lock, e.g.:

- If Failover 1 and Failover 2 both parse as **Warning** → treat as **Red Alert** (configurable).
- Other composites as needed (product to specify in Admin UI).

**Notes:**
- Unlock / All Clear release authority (`primary_only` / `failover_vote`) stays separate — this item is about **enter** rules only.
- Preserve v1.1.1/v1.1.2 baseline: Unknown ignore, All Clear acceptance gates, primary_only behavior unless explicitly changed.
- Distant-sensor policy should be Admin-configurable, not hardcoded miles.
- UI for these rules belongs on **Lightning Alerts (Advanced)** (or a clearly labeled subsection there).

### 3. Admin UI theme — match Main (`index.html`) — done

**Problem:** Admin is still light Bootstrap defaults; Main Control uses dark `#1a1a1a` / `#2a2a2a` panels and green `#4CAF50` primary actions.

**Intent:** Restyle Admin so color scheme and button style match Main:

- Page background `#1a1a1a`, panels/cards `#2a2a2a`, borders `#444`, text `#f0f0f0`
- Primary / success actions: solid green `#4CAF50` (hover `#45a049`), same spirit as Main `.ctrl-btn`
- Warning / danger accents aligned with Main (orange / red)
- Dark form controls, tabs, tables, modals, alerts (readable, not inverted illegibly)
- Header links (Main Interface, API Docs, Logout) use the same button language

**Files:** `data/templates/admin.html` (and `api_docs.html` if it should stay consistent when opened from Admin).

### 4. Split Lightning Alerts into Basic + Advanced tabs — done

**Problem:** Admin **Lightning Alerts** is too long vertically, poorly organized, and Save buttons are scattered far apart.

**Intent:**

| Tab | Contents (straightforward / day-to-day) |
|-----|------------------------------------------|
| **⚡ Lightning Alerts** | Live Status; enable + fetch interval + timeout; feeds; All Clear release authority; which conditions may announce; Red Alert policy (preempt / suppress / reminder); Manual Tests, Pin, Reset; **manual override** (item 1); **one sticky save bar** grouping relevant Saves |
| **⚡ Lightning Alerts (Advanced)** | Advanced timing / cooldowns; full failover & failback policy + failure triggers; during-Red-Alert feed-switch / preserve; feed-switch announcement matrix; global condition audio (horn + voice MP3s); **composite Red Alert enter rules** (item 2); sticky save bar for monitor + condition audio |

**Notes:**
- Keep a single set of form field IDs (elements may live on either tab; JS save helpers must still find them).
- Unsaved-edits banner applies to both tabs.
- Dirty-form tracking must cover **both** panels.
- Avoid duplicate element IDs for Save buttons — use shared classes or distinct IDs wired to the same handlers.

### 5. Ship v1.1.3 Pi package — done

- Bump `AppVersion` → **1.1.3**
- CHANGELOG `[1.1.3]` (override, composite triggers, Admin theme, Lightning tab split, api_docs if not already noted)
- Update `docs/plan_v1.1.3.md` status → shipped when done
- Update operator [`MANUAL.md`](../MANUAL.md) for override + composite triggers + new Admin Lightning layout
- Rebuild `releases/TARR_Annunciator_Pi_arm64_v1.1.3` (+ `.tar.gz`)
- Include updated templates (`admin.html`, `api_docs.html`, etc.)

---

## Explicitly out of scope for v1.1.3

### Schedule Management UI

- **Keep raw JSON** schedule editor for v1.1.3.
- Richer dynamic forms + automatic discovery of new JSON / MP3 into Admin forms → **v1.1.4**.

---

## Deferred to v1.1.4 (reminder)

- Schedule / catalog forms that auto-pick up new JSON entries and MP3 files
- Further Admin UX polish beyond theme + Lightning split

---

## Optional hygiene (include if cheap during the same pass)

- Ship `MANUAL.md` inside the Pi package (or document where it lives)
- Gate or remove noisy `DEBUG:` production logs
- Default Admin credential first-login nudge
- Commit gitignore / untrack `FromLiveRaspberryPi` + `fixit_scripts` if not already on the release branch

---

## Suggested implementation order

1. Admin theme (item 3) — visual baseline  
2. Lightning Basic / Advanced split + sticky saves (item 4)  
3. Manual override (item 1) on basic tab  
4. Composite Red Alert enter rules (item 2) in Advanced + runtime  
5. MANUAL + CHANGELOG + version bump + Pi package (item 5)

---

## Acceptance

- [x] With all three feeds failed, Admin can override to a defined lightning/Red Alert state and clear it again, with clear Live Status indication
- [x] Composite enter rule (e.g. both failovers Warning → Red Alert) is configurable and does not weaken All Clear release rules
- [x] Admin colors/buttons match Main Control look and feel
- [x] Lightning basic tab is short/usable; Advanced holds deep settings; saves are grouped (sticky bars), not scattered
- [x] Schedule UI unchanged (still JSON)
- [x] `TARR_Annunciator_Pi_arm64_v1.1.3.tar.gz` builds and documents 1.1.3
- [x] MANUAL updated for new Lightning layout + override + composite triggers

---

## Related docs

- [`docs/plan_v1.1.2.md`](plan_v1.1.2.md) — shipped multi-feed / release authority baseline  
- [`CHANGELOG.md`](../CHANGELOG.md) — add `[1.1.3]` when implementing  
- [`MANUAL.md`](../MANUAL.md) — update when shipping  
- Main UI reference styling: [`data/templates/index.html`](../data/templates/index.html)  
