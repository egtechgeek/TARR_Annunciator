package main

import (
	"strings"
	"time"
)

// FeedAnnounceConfig controls per-feed auto-announce overrides.
type FeedAnnounceConfig struct {
	InheritGlobal bool `json:"inherit_global"`
	RedAlert      bool `json:"RedAlert"`
	Warning       bool `json:"Warning"`
	Caution       bool `json:"Caution"`
	AllClear      bool `json:"AllClear"`
	Unknown       bool `json:"Unknown"`
}

// FeedHornOverride controls per-feed horn precede for RedAlert / AllClear.
// When InheritGlobal is true, ConditionAudio global defaults are used.
type FeedHornOverride struct {
	InheritGlobal bool   `json:"inherit_global"`
	Enabled       bool   `json:"enabled"`
	AudioFile     string `json:"audio_file"`
}

// LightningFeedConfig is one XML feed slot (primary / failover_1 / failover_2).
type LightningFeedConfig struct {
	ID                  string                      `json:"id"`
	Label               string                      `json:"label"`
	Enabled             bool                        `json:"enabled"`
	SensorID            string                      `json:"sensor_id"`
	Source              string                      `json:"source"` // catalog | custom
	ExpectedDisplayname string                      `json:"expected_displayname"`
	ExpectedUniqueID    string                      `json:"expected_uniqueid"`
	URL                 string                      `json:"url"`
	TimeoutSeconds      int                         `json:"timeout_seconds"` // 0 = use monitor.timeout
	Announce            FeedAnnounceConfig          `json:"announce"`
	AudioByCondition    map[string]string           `json:"audio_by_condition"`
	HornByCondition     map[string]FeedHornOverride `json:"horn_by_condition"`
}

// FailoverTriggers which failure classes count toward consecutive_failures.
type FailoverTriggers struct {
	HTTPError             bool `json:"http_error"`
	HTTPStatusNotOK       bool `json:"http_status_not_ok"`
	EmptyBody             bool `json:"empty_body"`
	EncodingError         bool `json:"encoding_error"`
	MissingLightningAlert bool `json:"missing_lightningalert"`
	UnknownCondition      bool `json:"unknown_condition"`
	DisplaynameMismatch   bool `json:"displayname_mismatch"`
	UniqueIDMismatch      bool `json:"uniqueid_mismatch"`
	StaleLocaltime        bool `json:"stale_localtime"`      // XML <localtime> date >24h behind app time
	TelemetryCollapse     bool `json:"telemetry_collapse"`   // sticky LHL/DI/AD cliff — Layer B feed switch only
}

// TelemetryCollapseThresholds controls the LHL/DI/AD cliff detector (JSON/MANUAL defaults).
type TelemetryCollapseThresholds struct {
	HistorySamples   int     `json:"history_samples"`    // lookback N (default 3)
	ElevatedLHL      float64 `json:"elevated_lhl"`       // default 3
	ElevatedDI       float64 `json:"elevated_di"`        // default 2.3
	ElevatedAD       float64 `json:"elevated_ad"`        // default 1
	FloorLHLMax      float64 `json:"floor_lhl_max"`      // default 1 (0 or 1 = floor)
}

func defaultTelemetryCollapseThresholds() TelemetryCollapseThresholds {
	return TelemetryCollapseThresholds{
		HistorySamples: 3,
		ElevatedLHL:    3,
		ElevatedDI:     2.3,
		ElevatedAD:     1,
		FloorLHLMax:    1,
	}
}

// LightningFailoverPolicy is Admin-owned failover/failback policy (no PA fields).
type LightningFailoverPolicy struct {
	ConsecutiveFailures           int              `json:"consecutive_failures"`
	FailureWindowSeconds          int              `json:"failure_window_seconds"`
	MinDwellOnFeedSeconds         int              `json:"min_dwell_on_feed_seconds"`
	FailbackMode                  string           `json:"failback_mode"` // sticky | prefer_primary | prefer_highest_priority
	FailbackAfterSuccesses        int              `json:"failback_after_successes"`
	FailbackProbeIntervalSeconds  int              `json:"failback_probe_interval_seconds"`
	AllowFailoverDuringRedAlert   bool             `json:"allow_failover_during_red_alert"`
	AllClearReleaseMode           string           `json:"allclear_release_mode"` // primary_only | failover_vote
	RequireAllClearFromSameFeed   bool             `json:"require_allclear_from_same_feed,omitempty"` // legacy; ignored at runtime
	PreserveConditionAcrossFail   bool             `json:"preserve_condition_across_failover"`
	OnAllFeedsFailed              string                       `json:"on_all_feeds_failed"` // hold_last_condition | force_unknown_status
	Triggers                      FailoverTriggers             `json:"triggers"`
	TelemetryCollapse             TelemetryCollapseThresholds  `json:"telemetry_collapse,omitempty"`
}

// CompositeRedAlertRule enters Red Alert lock when multiple feeds each report a required condition.
// Unlock / All Clear release authority is unchanged (enter-only).
type CompositeRedAlertRule struct {
	ID               string   `json:"id"`
	Enabled          bool     `json:"enabled"`
	Label            string   `json:"label"`
	RequireFeedIDs   []string `json:"require_feed_ids"`
	RequireCondition string   `json:"require_condition"` // e.g. Warning
}

// FeedSwitchAnnouncement is one optional PA rule for an active-feed transition.
type FeedSwitchAnnouncement struct {
	ID               string `json:"id"`
	Enabled          bool   `json:"enabled"`
	FromFeedID       string `json:"from_feed_id"` // primary|failover_1|failover_2|*
	ToFeedID         string `json:"to_feed_id"`
	Reason           string `json:"reason"` // any|failover|failback|pin
	MatchSensorID    string `json:"match_sensor_id"`
	MatchDisplayname string `json:"match_displayname"`
	AudioFile        string `json:"audio_file"`
	Label            string `json:"label"`
}

// LightningAnnounceTiming is optional anti-spam for condition announces.
type LightningAnnounceTiming struct {
	MinSecondsBetweenSameCondition      int `json:"min_seconds_between_same_condition"`
	MinSecondsBetweenAnyLightningAnnounce int `json:"min_seconds_between_any_lightning_announce"`
}

// DisplaynameOverride maps live XML displayname to per-condition MP3 overrides.
type DisplaynameOverride struct {
	MatchDisplayname string            `json:"match_displayname"`
	Enabled          bool              `json:"enabled"`
	AudioByCondition map[string]string `json:"audio_by_condition"`
}

// TelemetrySample is one poll's LHL/DI/AD snapshot for cliff detection.
type TelemetrySample struct {
	At  time.Time `json:"at"`
	LHL float64   `json:"lhl"`
	DI  float64   `json:"di"`
	AD  float64   `json:"ad"`
	OK  bool      `json:"ok"` // false if any metric tag was missing/unparseable
}

// FeedHealth is runtime health for one feed slot (status API).
type FeedHealth struct {
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	ConsecutiveSuccesses int       `json:"consecutive_successes"`
	LastOK               time.Time `json:"last_ok,omitempty"`
	LastError            string    `json:"last_error,omitempty"`
	LastErrorAt          time.Time `json:"last_error_at,omitempty"`
	LastDisplayname      string    `json:"last_displayname,omitempty"`
	LastUniqueID         string    `json:"last_uniqueid,omitempty"`
	LastAlert            string    `json:"last_alert,omitempty"`
	LastLHL              *float64  `json:"last_lhl,omitempty"`
	LastDI               *float64  `json:"last_di,omitempty"`
	LastAD               *float64  `json:"last_ad,omitempty"`
	TelemetryCollapse    bool      `json:"telemetry_collapse,omitempty"` // sticky until DI>0 or AD>0
	FailureTimes         []time.Time `json:"-"`
	TelemetryHistory     []TelemetrySample `json:"-"`
}

func defaultFailoverTriggers() FailoverTriggers {
	return FailoverTriggers{
		HTTPError:             true,
		HTTPStatusNotOK:       true,
		EmptyBody:             true,
		EncodingError:         true,
		MissingLightningAlert: true,
		UnknownCondition:      false,
		DisplaynameMismatch:   false,
		UniqueIDMismatch:      false,
		StaleLocaltime:        true,
		TelemetryCollapse:     true,
	}
}

func defaultLightningFailoverPolicy() LightningFailoverPolicy {
	return LightningFailoverPolicy{
		ConsecutiveFailures:          3,
		FailureWindowSeconds:         0,
		MinDwellOnFeedSeconds:        0,
		FailbackMode:                 "prefer_primary",
		FailbackAfterSuccesses:       2,
		FailbackProbeIntervalSeconds: 0,
		AllowFailoverDuringRedAlert:  true,
		AllClearReleaseMode:          "primary_only",
		RequireAllClearFromSameFeed:  false,
		PreserveConditionAcrossFail:  true,
		OnAllFeedsFailed:             "hold_last_condition",
		Triggers:                     defaultFailoverTriggers(),
		TelemetryCollapse:            defaultTelemetryCollapseThresholds(),
	}
}

func normalizeAllClearReleaseMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "failover_vote":
		return "failover_vote"
	default:
		return "primary_only"
	}
}

func defaultFeedAnnounceInherit() FeedAnnounceConfig {
	return FeedAnnounceConfig{InheritGlobal: true}
}

func defaultFeedHornInherit() map[string]FeedHornOverride {
	return map[string]FeedHornOverride{
		"RedAlert": {InheritGlobal: true, Enabled: true, AudioFile: ""},
		"AllClear": {InheritGlobal: true, Enabled: true, AudioFile: ""},
	}
}

func defaultFeedSlots() []LightningFeedConfig {
	return []LightningFeedConfig{
		{
			ID:                  "primary",
			Label:               "Primary",
			Enabled:             false,
			Source:              "custom",
			URL:                 "",
			Announce:            defaultFeedAnnounceInherit(),
			AudioByCondition:    map[string]string{},
			HornByCondition:     defaultFeedHornInherit(),
		},
		{
			ID:               "failover_1",
			Label:            "Failover 1",
			Enabled:          false,
			Source:           "custom",
			URL:              "",
			Announce:         defaultFeedAnnounceInherit(),
			AudioByCondition: map[string]string{},
			HornByCondition:  defaultFeedHornInherit(),
		},
		{
			ID:               "failover_2",
			Label:            "Failover 2",
			Enabled:          false,
			Source:           "custom",
			URL:              "",
			Announce:         defaultFeedAnnounceInherit(),
			AudioByCondition: map[string]string{},
			HornByCondition:  defaultFeedHornInherit(),
		},
	}
}

func defaultFeedSwitchAnnouncementSeeds() []FeedSwitchAnnouncement {
	return []FeedSwitchAnnouncement{
		{
			ID:         "p_to_f1",
			Enabled:    false,
			FromFeedID: "primary",
			ToFeedID:   "failover_1",
			Reason:     "failover",
			Label:      "Primary to Failover 1",
		},
		{
			ID:         "p_to_f2",
			Enabled:    false,
			FromFeedID: "primary",
			ToFeedID:   "failover_2",
			Reason:     "failover",
			Label:      "Primary to Failover 2",
		},
	}
}

func conditionKeyNormalize(cond string) string {
	c := strings.ToLower(strings.TrimSpace(cond))
	switch c {
	case "redalert", "red_alert":
		return "RedAlert"
	case "warning":
		return "Warning"
	case "caution":
		return "Caution"
	case "allclear", "all_clear":
		return "AllClear"
	case "unknown":
		return "Unknown"
	default:
		if cond == "" {
			return ""
		}
		// Preserve common casing for map lookups
		if strings.EqualFold(cond, "RedAlert") {
			return "RedAlert"
		}
		return cond
	}
}

func feedAnnounceEnabled(feed *LightningFeedConfig, condition string) bool {
	if feed == nil || feed.Announce.InheritGlobal {
		return isConditionAnnounceEnabled(condition)
	}
	switch strings.ToLower(strings.TrimSpace(condition)) {
	case "redalert":
		return feed.Announce.RedAlert
	case "warning":
		return feed.Announce.Warning
	case "caution":
		return feed.Announce.Caution
	case "allclear":
		return feed.Announce.AllClear
	case "unknown":
		return feed.Announce.Unknown
	default:
		return isConditionAnnounceEnabled(condition)
	}
}

func lookupAudioMap(m map[string]string, condition string) string {
	if m == nil {
		return ""
	}
	key := conditionKeyNormalize(condition)
	if v := strings.TrimSpace(m[key]); v != "" {
		return v
	}
	// try raw / lower keys
	if v := strings.TrimSpace(m[condition]); v != "" {
		return v
	}
	if v := strings.TrimSpace(m[strings.ToLower(condition)]); v != "" {
		return v
	}
	return ""
}

// resolveConditionAnnounceFile picks the spoken/announce MP3 basename for a condition.
func resolveConditionAnnounceFile(feed *LightningFeedConfig, condition, liveDisplayname string) string {
	key := conditionKeyNormalize(condition)
	if feed != nil {
		if f := lookupAudioMap(feed.AudioByCondition, key); f != "" {
			return f
		}
	}
	if lightningConfig != nil {
		for _, ov := range lightningConfig.DisplaynameOverrides {
			if !ov.Enabled {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(ov.MatchDisplayname), strings.TrimSpace(liveDisplayname)) {
				if f := lookupAudioMap(ov.AudioByCondition, key); f != "" {
					return f
				}
			}
		}
		if clip, ok := getConditionAudioClip(key); ok && strings.TrimSpace(clip.AnnounceFile) != "" {
			return strings.TrimSpace(clip.AnnounceFile)
		}
		// Legacy fallback: lightning_announcements[].audio_file
		for i := range lightningConfig.LightningAnnouncements {
			a := &lightningConfig.LightningAnnouncements[i]
			id := strings.ToLower(a.ID)
			match := false
			switch strings.ToLower(key) {
			case "redalert":
				match = strings.Contains(id, "redalert") || strings.Contains(id, "red_alert")
			case "warning":
				match = strings.Contains(id, "warning") && !strings.Contains(id, "red")
			case "caution":
				match = strings.Contains(id, "caution")
			case "allclear":
				match = strings.Contains(id, "allclear") || strings.Contains(id, "all_clear")
			case "unknown":
				match = strings.Contains(id, "unknown")
			}
			if match && strings.TrimSpace(a.AudioFile) != "" {
				return strings.TrimSpace(a.AudioFile)
			}
		}
	}
	return ""
}

// resolveConditionHorn returns whether to play a precede horn and which file.
func resolveConditionHorn(feed *LightningFeedConfig, condition string) (enabled bool, file string) {
	key := conditionKeyNormalize(condition)
	switch strings.ToLower(key) {
	case "redalert", "allclear":
		// only these two support horns by design
	default:
		return false, ""
	}

	if feed != nil && feed.HornByCondition != nil {
		if ov, ok := feed.HornByCondition[key]; ok {
			if !ov.InheritGlobal {
				return ov.Enabled, strings.TrimSpace(ov.AudioFile)
			}
		}
		// also try lowercase key
		if ov, ok := feed.HornByCondition[strings.ToLower(key)]; ok && !ov.InheritGlobal {
			return ov.Enabled, strings.TrimSpace(ov.AudioFile)
		}
	}

	if clip, ok := getConditionAudioClip(key); ok {
		return clip.HornEnabled, strings.TrimSpace(clip.HornFile)
	}
	return false, ""
}

// resolveLightningAudioFiles returns the full play sequence: optional horn then announce MP3.
func resolveLightningAudioFiles(feed *LightningFeedConfig, condition, liveDisplayname string) []string {
	key := conditionKeyNormalize(condition)
	announce := resolveConditionAnnounceFile(feed, key, liveDisplayname)
	hornOn, hornFile := resolveConditionHorn(feed, key)

	var files []string
	if hornOn && hornFile != "" {
		files = append(files, hornFile)
	}
	if announce != "" {
		// Avoid duplicating the same clip if misconfigured
		if len(files) == 0 || !strings.EqualFold(files[0], announce) {
			files = append(files, announce)
		}
	}
	return files
}

func feedSwitchRuleSpecificity(rule FeedSwitchAnnouncement, fromID, toID, reason, destSensorID, destDisplayname string) int {
	if !rule.Enabled {
		return -1
	}
	fromOK := rule.FromFeedID == "*" || strings.EqualFold(rule.FromFeedID, fromID)
	toOK := rule.ToFeedID == "*" || strings.EqualFold(rule.ToFeedID, toID)
	if !fromOK || !toOK {
		return -1
	}
	r := strings.ToLower(strings.TrimSpace(rule.Reason))
	if r == "" {
		r = "any"
	}
	reasonOK := r == "any" || strings.EqualFold(r, reason)
	if !reasonOK {
		return -1
	}
	sensorMatch := strings.TrimSpace(rule.MatchSensorID) == "" && strings.TrimSpace(rule.MatchDisplayname) == ""
	if strings.TrimSpace(rule.MatchSensorID) != "" {
		if !strings.EqualFold(strings.TrimSpace(rule.MatchSensorID), strings.TrimSpace(destSensorID)) {
			return -1
		}
		sensorMatch = true
	}
	if strings.TrimSpace(rule.MatchDisplayname) != "" {
		if !strings.EqualFold(strings.TrimSpace(rule.MatchDisplayname), strings.TrimSpace(destDisplayname)) {
			return -1
		}
		sensorMatch = true
	}
	if !sensorMatch {
		return -1
	}

	score := 0
	exactFrom := rule.FromFeedID != "*"
	exactTo := rule.ToFeedID != "*"
	exactReason := r != "any"
	hasSensor := strings.TrimSpace(rule.MatchSensorID) != "" || strings.TrimSpace(rule.MatchDisplayname) != ""

	// Specificity ladder from plan
	if exactFrom && exactTo && exactReason && hasSensor {
		score = 100
	} else if exactFrom && exactTo && hasSensor {
		score = 90
	} else if exactFrom && exactTo && exactReason {
		score = 80
	} else if exactFrom && exactTo {
		score = 70
	} else if !exactFrom && exactTo && hasSensor && exactReason {
		score = 60
	} else if !exactFrom && exactTo && hasSensor {
		score = 55
	} else if !exactFrom && exactTo && exactReason {
		score = 50
	} else if !exactFrom && exactTo {
		score = 45
	} else if exactFrom && !exactTo && hasSensor {
		score = 40
	} else if exactFrom && !exactTo {
		score = 30
	} else {
		score = 10
	}
	return score
}

func resolveFeedSwitchAnnouncement(fromID, toID, reason, destSensorID, destDisplayname string) *FeedSwitchAnnouncement {
	if lightningConfig == nil {
		return nil
	}
	var best *FeedSwitchAnnouncement
	bestScore := -1
	for i := range lightningConfig.Monitor.FeedSwitchAnnouncements {
		rule := &lightningConfig.Monitor.FeedSwitchAnnouncements[i]
		score := feedSwitchRuleSpecificity(*rule, fromID, toID, reason, destSensorID, destDisplayname)
		if score > bestScore {
			bestScore = score
			best = rule
		}
	}
	if best == nil || strings.TrimSpace(best.AudioFile) == "" {
		return nil
	}
	cp := *best
	return &cp
}

func primaryFeedURL(mon LightningMonitorConfig) string {
	for _, f := range mon.Feeds {
		if f.ID == "primary" {
			return strings.TrimSpace(f.URL)
		}
	}
	if len(mon.Feeds) > 0 {
		return strings.TrimSpace(mon.Feeds[0].URL)
	}
	return strings.TrimSpace(mon.URL)
}

func firstEnabledFeedURL(mon LightningMonitorConfig) string {
	for _, f := range mon.Feeds {
		if f.Enabled && strings.TrimSpace(f.URL) != "" {
			return strings.TrimSpace(f.URL)
		}
	}
	return primaryFeedURL(mon)
}

func feedByID(mon LightningMonitorConfig, id string) *LightningFeedConfig {
	for i := range mon.Feeds {
		if mon.Feeds[i].ID == id {
			return &mon.Feeds[i]
		}
	}
	return nil
}

func enabledFeedsInOrder(mon LightningMonitorConfig) []LightningFeedConfig {
	var out []LightningFeedConfig
	for _, f := range mon.Feeds {
		if f.Enabled && strings.TrimSpace(f.URL) != "" {
			out = append(out, f)
		}
	}
	return out
}

func nextEnabledFeedAfter(mon LightningMonitorConfig, currentID string) *LightningFeedConfig {
	feeds := enabledFeedsInOrder(mon)
	if len(feeds) == 0 {
		return nil
	}
	found := false
	for i := range feeds {
		if feeds[i].ID == currentID {
			found = true
			continue
		}
		if found {
			cp := feeds[i]
			return &cp
		}
	}
	return nil
}

func preferredFailbackFeed(mon LightningMonitorConfig, currentID string) *LightningFeedConfig {
	policy := mon.Failover
	feeds := enabledFeedsInOrder(mon)
	if len(feeds) == 0 {
		return nil
	}
	switch policy.FailbackMode {
	case "sticky":
		return nil
	case "prefer_highest_priority":
		// Highest priority = first enabled feed in slot order.
		if feeds[0].ID == currentID {
			return nil
		}
		cp := feeds[0]
		return &cp
	default: // prefer_primary
		for i := range feeds {
			if feeds[i].ID == "primary" && currentID != "primary" {
				cp := feeds[i]
				return &cp
			}
		}
		// Primary missing/disabled: fail back to highest-priority feed above current.
		if feeds[0].ID == currentID {
			return nil
		}
		cp := feeds[0]
		return &cp
	}
}
