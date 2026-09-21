# TARR Annunciator — Operator Manual

**Product:** TARR Annunciator (`tarr-annunciator`)  
**Current version:** v1.1.3  
**Audience:** Station staff, supervisors, and site operators who need to run and configure the system day to day — not software developers.

This manual explains **what the system does**, **where you click**, **what each setting means**, and **what happens when something goes wrong**. Read sections in order the first time; afterward use the table of contents to jump to a topic.

---

## Table of contents

1. [What is TARR Annunciator?](#1-what-is-tarr-annunciator)
2. [Words you will see (glossary)](#2-words-you-will-see-glossary)
3. [Starting, stopping, and opening the screens](#3-starting-stopping-and-opening-the-screens)
4. [Two web screens: Main UI vs Admin](#4-two-web-screens-main-ui-vs-admin)
5. [Main Control page (public UI)](#5-main-control-page-public-ui)
6. [Logging into Admin](#6-logging-into-admin)
7. [Admin — System Status](#7-admin--system-status)
8. [Admin — Software Updates](#8-admin--software-updates)
9. [Admin — Application Control (restart)](#9-admin--application-control-restart)
10. [Admin — Audio Controls](#10-admin--audio-controls)
11. [Admin — Bluetooth](#11-admin--bluetooth)
12. [Admin — Time Sync & Operating Hours](#12-admin--time-sync--operating-hours)
13. [Admin — Schedule Management (automatic announcements)](#13-admin--schedule-management-automatic-announcements)
14. [Admin — Announcement Queue & Emergencies](#14-admin--announcement-queue--emergencies)
15. [Admin — User & API Management](#15-admin--user--api-management)
16. [Admin — Track Layout](#16-admin--track-layout)
17. [Admin — Lightning Alerts (Thor Guard) — overview](#17-admin--lightning-alerts-thor-guard--overview)
18. [Lightning — Live Status](#18-lightning--live-status)
19. [Lightning — Global Monitor Timing](#19-lightning--global-monitor-timing)
20. [Lightning — Feeds (Primary / Failover 1 / Failover 2)](#20-lightning--feeds-primary--failover-1--failover-2)
21. [Lightning — Failover & Failback Policy](#21-lightning--failover--failback-policy)
22. [Lightning — During Red Alert settings](#22-lightning--during-red-alert-settings)
23. [Lightning — All Clear release authority](#23-lightning--all-clear-release-authority)
24. [Lightning — Feed-switch Announcements](#24-lightning--feed-switch-announcements)
25. [Lightning — Global Condition Announcements](#25-lightning--global-condition-announcements)
26. [Lightning — Global Condition Audio (horn + voice)](#26-lightning--global-condition-audio-horn--voice)
27. [Lightning — Red Alert Policy](#27-lightning--red-alert-policy)
28. [Lightning — Manual Tests, Pin, Reset, and Manual Override](#28-lightning--manual-tests-pin-reset-and-manual-override)
29. [How Red Alert / All Clear really work (rules)](#29-how-red-alert--all-clear-really-work-rules)
30. [Announcement types and priority](#30-announcement-types-and-priority)
31. [Sound files (MP3 folders)](#31-sound-files-mp3-folders)
32. [Where settings are stored on the Pi](#32-where-settings-are-stored-on-the-pi)
33. [API Docs (for integrations)](#33-api-docs-for-integrations)
34. [Everyday checklists](#34-everyday-checklists)
35. [Troubleshooting](#35-troubleshooting)
36. [Safety reminders](#36-safety-reminders)

---

## 1. What is TARR Annunciator?

TARR Annunciator is the **public-address (PA) computer program** that runs on a **Raspberry Pi** at the station. It:

- Plays **train (station) announcements** (train number, direction, destination, track)
- Plays **safety** and **promo** messages
- Plays **emergency** messages when an operator triggers them
- Watches **Thor Guard lightning sensors** on the internet and can lock the station into a **Red Alert** mode with repeating reminders until **All Clear**
- Runs on a schedule during **operating hours**
- Lets you manage almost everything from a web browser

You do **not** need to edit computer code. Almost everything is controlled from:

| Screen | Address (on the Pi or your network) | Who uses it |
|--------|-------------------------------------|-------------|
| **Main Control** | `http://localhost:8080` or `http://<pi-ip>:8080` | Operators playing day-to-day announcements |
| **Admin** | `http://localhost:8080/admin` | Supervisors / trained operators configuring the system |

`localhost` means “this same computer.” If you open the page from another PC on the same network, use the Pi’s IP address instead (ask your IT/site tech if you do not know it).

---

## 2. Words you will see (glossary)

| Term | Plain meaning |
|------|----------------|
| **Raspberry Pi** | The small computer that runs TARR Annunciator, usually near the PA amp |
| **PA / announce** | Sound played over the station speakers |
| **Queue** | A waiting line of announcements. Higher priority items play first |
| **Scheduler** | The automatic “play this at these times” system (cron schedule) |
| **Operating hours** | The daily open/close window when scheduled (automatic) announcements are allowed |
| **Admin** | The password-protected settings website |
| **Thor Guard** | Broward’s lightning monitoring service. The Pi downloads a small XML status file from each sensor |
| **Feed** | One Thor Guard sensor URL the Pi is watching (Primary, Failover 1, Failover 2) |
| **Active feed** | The one feed currently trusted for lightning status |
| **Failover** | Automatically switching to a backup feed when the active one fails |
| **Failback** | Switching back toward Primary when it is healthy again |
| **Condition** | Thor’s status word: RedAlert, Warning, Caution, AllClear, or Unknown |
| **Red Alert lock** | Special “danger mode” after a Red Alert. Many normal announcements are blocked until All Clear is accepted |
| **All Clear release** | The rules for *who* is allowed to unlock Red Alert (Primary only, or a vote of both failovers) |
| **Horn** | Short attention tone MP3 played *before* a spoken lightning message |
| **Voice / announce MP3** | The spoken lightning message file |
| **Pin (feed)** | Temporarily force one feed to stay active (for drills) |
| **Migration / update** | When you install a new version, the app carefully adds new settings without wiping your choices |

---

## 3. Starting, stopping, and opening the screens

### 3.1 After a normal one-click install

On the Pi, as user `pi`, these helper scripts are usually in the home folder:

| Action | Command to type |
|--------|-----------------|
| Start | `~/tarr-start.sh` |
| Stop | `~/tarr-stop.sh` |
| Restart | `~/tarr-restart.sh` |
| Watch live console (advanced) | `~/tarr-view.sh` or `screen -r tarr-annunciator` |

The program files live here:

```text
~/TARR_Annunciator_RaspberryPi_ARM64/
```

Inside that folder you will see things like: `tarr-annunciator` (the program), `templates/`, `static/`, `json/`, `logs/`.

### 3.2 Is it running?

1. Open a browser to `http://localhost:8080`
2. If the Main Control page loads, the program is running
3. If the page will not load, start it with `~/tarr-start.sh`, wait ~10 seconds, try again

### 3.3 Auto-start

Many installs also start the app when the Pi boots (via Screen / login scripts). If the Pi reboots after a power blip, wait a minute, then check the web page before restarting manually.

---

## 4. Two web screens: Main UI vs Admin

### Main Control (`/`)

- **No login** for normal announcement buttons
- Used for playing Station / Promo / Safety announcements
- Shows a simple lightning dashboard and queue pause/resume/stop
- Shows a **red banner** when Red Alert is locked

### Admin (`/admin`)

- **Requires login**
- Used for volume, schedules, hours, users, lightning policy, updates, emergencies, etc.
- This is where almost all “settings” live

**Rule of thumb:** If you are *playing* an announcement, use Main. If you are *changing how the system behaves*, use Admin.

---

## 5. Main Control page (public UI)

Open: `http://localhost:8080` (or the Pi’s IP on port **8080**).

### 5.1 Header status line

At the top you see something like:

- **Audio System:** Active or Unavailable  
- **Scheduler:** e.g. `Running (within operating hours)` or `Paused (outside operating hours)`  
- Link to **Admin Panel**

These update automatically about every **5 seconds**. They match Admin’s System Status scheduler wording.

| What you see | Meaning |
|--------------|---------|
| Audio System: **Active** | The program believes it can play sound |
| Audio System: **Unavailable** | Audio failed to initialize (common on a test PC without speakers; on a Pi, check cables/device in Admin → Audio) |
| Scheduler: **Running (within operating hours)** | Automatic scheduled announcements are allowed right now |
| Scheduler: **Paused (outside operating hours)** | Automatic schedule is paused; lightning / emergency / manual still work |
| Scheduler: **Running (24/7)** | Operating-hours gate is turned off |

### 5.2 Red Alert banner

If Thor Guard Red Alert is **locked**, a red banner appears:

> THOR GUARD RED ALERT ACTIVE — station, promo, safety, and scheduled announcements are suspended until All Clear

This is intentional life-safety behavior (when suppress is enabled in Admin).

### 5.3 Tabs on Main Control

| Tab | What it is for |
|-----|----------------|
| **Dashboard** | Lightning summary + pause / resume / stop queue buttons |
| **Station** | Play a train announcement (train, direction, destination, track) |
| **Promo** | Play a promotional message |
| **Safety** | Play a safety message in a chosen language |

### 5.4 Dashboard — Lightning Alert Status

Shows (auto-refreshing):

- **Active feed** (which Thor sensor is currently trusted)
- Sensor **displayname** when known
- Whether the system is **on failover**
- Current **condition** badge (RedAlert / Warning / Caution / AllClear / Unknown)
- Whether the **LOCK** is on
- A small table per feed: Enabled, **Last Status**, Displayname

**Last Status** = last condition successfully read from that feed’s XML (not a network-error message).

### 5.5 Dashboard — Announcement Controls

| Button | What it does |
|--------|----------------|
| **Pause Announcement Queue** | Stops starting the *next* queued item (does not erase the queue) |
| **Resume All Announcements** | Allows the queue to play again |
| **Stop Current Announcement** | Stops whatever is playing *right now* |

Use Pause if you need quiet for a moment without changing schedules. Remember to **Resume** afterward.

### 5.6 Playing Station / Promo / Safety

1. Open the matching tab  
2. Choose the dropdown values  
3. Press the green play button  

The announcement goes into the **queue** and plays when its turn comes (and when not suppressed by Red Alert).

---

## 6. Logging into Admin

1. Open `http://localhost:8080/admin`  
2. You will be sent to the login page if not already signed in  
3. Enter the **username** and **password** provided by your site administrator  
4. After login you see tabs across the top of Admin  

**Change passwords** under **User & API Management** (do not leave factory/default passwords on a live station).

**Session timeout:** After a period of no activity (often 60 minutes, configurable), Admin may ask you to log in again.

**Logout:** Use the logout control / visit `/admin/logout` when finished on a shared computer.

---

## 7. Admin — System Status

**Tab:** 📊 System Status

At a glance:

| Field | Meaning |
|-------|---------|
| **App Version** | Software version currently running (e.g. `v1.1.2`) |
| **Platform** | Computer type (should be linux/arm64 on the Pi) |
| **Volume** | Current volume percent |
| **Selected Device** | Which sound output is selected |
| **Clock** | Time the app is using |
| **Scheduler** | Same operating-hours label as the Main page |

Also on this tab:

- **Software Updates** (next section)
- **Application Control** / System Information
- Link to the Main web interface

---

## 8. Admin — Software Updates

**Location:** System Status → Software Updates

### What an update does

1. Checks GitHub for published Pi packages  
2. Lets you **choose** which version to install (you are not forced to take the newest if several are listed)  
3. Downloads a slim package  
4. Verifies the program file checksum  
5. Updates templates and sound files carefully  
6. Adds any **new** settings your older config was missing (without wiping your choices)  
7. Replaces `tarr-annunciator` and restarts  

### How to update (step by step)

1. Log into Admin  
2. Go to **System Status**  
3. Click **Check for Updates**  
4. Wait for the release list to fill  
5. In **Target release**, pick the version your site wants (usually the newest approved release, e.g. `v1.1.2`)  
6. Click **Download and Install Selected**  
7. Watch the log area for progress  
8. Wait for the app to come back; hard-refresh the browser (Ctrl+F5)  
9. Confirm **App Version** shows the new number  

### What updates do *not* do

- They do **not** wipe your train list, passwords, or lightning feed URLs  
- They do **not** delete old leftover sound files on disk (they add the new ones)  
- They **do** remap known old lightning sound *names* in settings when upgrading to 1.1.2  

### If an update fails

- Read the update log text on the page  
- Confirm the Pi can reach the internet / GitHub  
- You can try again later; a backup folder named like `update_backup_...` may exist under the install directory  

---

## 9. Admin — Application Control (restart)

**Location:** System Status → Application Control

| Control | Use when |
|---------|----------|
| **Restart Application** | Settings need a clean reload, or the UI seems stuck |
| **Refresh Info** | Refresh uptime / memory / version display |

Restart briefly interrupts announcements. Prefer quiet moments if possible.

---

## 10. Admin — Audio Controls

**Tab:** 🔊 Audio Controls

### Volume

- Drag the **Volume** slider (0–100%)  
- Changes are saved and reapplied after reboot  

### Audio output device

1. Open **Audio Output Device**  
2. Pick the correct speakers / amp / USB device  
3. Use **Redetect** if you plugged something in after boot  
4. Click **Test Audio** and listen at the platform speakers (not only on a desk headphone if those are a different device)

### If sound is wrong

- Wrong device selected → choose the correct one and test again  
- Volume at 0 → raise it  
- Pi mixer / PipeWire issues → note the mixer help text on the page; a site tech may need to fix OS audio  

---

## 11. Admin — Bluetooth

**Location:** under Audio Controls → Bluetooth Functions

Used only if your site routes audio through Bluetooth speakers/adapters.

Typical flow:

1. **Scan** for devices  
2. Pair / select a paired device  
3. Optionally enable auto-pair behavior if your site uses it  
4. Test audio after pairing  

If you do not use Bluetooth, you can ignore this section.

---

## 12. Admin — Time Sync & Operating Hours

**Tab:** ⏰ Schedule Management (top section: Time Sync & Operating Hours)

### Why this matters

The scheduler uses the **app clock**. If the clock is wrong, automatic announcements play at the wrong time, and “outside operating hours” may be wrong.

### Date & Time panel

| Setting | Meaning |
|---------|---------|
| **App clock** | Time the program is using |
| **System clock** | The Pi’s OS clock |
| **Last NTP sync** | Last successful internet time check |
| **Enable time sync** | App periodically syncs from NTP servers |
| **Allow setting system clock** | Also try to adjust the OS clock (may fail without privileges; app still keeps its own corrected time) |
| **Sync Time Now** | Force a sync immediately |

### Operating Hours panel

| Setting | Meaning |
|---------|---------|
| **Pause scheduled announcements outside operating hours** | When checked, the schedule is silent outside hours |
| **Timezone** | Usually `America/New_York` for Broward |
| **Default open / close** | Used when applying defaults to days |
| **Per-day rows** | Enable/disable each weekday and set that day’s open/close |
| **Apply default hours to all days** | Copies the default open/close onto every day |
| **Save Hours & Time Settings** | **Required** — nothing sticks until you save |

**Important:** Lightning, emergency, and **manual** announcements still run when the scheduler is paused for hours. Only the *automatic schedule* pauses.

---

## 13. Admin — Schedule Management (automatic announcements)

**Tab:** ⏰ Schedule Management → “Schedule Configuration (JSON)”

This is the list of **automatic** station / promo / safety plays.

The box contains JSON (structured text). You normally edit it carefully, then click **Update Schedule**.

### What can be scheduled

From the `cron.json` concept:

1. **Station announcements** — train / direction / destination / track at a time  
2. **Promo announcements** — a promo MP3 file at a time  
3. **Safety announcements** — one or more languages, optional delay between languages  

### The `cron` time field (simple guide)

Cron has five parts: `minute hour day-of-month month day-of-week`

Examples:

| Cron | Meaning |
|------|---------|
| `0 8 * * *` | Every day at 8:00 AM |
| `0 12 * * *` | Every day at 12:00 noon |
| `*/5 * * * *` | Every 5 minutes |
| `*/3 * * * *` | Every 3 minutes |
| `30 9 * * 1-5` | 9:30 AM, Monday–Friday |

Each scheduled item also has `"enabled": true/false`. **False** means it will not run even if the time matches.

### Multi-language safety example

A safety job can list several languages and a **delay** (seconds) between them so English and Spanish do not overlap.

### After editing

1. Click **Update Schedule**  
2. Confirm Scheduler status still looks sensible  
3. Remember: outside operating hours, enabled jobs are skipped  

### “Available Configuration Options” lists

Below the schedule editor, Admin shows the currently loaded **Train / Destination / Track / Safety language** IDs and names. Use these exact IDs inside schedule JSON and on the Main Station form.

---

## 14. Admin — Announcement Queue & Emergencies

**Tab:** 📋 Announcement Queue

### Queue Status

Shows whether the queue is running / paused, what is playing, and what is waiting. Use **Refresh** if the display looks stale.

### Recent History

Shows recently completed (or attempted) announcements — useful after “why didn’t that play?”

### Emergency Announcements

1. Choose an emergency type (from the configured list, e.g. Severe Weather, Power Outage, System Shutdown, All Clear)  
2. Click **TRIGGER EMERGENCY**  

Emergencies are **high priority** and are intended to cut through normal traffic. They are also treated specially during Red Alert suppression (emergency audio can still be allowed when station/promo/safety are blocked — depending on policy).

---

## 15. Admin — User & API Management

**Tab:** 🔐 User & API Management

### Admin Users

- Add / edit / disable users who may open Admin  
- Set role and permissions  
- Change passwords here  

### API Keys

- For external systems that call the HTTP API  
- Each key has permissions (announce / status / config, etc.)  
- Keep keys secret; disable unused keys  

### Settings

- **Session Timeout (minutes)** — how long Admin stays logged in when idle  
- Save with **Save Settings**  

### API Docs link

API documentation is available to logged-in admins at:

`http://localhost:8080/admin/api-docs`

(It is **not** shown on the public Main page.)

---

## 16. Admin — Track Layout

**Tab:** 🚂 Track Layout

Used to configure which trains / destinations / tracks are **available for selection** on the Main Station form (and related lists).

Typical actions:

1. Adjust which items are selected / available  
2. Click **Save Track Layout**  
3. Use **Reset to Default** only if you intentionally want the stock lists back  

Exact on-screen controls vary by install; always **Save** before leaving the tab.

---

## 17. Admin — Lightning Alerts (Thor Guard) — overview

**Tab:** ⚡ Lightning Alerts

This is the life-safety heart of the system. Settings are grouped into numbered sections on one tab. **Each settings card has its own Save button** so you do not need to scroll to the bottom after a small change. A sticky Save bar also sits at the bottom of the tab. Jump links at the top skip to Status, Monitor, Feeds, Failover, and so on.

### Big ideas (read these first)

1. The Pi **downloads** Thor Guard XML from one or more sensors on a timer.  
2. Only **one active feed** drives the “official” condition at a time.  
3. Turning **announce audio off** does **not** turn monitoring off, and does **not** remove the Red Alert **lock**.  
4. **Failover policy** (which feed is trusted) is separate from **feed-switch announcements** (optional PA when the active feed changes).  
5. **All Clear release authority** only controls **unlocking** Red Alert — not what can *enter* Red Alert.  
6. **Composite enter rules** can treat distant failover Warning combinations as Red Alert enter — unlock rules stay separate.  
7. **Manual Override** is for total sensor failure — it sets real lock state. It is **not** the same as Manual Test (audio drill).  
8. Unsaved edits show a yellow banner; live status still refreshes, but typed values are not overwritten until you Discard or Save.

### Sections (top to bottom)

| # | Section | Save button |
|---|---------|-------------|
| — | Live Status / Manual Tests / Override | (actions only) |
| 1 | Monitoring & timing (+ All Clear release) | Save Monitor |
| 2 | Feeds | Save Monitor (feeds) |
| 3 | Failover & Failback | Save Monitor (failover) |
| 4 | Composite Red Alert enter rules | Save Monitor (composite) |
| 5 | Which conditions may announce | Save Announcements |
| 6 | Red Alert Policy | Save Red Alert Policy |
| 7 | Feed-switch Announcements | Save Monitor (feed-switch) |
| 8 | Global Condition Audio | Save Condition Audio |

If you change something and forget to press the matching Save, it will revert after reload.

---

## 18. Lightning — Live Status

Shows real-time information such as:

- Whether monitoring is enabled / running  
- Active feed ID and displayname  
- Current condition and Red Alert lock state  
- Reminder countdown / count when locked  
- Per-feed health table:

| Column | Meaning |
|--------|---------|
| Feed | `primary`, `failover_1`, `failover_2` |
| Enabled | Whether that slot is turned on |
| Last OK | Last successful fetch time |
| Fails | Consecutive failure count |
| Last error | Why the last failure happened |
| **Last Status** | Last successfully parsed Thor condition |
| Displayname | Sensor name from XML / catalog |

Use this table first when “lightning seems stuck.”

---

## 19. Lightning — Global Monitor Timing

| Setting | Meaning | Typical |
|---------|---------|---------|
| **Enable Lightning Monitoring** | Master on/off for polling Thor | On for production |
| **Fetch Interval (seconds)** | How often to download XML | 60 (minimum 30) |
| **Default Request Timeout (seconds)** | How long to wait for each download | 30 |
| **Min seconds between same condition** | Cooldown before re-announcing the *same* condition | 0 = no extra cooldown |
| **Min seconds between any lightning announce** | Global lightning PA cooldown | 0 = off |
| **Failure window seconds** | Advanced failure timing window; `0` = off | 0 |

Click **Save Monitor** after changes.

---

## 20. Lightning — Feeds (Primary / Failover 1 / Failover 2)

You can configure up to **three** feeds.

### Per-feed fields (conceptually)

| Field | Meaning |
|-------|---------|
| **Enabled** | Include this feed in monitoring / failover order |
| **Label** | Friendly name in the UI |
| **Sensor** | Pick from the Broward catalog dropdown when possible |
| **URL** | Full Thor XML address (filled from catalog, or custom) |
| **Expected displayname** | Optional check that XML matches the expected park/sensor name |
| **Expected unique ID** | Optional check against Thor UNIQUEID |
| **Timeout override** | Per-feed timeout; `0` = use global default |
| **Announce: Inherit global / Custom** | Whether this feed uses the global announce checkboxes or its own |
| **Audio overrides** | Optional per-condition MP3 overrides (blank = use global Condition Audio) |
| **Horn overrides** | For Red Alert / All Clear: inherit global horn, or custom on/off + file |

### Recommended production pattern

- **Primary:** your closest / preferred sensor — announce Red Alert + All Clear  
- **Failover 1 / 2:** nearby sensors — often announce-off, but still eligible to *enter* Red Alert if they become active while Primary is down (per your site policy)  
- All three should have valid URLs if you use **failover_vote** All Clear mode  

### Catalog vs custom URL

Prefer the **catalog** picker so path prefixes stay correct. Only use a fully custom URL if your site tech provides one.

---

## 21. Lightning — Failover & Failback Policy

These controls answer: **“Which XML do we trust?”** They do **not** change the meaning of Red Alert itself.

| Setting | Meaning |
|---------|---------|
| **Consecutive failures before failover** | How many failed polls in a row before switching feeds |
| **Failback mode** | `prefer_primary` (usual), `prefer_highest_priority`, or `sticky` (stay on current until forced) |
| **Failback after successes** | How many good polls before returning toward preferred feed |
| **Failback probe interval** | `0` = probe on the same cadence as normal fetches |
| **Min dwell on feed** | Minimum seconds to stay on a feed before another switch |
| **When all feeds failed** | `hold_last_condition` (usual) or `force_unknown_status` (status only; does **not** auto All Clear) |

### Failure triggers (checkboxes)

A checked trigger means “count this problem as a failure toward failover.”

| Trigger | Plain meaning |
|---------|----------------|
| **http_error** | Network / connection failure |
| **http_status_not_ok** | Server returned an error page |
| **empty_body** | Download succeeded but empty |
| **encoding_error** | Response could not be decoded |
| **missing_lightningalert** | XML missing the lightning alert field |
| **unknown_condition** | Thor reported Unknown (only if you want that to force failover) |
| **displayname_mismatch** | Name in XML ≠ expected |
| **uniqueid_mismatch** | ID in XML ≠ expected |

---

## 22. Lightning — During Red Alert settings

These appear under Failover policy → **During Red Alert**.

### Allow active feed to switch while Red Alert is locked

- **On:** If the active feed dies during Red Alert, monitoring may move to the next failover feed  
- **Off:** Stay on the current feed until All Clear (no feed switching while locked)  

This does **not** decide who may clear/unlock Red Alert.

### Preserve condition across feed switch

- **On:** Switching feeds will not re-play the same Red Alert audio just because you changed feeds  
- **Off:** A feed switch may be treated more like a fresh condition change (can re-announce)

---

## 23. Lightning — All Clear release authority

**Setting name:** All Clear release authority  
**Values:**

| Mode | Behavior |
|------|----------|
| **primary_only** (recommended) | Only the **Primary** feed may unlock Red Alert |
| **failover_vote** | Primary may unlock alone; **or**, if a non-primary path is used, **both** Failover 1 and Failover 2 must return **All Clear** at vote time |

### Critical details for failover_vote

- Enabled feeds with URLs are **required**, but **not enough** by themselves  
- The vote counts only strict **All Clear** results  
- **Unknown**, Warning, Caution, Red Alert, bad XML, or download errors count as **0**  
- One All Clear + one Unknown = **stay locked**  
- If only one failover is configured, you **cannot** clear via vote  

### What this setting does *not* do

- Does not stop a failover feed from **entering** Red Alert under normal enter rules  
- Does not replace the older “must accept All Clear after Red Alert” safety gate (see section 29)  

---

## 24. Lightning — Feed-switch Announcements

Optional PA when the **active feed changes** (for example Primary → Failover 1).

| Column | Meaning |
|--------|---------|
| On | Enable this rule |
| From / To | Which feed transition |
| Reason | `any`, `failover`, `failback`, `pin`, etc. |
| Sensor | Optional match so the rule only fires for a specific sensor |
| MP3 | Sound to play |

Default: quiet (rules off). These announcements **do not** change the Red Alert lock.

Click **Save Monitor** after editing rules.

---

## 25. Lightning — Global Condition Announcements

Checkboxes:

- Red Alert  
- Warning  
- Caution  
- All Clear  
- Unknown  

**Checked** = allowed to make PA sound when that condition is accepted (and the active feed inherits global, or custom enables it).  
**Unchecked** = monitoring continues, but that condition stays silent (except Manual Test, which always plays).

**Save Condition Announcements** after changes.

**Remember:** Disabling All Clear *audio* does not prevent unlock logic if All Clear is accepted; it only mutes the PA for that event (lock exit can still occur).

---

## 26. Lightning — Global Condition Audio (horn + voice)

For each major condition, the play order is:

1. Optional **horn** MP3 (if enabled)  
2. **Announce / voice** MP3  

### Red Alert / All Clear blocks

| Control | Meaning |
|---------|---------|
| Play horn before announce | On/off |
| Horn MP3 | e.g. `Horn_RedAlert.mp3` |
| Announce MP3 | e.g. `Voice_RedAlert.mp3` |

### Warning / Caution / Unknown

Usually voice-only fields (horn optional/empty).

File names are chosen from the lightning MP3 folder (dropdown/datalist). After changing files on disk, restart or re-open Admin if the list looks stale.

**Save Condition Audio** after changes.

Individual feeds may override Red Alert / All Clear horns.

---

## 27. Lightning — Red Alert Policy

| Setting | Meaning |
|---------|---------|
| **Cancel queued/playing announcements on Red Alert** (preempt) | Clears the current queue/play when Red Alert engages |
| **Suppress non-emergency until All Clear** | Blocks station/promo/safety/scheduled while locked |
| **Repeating Red Alert reminder** | Plays a reminder on an interval while locked |
| **Reminder interval (minutes)** | e.g. every 5 minutes |
| **Reminder MP3** | e.g. `Voice_RedAlert_Reminder.mp3` |
| **Play horn before each reminder** | Optional |
| **Reminder horn MP3** | Used only if horn-before-reminder is on |

Buttons:

- **Save Red Alert Policy**  
- **Play Reminder Now** — only works while Red Alert is actually active  

---

## 28. Lightning — Manual Tests, Pin, Reset, and Manual Override

### Manual Tests

Buttons: Test Red Alert / Warning / Caution / All Clear / Unknown  

These **always play audio** for drill/testing, even if that condition’s announce checkbox is off.  
Treat **Test Red Alert** carefully on a live PA — it can engage lock behavior depending on test path; prefer low-traffic times and know how to **Reset THOR Guard State** / All Clear.

### Pin active feed

Forces Primary / Failover 1 / Failover 2 for drills.  
**Unpin** returns to automatic failover selection.

### Manual Override (all sensors failed)

Use when **Primary, Failover 1, and Failover 2** are unreachable or unusable and operators must set lock state by hand.

| Action | Meaning |
|--------|---------|
| **Force Red Alert (override)** | Sets real Red Alert lock + plays Red Alert sequence; Live Status shows **MANUAL OVERRIDE ACTIVE** |
| **Force All Clear / unlock (override)** | Clears the Red Alert lock + plays All Clear; override flag remains until cleared or feeds recover |
| **Clear override flag only** | Removes the override banner; does **not** change the current lock/condition |

Notes:
- This is **not** Manual Test. Override changes operational lock state.
- When any Thor feed recovers successfully, the override **flag** auto-clears; the next real condition drives state.
- **Reset THOR Guard State** also clears the override flag.

### Reset THOR Guard State

Clears cached condition, Red Alert lock, reminder timer, and manual override.  
Use after a drill or if the lock is stuck because of a bad test — **not** as a substitute for a real All Clear during an actual storm.

---

## 29. How Red Alert / All Clear really work (rules)

These rules are intentional and safety-critical (preserved from v1.1.1 / v1.1.2, still true in v1.1.3):

1. **Unknown** from Thor is ignored for lock/announce purposes (it does not update the “previous condition” used for All Clear decisions).  
2. **All Clear is accepted** only if:
   - Red Alert lock is already active, **or**
   - The previous accepted condition was Red Alert  
   Otherwise All Clear is ignored for unlock/PA (tracking may still update).  
3. On **reject**, there is **no** unlock and **no** All Clear PA.  
4. On **accept**, the lock exits, then All Clear may announce (unless announce audio is disabled).  
5. **Red Alert enters the lock** when a feed reports Red Alert, **or** when an enabled **composite enter rule** matches (Advanced — e.g. both failovers Warning). Caution and Warning alone never take the lock.  
6. **All Clear release authority** runs **after** rule (2) succeeds. Primary under `primary_only` behaves like the classic single-feed system. Composite enter does **not** change unlock rules.

### Composite enter (Advanced)

Example rule operators may enable: require **failover_1** and **failover_2** each report **Warning** → treat as **Red Alert** enter (lock + Red Alert PA). Distant failovers may be 1–5 miles away; this is Admin-configurable, not hardcoded miles.

### Practical examples

| Situation | Result |
|-----------|--------|
| Primary Red Alert → Primary All Clear (`primary_only`) | Unlock + All Clear PA (if enabled) |
| Primary Red Alert → Failover All Clear (`primary_only`) | Stay locked |
| Caution → All Clear (never had Red Alert) | No unlock |
| Failover enters Red Alert (Primary down) → only that failover says All Clear | Stay locked under both release modes |
| `failover_vote`, both failovers All Clear | Unlock allowed (after rule 2) |
| `failover_vote`, one All Clear + one Unknown | Stay locked |
| Composite: both failovers Warning → Red Alert | Lock enters; unlock still needs authorized All Clear |
| All feeds down → Manual Override Force Red Alert | Lock active; Live Status shows override |

---

## 30. Announcement types and priority

Higher priority plays before lower priority when both are waiting.

| Type | Typical use | Priority idea |
|------|-------------|----------------|
| Emergency | Operator-triggered emergencies | Highest |
| Lightning (Red Alert, etc.) | Thor life-safety | Very high / critical |
| Station | Train calls | Normal/high |
| Safety | Safety messages | Normal |
| Promo | Promotional clips | Lower |

During Red Alert with **suppress** enabled, non-emergency items are blocked until All Clear.

---

## 31. Sound files (MP3 folders)

Under the install directory, sounds live in:

```text
static/mp3/
  lightning/     ← Thor horns and voices (Horn_*.mp3, Voice_*.mp3)
  emergency/
  promo/
  safety/
  … (station pieces as configured)
```

### Lightning files used by v1.1.2 defaults

| File | Role |
|------|------|
| `Horn_RedAlert.mp3` | Red Alert attention horn |
| `Voice_RedAlert.mp3` | Red Alert spoken message |
| `Horn_AllClear.mp3` | All Clear horn |
| `Voice_AllClear.mp3` | All Clear spoken message |
| `Voice_Warning.mp3` | Warning |
| `Voice_Caution.mp3` | Caution |
| `Voice_RedAlert_Reminder.mp3` | Repeating reminder while locked |

If you replace a file, keep the **same filename** or update Admin Condition Audio to the new name, then test.

---

## 32. Where settings are stored on the Pi

You usually should **not** hand-edit these unless a technician asks you to. Prefer Admin screens.

| File (under `json/`) | Contents |
|----------------------|----------|
| `lightning.json` | Thor feeds, failover, audio, Red Alert policy |
| `operating_hours.json` | Hours + time sync |
| `cron.json` | Automatic schedule |
| `admin_config.json` | Users, API keys, session settings |
| `trains_*.json`, `destinations_*.json`, `tracks.json` | Station dropdown data |
| `promo.json`, `safety.json`, `emergencies.json` | Message catalogs |
| `browardtg.json` | Broward Thor sensor catalog |
| `install_version.json` | Which app version is installed |
| `audio_settings.json` | Volume / device persistence (runtime) |

Also:

| Folder | Contents |
|--------|----------|
| `logs/` | Application log files (for technicians) |
| `xml/` | Cached Thor XML snapshots |

---

## 33. API Docs (for integrations)

For programmers connecting other systems:

1. Log into Admin  
2. Open `http://localhost:8080/admin/api-docs`  

Public Main UI does not advertise API Docs. Legacy `/api/docs` requires the same Admin login.

API calls that change state need a configured **API key** (User & API Management).

---

## 34. Everyday checklists

### Opening the station

1. Confirm Main page loads  
2. Confirm **Audio System** looks healthy; play **Test Audio** in Admin if unsure  
3. Confirm **Scheduler** matches expectations for the time of day  
4. Glance at Lightning Dashboard: active feed OK, no unexpected Red Alert lock  
5. Confirm volume is appropriate for the platform  

### During a storm / Red Alert

1. Trust the red banner and suppress behavior  
2. Do not repeatedly click Test buttons on the live PA  
3. Watch Live Status: which feed is active, Last Status values  
4. Wait for a valid All Clear under your release mode (usually Primary)  
5. After All Clear, confirm the banner is gone and normal announcements work again  

### Closing / end of day

1. Optional: Pause queue if needed for maintenance quiet  
2. Confirm operating hours will pause the overnight schedule  
3. Log out of Admin on shared PCs  

### After a software update

1. Confirm App Version  
2. Hard-refresh browsers  
3. Spot-check audio test + lightning status  
4. Confirm feeds still enabled with correct sensors  

---

## 35. Troubleshooting

| Symptom | Things to try |
|---------|----------------|
| Web page will not load | Start with `~/tarr-start.sh`; confirm port 8080; wait for boot |
| No sound | Admin → Audio device + volume + Test Audio; check amp power/mute |
| Scheduler says Paused but you expected plays | Check operating hours + timezone + whether the job is `enabled` |
| Scheduled job never fires | Confirm cron time, enabled flag, operating hours, queue not paused, not in Red Alert suppress |
| Lightning stuck on old condition | Check Live Status errors; feed URLs; network; Reset only if appropriate |
| Cannot clear Red Alert | Confirm Primary All Clear (or valid failover_vote); Unknown does not count; release mode settings |
| Admin login fails | Verify username/password; wait out lockout after too many failures; ask site admin to reset |
| Update fails | Check internet/GitHub access; retry; read update log; call support with log text |
| Form values “jump back” on Lightning tab | You have unsaved edits while status refreshes — Save or Discard |
| Main Scheduler still says Running always | Hard-refresh; ensure you are on a build that includes dynamic status (v1.1.2+) |

### Getting help ready

When contacting support, collect:

- App Version from Admin  
- Approximate time of the problem  
- Screenshot of Lightning Live Status  
- Whether Red Alert lock was on  
- Relevant lines from the newest file in `logs/` (technician)

---

## 36. Safety reminders

- Thor Guard / lightning features are **life-safety**. Prefer the recommended defaults (`primary_only` All Clear release, Primary as the clearing authority).  
- Never disable monitoring to “stop the noise” during a real event — disable announce audio if you must quiet a drill, understanding lock behavior still applies.  
- Test Red Alert on the live PA only with a plan to unlock/reset.  
- Keep Admin passwords and API keys private.  
- After any change to lightning feeds or release mode, verify with Live Status before leaving the site.

---

## Quick reference — where to click

| I want to… | Go here |
|------------|---------|
| Play a train announcement | Main → Station |
| Pause all queued announcements | Main → Dashboard |
| Change volume / speakers | Admin → Audio Controls |
| Change open/close hours | Admin → Schedule Management → Operating Hours |
| Change automatic schedule | Admin → Schedule Management → Schedule JSON |
| Trigger emergency PA | Admin → Announcement Queue |
| Change Thor sensors / failover | Admin → Lightning Alerts → feeds + Save Monitor |
| Change who can All Clear | Admin → Lightning → All Clear release authority → Save Monitor |
| Change Red Alert reminder timing | Admin → Lightning → Red Alert Policy → Save |
| Install a new version | Admin → System Status → Software Updates |
| Add an Admin user | Admin → User & API Management |
| Read API documentation | Admin → `/admin/api-docs` |

---

*End of operator manual for TARR Annunciator v1.1.2.*
