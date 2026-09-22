package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// runSchemaMigrations applies additive migrations from installedVersion → AppVersion.
// Never overwrites existing operator/runtime settings; only creates missing files/keys/ids.
func runSchemaMigrations(fromVersion string) error {
	fromVersion = normalizeVersion(fromVersion)
	toVersion := normalizeVersion(AppVersion)
	if compareSemver(fromVersion, toVersion) >= 0 {
		log.Printf("Schema migrations: nothing to do (%s → %s)", fromVersion, toVersion)
		return nil
	}

	log.Printf("Schema migrations: %s → %s", fromVersion, toVersion)

	// 1.1.0: operating hours file, lightning additive schema
	if compareSemver(fromVersion, "1.1.0") < 0 && compareSemver(toVersion, "1.1.0") >= 0 {
		if err := migrateEnsureOperatingHours(); err != nil {
			return fmt.Errorf("operating_hours migration: %w", err)
		}
		if err := migrateLightningAdditive(nil); err != nil {
			return fmt.Errorf("lightning migration: %w", err)
		}
	}

	// 1.1.1: persist lightning monitor block in lightning.json (additive)
	if compareSemver(fromVersion, "1.1.1") < 0 && compareSemver(toVersion, "1.1.1") >= 0 {
		if err := migrateLightningMonitorBlock(); err != nil {
			return fmt.Errorf("lightning monitor migration: %w", err)
		}
	}

	// 1.1.2: multi-feed monitor, failover policy, browardtg catalog, audio filename remap
	if compareSemver(fromVersion, "1.1.2") < 0 && compareSemver(toVersion, "1.1.2") >= 0 {
		if err := migrateLightningMultiFeed(); err != nil {
			return fmt.Errorf("lightning multi-feed migration: %w", err)
		}
		if err := migrateEnsureBrowardTGCatalog(""); err != nil {
			return fmt.Errorf("browardtg catalog migration: %w", err)
		}
		if err := migrateLightningAudioFilenames(); err != nil {
			return fmt.Errorf("lightning audio filename migration: %w", err)
		}
	}

	// 1.1.3: composite Red Alert enter rules, stale localtime trigger, Voice_Unknown default
	if compareSemver(fromVersion, "1.1.3") < 0 && compareSemver(toVersion, "1.1.3") >= 0 {
		if err := migrateLightning113Additive(); err != nil {
			return fmt.Errorf("lightning 1.1.3 migration: %w", err)
		}
	}

	// 1.1.4: telemetry_collapse failover trigger default
	if compareSemver(fromVersion, "1.1.4") < 0 && compareSemver(toVersion, "1.1.4") >= 0 {
		if err := migrateLightning114Additive(); err != nil {
			return fmt.Errorf("lightning 1.1.4 migration: %w", err)
		}
	}

	// Idempotent repairs — run even when from == to was skipped above via early return;
	// callers that hit early return must still invoke ensureEmbeddedJSONSeeds / pending handoff separately.
	if compareSemver(toVersion, "1.1.3") >= 0 {
		if err := migrateLightning113Additive(); err != nil {
			return fmt.Errorf("lightning 1.1.3 additive repair: %w", err)
		}
	}
	if compareSemver(toVersion, "1.1.4") >= 0 {
		if err := migrateLightning114Additive(); err != nil {
			return fmt.Errorf("lightning 1.1.4 additive repair: %w", err)
		}
	}

	return nil
}

func migrateLightningMonitorBlock() error {
	path := lightningConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Migration: lightning.json missing — skip monitor block")
			return nil
		}
		return err
	}
	var live map[string]interface{}
	if err := json.Unmarshal(data, &live); err != nil {
		return fmt.Errorf("parse lightning.json: %w", err)
	}
	if _, ok := live["monitor"]; ok {
		log.Printf("Migration: lightning.json monitor already present — leaving unchanged")
		return nil
	}
	def := defaultLightningMonitorConfig()
	live["monitor"] = map[string]interface{}{
		"enabled":        false,
		"url":            "",
		"fetch_interval": def.FetchInterval,
		"timeout":        def.Timeout,
	}
	out, err := json.MarshalIndent(live, "", "    ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return err
	}
	log.Printf("Migration: lightning.json added empty monitor block (set URL in Admin — not invented in code)")
	return nil
}

// migrateLightning113Additive is idempotent: composite rules array, stale_localtime trigger default,
// and thor_unknown → Voice_Unknown.mp3 for Unknown announce paths.
func migrateLightning113Additive() error {
	path := lightningConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Migration: lightning.json missing — skip 1.1.3 additive")
			return nil
		}
		return err
	}
	var live map[string]interface{}
	if err := json.Unmarshal(data, &live); err != nil {
		return fmt.Errorf("parse lightning.json: %w", err)
	}
	changed := false

	mon, _ := live["monitor"].(map[string]interface{})
	if mon != nil {
		if _, ok := mon["composite_red_alert_rules"]; !ok {
			mon["composite_red_alert_rules"] = []interface{}{}
			changed = true
			log.Printf("Migration: lightning.json composite_red_alert_rules initialized")
		}
		fo, _ := mon["failover"].(map[string]interface{})
		if fo != nil {
			tr, _ := fo["triggers"].(map[string]interface{})
			if tr == nil {
				tr = map[string]interface{}{}
				fo["triggers"] = tr
			}
			if _, ok := tr["stale_localtime"]; !ok {
				tr["stale_localtime"] = true
				changed = true
				log.Printf("Migration: lightning.json failover.triggers.stale_localtime defaulted to true")
			}
		}
		live["monitor"] = mon
	}

	if ca, ok := live["condition_audio"].(map[string]interface{}); ok {
		if unk, ok := ca["Unknown"].(map[string]interface{}); ok {
			af, _ := unk["announce_file"].(string)
			af = strings.TrimSpace(af)
			if af == "" || strings.EqualFold(filepath.Base(af), "thor_unknown.mp3") {
				unk["announce_file"] = "Voice_Unknown.mp3"
				ca["Unknown"] = unk
				live["condition_audio"] = ca
				changed = true
				log.Printf("Migration: condition_audio.Unknown.announce_file → Voice_Unknown.mp3")
			}
		}
	}

	if anns, ok := live["lightning_announcements"].([]interface{}); ok {
		for _, raw := range anns {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			af, _ := m["audio_file"].(string)
			base := strings.ToLower(filepath.Base(strings.TrimSpace(af)))
			if strings.EqualFold(id, "THOR_Unknown") && (base == "" || base == "thor_unknown.mp3") {
				m["audio_file"] = "Voice_Unknown.mp3"
				changed = true
				log.Printf("Migration: THOR_Unknown audio_file → Voice_Unknown.mp3")
			}
		}
	}

	if !changed {
		return nil
	}
	out, err := json.MarshalIndent(live, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// migrateLightning114Additive is idempotent: telemetry_collapse trigger default + thresholds block.
func migrateLightning114Additive() error {
	path := lightningConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Migration: lightning.json missing — skip 1.1.4 additive")
			return nil
		}
		return err
	}
	var live map[string]interface{}
	if err := json.Unmarshal(data, &live); err != nil {
		return fmt.Errorf("parse lightning.json: %w", err)
	}
	changed := false

	mon, _ := live["monitor"].(map[string]interface{})
	if mon != nil {
		fo, _ := mon["failover"].(map[string]interface{})
		if fo != nil {
			tr, _ := fo["triggers"].(map[string]interface{})
			if tr == nil {
				tr = map[string]interface{}{}
				fo["triggers"] = tr
			}
			if _, ok := tr["telemetry_collapse"]; !ok {
				tr["telemetry_collapse"] = true
				changed = true
				log.Printf("Migration: lightning.json failover.triggers.telemetry_collapse defaulted to true")
			}
			if _, ok := fo["telemetry_collapse"]; !ok {
				def := defaultTelemetryCollapseThresholds()
				fo["telemetry_collapse"] = map[string]interface{}{
					"history_samples": def.HistorySamples,
					"elevated_lhl":    def.ElevatedLHL,
					"elevated_di":     def.ElevatedDI,
					"elevated_ad":     def.ElevatedAD,
					"floor_lhl_max":   def.FloorLHLMax,
				}
				changed = true
				log.Printf("Migration: lightning.json failover.telemetry_collapse thresholds added")
			}
		}
		live["monitor"] = mon
	}

	if !changed {
		return nil
	}
	out, err := json.MarshalIndent(live, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

func migrateLightningMultiFeed() error {
	path := lightningConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Migration: lightning.json missing — skip multi-feed")
			return nil
		}
		return err
	}
	var live map[string]interface{}
	if err := json.Unmarshal(data, &live); err != nil {
		return fmt.Errorf("parse lightning.json: %w", err)
	}
	mon, _ := live["monitor"].(map[string]interface{})
	if mon == nil {
		mon = map[string]interface{}{
			"enabled":        false,
			"url":            "",
			"fetch_interval": 60,
			"timeout":        30,
		}
		live["monitor"] = mon
	}

	changed := false
	if _, ok := mon["feeds"]; !ok {
		legacyURL, _ := mon["url"].(string)
		enabled, _ := mon["enabled"].(bool)
		primary := map[string]interface{}{
			"id":                   "primary",
			"label":                "Primary",
			"enabled":              enabled && strings.TrimSpace(legacyURL) != "",
			"sensor_id":            "",
			"source":               "custom",
			"expected_displayname": "",
			"expected_uniqueid":    "",
			"url":                  strings.TrimSpace(legacyURL),
			"timeout_seconds":      0,
			"announce": map[string]interface{}{"inherit_global": true},
			"audio_by_condition":   map[string]interface{}{},
			"horn_by_condition": map[string]interface{}{
				"RedAlert": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
				"AllClear": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
			},
		}
		// Match catalog if available
		_ = loadBrowardTGCatalog()
		if s := findBrowardTGSensorByURL(legacyURL); s != nil {
			primary["sensor_id"] = s.ID
			primary["expected_displayname"] = s.DisplayName
			primary["source"] = "catalog"
		}
		mon["feeds"] = []interface{}{
			primary,
			map[string]interface{}{
				"id": "failover_1", "label": "Failover 1", "enabled": false, "sensor_id": "", "source": "custom",
				"expected_displayname": "", "expected_uniqueid": "", "url": "", "timeout_seconds": 0,
				"announce": map[string]interface{}{"inherit_global": true}, "audio_by_condition": map[string]interface{}{},
				"horn_by_condition": map[string]interface{}{
					"RedAlert": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
					"AllClear": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
				},
			},
			map[string]interface{}{
				"id": "failover_2", "label": "Failover 2", "enabled": false, "sensor_id": "", "source": "custom",
				"expected_displayname": "", "expected_uniqueid": "", "url": "", "timeout_seconds": 0,
				"announce": map[string]interface{}{"inherit_global": true}, "audio_by_condition": map[string]interface{}{},
				"horn_by_condition": map[string]interface{}{
					"RedAlert": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
					"AllClear": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
				},
			},
		}
		changed = true
		log.Printf("Migration: lightning.json monitor.feeds created from legacy url")
	}
	if _, ok := mon["failover"]; !ok {
		def := defaultLightningFailoverPolicy()
		mon["failover"] = map[string]interface{}{
			"consecutive_failures":             def.ConsecutiveFailures,
			"failure_window_seconds":           def.FailureWindowSeconds,
			"min_dwell_on_feed_seconds":        def.MinDwellOnFeedSeconds,
			"failback_mode":                    def.FailbackMode,
			"failback_after_successes":         def.FailbackAfterSuccesses,
			"failback_probe_interval_seconds":  def.FailbackProbeIntervalSeconds,
			"allow_failover_during_red_alert":  def.AllowFailoverDuringRedAlert,
			"allclear_release_mode":            def.AllClearReleaseMode,
			"require_allclear_from_same_feed":  false,
			"preserve_condition_across_failover": def.PreserveConditionAcrossFail,
			"on_all_feeds_failed":              def.OnAllFeedsFailed,
			"triggers": map[string]interface{}{
				"http_error": true, "http_status_not_ok": true, "empty_body": true, "encoding_error": true,
				"missing_lightningalert": true, "unknown_condition": false, "displayname_mismatch": false, "uniqueid_mismatch": false,
			},
		}
		changed = true
		log.Printf("Migration: lightning.json monitor.failover defaults added")
	}
	if fo, ok := mon["failover"].(map[string]interface{}); ok {
		if _, has := fo["allclear_release_mode"]; !has {
			fo["allclear_release_mode"] = "primary_only"
			fo["require_allclear_from_same_feed"] = false
			changed = true
			log.Printf("Migration: lightning.json allclear_release_mode defaulted to primary_only")
		} else {
			mode, _ := fo["allclear_release_mode"].(string)
			fo["allclear_release_mode"] = normalizeAllClearReleaseMode(mode)
			fo["require_allclear_from_same_feed"] = false
		}
	}
	if _, ok := mon["feed_switch_announcements"]; !ok {
		mon["feed_switch_announcements"] = []interface{}{
			map[string]interface{}{"id": "p_to_f1", "enabled": false, "from_feed_id": "primary", "to_feed_id": "failover_1", "reason": "failover", "match_sensor_id": "", "match_displayname": "", "audio_file": "", "label": "Primary to Failover 1"},
			map[string]interface{}{"id": "p_to_f2", "enabled": false, "from_feed_id": "primary", "to_feed_id": "failover_2", "reason": "failover", "match_sensor_id": "", "match_displayname": "", "audio_file": "", "label": "Primary to Failover 2"},
		}
		changed = true
	}
	if _, ok := live["announce_timing"]; !ok {
		live["announce_timing"] = map[string]interface{}{
			"min_seconds_between_same_condition":        0,
			"min_seconds_between_any_lightning_announce": 0,
		}
		changed = true
	}
	if _, ok := live["displayname_overrides"]; !ok {
		live["displayname_overrides"] = []interface{}{}
		changed = true
	}
	if _, ok := live["condition_audio"]; !ok {
		live["condition_audio"] = map[string]interface{}{
			"RedAlert": map[string]interface{}{"horn_enabled": true, "horn_file": "Horn_RedAlert.mp3", "announce_file": "Voice_RedAlert.mp3"},
			"AllClear": map[string]interface{}{"horn_enabled": true, "horn_file": "Horn_AllClear.mp3", "announce_file": "Voice_AllClear.mp3"},
			"Warning":  map[string]interface{}{"horn_enabled": false, "horn_file": "", "announce_file": "Voice_Warning.mp3"},
			"Caution":  map[string]interface{}{"horn_enabled": false, "horn_file": "", "announce_file": "Voice_Caution.mp3"},
			"Unknown":  map[string]interface{}{"horn_enabled": false, "horn_file": "", "announce_file": ""},
		}
		changed = true
		log.Printf("Migration: lightning.json condition_audio defaults added")
	}
	if feeds, ok := mon["feeds"].([]interface{}); ok {
		for _, fi := range feeds {
			fm, ok := fi.(map[string]interface{})
			if !ok {
				continue
			}
			if _, ok := fm["horn_by_condition"]; !ok {
				fm["horn_by_condition"] = map[string]interface{}{
					"RedAlert": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
					"AllClear": map[string]interface{}{"inherit_global": true, "enabled": true, "audio_file": ""},
				}
				changed = true
			}
		}
	}
	if !changed {
		log.Printf("Migration: lightning.json multi-feed already present — leaving unchanged")
		return nil
	}
	out, err := json.MarshalIndent(live, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// lightningAudioRenameMap maps pre-1.1.2 Thor MP3 basenames to the Horn_/Voice_ names shipped in 1.1.2.
// Custom operator filenames are left untouched. Unknown has no replacement file in 1.1.2.
func lightningAudioRenameMap() map[string]string {
	return map[string]string{
		"thor_red_alert.mp3":  "Voice_RedAlert.mp3",
		"redalert.mp3":        "Voice_RedAlert.mp3",
		"thor_all_clear.mp3":  "Voice_AllClear.mp3",
		"all_clear.mp3":       "Voice_AllClear.mp3",
		"thor_warning.mp3":    "Voice_Warning.mp3",
		"warning.mp3":         "Voice_Warning.mp3",
		"thor_caution.mp3":    "Voice_Caution.mp3",
		"thor_repeat1.mp3":    "Voice_RedAlert_Reminder.mp3",
		"thor_unknown.mp3":    "Voice_Unknown.mp3",
		// Legacy horn was the same file as the red-alert announce clip
		"thor_red_alert_horn.mp3": "Horn_RedAlert.mp3",
	}
}

func remapLightningAudioBasename(name string) (string, bool) {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/"))))
	if base == "" {
		return name, false
	}
	if next, ok := lightningAudioRenameMap()[base]; ok {
		return next, true
	}
	return name, false
}

func remapLightningAudioStringField(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	raw, _ := m[key].(string)
	if next, ok := remapLightningAudioBasename(raw); ok {
		m[key] = next
		return true
	}
	return false
}

// migrateLightningAudioFilenames remaps known legacy lightning MP3 names in lightning.json
// so the Admin updater can install 1.1.2 Voice_/Horn_ assets without manual JSON edits.
// Does not delete old files on disk (mergeCopyDir leaves them); only updates references.
func migrateLightningAudioFilenames() error {
	path := lightningConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Migration: lightning.json missing — skip audio filename remap")
			return nil
		}
		return err
	}
	var live map[string]interface{}
	if err := json.Unmarshal(data, &live); err != nil {
		return fmt.Errorf("parse lightning.json: %w", err)
	}
	changed := false

	if anns, ok := live["lightning_announcements"].([]interface{}); ok {
		for _, item := range anns {
			a, _ := item.(map[string]interface{})
			if remapLightningAudioStringField(a, "audio_file") {
				changed = true
			}
		}
	}

	if pol, ok := live["red_alert_policy"].(map[string]interface{}); ok {
		if remapLightningAudioStringField(pol, "reminder_audio_file") {
			changed = true
		}
		rawHorn, _ := pol["horn_audio_file"].(string)
		hornBase := strings.ToLower(strings.TrimSpace(filepath.Base(rawHorn)))
		if hornBase == "thor_red_alert.mp3" || hornBase == "redalert.mp3" {
			pol["horn_audio_file"] = "Horn_RedAlert.mp3"
			changed = true
		} else if remapLightningAudioStringField(pol, "horn_audio_file") {
			changed = true
		}
	}

	if ca, ok := live["condition_audio"].(map[string]interface{}); ok {
		for _, clip := range ca {
			c, _ := clip.(map[string]interface{})
			if remapLightningAudioStringField(c, "horn_file") {
				changed = true
			}
			if remapLightningAudioStringField(c, "announce_file") {
				changed = true
			}
		}
	}

	if mon, ok := live["monitor"].(map[string]interface{}); ok {
		if feeds, ok := mon["feeds"].([]interface{}); ok {
			for _, item := range feeds {
				f, _ := item.(map[string]interface{})
				if abc, ok := f["audio_by_condition"].(map[string]interface{}); ok {
					for k, v := range abc {
						s, _ := v.(string)
						if next, ok := remapLightningAudioBasename(s); ok {
							abc[k] = next
							changed = true
						}
					}
				}
				if hbc, ok := f["horn_by_condition"].(map[string]interface{}); ok {
					for _, hv := range hbc {
						h, _ := hv.(map[string]interface{})
						if remapLightningAudioStringField(h, "audio_file") {
							changed = true
						}
					}
				}
			}
		}
		if rules, ok := mon["feed_switch_announcements"].([]interface{}); ok {
			for _, item := range rules {
				r, _ := item.(map[string]interface{})
				if remapLightningAudioStringField(r, "audio_file") {
					changed = true
				}
			}
		}
	}

	if !changed {
		log.Printf("Migration: lightning audio filenames already current — leaving unchanged")
		return nil
	}
	out, err := json.MarshalIndent(live, "", "    ")
	if err != nil {
		return err
	}
	log.Printf("Migration: lightning.json remapped legacy Thor MP3 filenames to Voice_/Horn_ names")
	return os.WriteFile(path, out, 0644)
}

// migrateEnsureBrowardTGCatalog copies/merges browardtg.json from package seed or data seed.
// packageJSONDir empty → try sibling package paths / leave if present.
func migrateEnsureBrowardTGCatalog(packageJSONDir string) error {
	livePath := browardtgPath()
	var seedPath string
	if packageJSONDir != "" {
		seedPath = filepath.Join(packageJSONDir, "browardtg.json")
	}
	if seedPath == "" || !fileExists(seedPath) {
		// Try repo data path relative to binary cwd
		candidates := []string{
			filepath.Join("data", "json", "browardtg.json"),
			filepath.Join("json", "browardtg.json"),
		}
		if app != nil && app.Config != nil && app.Config.BaseDir != "" {
			candidates = append([]string{filepath.Join(app.Config.BaseDir, "json", "browardtg.json")}, candidates...)
		}
		for _, c := range candidates {
			if fileExists(c) && c != livePath {
				seedPath = c
				break
			}
		}
	}
	if seedPath == "" || !fileExists(seedPath) {
		if fileExists(livePath) {
			log.Printf("Migration: browardtg.json already present")
			_ = loadBrowardTGCatalog()
			return nil
		}
		log.Printf("Migration: browardtg.json seed not found — catalog will be empty until packaged")
		return nil
	}

	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		return err
	}
	var seed BrowardTGCatalog
	if err := json.Unmarshal(seedData, &seed); err != nil {
		return fmt.Errorf("parse seed browardtg.json: %w", err)
	}

	if !fileExists(livePath) {
		if err := os.MkdirAll(filepath.Dir(livePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(livePath, seedData, 0644); err != nil {
			return err
		}
		log.Printf("Migration: installed browardtg.json (%d sensors)", len(seed.Sensors))
		_ = loadBrowardTGCatalog()
		return nil
	}

	liveData, err := os.ReadFile(livePath)
	if err != nil {
		return err
	}
	var live BrowardTGCatalog
	if err := json.Unmarshal(liveData, &live); err != nil {
		return fmt.Errorf("parse live browardtg.json: %w", err)
	}
	byID := map[string]int{}
	for i, s := range live.Sensors {
		byID[s.ID] = i
	}
	changed := false
	for _, s := range seed.Sensors {
		if idx, ok := byID[s.ID]; ok {
			// update known fields
			if live.Sensors[idx].URL != s.URL || live.Sensors[idx].DisplayName != s.DisplayName || live.Sensors[idx].PathPrefix != s.PathPrefix {
				live.Sensors[idx] = s
				changed = true
			}
		} else {
			live.Sensors = append(live.Sensors, s)
			changed = true
		}
	}
	if live.SchemaVersion == 0 {
		live.SchemaVersion = seed.SchemaVersion
		changed = true
	}
	if !changed {
		log.Printf("Migration: browardtg.json merge — no changes")
		_ = loadBrowardTGCatalog()
		return nil
	}
	out, err := json.MarshalIndent(live, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if err := os.WriteFile(livePath, out, 0644); err != nil {
		return err
	}
	log.Printf("Migration: merged browardtg.json (%d sensors)", len(live.Sensors))
	_ = loadBrowardTGCatalog()
	return nil
}

func migrateEnsureOperatingHours() error {
	path := operatingHoursPath()
	if _, err := os.Stat(path); err == nil {
		log.Printf("Migration: operating_hours.json already present — leaving unchanged")
		return nil
	}
	cfg := defaultOperatingHours()
	data, err := json.MarshalIndent(normalizeOperatingHours(cfg), "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	log.Printf("Migration: created operating_hours.json with defaults")
	return nil
}

// migrateLightningAdditive merges missing announcements (by id) and missing top-level
// object keys from seed into live lightning.json. Existing values are never replaced.
// If seed is nil, built-in 1.1 defaults are used.
func migrateLightningAdditive(seed map[string]interface{}) error {
	path := lightningConfigPath()
	live := map[string]interface{}{}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &live); err != nil {
			return fmt.Errorf("parse live lightning.json: %w", err)
		}
	} else if os.IsNotExist(err) {
		live = map[string]interface{}{}
	} else {
		return err
	}

	if seed == nil {
		seed = defaultLightningSeed()
	}

	changed := false

	// Top-level object keys (e.g. red_alert_policy): add if missing entirely
	for key, seedVal := range seed {
		if key == "lightning_announcements" || key == "metadata" {
			continue
		}
		if _, exists := live[key]; !exists {
			live[key] = seedVal
			changed = true
			log.Printf("Migration: lightning.json added missing key %q", key)
		}
	}

	// Announcements by id
	seedList, _ := seed["lightning_announcements"].([]interface{})
	liveList, _ := live["lightning_announcements"].([]interface{})
	if liveList == nil {
		liveList = []interface{}{}
	}
	have := map[string]bool{}
	for _, item := range liveList {
		if m, ok := item.(map[string]interface{}); ok {
			if id, _ := m["id"].(string); id != "" {
				have[id] = true
			}
		}
	}
	for _, item := range seedList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" || have[id] {
			continue
		}
		liveList = append(liveList, m)
		have[id] = true
		changed = true
		log.Printf("Migration: lightning.json added announcement %q", id)
	}
	live["lightning_announcements"] = liveList

	if !changed {
		log.Printf("Migration: lightning.json already complete — leaving unchanged")
		return nil
	}

	data, err := json.MarshalIndent(live, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func defaultLightningSeed() map[string]interface{} {
	// Minimal additive seed for 1.1.0 features; does not replace local announcements.
	return map[string]interface{}{
		"red_alert_policy": map[string]interface{}{
			"preempt_queue":             true,
			"suppress_non_emergency":    true,
			"reminder_enabled":          true,
			"reminder_interval_minutes": 5,
			"reminder_audio_file":       "Voice_RedAlert_Reminder.mp3",
			"reminder_include_horn":     false,
			"horn_audio_file":           "Horn_RedAlert.mp3",
		},
		"lightning_announcements": []interface{}{
			map[string]interface{}{
				"id":          "THOR_Caution",
				"name":        "THOR Caution",
				"description": "THOR system caution - lightning risk developing",
				"category":    "system_alert",
				"audio_file":  "Voice_Caution.mp3",
				"tts_text":    "THOR Caution: Lightning monitoring system reports developing weather risk. Remain aware of changing conditions.",
				"priority":    6,
				"enabled":     false,
			},
			map[string]interface{}{
				"id":          "THOR_Warning",
				"name":        "THOR Warning",
				"description": "THOR system warning - elevated lightning risk",
				"category":    "system_alert",
				"audio_file":  "Voice_Warning.mp3",
				"tts_text":    "THOR Warning: Lightning activity detected by monitoring system. Exercise caution and be prepared to seek shelter.",
				"priority":    8,
				"enabled":     false,
			},
		},
	}
}

func loadJSONSeedFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func migrateFromPackageSeeds(packageJSONDir string) error {
	if packageJSONDir == "" {
		return nil
	}

	// Generic installer-parity: copy any missing non-protected json/* from package.
	if err := installMissingJSONSeeds(packageJSONDir); err != nil {
		log.Printf("Warning: generic JSON seed install: %v", err)
	}

	if err := migrateEnsureBrowardTGCatalog(packageJSONDir); err != nil {
		log.Printf("Warning: browardtg catalog migrate from package: %v", err)
	}

	// operating_hours: create only if missing
	liveOH := operatingHoursPath()
	if _, err := os.Stat(liveOH); os.IsNotExist(err) {
		seedPath := filepath.Join(packageJSONDir, "operating_hours.json")
		if data, err := os.ReadFile(seedPath); err == nil {
			if err := os.MkdirAll(filepath.Dir(liveOH), 0755); err == nil {
				_ = os.WriteFile(liveOH, data, 0644)
				log.Printf("Migration: created operating_hours.json from package seed")
			}
		} else {
			_ = migrateEnsureOperatingHours()
		}
	}

	// lightning additive from package seed if present
	seedPath := filepath.Join(packageJSONDir, "lightning.json")
	if seed, err := loadJSONSeedFile(seedPath); err == nil {
		return migrateLightningAdditive(seed)
	}
	return migrateLightningAdditive(nil)
}

// installMissingJSONSeeds copies package json/* into live json/ only when the live file is absent.
// Protected basenames are never copied (operator secrets / selections).
func installMissingJSONSeeds(packageJSONDir string) error {
	entries, err := os.ReadDir(packageJSONDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	liveDir := ""
	if app != nil && app.Config != nil && app.Config.JSONDir != "" {
		liveDir = app.Config.JSONDir
	} else {
		liveDir = "json"
	}
	if err := os.MkdirAll(liveDir, 0755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		if shouldSkipJSONOverwrite(name) {
			continue
		}
		dest := filepath.Join(liveDir, name)
		if fileExists(dest) {
			continue
		}
		src := filepath.Join(packageJSONDir, name)
		data, err := os.ReadFile(src)
		if err != nil {
			log.Printf("Warning: read package seed %s: %v", name, err)
			continue
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			log.Printf("Warning: write live seed %s: %v", name, err)
			continue
		}
		log.Printf("Migration: created json/%s from package seed", name)
	}
	return nil
}

func protectedJSONBasenames() map[string]bool {
	return map[string]bool{
		"admin_config.json":          true,
		"cron.json":                  true,
		"trains_selected.json":       true,
		"destinations_selected.json": true,
		"audio_settings.json":        true,
		"operating_hours.json":       true,
		"install_version.json":       true,
		"lightning.json":             true, // additive merge only — never bulk-copy over operator config
	}
}

func shouldSkipJSONOverwrite(name string) bool {
	return protectedJSONBasenames()[strings.ToLower(name)]
}
