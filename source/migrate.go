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
			"preempt_queue":              true,
			"suppress_non_emergency":     true,
			"reminder_enabled":           true,
			"reminder_interval_minutes":  5,
			"reminder_audio_file":        "thor_repeat1.mp3",
			"reminder_include_horn":      false,
			"horn_audio_file":            "thor_red_alert.mp3",
		},
		"lightning_announcements": []interface{}{
			map[string]interface{}{
				"id":          "THOR_Caution",
				"name":        "THOR Caution",
				"description": "THOR system caution - lightning risk developing",
				"category":    "system_alert",
				"audio_file":  "thor_caution.mp3",
				"tts_text":    "THOR Caution: Lightning monitoring system reports developing weather risk. Remain aware of changing conditions.",
				"priority":    6,
				"enabled":     true,
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

func protectedJSONBasenames() map[string]bool {
	return map[string]bool{
		"admin_config.json":          true,
		"cron.json":                  true,
		"trains_selected.json":       true,
		"destinations_selected.json": true,
		"audio_settings.json":        true,
		"operating_hours.json":       true,
		"install_version.json":       true,
	}
}

func shouldSkipJSONOverwrite(name string) bool {
	return protectedJSONBasenames()[strings.ToLower(name)]
}
