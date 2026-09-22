package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// LightningTrigger represents a lightning monitoring trigger
type LightningTrigger struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Enabled           bool      `json:"enabled"`
	URL               string    `json:"url"` // compatibility: active or primary URL
	FetchInterval     int       `json:"fetch_interval"` // seconds
	Timeout           int       `json:"timeout"`        // seconds
	LastCondition     string    `json:"last_condition"`
	LastFetch         time.Time `json:"last_fetch"`
	LastConditionTime time.Time `json:"last_condition_time"`

	// Internal state
	isRunning              bool
	stopOnce               sync.Once
	stopChan               chan struct{}
	mu                     sync.Mutex
	redAlertActive         bool
	redAlertSince          time.Time
	reminderCount          int
	lastReminderAt         time.Time
	nextReminderAt         time.Time
	reminderStop           chan struct{}
	lastFetchError         string
	lastFetchErrorLog      time.Time
	loggedFetchRecover     bool
	feeds                  []LightningFeedConfig
	failover               LightningFailoverPolicy
	announceTiming         LightningAnnounceTiming
	activeFeedID           string
	pinnedFeedID           string
	feedHealth             map[string]*FeedHealth
	allFeedsFailed         bool
	activeSince            time.Time
	enteredRedAlertOnFeed  string
	skipNextConditionAnnounce bool
	lastAnnounceCondition  string
	lastAnnounceAt         time.Time
	lastAnyAnnounceAt      time.Time
	activeDisplayname      string
	activeUniqueID         string
	lastProbeAt            time.Time
	lastAllClearVoteSummary string
	// Manual override (Admin) — distinct from Manual Test audio drills.
	manualOverrideActive    bool
	manualOverrideCondition string
	manualOverrideAt        time.Time
	manualOverrideNote      string
	lastCompositeEnterSummary string
}

// LightningAnnouncement represents a lightning announcement from the JSON config
type LightningAnnouncement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	AudioFile   string `json:"audio_file"`
	TTSText     string `json:"tts_text"`
	Priority    int    `json:"priority"`
	Enabled     bool   `json:"enabled"`
}

// LightningMonitorConfig is the Thor Guard XML poller settings (persisted in lightning.json).
type LightningMonitorConfig struct {
	Enabled                 bool                     `json:"enabled"`
	URL                     string                   `json:"url,omitempty"` // compatibility mirror of primary URL
	FetchInterval           int                      `json:"fetch_interval"`
	Timeout                 int                      `json:"timeout"`
	Failover                LightningFailoverPolicy  `json:"failover"`
	Feeds                   []LightningFeedConfig    `json:"feeds"`
	FeedSwitchAnnouncements []FeedSwitchAnnouncement `json:"feed_switch_announcements"`
	CompositeRedAlertRules  []CompositeRedAlertRule  `json:"composite_red_alert_rules"`
}

// LightningConfig represents the lightning.json configuration
type LightningConfig struct {
	Monitor                LightningMonitorConfig   `json:"monitor"`
	LightningAnnouncements []LightningAnnouncement  `json:"lightning_announcements"`
	RedAlertPolicy         RedAlertPolicy           `json:"red_alert_policy"`
	ConditionAudio         ConditionAudioConfig     `json:"condition_audio"`
	AnnounceTiming         LightningAnnounceTiming  `json:"announce_timing"`
	DisplaynameOverrides   []DisplaynameOverride    `json:"displayname_overrides"`
	Metadata               json.RawMessage          `json:"metadata,omitempty"`
}

// ConditionAudioClip is the Admin-owned horn + announce pair for one Thor condition.
type ConditionAudioClip struct {
	HornEnabled  bool   `json:"horn_enabled"`
	HornFile     string `json:"horn_file"`
	AnnounceFile string `json:"announce_file"`
}

// ConditionAudioConfig holds global defaults for assembled lightning PA sequences.
type ConditionAudioConfig struct {
	RedAlert ConditionAudioClip `json:"RedAlert"`
	AllClear ConditionAudioClip `json:"AllClear"`
	Warning  ConditionAudioClip `json:"Warning"`
	Caution  ConditionAudioClip `json:"Caution"`
	Unknown  ConditionAudioClip `json:"Unknown"`
}

// RedAlertPolicy controls THOR Guard Red Alert preemption, suppression, and reminders.
type RedAlertPolicy struct {
	PreemptQueue            bool   `json:"preempt_queue"`
	SuppressNonEmergency    bool   `json:"suppress_non_emergency"`
	ReminderEnabled         bool   `json:"reminder_enabled"`
	ReminderIntervalMinutes int    `json:"reminder_interval_minutes"`
	ReminderAudioFile       string `json:"reminder_audio_file"`
	ReminderIncludeHorn     bool   `json:"reminder_include_horn"`
	HornAudioFile           string `json:"horn_audio_file,omitempty"`
}

// Global lightning trigger instance
var lightningTrigger *LightningTrigger
var lightningConfig *LightningConfig

// Initialize lightning trigger system
func initializeLightningTrigger() error {
	_ = loadBrowardTGCatalog()

	if err := loadLightningConfig(); err != nil {
		log.Printf("Warning: Failed to load lightning configuration: %v", err)
		return err
	}

	mon := getLightningMonitorConfig()
	primaryURL := firstEnabledFeedURL(mon)
	if primaryURL == "" {
		log.Printf("Lightning monitor has no enabled feed URL — monitoring will stay stopped until configured in Admin")
		mon.Enabled = false
	}

	lightningTrigger = &LightningTrigger{
		ID:            "lightning_monitor",
		Name:          "Lightning Alert Monitor",
		Enabled:       mon.Enabled,
		URL:           primaryURL,
		FetchInterval: mon.FetchInterval,
		Timeout:       mon.Timeout,
		LastCondition: "Reset",
		stopChan:      make(chan struct{}),
		feeds:         append([]LightningFeedConfig(nil), mon.Feeds...),
		failover:      mon.Failover,
		announceTiming: lightningConfig.AnnounceTiming,
		feedHealth:    map[string]*FeedHealth{},
	}
	for _, f := range mon.Feeds {
		lightningTrigger.feedHealth[f.ID] = &FeedHealth{}
	}
	if len(enabledFeedsInOrder(mon)) > 0 {
		lightningTrigger.activeFeedID = enabledFeedsInOrder(mon)[0].ID
		lightningTrigger.activeSince = time.Now()
	}

	if lightningTrigger.Enabled {
		go lightningTrigger.Start()
		log.Printf("✓ Lightning trigger system initialized and started")
		log.Printf("  - Active feed: %s URL: %s", lightningTrigger.activeFeedID, lightningTrigger.URL)
		log.Printf("  - Fetch interval: %d seconds", lightningTrigger.FetchInterval)
	} else {
		log.Printf("✓ Lightning trigger system initialized (disabled)")
	}

	return nil
}

func lightningConfigPath() string {
	if app != nil && app.Config != nil && app.Config.JSONDir != "" {
		return filepath.Join(app.Config.JSONDir, "lightning.json")
	}
	return filepath.Join("json", "lightning.json")
}

func defaultRedAlertPolicy() RedAlertPolicy {
	return RedAlertPolicy{
		PreemptQueue:            true,
		SuppressNonEmergency:    true,
		ReminderEnabled:         true,
		ReminderIntervalMinutes: 5,
		ReminderAudioFile:       "Voice_RedAlert_Reminder.mp3",
		ReminderIncludeHorn:     false,
		HornAudioFile:           "Horn_RedAlert.mp3",
	}
}

func defaultConditionAudioConfig() ConditionAudioConfig {
	return ConditionAudioConfig{
		RedAlert: ConditionAudioClip{HornEnabled: true, HornFile: "Horn_RedAlert.mp3", AnnounceFile: "Voice_RedAlert.mp3"},
		AllClear: ConditionAudioClip{HornEnabled: true, HornFile: "Horn_AllClear.mp3", AnnounceFile: "Voice_AllClear.mp3"},
		Warning:  ConditionAudioClip{HornEnabled: false, HornFile: "", AnnounceFile: "Voice_Warning.mp3"},
		Caution:  ConditionAudioClip{HornEnabled: false, HornFile: "", AnnounceFile: "Voice_Caution.mp3"},
		Unknown:  ConditionAudioClip{HornEnabled: false, HornFile: "", AnnounceFile: "Voice_Unknown.mp3"},
	}
}

func getConditionAudioConfig() ConditionAudioConfig {
	if lightningConfig != nil {
		lightningConfig.ensurePolicyDefaults()
		return lightningConfig.ConditionAudio
	}
	return defaultConditionAudioConfig()
}

func getConditionAudioClip(condition string) (ConditionAudioClip, bool) {
	cfg := getConditionAudioConfig()
	switch strings.ToLower(strings.TrimSpace(condition)) {
	case "redalert":
		return cfg.RedAlert, true
	case "allclear":
		return cfg.AllClear, true
	case "warning":
		return cfg.Warning, true
	case "caution":
		return cfg.Caution, true
	case "unknown":
		return cfg.Unknown, true
	default:
		return ConditionAudioClip{}, false
	}
}

func defaultLightningMonitorConfig() LightningMonitorConfig {
	return LightningMonitorConfig{
		Enabled:                 false,
		URL:                     "",
		FetchInterval:           60,
		Timeout:                 30,
		Failover:                defaultLightningFailoverPolicy(),
		Feeds:                   defaultFeedSlots(),
		FeedSwitchAnnouncements: defaultFeedSwitchAnnouncementSeeds(),
	}
}

func (c *LightningConfig) ensurePolicyDefaults() {
	if c.Monitor.FetchInterval < 30 {
		if c.Monitor.FetchInterval == 0 && c.Monitor.Timeout == 0 && len(c.Monitor.Feeds) == 0 && c.Monitor.URL == "" {
			def := defaultLightningMonitorConfig()
			c.Monitor.FetchInterval = def.FetchInterval
			c.Monitor.Timeout = def.Timeout
			c.Monitor.Enabled = false
		} else if c.Monitor.FetchInterval < 30 {
			c.Monitor.FetchInterval = 30
		}
	}
	if c.Monitor.Timeout < 5 {
		c.Monitor.Timeout = 30
	}
	if c.Monitor.Failover.ConsecutiveFailures < 1 {
		c.Monitor.Failover = defaultLightningFailoverPolicy()
	} else {
		if c.Monitor.Failover.FailbackMode == "" {
			c.Monitor.Failover.FailbackMode = "prefer_primary"
		}
		if c.Monitor.Failover.FailbackAfterSuccesses < 1 {
			c.Monitor.Failover.FailbackAfterSuccesses = 2
		}
		if c.Monitor.Failover.OnAllFeedsFailed == "" {
			c.Monitor.Failover.OnAllFeedsFailed = "hold_last_condition"
		}
		tr := c.Monitor.Failover.Triggers
		if !tr.HTTPError && !tr.HTTPStatusNotOK && !tr.EmptyBody && !tr.EncodingError &&
			!tr.MissingLightningAlert && !tr.UnknownCondition && !tr.DisplaynameMismatch && !tr.UniqueIDMismatch && !tr.StaleLocaltime {
			c.Monitor.Failover.Triggers = defaultFailoverTriggers()
		}
	}
	c.Monitor.Failover.AllClearReleaseMode = normalizeAllClearReleaseMode(c.Monitor.Failover.AllClearReleaseMode)
	// Legacy require_allclear_from_same_feed is ignored; never re-assert as release authority.
	c.Monitor.Failover.RequireAllClearFromSameFeed = false
	if len(c.Monitor.Feeds) == 0 {
		slots := defaultFeedSlots()
		if strings.TrimSpace(c.Monitor.URL) != "" {
			slots[0].URL = strings.TrimSpace(c.Monitor.URL)
			slots[0].Enabled = c.Monitor.Enabled
			if s := findBrowardTGSensorByURL(slots[0].URL); s != nil {
				slots[0].SensorID = s.ID
				slots[0].ExpectedDisplayname = s.DisplayName
				slots[0].Source = "catalog"
			}
		}
		c.Monitor.Feeds = slots
	} else {
		// Ensure three slots exist by id without wiping
		have := map[string]bool{}
		for _, f := range c.Monitor.Feeds {
			have[f.ID] = true
		}
		for _, stub := range defaultFeedSlots() {
			if !have[stub.ID] {
				c.Monitor.Feeds = append(c.Monitor.Feeds, stub)
			}
		}
		for i := range c.Monitor.Feeds {
			if c.Monitor.Feeds[i].AudioByCondition == nil {
				c.Monitor.Feeds[i].AudioByCondition = map[string]string{}
			}
			if c.Monitor.Feeds[i].HornByCondition == nil {
				c.Monitor.Feeds[i].HornByCondition = defaultFeedHornInherit()
			} else {
				for _, key := range []string{"RedAlert", "AllClear"} {
					if _, ok := c.Monitor.Feeds[i].HornByCondition[key]; !ok {
						c.Monitor.Feeds[i].HornByCondition[key] = FeedHornOverride{InheritGlobal: true, Enabled: true}
					}
				}
			}
			if c.Monitor.Feeds[i].Source == "" {
				c.Monitor.Feeds[i].Source = "custom"
			}
		}
	}
	// Compatibility mirror
	c.Monitor.URL = primaryFeedURL(c.Monitor)

	if c.Monitor.FeedSwitchAnnouncements == nil {
		c.Monitor.FeedSwitchAnnouncements = []FeedSwitchAnnouncement{}
	}
	if c.Monitor.CompositeRedAlertRules == nil {
		c.Monitor.CompositeRedAlertRules = []CompositeRedAlertRule{}
	}
	if c.DisplaynameOverrides == nil {
		c.DisplaynameOverrides = []DisplaynameOverride{}
	}

	defAudio := defaultConditionAudioConfig()
	if c.ConditionAudio.RedAlert.AnnounceFile == "" && c.ConditionAudio.RedAlert.HornFile == "" {
		c.ConditionAudio.RedAlert = defAudio.RedAlert
	} else {
		if c.ConditionAudio.RedAlert.AnnounceFile == "" {
			c.ConditionAudio.RedAlert.AnnounceFile = defAudio.RedAlert.AnnounceFile
		}
		if c.ConditionAudio.RedAlert.HornEnabled && c.ConditionAudio.RedAlert.HornFile == "" {
			c.ConditionAudio.RedAlert.HornFile = defAudio.RedAlert.HornFile
		}
	}
	if c.ConditionAudio.AllClear.AnnounceFile == "" && c.ConditionAudio.AllClear.HornFile == "" {
		c.ConditionAudio.AllClear = defAudio.AllClear
	} else {
		if c.ConditionAudio.AllClear.AnnounceFile == "" {
			c.ConditionAudio.AllClear.AnnounceFile = defAudio.AllClear.AnnounceFile
		}
		if c.ConditionAudio.AllClear.HornEnabled && c.ConditionAudio.AllClear.HornFile == "" {
			c.ConditionAudio.AllClear.HornFile = defAudio.AllClear.HornFile
		}
	}
	if c.ConditionAudio.Warning.AnnounceFile == "" {
		c.ConditionAudio.Warning = defAudio.Warning
	}
	if c.ConditionAudio.Caution.AnnounceFile == "" {
		c.ConditionAudio.Caution = defAudio.Caution
	}
	if c.ConditionAudio.Unknown.AnnounceFile == "" {
		c.ConditionAudio.Unknown = defAudio.Unknown
	}

	if c.RedAlertPolicy.ReminderIntervalMinutes <= 0 {
		c.RedAlertPolicy = defaultRedAlertPolicy()
	}
	if c.RedAlertPolicy.ReminderAudioFile == "" {
		c.RedAlertPolicy.ReminderAudioFile = "Voice_RedAlert_Reminder.mp3"
	}
	if c.RedAlertPolicy.HornAudioFile == "" {
		if c.ConditionAudio.RedAlert.HornFile != "" {
			c.RedAlertPolicy.HornAudioFile = c.ConditionAudio.RedAlert.HornFile
		} else {
			c.RedAlertPolicy.HornAudioFile = "Horn_RedAlert.mp3"
		}
	}
}

func getLightningMonitorConfig() LightningMonitorConfig {
	if lightningConfig != nil {
		lightningConfig.ensurePolicyDefaults()
		return lightningConfig.Monitor
	}
	return defaultLightningMonitorConfig()
}

// isConditionAnnounceEnabled reports whether automatic announcements for a Thor condition
// should play. Manual Admin "Test" buttons bypass this. Red Alert lock enter/exit is separate.
func isConditionAnnounceEnabled(condition string) bool {
	if lightningConfig == nil {
		return true
	}
	cond := strings.ToLower(strings.TrimSpace(condition))
	for i := range lightningConfig.LightningAnnouncements {
		a := &lightningConfig.LightningAnnouncements[i]
		id := strings.ToLower(a.ID)
		switch cond {
		case "redalert":
			if strings.Contains(id, "redalert") || strings.Contains(id, "red_alert") {
				return a.Enabled
			}
		case "warning":
			if strings.Contains(id, "warning") && !strings.Contains(id, "red") {
				return a.Enabled
			}
		case "caution":
			if strings.Contains(id, "caution") {
				return a.Enabled
			}
		case "allclear":
			if strings.Contains(id, "allclear") || strings.Contains(id, "all_clear") {
				return a.Enabled
			}
		case "unknown":
			if strings.Contains(id, "unknown") {
				return a.Enabled
			}
		}
	}
	return true
}

func setConditionAnnounceEnabled(condition string, enabled bool) bool {
	if lightningConfig == nil {
		return false
	}
	cond := strings.ToLower(strings.TrimSpace(condition))
	updated := false
	for i := range lightningConfig.LightningAnnouncements {
		a := &lightningConfig.LightningAnnouncements[i]
		id := strings.ToLower(a.ID)
		match := false
		switch cond {
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
		if match {
			a.Enabled = enabled
			updated = true
		}
	}
	return updated
}

func announcementEnableSnapshot() map[string]bool {
	return map[string]bool{
		"RedAlert": isConditionAnnounceEnabled("RedAlert"),
		"Warning":  isConditionAnnounceEnabled("Warning"),
		"Caution":  isConditionAnnounceEnabled("Caution"),
		"AllClear": isConditionAnnounceEnabled("AllClear"),
		"Unknown":  isConditionAnnounceEnabled("Unknown"),
	}
}

func getRedAlertPolicy() RedAlertPolicy {
	if lightningConfig != nil {
		lightningConfig.ensurePolicyDefaults()
		return lightningConfig.RedAlertPolicy
	}
	return defaultRedAlertPolicy()
}

func lightningAudioPath(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "lightning/")
	name = strings.TrimPrefix(name, "horns-chimes-tones/")
	name = filepath.Base(name)
	if name == "" {
		name = "Voice_RedAlert.mp3"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".mp3") {
		name += ".mp3"
	}
	mp3Root := "static/mp3"
	if app != nil && app.Config != nil && app.Config.MP3Dir != "" {
		mp3Root = app.Config.MP3Dir
	}
	// Horns live under horns-chimes-tones/ (prefer), with fallback to lightning/ for upgrades.
	if strings.HasPrefix(strings.ToLower(name), "horn_") {
		preferred := filepath.Join(mp3Root, "horns-chimes-tones", name)
		if fileExists(preferred) {
			return preferred
		}
		legacy := filepath.Join(mp3Root, "lightning", name)
		if fileExists(legacy) {
			return legacy
		}
		return preferred
	}
	return filepath.Join(mp3Root, "lightning", name)
}

func stationChimePath() string {
	mp3Root := "static/mp3"
	if app != nil && app.Config != nil && app.Config.MP3Dir != "" {
		mp3Root = app.Config.MP3Dir
	}
	preferred := filepath.Join(mp3Root, "horns-chimes-tones", "chime.mp3")
	if fileExists(preferred) {
		return preferred
	}
	legacy := filepath.Join(mp3Root, "chime.mp3")
	if fileExists(legacy) {
		return legacy
	}
	return preferred
}

// Load lightning configuration from JSON
func loadLightningConfig() error {
	configPath := lightningConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("lightning.json not found at %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read lightning.json: %v", err)
	}

	lightningConfig = &LightningConfig{}
	if err := json.Unmarshal(data, lightningConfig); err != nil {
		return fmt.Errorf("failed to parse lightning.json: %v", err)
	}
	lightningConfig.ensurePolicyDefaults()

	log.Printf("✓ Loaded lightning configuration with %d announcements", len(lightningConfig.LightningAnnouncements))
	log.Printf("  - Red Alert policy: preempt=%v suppress=%v reminder=%v every %dm file=%s",
		lightningConfig.RedAlertPolicy.PreemptQueue,
		lightningConfig.RedAlertPolicy.SuppressNonEmergency,
		lightningConfig.RedAlertPolicy.ReminderEnabled,
		lightningConfig.RedAlertPolicy.ReminderIntervalMinutes,
		lightningConfig.RedAlertPolicy.ReminderAudioFile)
	return nil
}

func saveLightningConfig() error {
	if lightningConfig == nil {
		return fmt.Errorf("lightning configuration not loaded")
	}
	lightningConfig.ensurePolicyDefaults()
	data, err := json.MarshalIndent(lightningConfig, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(lightningConfigPath(), data, 0644)
}

// Start the lightning trigger monitoring
func (t *LightningTrigger) Start() {
	t.mu.Lock()
	if t.isRunning {
		t.mu.Unlock()
		return
	}
	t.isRunning = true
	t.stopOnce = sync.Once{}
	t.stopChan = make(chan struct{})
	interval := t.FetchInterval
	if interval < 30 {
		interval = 30
	}
	t.mu.Unlock()

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	log.Printf("Lightning trigger '%s' started with %d second interval", t.Name, interval)

	t.fetchAndCheck()

	for {
		select {
		case <-ticker.C:
			t.fetchAndCheck()
		case <-t.stopChan:
			t.mu.Lock()
			t.isRunning = false
			t.mu.Unlock()
			log.Printf("Lightning trigger '%s' stopped", t.Name)
			return
		}
	}
}

// Stop the lightning trigger
func (t *LightningTrigger) Stop() {
	t.stopOnce.Do(func() {
		if t.stopChan != nil {
			close(t.stopChan)
		}
	})
}

func (t *LightningTrigger) healthFor(feedID string) *FeedHealth {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.feedHealth == nil {
		t.feedHealth = map[string]*FeedHealth{}
	}
	h, ok := t.feedHealth[feedID]
	if !ok {
		h = &FeedHealth{}
		t.feedHealth[feedID] = h
	}
	return h
}

func (t *LightningTrigger) monitorSnapshot() LightningMonitorConfig {
	if lightningConfig != nil {
		lightningConfig.ensurePolicyDefaults()
		return lightningConfig.Monitor
	}
	return defaultLightningMonitorConfig()
}

func (t *LightningTrigger) resolveActiveFeedLocked(mon LightningMonitorConfig) *LightningFeedConfig {
	if t.pinnedFeedID != "" {
		if f := feedByID(mon, t.pinnedFeedID); f != nil && f.Enabled && strings.TrimSpace(f.URL) != "" {
			return f
		}
	}
	if t.activeFeedID != "" {
		if f := feedByID(mon, t.activeFeedID); f != nil && f.Enabled && strings.TrimSpace(f.URL) != "" {
			return f
		}
	}
	enabled := enabledFeedsInOrder(mon)
	if len(enabled) == 0 {
		return nil
	}
	return &enabled[0]
}

func (t *LightningTrigger) switchActiveFeed(to *LightningFeedConfig, reason string) {
	if to == nil {
		return
	}
	t.mu.Lock()
	fromID := t.activeFeedID
	same := fromID == to.ID
	if !same {
		t.activeFeedID = to.ID
		t.URL = strings.TrimSpace(to.URL)
		t.activeSince = time.Now()
		if t.failover.PreserveConditionAcrossFail {
			t.skipNextConditionAnnounce = true
		}
		t.allFeedsFailed = false
	}
	destSensor := to.SensorID
	destDisplay := to.ExpectedDisplayname
	t.mu.Unlock()

	if same {
		return
	}
	log.Printf("Lightning: active feed changed %s → %s (reason=%s)", fromID, to.ID, reason)
	t.maybePlayFeedSwitchAnnouncement(fromID, to.ID, reason, destSensor, destDisplay)
}

func (t *LightningTrigger) maybePlayFeedSwitchAnnouncement(fromID, toID, reason, destSensor, destDisplay string) {
	rule := resolveFeedSwitchAnnouncement(fromID, toID, reason, destSensor, destDisplay)
	if rule == nil {
		return
	}
	path := lightningAudioPath(rule.AudioFile)
	if _, err := os.Stat(path); err != nil {
		log.Printf("Lightning: feed-switch audio missing %s — skipping", rule.AudioFile)
		return
	}
	if announcementManager == nil {
		return
	}
	params := map[string]interface{}{
		"condition":      "feed_switch",
		"audio_files":    []string{rule.AudioFile},
		"trigger_source": "FEED_SWITCH",
		"message":        rule.Label,
	}
	if _, err := announcementManager.QueueAnnouncement(TypeLightning, AnnouncementPriority(10), params, time.Now()); err != nil {
		log.Printf("Lightning: failed to queue feed-switch announcement: %v", err)
	} else {
		log.Printf("Lightning: queued feed-switch PA %s (%s → %s)", rule.AudioFile, fromID, toID)
	}
}

func extractXMLTag(xmlStr, tag string) string {
	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"
	startIndex := strings.Index(xmlStr, startTag)
	if startIndex == -1 {
		// case-insensitive search for common tags
		lower := strings.ToLower(xmlStr)
		ls := strings.ToLower(startTag)
		le := strings.ToLower(endTag)
		startIndex = strings.Index(lower, ls)
		if startIndex == -1 {
			return ""
		}
		startIndex += len(startTag)
		endIndex := strings.Index(lower[startIndex:], le)
		if endIndex == -1 {
			return ""
		}
		return strings.TrimSpace(xmlStr[startIndex : startIndex+endIndex])
	}
	startIndex += len(startTag)
	endIndex := strings.Index(xmlStr[startIndex:], endTag)
	if endIndex == -1 {
		return ""
	}
	return strings.TrimSpace(xmlStr[startIndex : startIndex+endIndex])
}

type feedFetchResult struct {
	ok            bool
	failureClass  string // empty if ok
	errMsg        string
	alert         string
	displayname   string
	uniqueid      string
	xmlString     string
	lhl           float64
	di            float64
	ad            float64
	metricsOK     bool
}

func (t *LightningTrigger) fetchOneFeed(feed LightningFeedConfig, globalTimeout int) feedFetchResult {
	timeout := feed.TimeoutSeconds
	if timeout < 5 {
		timeout = globalTimeout
	}
	if timeout < 5 {
		timeout = 30
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Get(strings.TrimSpace(feed.URL))
	if err != nil {
		return feedFetchResult{failureClass: "http_error", errMsg: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return feedFetchResult{failureClass: "http_status_not_ok", errMsg: fmt.Sprintf("status %d", resp.StatusCode)}
	}
	xmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return feedFetchResult{failureClass: "http_error", errMsg: err.Error()}
	}
	if len(xmlData) == 0 {
		return feedFetchResult{failureClass: "empty_body", errMsg: "empty body"}
	}
	_ = t.saveXMLFileForFeed(feed.ID, feed.URL, xmlData)
	xmlString, err := t.convertXMLEncoding(xmlData)
	if err != nil {
		return feedFetchResult{failureClass: "encoding_error", errMsg: err.Error()}
	}
	alert := extractXMLTag(xmlString, "lightningalert")
	if alert == "" {
		// try case from legacy helper path
		alert = t.extractLightningAlertFromString(xmlString)
	}
	dn := extractXMLTag(xmlString, "displayname")
	uid := extractXMLTag(xmlString, "uniqueid")
	if alert == "" {
		return feedFetchResult{failureClass: "missing_lightningalert", errMsg: "missing lightningalert", displayname: dn, uniqueid: uid, xmlString: xmlString}
	}
	// Stale <localtime> date check — ignore clock, use calendar date vs app time.
	// Stuck sensors (e.g. Fern Forest frozen on RedAlert) publish an old date.
	localtime := extractXMLTag(xmlString, "localtime")
	if stale, sensorDay, reason := thorLocaltimeStale(localtime, appNow()); stale {
		msg := reason
		if !sensorDay.IsZero() {
			msg = fmt.Sprintf("%s (sensor date %s)", reason, sensorDay.Format("01/02/2006"))
		}
		return feedFetchResult{
			failureClass: "stale_localtime",
			errMsg:       msg,
			alert:        "Unknown",
			displayname:  dn,
			uniqueid:     uid,
			xmlString:    xmlString,
		}
	}
	return feedFetchResult{ok: true, alert: alert, displayname: dn, uniqueid: uid, xmlString: xmlString}
}

// thorLocaltimeStale reports whether Thor <localtime> is more than 24h behind app time.
// Time-of-day is ignored (sensor clocks/timezones are unreliable); only the date is used.
// Empty localtime does not reject the feed (legacy/missing tag).
func thorLocaltimeStale(localtimeStr string, now time.Time) (stale bool, sensorDay time.Time, reason string) {
	localtimeStr = strings.TrimSpace(localtimeStr)
	if localtimeStr == "" {
		return false, time.Time{}, ""
	}
	parsed, err := parseThorLocaltime(localtimeStr)
	if err != nil {
		return true, time.Time{}, "unparseable localtime: " + localtimeStr
	}
	loc := now.Location()
	sensorDay = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc)
	// More than 24h after the start of the sensor's calendar day → date is too old.
	if now.After(sensorDay.Add(24 * time.Hour)) {
		return true, sensorDay, "localtime date more than 24h behind app time"
	}
	return false, sensorDay, ""
}

func parseThorLocaltime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"03:04:05 PM 01/02/2006",
		"3:04:05 PM 01/02/2006",
		"03:04:05 PM 1/2/2006",
		"3:04:05 PM 1/2/2006",
		"15:04:05 01/02/2006",
		"15:04:05 1/2/2006",
	}
	var lastErr error
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func (t *LightningTrigger) triggerEnabled(class string) bool {
	tr := t.failover.Triggers
	switch class {
	case "http_error":
		return tr.HTTPError
	case "http_status_not_ok":
		return tr.HTTPStatusNotOK
	case "empty_body":
		return tr.EmptyBody
	case "encoding_error":
		return tr.EncodingError
	case "missing_lightningalert":
		return tr.MissingLightningAlert
	case "unknown_condition":
		return tr.UnknownCondition
	case "displayname_mismatch":
		return tr.DisplaynameMismatch
	case "uniqueid_mismatch":
		return tr.UniqueIDMismatch
	case "stale_localtime":
		return tr.StaleLocaltime
	case "telemetry_collapse":
		return tr.TelemetryCollapse
	default:
		return true
	}
}

func (t *LightningTrigger) recordFeedFailure(feedID, class, msg string) {
	h := t.healthFor(feedID)
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	h.LastError = fmt.Sprintf("%s: %s", class, msg)
	h.LastErrorAt = now
	h.ConsecutiveSuccesses = 0
	if class == "stale_localtime" || class == "telemetry_collapse" {
		h.LastAlert = "Unknown"
	}
	window := t.failover.FailureWindowSeconds
	if window > 0 {
		cutoff := now.Add(-time.Duration(window) * time.Second)
		filtered := h.FailureTimes[:0]
		for _, ft := range h.FailureTimes {
			if ft.After(cutoff) {
				filtered = append(filtered, ft)
			}
		}
		h.FailureTimes = append(filtered, now)
		h.ConsecutiveFailures = len(h.FailureTimes)
	} else {
		h.ConsecutiveFailures++
	}
	t.lastFetchError = h.LastError
	t.logFetchProblem(fmt.Sprintf("Lightning feed %s failure (%s): %s", feedID, class, msg))
}

func (t *LightningTrigger) recordFeedSuccess(feedID, alert, dn, uid string) {
	h := t.healthFor(feedID)
	t.mu.Lock()
	defer t.mu.Unlock()
	h.ConsecutiveFailures = 0
	h.FailureTimes = nil
	h.ConsecutiveSuccesses++
	h.LastOK = time.Now()
	h.LastError = ""
	h.LastAlert = alert
	h.LastDisplayname = dn
	h.LastUniqueID = uid
	// TelemetryCollapse is sticky; cleared only in applyTelemetryToResult on DI/AD activity.
	t.noteFetchSuccess()
}

// recordStandbyObservation updates Live Status health for a non-active enabled feed.
// It never drives failover / condition changes / global lastFetchError.
func (t *LightningTrigger) recordStandbyObservation(feedID string, res feedFetchResult) {
	h := t.healthFor(feedID)
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if res.ok {
		h.ConsecutiveFailures = 0
		h.FailureTimes = nil
		h.ConsecutiveSuccesses++
		h.LastOK = now
		h.LastError = ""
		h.LastAlert = res.alert
		h.LastDisplayname = res.displayname
		h.LastUniqueID = res.uniqueid
		// TelemetryCollapse is sticky; cleared only in applyTelemetryToResult on DI/AD activity.
		return
	}
	h.ConsecutiveSuccesses = 0
	h.ConsecutiveFailures++
	h.LastError = fmt.Sprintf("%s: %s", res.failureClass, res.errMsg)
	h.LastErrorAt = now
	if res.failureClass == "stale_localtime" || res.failureClass == "telemetry_collapse" {
		h.LastAlert = "Unknown"
		if res.failureClass == "telemetry_collapse" {
			h.TelemetryCollapse = true
		}
		if res.displayname != "" {
			h.LastDisplayname = res.displayname
		}
		if res.uniqueid != "" {
			h.LastUniqueID = res.uniqueid
		}
	}
}

// recordTelemetryCollapseStatusOnly surfaces cliff in Live Status without counting toward failover.
func (t *LightningTrigger) recordTelemetryCollapseStatusOnly(feedID string, res feedFetchResult) {
	h := t.healthFor(feedID)
	t.mu.Lock()
	defer t.mu.Unlock()
	h.LastError = fmt.Sprintf("%s: %s", res.failureClass, res.errMsg)
	h.LastErrorAt = time.Now()
	h.LastAlert = "Unknown"
	h.TelemetryCollapse = true
	h.ConsecutiveSuccesses = 0
	if res.displayname != "" {
		h.LastDisplayname = res.displayname
	}
	if res.uniqueid != "" {
		h.LastUniqueID = res.uniqueid
	}
	t.logFetchProblem(fmt.Sprintf("Lightning feed %s telemetry_collapse (Layer A only, failover trigger off): %s", feedID, res.errMsg))
}

// recordUnknownSoft surfaces Thor Unknown in Live Status without counting a failure.
// Unknown is a normal Thor update-cycle flicker, not http/sensor death.
func (t *LightningTrigger) recordUnknownSoft(feedID string, res feedFetchResult) {
	h := t.healthFor(feedID)
	t.mu.Lock()
	defer t.mu.Unlock()
	h.LastAlert = "Unknown"
	h.LastError = "unknown_condition: Thor Unknown (flicker; not counted as failure)"
	h.LastErrorAt = time.Now()
	if res.displayname != "" {
		h.LastDisplayname = res.displayname
	}
	if res.uniqueid != "" {
		h.LastUniqueID = res.uniqueid
	}
	// Do not touch ConsecutiveFailures / ConsecutiveSuccesses.
	t.logFetchProblem(fmt.Sprintf("Lightning feed %s: Unknown (Thor flicker — ignored for failover)", feedID))
}

func (t *LightningTrigger) fetchEnabledFeedsParallel(feeds []LightningFeedConfig, timeout int) map[string]feedFetchResult {
	out := make(map[string]feedFetchResult, len(feeds))
	if len(feeds) == 0 {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range feeds {
		wg.Add(1)
		go func(feed LightningFeedConfig) {
			defer wg.Done()
			r := t.fetchOneFeedResolvingUnknown(feed, timeout)
			r = t.applyTelemetryToResult(feed.ID, r)
			mu.Lock()
			out[feed.ID] = r
			mu.Unlock()
		}(feeds[i])
	}
	wg.Wait()
	return out
}

// unknownFlickerRetries is how many immediate re-fetches to attempt when Thor
// returns <lightningalert>Unknown</lightningalert> (server update-cycle flicker).
const unknownFlickerRetries = 2
const unknownFlickerRetryDelay = 200 * time.Millisecond

// fetchOneFeedResolvingUnknown fetches once, then re-fetches briefly if alert is Unknown.
// Thor often flickers Unknown between update cycles; a short retry usually recovers AllClear/etc.
func (t *LightningTrigger) fetchOneFeedResolvingUnknown(feed LightningFeedConfig, globalTimeout int) feedFetchResult {
	res := t.fetchOneFeed(feed, globalTimeout)
	if !res.ok || !strings.EqualFold(strings.TrimSpace(res.alert), "unknown") {
		return res
	}
	for attempt := 1; attempt <= unknownFlickerRetries; attempt++ {
		time.Sleep(unknownFlickerRetryDelay)
		retry := t.fetchOneFeed(feed, globalTimeout)
		if !retry.ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(retry.alert), "unknown") {
			log.Printf("Lightning feed %s: Unknown flicker resolved on retry %d → %s", feed.ID, attempt, retry.alert)
			return retry
		}
		res = retry
	}
	return res
}

// Fetch XML and check for lightning conditions (multi-feed).
// All enabled feeds are fetched every cycle so Live Status shows each sensor's last condition.
// Only the active feed drives failover decisions and processConditionChange.
func (t *LightningTrigger) fetchAndCheck() {
	defer func() {
		t.mu.Lock()
		t.LastFetch = time.Now()
		t.mu.Unlock()
	}()

	mon := t.monitorSnapshot()
	t.mu.Lock()
	t.failover = mon.Failover
	t.feeds = append([]LightningFeedConfig(nil), mon.Feeds...)
	t.FetchInterval = mon.FetchInterval
	t.Timeout = mon.Timeout
	t.mu.Unlock()

	enabled := enabledFeedsInOrder(mon)
	if len(enabled) == 0 {
		t.mu.Lock()
		t.allFeedsFailed = true
		t.mu.Unlock()
		return
	}

	t.mu.Lock()
	active := t.resolveActiveFeedLocked(mon)
	pinned := t.pinnedFeedID
	t.mu.Unlock()
	if active == nil {
		t.mu.Lock()
		t.allFeedsFailed = true
		t.mu.Unlock()
		return
	}

	results := t.fetchEnabledFeedsParallel(enabled, mon.Timeout)

	anyOK := false
	for _, f := range enabled {
		res, ok := results[f.ID]
		if !ok {
			continue
		}
		if f.ID == active.ID {
			if res.ok {
				anyOK = true
			}
			continue // active handled below
		}
		// Standby enabled feeds: status only
		if res.ok && strings.EqualFold(strings.TrimSpace(res.alert), "unknown") {
			// Unknown flicker — show in status, do not treat as healthy success for failback.
			t.recordUnknownSoft(f.ID, res)
			anyOK = true // feed is reachable
			continue
		}
		if res.ok {
			anyOK = true
		}
		t.recordStandbyObservation(f.ID, res)
	}

	res, hasActive := results[active.ID]
	if !hasActive {
		t.mu.Lock()
		t.allFeedsFailed = !anyOK
		t.mu.Unlock()
		return
	}

	if !res.ok {
		// Layer A: any !ok path returns without processConditionChange (lock/announce safe).
		// Layer B: consecutive failures / failover — stale always; telemetry_collapse if trigger on.
		applyFail := false
		switch res.failureClass {
		case "stale_localtime":
			applyFail = true
		case "telemetry_collapse":
			applyFail = t.triggerEnabled("telemetry_collapse")
			if !applyFail {
				t.recordTelemetryCollapseStatusOnly(active.ID, res)
			}
		default:
			applyFail = t.triggerEnabled(res.failureClass)
		}
		if applyFail {
			t.recordFeedFailure(active.ID, res.failureClass, res.errMsg)
			t.maybeFailover(mon, active)
		}
		t.mu.Lock()
		t.allFeedsFailed = !anyOK
		t.mu.Unlock()
		return
	}

	// Mismatch checks (active feed only — may trigger failover)
	if strings.TrimSpace(active.ExpectedDisplayname) != "" && res.displayname != "" &&
		!strings.EqualFold(active.ExpectedDisplayname, res.displayname) && t.triggerEnabled("displayname_mismatch") {
		t.recordFeedFailure(active.ID, "displayname_mismatch", fmt.Sprintf("expected %q got %q", active.ExpectedDisplayname, res.displayname))
		t.maybeFailover(mon, active)
		t.mu.Lock()
		t.allFeedsFailed = !anyOK
		t.mu.Unlock()
		return
	}
	if strings.TrimSpace(active.ExpectedUniqueID) != "" && res.uniqueid != "" &&
		!strings.EqualFold(active.ExpectedUniqueID, res.uniqueid) && t.triggerEnabled("uniqueid_mismatch") {
		t.recordFeedFailure(active.ID, "uniqueid_mismatch", fmt.Sprintf("expected %q got %q", active.ExpectedUniqueID, res.uniqueid))
		t.maybeFailover(mon, active)
		t.mu.Lock()
		t.allFeedsFailed = !anyOK
		t.mu.Unlock()
		return
	}

	if strings.EqualFold(res.alert, "unknown") {
		// Thor briefly (sometimes for many seconds) publishes Unknown between XML
		// update cycles. That is not a dead feed — never count toward failover.
		// Condition changes are already ignored for Unknown below.
		t.recordUnknownSoft(active.ID, res)
		t.mu.Lock()
		t.allFeedsFailed = !anyOK
		t.mu.Unlock()
		return
	}

	t.recordFeedSuccess(active.ID, res.alert, res.displayname, res.uniqueid)
	t.mu.Lock()
	t.activeDisplayname = res.displayname
	t.activeUniqueID = res.uniqueid
	t.URL = strings.TrimSpace(active.URL)
	t.allFeedsFailed = false
	t.mu.Unlock()

	// Failback using this cycle's standby success counters (no extra probe fetch needed)
	if pinned == "" {
		if pref := preferredFailbackFeed(mon, active.ID); pref != nil && pref.ID != active.ID {
			h := t.healthFor(pref.ID)
			t.mu.Lock()
			need := t.failover.FailbackAfterSuccesses
			okCount := h.ConsecutiveSuccesses
			probeEvery := t.failover.FailbackProbeIntervalSeconds
			sinceProbe := time.Since(t.lastProbeAt)
			t.mu.Unlock()
			if need < 1 {
				need = 1
			}
			allowProbe := probeEvery <= 0 || sinceProbe >= time.Duration(probeEvery)*time.Second
			if allowProbe {
				t.mu.Lock()
				t.lastProbeAt = time.Now()
				t.mu.Unlock()
				if okCount >= need {
					if prefRes, ok := results[pref.ID]; ok && prefRes.ok {
						t.switchActiveFeed(pref, "failback")
						active = pref
						t.mu.Lock()
						t.activeDisplayname = prefRes.displayname
						t.activeUniqueID = prefRes.uniqueid
						t.URL = strings.TrimSpace(active.URL)
						t.mu.Unlock()
						t.applyThorDrivenCondition(active, prefRes.alert)
						return
					}
				}
			}
		}
	}

	t.applyThorDrivenCondition(active, res.alert)
}

// applyThorDrivenCondition applies XML/composite-driven condition changes only when
// manual override lock is inactive. Fetches, failover, and Live Status still update.
func (t *LightningTrigger) applyThorDrivenCondition(feed *LightningFeedConfig, alert string) {
	if t.isManualOverrideActive() {
		log.Printf("Thor condition ignored — manual override lock active (%s); release from Admin to resume Thor-driven changes", t.manualOverrideConditionLocked())
		return
	}
	t.processConditionChange(feed, alert)
	t.evaluateCompositeRedAlertEnter()
}

func (t *LightningTrigger) isManualOverrideActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.manualOverrideActive
}

func (t *LightningTrigger) manualOverrideConditionLocked() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.manualOverrideCondition
}

func (t *LightningTrigger) maybeFailover(mon LightningMonitorConfig, active *LightningFeedConfig) {
	if active == nil {
		return
	}
	t.mu.Lock()
	allow := t.failover.AllowFailoverDuringRedAlert || !t.redAlertActive
	need := t.failover.ConsecutiveFailures
	if need < 1 {
		need = 1
	}
	dwell := t.failover.MinDwellOnFeedSeconds
	since := t.activeSince
	h := t.feedHealth[active.ID]
	failures := 0
	if h != nil {
		failures = h.ConsecutiveFailures
	}
	pinned := t.pinnedFeedID
	t.mu.Unlock()

	if pinned != "" || !allow {
		return
	}
	if dwell > 0 && time.Since(since) < time.Duration(dwell)*time.Second {
		return
	}
	if failures < need {
		return
	}
	next := nextEnabledFeedAfter(mon, active.ID)
	if next == nil {
		t.mu.Lock()
		t.allFeedsFailed = true
		mode := t.failover.OnAllFeedsFailed
		t.mu.Unlock()
		log.Printf("Lightning: all feeds failed (holding last condition; mode=%s)", mode)
		if mode == "force_unknown_status" {
			t.mu.Lock()
			t.LastCondition = "Unknown"
			t.mu.Unlock()
		}
		return
	}
	t.switchActiveFeed(next, "failover")
}

func (t *LightningTrigger) processConditionChange(feed *LightningFeedConfig, lightningAlert string) {
	if t.isManualOverrideActive() {
		log.Printf("processConditionChange skipped — manual override lock active")
		return
	}
	t.mu.Lock()
	prev := t.LastCondition
	skipAnnounce := t.skipNextConditionAnnounce
	if skipAnnounce {
		t.skipNextConditionAnnounce = false
	}
	t.mu.Unlock()

	if strings.EqualFold(lightningAlert, prev) {
		// Same condition after feed switch — do not re-announce
		return
	}
	log.Printf("Lightning condition changed from '%s' to '%s' (feed=%s)", prev, lightningAlert, feed.ID)

	if strings.ToLower(lightningAlert) == "unknown" {
		log.Printf("Lightning status 'Unknown' - ignoring condition change")
		return
	}

	if strings.ToLower(lightningAlert) == "allclear" {
		// 1.1.1 baseline gate first — never bypassed by release-mode logic.
		if !t.shouldAcceptAllClear() {
			log.Printf("AllClear condition ignored for announce/lock — previous condition was '%s' (not RedAlert)", prev)
			t.mu.Lock()
			t.LastCondition = lightningAlert
			t.LastConditionTime = time.Now()
			t.mu.Unlock()
			return
		}
		if !t.authorizeAllClearRelease(feed) {
			t.mu.Lock()
			t.LastCondition = lightningAlert
			t.LastConditionTime = time.Now()
			t.mu.Unlock()
			return
		}
		log.Printf("AllClear condition accepted — previous condition was '%s' (feed=%s mode=%s)", prev, feed.ID, normalizeAllClearReleaseMode(t.failover.AllClearReleaseMode))
	}

	t.mu.Lock()
	t.LastCondition = lightningAlert
	t.LastConditionTime = time.Now()
	t.mu.Unlock()

	t.applyConditionEffects(lightningAlert)
	if strings.EqualFold(lightningAlert, "redalert") {
		t.mu.Lock()
		t.enteredRedAlertOnFeed = feed.ID
		t.mu.Unlock()
	}
	if strings.EqualFold(lightningAlert, "allclear") {
		t.mu.Lock()
		t.enteredRedAlertOnFeed = ""
		t.mu.Unlock()
	}

	_ = skipAnnounce // preserved for future; equal-case already handled above

	if !feedAnnounceEnabled(feed, lightningAlert) {
		log.Printf("Lightning announce skipped for '%s' — disabled for feed %s / global", lightningAlert, feed.ID)
		return
	}
	if !t.announceTimingAllows(lightningAlert) {
		log.Printf("Lightning announce skipped for '%s' — announce_timing cooldown", lightningAlert)
		return
	}
	t.playLightningAnnouncementForFeed(feed, lightningAlert)
}

func (t *LightningTrigger) announceTimingAllows(condition string) bool {
	if lightningConfig == nil {
		return true
	}
	at := lightningConfig.AnnounceTiming
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	if at.MinSecondsBetweenAnyLightningAnnounce > 0 && !t.lastAnyAnnounceAt.IsZero() &&
		now.Sub(t.lastAnyAnnounceAt) < time.Duration(at.MinSecondsBetweenAnyLightningAnnounce)*time.Second {
		return false
	}
	if at.MinSecondsBetweenSameCondition > 0 && strings.EqualFold(t.lastAnnounceCondition, condition) &&
		!t.lastAnnounceAt.IsZero() && now.Sub(t.lastAnnounceAt) < time.Duration(at.MinSecondsBetweenSameCondition)*time.Second {
		return false
	}
	return true
}

func (t *LightningTrigger) markAnnouncePlayed(condition string) {
	t.mu.Lock()
	t.lastAnnounceCondition = condition
	t.lastAnnounceAt = time.Now()
	t.lastAnyAnnounceAt = t.lastAnnounceAt
	t.mu.Unlock()
}

func (t *LightningTrigger) saveXMLFileForFeed(feedID, feedURL string, xmlData []byte) error {
	xmlDir := "xml"
	if app != nil && app.Config != nil && app.Config.BaseDir != "" {
		xmlDir = filepath.Join(app.Config.BaseDir, "xml")
	}
	if err := os.MkdirAll(xmlDir, 0755); err != nil {
		return err
	}
	base := "feed.xml"
	if parsed, err := url.Parse(feedURL); err == nil {
		base = filepath.Base(parsed.Path)
		if base == "." || base == "/" || base == "" {
			base = "feed.xml"
		}
	}
	if !strings.HasSuffix(strings.ToLower(base), ".xml") {
		base += ".xml"
	}
	name := feedID + "_" + base
	return ioutil.WriteFile(filepath.Join(xmlDir, name), xmlData, 0644)
}

// Save XML file locally
func (t *LightningTrigger) saveXMLFile(xmlData []byte) error {
	// Create xml directory if it doesn't exist
	xmlDir := "xml"
	if err := os.MkdirAll(xmlDir, 0755); err != nil {
		return fmt.Errorf("failed to create xml directory: %v", err)
	}

	// Generate filename from URL
	fileName, err := t.generateFileName()
	if err != nil {
		return fmt.Errorf("failed to generate filename: %v", err)
	}

	// Full file path
	filePath := filepath.Join(xmlDir, fileName)

	// Write XML data to file (overwrite if exists)
	if err := ioutil.WriteFile(filePath, xmlData, 0644); err != nil {
		return fmt.Errorf("failed to write XML file: %v", err)
	}

	return nil
}

func (t *LightningTrigger) logFetchProblem(msg string) {
	now := time.Now()
	if msg == t.lastFetchError && now.Sub(t.lastFetchErrorLog) < 15*time.Minute {
		return
	}
	t.lastFetchError = msg
	t.lastFetchErrorLog = now
	t.loggedFetchRecover = false
	log.Printf("%s", msg)
}

func (t *LightningTrigger) noteFetchSuccess() {
	if t.lastFetchError != "" && !t.loggedFetchRecover {
		log.Printf("Lightning trigger fetch recovered after: %s", t.lastFetchError)
		t.loggedFetchRecover = true
	}
	t.lastFetchError = ""
}

// Generate filename from URL
func (t *LightningTrigger) generateFileName() (string, error) {
	parsedURL, err := url.Parse(t.URL)
	if err != nil {
		return "", err
	}

	// Extract filename from URL path
	fileName := filepath.Base(parsedURL.Path)

	// If no filename in path, generate one based on host
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = strings.ReplaceAll(parsedURL.Host, ".", "_") + ".xml"
	}

	// Ensure .xml extension
	if !strings.HasSuffix(strings.ToLower(fileName), ".xml") {
		fileName += ".xml"
	}

	return fileName, nil
}

// Convert XML encoding from UTF-16 to UTF-8 if needed
func (t *LightningTrigger) convertXMLEncoding(xmlData []byte) (string, error) {
	// Check if the data starts with a UTF-16 BOM
	if len(xmlData) >= 2 {
		// UTF-16 LE BOM
		if xmlData[0] == 0xFF && xmlData[1] == 0xFE {
			return t.decodeUTF16LE(xmlData[2:])
		}
		// UTF-16 BE BOM
		if xmlData[0] == 0xFE && xmlData[1] == 0xFF {
			return t.decodeUTF16BE(xmlData[2:])
		}
	}

	// Check if it looks like UTF-16 by checking for null bytes in even positions
	xmlStr := string(xmlData)
	if len(xmlData) > 20 && strings.Contains(xmlStr[:100], "\x00") {
		// Looks like UTF-16, try to decode as UTF-16 LE
		decoded, err := t.decodeUTF16LE(xmlData)
		if err == nil && strings.Contains(decoded, "<?xml") {
			return decoded, nil
		}
	}

	// Already UTF-8 or ASCII
	return string(xmlData), nil
}

// Decode UTF-16 Little Endian
func (t *LightningTrigger) decodeUTF16LE(data []byte) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("odd length data for UTF-16")
	}

	u16s := make([]uint16, len(data)/2)
	for i := 0; i < len(u16s); i++ {
		u16s[i] = uint16(data[i*2]) | uint16(data[i*2+1])<<8
	}

	runes := utf16.Decode(u16s)
	return string(runes), nil
}

// Decode UTF-16 Big Endian
func (t *LightningTrigger) decodeUTF16BE(data []byte) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("odd length data for UTF-16")
	}

	u16s := make([]uint16, len(data)/2)
	for i := 0; i < len(u16s); i++ {
		u16s[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
	}

	runes := utf16.Decode(u16s)
	return string(runes), nil
}

// Extract lightningalert value from XML string
func (t *LightningTrigger) extractLightningAlertFromString(xmlStr string) string {
	startTag := "<lightningalert>"
	endTag := "</lightningalert>"

	startIndex := strings.Index(xmlStr, startTag)
	if startIndex == -1 {
		lowerXML := strings.ToLower(xmlStr)
		if strings.Contains(lowerXML, "<lightningalert>") {
			t.logFetchProblem("Lightning: Found lightningalert tag in different case")
		} else {
			t.logXMLPreview("Lightning: No lightningalert tag found in XML", xmlStr)
		}
		return ""
	}

	startIndex += len(startTag)
	endIndex := strings.Index(xmlStr[startIndex:], endTag)
	if endIndex == -1 {
		t.logXMLPreview("Lightning: Found opening tag but no closing tag", xmlStr)
		return ""
	}

	value := strings.TrimSpace(xmlStr[startIndex : startIndex+endIndex])
	if value == "" {
		t.logFetchProblem("Lightning: empty lightningalert value")
	}
	return value
}

func (t *LightningTrigger) logXMLPreview(reason, xmlStr string) {
	xmlPreview := xmlStr
	if len(xmlStr) > 1000 {
		xmlPreview = xmlStr[:1000] + "..."
	}
	t.logFetchProblem(fmt.Sprintf("%s; preview: %s", reason, xmlPreview))
}

// Extract lightningalert value from XML (deprecated - use extractLightningAlertFromString)
func (t *LightningTrigger) extractLightningAlert(xmlData []byte) string {
	return t.extractLightningAlertFromString(string(xmlData))
}

// Play lightning announcement based on condition
func (t *LightningTrigger) playLightningAnnouncement(condition string) {
	var feed *LightningFeedConfig
	mon := t.monitorSnapshot()
	t.mu.Lock()
	id := t.activeFeedID
	t.mu.Unlock()
	if id != "" {
		feed = feedByID(mon, id)
	}
	t.playLightningAnnouncementForFeed(feed, condition)
}

func (t *LightningTrigger) playLightningAnnouncementForFeed(feed *LightningFeedConfig, condition string) {
	if lightningConfig == nil {
		log.Printf("Lightning configuration not loaded, cannot play announcement")
		return
	}

	var selectedAnnouncement *LightningAnnouncement
	for i := range lightningConfig.LightningAnnouncements {
		announcement := &lightningConfig.LightningAnnouncements[i]
		id := strings.ToLower(announcement.ID)
		match := false
		switch strings.ToLower(condition) {
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
		if match {
			selectedAnnouncement = announcement
			break
		}
	}

	liveDN := ""
	t.mu.Lock()
	liveDN = t.activeDisplayname
	t.mu.Unlock()

	audioOverrides := resolveLightningAudioFiles(feed, condition, liveDN)

	if selectedAnnouncement == nil && len(audioOverrides) == 0 && !strings.EqualFold(condition, "feed_switch") {
		log.Printf("No matching lightning announcement found for condition: %s", condition)
		return
	}

	name := condition
	tts := ""
	if selectedAnnouncement != nil {
		name = selectedAnnouncement.Name
		tts = selectedAnnouncement.TTSText
	}
	log.Printf("Playing lightning announcement: %s", name)

	if announcementManager != nil {
		parameters := map[string]interface{}{
			"condition":      condition,
			"message":        tts,
			"trigger_source": "LIGHTNING_TRIGGER",
		}
		if len(audioOverrides) > 0 {
			parameters["audio_files"] = audioOverrides
		}
		priority := AnnouncementPriority(10)
		announcement, err := announcementManager.QueueAnnouncement(TypeLightning, priority, parameters, time.Now())
		if err != nil {
			log.Printf("Failed to queue lightning announcement: %v", err)
		} else {
			t.markAnnouncePlayed(condition)
			log.Printf("Queued HIGHEST PRIORITY lightning announcement: %s (ID: %s)", name, announcement.ID)
		}
	} else {
		log.Printf("Announcement manager not available, cannot queue lightning announcement")
	}
}

func (t *LightningTrigger) shouldAcceptAllClear() bool {
	if isRedAlertActive() {
		return true
	}
	return strings.EqualFold(t.LastCondition, "redalert")
}

// authorizeAllClearRelease is an unlock-only gate after shouldAcceptAllClear succeeds.
// It never affects Red Alert enter. primary_only matches 1.1.1 for the primary feed.
func (t *LightningTrigger) authorizeAllClearRelease(feed *LightningFeedConfig) bool {
	if feed == nil {
		log.Printf("AllClear release denied — no feed context")
		return false
	}
	// Layer A: never unlock from a feed in sticky telemetry_collapse (even if Admin disabled Layer B).
	if t.feedHasTelemetryCollapse(feed.ID) {
		log.Printf("AllClear release denied — feed %s has telemetry_collapse", feed.ID)
		return false
	}
	mode := normalizeAllClearReleaseMode(t.failover.AllClearReleaseMode)
	if strings.EqualFold(feed.ID, "primary") {
		return true
	}
	switch mode {
	case "failover_vote":
		ok, summary := t.evaluateFailoverAllClearVote()
		t.mu.Lock()
		t.lastAllClearVoteSummary = summary
		t.mu.Unlock()
		if !ok {
			log.Printf("AllClear release denied — failover_vote failed: %s", summary)
			return false
		}
		log.Printf("AllClear release authorized — failover_vote: %s", summary)
		return true
	default: // primary_only
		log.Printf("AllClear release denied — primary_only (feed=%s)", feed.ID)
		return false
	}
}

// evaluateFailoverAllClearVote fetches failover_1 and failover_2 in parallel.
// Accept only when both are trusted AllClear for unlock: not collapsed/held, alert AllClear,
// and DI > 0 or AD > 0 (floor AllClear alone cannot unlock — regional Thor fake-AllClear).
func (t *LightningTrigger) evaluateFailoverAllClearVote() (bool, string) {
	mon := t.monitorSnapshot()
	f1 := feedByID(mon, "failover_1")
	f2 := feedByID(mon, "failover_2")
	if f1 == nil || f2 == nil || !f1.Enabled || !f2.Enabled ||
		strings.TrimSpace(f1.URL) == "" || strings.TrimSpace(f2.URL) == "" {
		return false, "vote requires failover_1 and failover_2 enabled with URLs"
	}

	timeout := mon.Timeout
	if timeout < 5 {
		timeout = 30
	}
	type voteResult struct {
		id     string
		detail string
		ok     bool
	}
	results := make([]voteResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		r := t.fetchOneFeedResolvingUnknown(*f1, timeout)
		r = t.applyTelemetryToResult(f1.ID, r)
		ok, detail := allClearVoteBallotAccepted(r)
		if t.feedHasTelemetryCollapse(f1.ID) {
			ok = false
			if !strings.Contains(detail, "telemetry_collapse") {
				detail = "telemetry_collapse: sticky hold"
			}
		}
		results[0] = voteResult{id: f1.ID, detail: detail, ok: ok}
	}()
	go func() {
		defer wg.Done()
		r := t.fetchOneFeedResolvingUnknown(*f2, timeout)
		r = t.applyTelemetryToResult(f2.ID, r)
		ok, detail := allClearVoteBallotAccepted(r)
		if t.feedHasTelemetryCollapse(f2.ID) {
			ok = false
			if !strings.Contains(detail, "telemetry_collapse") {
				detail = "telemetry_collapse: sticky hold"
			}
		}
		results[1] = voteResult{id: f2.ID, detail: detail, ok: ok}
	}()
	wg.Wait()

	count := 0
	parts := make([]string, 0, 2)
	for _, vr := range results {
		parts = append(parts, fmt.Sprintf("%s=%s", vr.id, vr.detail))
		if vr.ok {
			count++
		}
	}
	summary := fmt.Sprintf("trusted_allclear_count=%d [%s]", count, strings.Join(parts, ", "))
	return count == 2, summary
}

// TestCondition manually triggers a lightning announcement for testing
func (t *LightningTrigger) TestCondition(condition string) (string, bool) {
	log.Printf("DEBUG: Manual test for condition: %s", condition)

	if strings.EqualFold(condition, "allclear") && !t.shouldAcceptAllClear() {
		prev := t.LastCondition
		t.LastCondition = condition
		t.LastConditionTime = time.Now()
		msg := fmt.Sprintf("AllClear ignored — previous condition was '%s' (AllClear only sounds after RedAlert)", prev)
		log.Printf("%s", msg)
		return msg, false
	}

	t.LastCondition = condition
	t.LastConditionTime = time.Now()
	t.applyConditionEffects(condition)
	t.playLightningAnnouncement(condition)
	return fmt.Sprintf("%s test triggered", condition), true
}

func (t *LightningTrigger) ResetState() {
	t.stopRedAlertReminders()

	t.mu.Lock()
	t.redAlertActive = false
	t.redAlertSince = time.Time{}
	t.reminderCount = 0
	t.lastReminderAt = time.Time{}
	t.nextReminderAt = time.Time{}
	t.manualOverrideActive = false
	t.manualOverrideCondition = ""
	t.manualOverrideAt = time.Time{}
	t.manualOverrideNote = ""
	t.enteredRedAlertOnFeed = ""
	t.lastCompositeEnterSummary = ""
	t.mu.Unlock()

	t.LastCondition = "Reset"
	t.LastConditionTime = time.Time{}
	log.Printf("THOR Guard cached state reset to Reset")
}

// ApplyManualOverride sets operational lightning lock state and holds it until Admin
// releases the override lock. While active, Thor XML / composite enter cannot change
// lock or condition (fetches and failover still run). Distinct from TestCondition (audio drill).
// action: redalert | allclear | clear
func (t *LightningTrigger) ApplyManualOverride(action, note string) (string, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	note = strings.TrimSpace(note)
	if note == "" {
		note = "admin"
	}
	switch action {
	case "redalert", "force_redalert":
		t.mu.Lock()
		t.manualOverrideActive = true
		t.manualOverrideCondition = "RedAlert"
		t.manualOverrideAt = time.Now()
		t.manualOverrideNote = note
		t.LastCondition = "RedAlert"
		t.LastConditionTime = time.Now()
		t.enteredRedAlertOnFeed = "manual_override"
		t.mu.Unlock()
		t.applyConditionEffects("RedAlert")
		t.playLightningAnnouncement("RedAlert")
		msg := "Manual override lock: Force Red Alert — Thor cannot change lock until Admin releases override"
		log.Printf("%s (by %s)", msg, note)
		return msg, nil
	case "allclear", "force_allclear":
		t.mu.Lock()
		t.manualOverrideActive = true
		t.manualOverrideCondition = "AllClear"
		t.manualOverrideAt = time.Now()
		t.manualOverrideNote = note
		t.LastCondition = "AllClear"
		t.LastConditionTime = time.Now()
		t.enteredRedAlertOnFeed = ""
		t.mu.Unlock()
		t.applyConditionEffects("AllClear")
		t.playLightningAnnouncement("AllClear")
		msg := "Manual override lock: Force All Clear / unlock — Thor cannot change lock until Admin releases override"
		log.Printf("%s (by %s)", msg, note)
		return msg, nil
	case "clear", "clear_flag":
		t.mu.Lock()
		was := t.manualOverrideActive
		cond := t.manualOverrideCondition
		t.manualOverrideActive = false
		t.manualOverrideCondition = ""
		t.manualOverrideAt = time.Time{}
		t.manualOverrideNote = note
		t.mu.Unlock()
		if !was {
			return "Manual override lock was already inactive", nil
		}
		msg := fmt.Sprintf("Manual override lock released (was %s); lock/condition unchanged — Thor may drive again on next poll", cond)
		log.Printf("%s (by %s)", msg, note)
		return msg, nil
	default:
		return "", fmt.Errorf("unknown override action %q (use redalert, allclear, or clear)", action)
	}
}

// evaluateCompositeRedAlertEnter checks Admin composite enter rules
// (e.g. failover_1=Warning AND failover_2=RedAlert → enter Red Alert).
// Unlock / All Clear release authority is never modified here. Skipped while manual override lock is active.
func (t *LightningTrigger) evaluateCompositeRedAlertEnter() {
	if t.isManualOverrideActive() {
		return
	}
	if isRedAlertActive() {
		return
	}
	mon := t.monitorSnapshot()
	rules := mon.CompositeRedAlertRules
	if len(rules) == 0 {
		return
	}
	timeout := mon.Timeout
	if timeout < 5 {
		timeout = 30
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		reqs := rule.normalizedRequirements()
		if len(reqs) < 2 {
			continue
		}
		type probe struct {
			id       string
			needCond string
			alert    string
			ok       bool
			err      string
		}
		results := make([]probe, len(reqs))
		var wg sync.WaitGroup
		for i, req := range reqs {
			fid := strings.TrimSpace(req.FeedID)
			needCond := strings.TrimSpace(req.Condition)
			f := feedByID(mon, fid)
			results[i].id = fid
			results[i].needCond = needCond
			if f == nil || !f.Enabled || strings.TrimSpace(f.URL) == "" {
				results[i].err = "feed missing or disabled"
				continue
			}
			if needCond == "" {
				results[i].err = "missing required condition"
				continue
			}
			wg.Add(1)
			go func(idx int, feed LightningFeedConfig, want string) {
				defer wg.Done()
				r := t.fetchOneFeedResolvingUnknown(feed, timeout)
				if !r.ok {
					results[idx].err = r.failureClass + ": " + r.errMsg
					return
				}
				results[idx].alert = r.alert
				results[idx].ok = strings.EqualFold(strings.TrimSpace(r.alert), want)
			}(i, *f, needCond)
		}
		wg.Wait()

		allMatch := true
		parts := make([]string, 0, len(results))
		for _, p := range results {
			if p.err != "" {
				parts = append(parts, fmt.Sprintf("%s=error(%s)", p.id, p.err))
				allMatch = false
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s(need %s)", p.id, p.alert, p.needCond))
			if !p.ok {
				allMatch = false
			}
		}
		summary := fmt.Sprintf("rule=%s [%s]", rule.ID, strings.Join(parts, ", "))
		if !allMatch {
			continue
		}

		label := rule.Label
		if label == "" {
			label = rule.ID
		}
		log.Printf("Composite Red Alert enter matched: %s — %s", label, summary)
		t.mu.Lock()
		t.LastCondition = "RedAlert"
		t.LastConditionTime = time.Now()
		t.enteredRedAlertOnFeed = "composite:" + rule.ID
		t.lastCompositeEnterSummary = summary
		t.mu.Unlock()
		t.applyConditionEffects("RedAlert")

		// Announce as Red Alert using primary feed context if available, else first required feed.
		announceFeed := feedByID(mon, "primary")
		if announceFeed == nil || !announceFeed.Enabled {
			announceFeed = feedByID(mon, reqs[0].FeedID)
		}
		if announceFeed != nil && feedAnnounceEnabled(announceFeed, "RedAlert") && t.announceTimingAllows("RedAlert") {
			t.playLightningAnnouncementForFeed(announceFeed, "RedAlert")
		} else if announceFeed == nil {
			t.playLightningAnnouncement("RedAlert")
		}
		return // one enter per poll cycle
	}
}

func (t *LightningTrigger) applyConditionEffects(condition string) {
	switch strings.ToLower(condition) {
	case "redalert":
		t.enterRedAlert()
	case "allclear":
		t.exitRedAlert()
	}
}

func (t *LightningTrigger) enterRedAlert() {
	policy := getRedAlertPolicy()

	t.mu.Lock()
	alreadyActive := t.redAlertActive
	t.redAlertActive = true
	if !alreadyActive {
		t.redAlertSince = time.Now()
		t.reminderCount = 0
		t.lastReminderAt = time.Time{}
	}
	t.mu.Unlock()

	if alreadyActive {
		log.Printf("THOR Guard Red Alert already active — refreshing preemption and reminder timer")
	} else {
		log.Printf("THOR Guard Red Alert ACTIVE — suppressing non-emergency announcements until All Clear")
	}

	if policy.PreemptQueue && announcementManager != nil {
		announcementManager.PreemptForRedAlert()
	}

	t.startRedAlertReminders()
}

func (t *LightningTrigger) exitRedAlert() {
	wasActive := isRedAlertActive()
	t.stopRedAlertReminders()

	t.mu.Lock()
	t.redAlertActive = false
	t.nextReminderAt = time.Time{}
	t.mu.Unlock()

	if wasActive {
		log.Printf("THOR Guard All Clear — Red Alert lock released, normal announcements resumed")
	}
}

func (t *LightningTrigger) startRedAlertReminders() {
	policy := getRedAlertPolicy()
	t.stopRedAlertReminders()
	if !policy.ReminderEnabled {
		log.Printf("THOR Guard Red Alert reminders disabled in policy")
		return
	}

	intervalMinutes := policy.ReminderIntervalMinutes
	if intervalMinutes < 1 {
		intervalMinutes = 5
	}
	interval := time.Duration(intervalMinutes) * time.Minute
	stop := make(chan struct{})

	t.mu.Lock()
	t.reminderStop = stop
	t.nextReminderAt = time.Now().Add(interval)
	t.mu.Unlock()

	log.Printf("THOR Guard Red Alert reminders armed: every %d minute(s) using %s", intervalMinutes, policy.ReminderAudioFile)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.playRedAlertReminder()
			case <-stop:
				return
			}
		}
	}()
}

func (t *LightningTrigger) stopRedAlertReminders() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.reminderStop != nil {
		close(t.reminderStop)
		t.reminderStop = nil
	}
}

func (t *LightningTrigger) playRedAlertReminder() {
	if !isRedAlertActive() {
		return
	}

	policy := getRedAlertPolicy()
	if !policy.ReminderEnabled {
		return
	}

	if announcementManager == nil {
		log.Printf("Cannot play Red Alert reminder — announcement manager unavailable")
		return
	}

	parameters := map[string]interface{}{
		"condition":      "redalert_reminder",
		"audio_file":     policy.ReminderAudioFile,
		"include_horn":   policy.ReminderIncludeHorn,
		"horn_file":      policy.HornAudioFile,
		"trigger_source": "THOR_RED_ALERT_REMINDER",
	}

	announcement, err := announcementManager.QueueAnnouncement(TypeLightning, AnnouncementPriority(10), parameters, time.Now())
	if err != nil {
		log.Printf("Failed to queue THOR Guard Red Alert reminder: %v", err)
		return
	}

	t.mu.Lock()
	t.reminderCount++
	t.lastReminderAt = time.Now()
	t.nextReminderAt = time.Now().Add(time.Duration(policy.ReminderIntervalMinutes) * time.Minute)
	count := t.reminderCount
	t.mu.Unlock()

	log.Printf("Queued THOR Guard Red Alert reminder #%d (ID: %s, file: %s)", count, announcement.ID, policy.ReminderAudioFile)
}

func isRedAlertActive() bool {
	if lightningTrigger == nil {
		return false
	}
	lightningTrigger.mu.Lock()
	defer lightningTrigger.mu.Unlock()
	return lightningTrigger.redAlertActive
}

func isRedAlertSuppressionActive() bool {
	if !isRedAlertActive() {
		return false
	}
	return getRedAlertPolicy().SuppressNonEmergency
}

func applyRedAlertPolicy(policy RedAlertPolicy) error {
	if policy.ReminderIntervalMinutes < 1 {
		return fmt.Errorf("reminder interval must be at least 1 minute")
	}
	if policy.ReminderAudioFile == "" {
		policy.ReminderAudioFile = "Voice_RedAlert_Reminder.mp3"
	}
	if policy.HornAudioFile == "" {
		if clip, ok := getConditionAudioClip("RedAlert"); ok && clip.HornFile != "" {
			policy.HornAudioFile = clip.HornFile
		} else {
			policy.HornAudioFile = "Horn_RedAlert.mp3"
		}
	}

	if lightningConfig == nil {
		lightningConfig = &LightningConfig{}
	}
	lightningConfig.RedAlertPolicy = policy
	if err := saveLightningConfig(); err != nil {
		return err
	}

	if lightningTrigger != nil && isRedAlertActive() {
		lightningTrigger.startRedAlertReminders()
	}
	return nil
}

func listMP3BasenamesInDirs(dirs []string, known []string) []string {
	seen := map[string]bool{}
	var files []string
	add := func(name string) {
		name = filepath.Base(strings.TrimSpace(name))
		if name == "" || seen[name] {
			return
		}
		if !strings.HasSuffix(strings.ToLower(name), ".mp3") {
			name += ".mp3"
		}
		seen[name] = true
		files = append(files, name)
	}
	for _, dir := range dirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mp3") {
					add(entry.Name())
				}
			}
		}
	}
	for _, name := range known {
		add(name)
	}
	sort.Strings(files)
	return files
}

func mp3Subdir(name string) string {
	mp3Root := "static/mp3"
	if app != nil && app.Config != nil && app.Config.MP3Dir != "" {
		mp3Root = app.Config.MP3Dir
	}
	return filepath.Join(mp3Root, name)
}

// listLightningVoiceFiles returns announce/voice MP3s from lightning/.
func listLightningVoiceFiles() []string {
	files := listMP3BasenamesInDirs([]string{mp3Subdir("lightning")}, []string{
		"Voice_RedAlert.mp3",
		"Voice_RedAlert_Reminder.mp3",
		"Voice_AllClear.mp3",
		"Voice_Warning.mp3",
		"Voice_Caution.mp3",
		"Voice_Unknown.mp3",
	})
	var voices []string
	seen := map[string]bool{}
	for _, f := range files {
		if strings.HasPrefix(strings.ToLower(f), "horn_") {
			continue
		}
		seen[f] = true
		voices = append(voices, f)
	}
	if lightningConfig != nil {
		add := func(name string) {
			name = filepath.Base(strings.TrimSpace(name))
			if name == "" || seen[name] {
				return
			}
			if strings.HasPrefix(strings.ToLower(name), "horn_") {
				return
			}
			if !strings.HasSuffix(strings.ToLower(name), ".mp3") {
				name += ".mp3"
			}
			seen[name] = true
			voices = append(voices, name)
		}
		for _, announcement := range lightningConfig.LightningAnnouncements {
			add(announcement.AudioFile)
		}
		add(lightningConfig.RedAlertPolicy.ReminderAudioFile)
		ca := lightningConfig.ConditionAudio
		add(ca.RedAlert.AnnounceFile)
		add(ca.AllClear.AnnounceFile)
		add(ca.Warning.AnnounceFile)
		add(ca.Caution.AnnounceFile)
		add(ca.Unknown.AnnounceFile)
	}
	sort.Strings(voices)
	return voices
}

// listLightningHornFiles returns Horn_*.mp3 from horns-chimes-tones/ (legacy lightning/ fallback).
func listLightningHornFiles() []string {
	files := listMP3BasenamesInDirs([]string{
		mp3Subdir("horns-chimes-tones"),
		mp3Subdir("lightning"),
	}, []string{
		"Horn_RedAlert.mp3",
		"Horn_AllClear.mp3",
	})
	var horns []string
	seen := map[string]bool{}
	for _, f := range files {
		if !strings.HasPrefix(strings.ToLower(f), "horn_") {
			continue
		}
		seen[f] = true
		horns = append(horns, f)
	}
	if lightningConfig != nil {
		add := func(name string) {
			name = filepath.Base(strings.TrimSpace(name))
			if name == "" || seen[name] {
				return
			}
			if !strings.HasSuffix(strings.ToLower(name), ".mp3") {
				name += ".mp3"
			}
			if !strings.HasPrefix(strings.ToLower(name), "horn_") {
				return
			}
			seen[name] = true
			horns = append(horns, name)
		}
		add(lightningConfig.RedAlertPolicy.HornAudioFile)
		ca := lightningConfig.ConditionAudio
		add(ca.RedAlert.HornFile)
		add(ca.AllClear.HornFile)
	}
	if len(horns) == 0 {
		return []string{"Horn_AllClear.mp3", "Horn_RedAlert.mp3"}
	}
	sort.Strings(horns)
	return horns
}

// listLightningAudioFiles returns combined voice+horn list (legacy callers).
func listLightningAudioFiles() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range append(listLightningVoiceFiles(), listLightningHornFiles()...) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// UpdateConfig is a legacy single-URL updater (maps onto primary feed).
func (t *LightningTrigger) UpdateConfig(url string, fetchInterval int, timeout int, enabled bool) error {
	mon := t.monitorSnapshot()
	mon.FetchInterval = fetchInterval
	mon.Timeout = timeout
	mon.Enabled = enabled
	if len(mon.Feeds) == 0 {
		mon.Feeds = defaultFeedSlots()
	}
	mon.Feeds[0].URL = strings.TrimSpace(url)
	mon.Feeds[0].Enabled = enabled && strings.TrimSpace(url) != ""
	if s := findBrowardTGSensorByURL(url); s != nil {
		mon.Feeds[0].SensorID = s.ID
		mon.Feeds[0].ExpectedDisplayname = s.DisplayName
		mon.Feeds[0].Source = "catalog"
	} else {
		mon.Feeds[0].Source = "custom"
	}
	mon.URL = strings.TrimSpace(url)
	return t.ApplyMonitorConfig(mon, nil, nil)
}

// ApplyMonitorConfig hot-applies feeds/failover/timing and persists.
func (t *LightningTrigger) ApplyMonitorConfig(mon LightningMonitorConfig, timing *LightningAnnounceTiming, overrides []DisplaynameOverride) error {
	if mon.FetchInterval < 30 {
		return fmt.Errorf("fetch interval must be at least 30 seconds")
	}
	if mon.Timeout < 5 {
		return fmt.Errorf("timeout must be at least 5 seconds")
	}
	if mon.Failover.ConsecutiveFailures < 1 {
		mon.Failover.ConsecutiveFailures = 3
	}
	mon.Failover.AllClearReleaseMode = normalizeAllClearReleaseMode(mon.Failover.AllClearReleaseMode)
	mon.Failover.RequireAllClearFromSameFeed = false
	mon.Failover.TelemetryCollapse = mon.Failover.TelemetryCollapse.normalized()
	if len(mon.Feeds) > 3 {
		mon.Feeds = mon.Feeds[:3]
	}
	for i := range mon.Feeds {
		if mon.Feeds[i].Enabled && strings.TrimSpace(mon.Feeds[i].URL) == "" {
			mon.Feeds[i].Enabled = false
		}
	}
	mon.URL = primaryFeedURL(mon)
	hasURL := firstEnabledFeedURL(mon) != ""
	mon.Enabled = mon.Enabled && hasURL

	wasRunning := false
	t.mu.Lock()
	wasRunning = t.isRunning
	t.mu.Unlock()
	if wasRunning {
		t.Stop()
		time.Sleep(100 * time.Millisecond)
	}

	if lightningConfig != nil {
		lightningConfig.Monitor = mon
		if timing != nil {
			lightningConfig.AnnounceTiming = *timing
		}
		if overrides != nil {
			lightningConfig.DisplaynameOverrides = overrides
		}
		if err := saveLightningConfig(); err != nil {
			return err
		}
	}

	t.mu.Lock()
	t.FetchInterval = mon.FetchInterval
	t.Timeout = mon.Timeout
	t.Enabled = mon.Enabled
	t.feeds = append([]LightningFeedConfig(nil), mon.Feeds...)
	t.failover = mon.Failover
	t.URL = firstEnabledFeedURL(mon)
	if t.feedHealth == nil {
		t.feedHealth = map[string]*FeedHealth{}
	}
	for _, f := range mon.Feeds {
		if _, ok := t.feedHealth[f.ID]; !ok {
			t.feedHealth[f.ID] = &FeedHealth{}
		}
	}
	if t.activeFeedID == "" || feedByID(mon, t.activeFeedID) == nil || !feedByID(mon, t.activeFeedID).Enabled {
		enabled := enabledFeedsInOrder(mon)
		if len(enabled) > 0 {
			t.activeFeedID = enabled[0].ID
			t.activeSince = time.Now()
			t.URL = enabled[0].URL
		} else {
			t.activeFeedID = ""
		}
	}
	t.mu.Unlock()

	if t.Enabled {
		t.stopOnce = sync.Once{}
		t.stopChan = make(chan struct{})
		go t.Start()
	}

	log.Printf("Lightning monitor config applied — enabled=%v active=%s feeds=%d", mon.Enabled, t.activeFeedID, len(mon.Feeds))
	return nil
}

// PinActiveFeed pins a feed for drills; empty feedID unpins.
func (t *LightningTrigger) PinActiveFeed(feedID string) error {
	feedID = strings.TrimSpace(feedID)
	mon := t.monitorSnapshot()
	if feedID == "" {
		t.mu.Lock()
		prev := t.pinnedFeedID
		t.pinnedFeedID = ""
		t.mu.Unlock()
		log.Printf("Lightning: unpinned active feed (was %s)", prev)
		return nil
	}
	f := feedByID(mon, feedID)
	if f == nil {
		return fmt.Errorf("unknown feed_id %q", feedID)
	}
	if !f.Enabled || strings.TrimSpace(f.URL) == "" {
		return fmt.Errorf("feed %q is not enabled or has empty URL", feedID)
	}
	t.mu.Lock()
	from := t.activeFeedID
	t.pinnedFeedID = feedID
	t.mu.Unlock()
	t.switchActiveFeed(f, "pin")
	log.Printf("Lightning: pinned active feed %s (from %s)", feedID, from)
	return nil
}

// Get lightning trigger status for API
func getLightningTriggerStatus() map[string]interface{} {
	if lightningTrigger == nil {
		return map[string]interface{}{
			"enabled":                  false,
			"running":                  false,
			"error":                    "Lightning trigger not initialized",
			"red_alert_active":         false,
			"reminder_count":           0,
			"red_alert_policy":         getRedAlertPolicy(),
			"available_reminder_files": listLightningVoiceFiles(),
			"available_voice_files":    listLightningVoiceFiles(),
			"available_horn_files":     listLightningHornFiles(),
			"known_sensors":            getBrowardTGSensors(),
		}
	}

	lightningTrigger.mu.Lock()
	redAlertActive := lightningTrigger.redAlertActive
	redAlertSince := ""
	if !lightningTrigger.redAlertSince.IsZero() {
		redAlertSince = lightningTrigger.redAlertSince.Format("2006-01-02 15:04:05")
	}
	lastReminder := ""
	if !lightningTrigger.lastReminderAt.IsZero() {
		lastReminder = lightningTrigger.lastReminderAt.Format("2006-01-02 15:04:05")
	}
	nextReminder := ""
	if redAlertActive && !lightningTrigger.nextReminderAt.IsZero() {
		nextReminder = lightningTrigger.nextReminderAt.Format("2006-01-02 15:04:05")
	}
	reminderCount := lightningTrigger.reminderCount
	activeID := lightningTrigger.activeFeedID
	pinnedID := lightningTrigger.pinnedFeedID
	activeDN := lightningTrigger.activeDisplayname
	activeUID := lightningTrigger.activeUniqueID
	allFailed := lightningTrigger.allFeedsFailed
	lastErr := lightningTrigger.lastFetchError
	enteredOn := lightningTrigger.enteredRedAlertOnFeed
	voteSummary := lightningTrigger.lastAllClearVoteSummary
	overrideActive := lightningTrigger.manualOverrideActive
	overrideCond := lightningTrigger.manualOverrideCondition
	overrideNote := lightningTrigger.manualOverrideNote
	overrideAt := ""
	if !lightningTrigger.manualOverrideAt.IsZero() {
		overrideAt = lightningTrigger.manualOverrideAt.Format("2006-01-02 15:04:05")
	}
	compositeSummary := lightningTrigger.lastCompositeEnterSummary
	healthCopy := map[string]interface{}{}
	for id, h := range lightningTrigger.feedHealth {
		if h == nil {
			continue
		}
		lastOK := ""
		if !h.LastOK.IsZero() {
			lastOK = h.LastOK.Format("2006-01-02 15:04:05")
		}
		entry := map[string]interface{}{
			"consecutive_failures":  h.ConsecutiveFailures,
			"consecutive_successes": h.ConsecutiveSuccesses,
			"last_ok":               lastOK,
			"last_error":            h.LastError,
			"last_displayname":      h.LastDisplayname,
			"last_uniqueid":         h.LastUniqueID,
			"last_alert":            h.LastAlert,
			"telemetry_collapse":    h.TelemetryCollapse,
		}
		if h.LastLHL != nil {
			entry["last_lhl"] = *h.LastLHL
		}
		if h.LastDI != nil {
			entry["last_di"] = *h.LastDI
		}
		if h.LastAD != nil {
			entry["last_ad"] = *h.LastAD
		}
		healthCopy[id] = entry
	}
	lightningTrigger.mu.Unlock()

	mon := getLightningMonitorConfig()
	onFailover := activeID != "" && activeID != "primary"

	out := map[string]interface{}{
		"id":                       lightningTrigger.ID,
		"name":                     lightningTrigger.Name,
		"enabled":                  lightningTrigger.Enabled,
		"running":                  lightningTrigger.isRunning,
		"url":                      lightningTrigger.URL,
		"fetch_interval":           lightningTrigger.FetchInterval,
		"timeout":                  lightningTrigger.Timeout,
		"last_fetch":               lightningTrigger.LastFetch.Format("2006-01-02 15:04:05"),
		"last_condition":           lightningTrigger.LastCondition,
		"last_condition_time":      lightningTrigger.LastConditionTime.Format("2006-01-02 15:04:05"),
		"red_alert_active":         redAlertActive,
		"red_alert_since":          redAlertSince,
		"reminder_count":           reminderCount,
		"last_reminder":            lastReminder,
		"next_reminder":            nextReminder,
		"red_alert_policy":         getRedAlertPolicy(),
		"condition_audio":          getConditionAudioConfig(),
		"available_reminder_files": listLightningVoiceFiles(),
		"available_voice_files":    listLightningVoiceFiles(),
		"available_horn_files":     listLightningHornFiles(),
		"condition_announce":       announcementEnableSnapshot(),
		"feeds":                    mon.Feeds,
		"failover":                 mon.Failover,
		"feed_switch_announcements": mon.FeedSwitchAnnouncements,
		"feed_health":              healthCopy,
		"active_feed_id":           activeID,
		"active_url":               lightningTrigger.URL,
		"active_displayname":       activeDN,
		"active_uniqueid":          activeUID,
		"on_failover":              onFailover,
		"all_feeds_failed":         allFailed,
		"pinned_feed_id":           pinnedID,
		"last_fetch_error":         lastErr,
		"entered_red_alert_on_feed": enteredOn,
		"allclear_release_mode":    normalizeAllClearReleaseMode(mon.Failover.AllClearReleaseMode),
		"last_allclear_vote":       voteSummary,
		"manual_override_active":   overrideActive,
		"manual_override_condition": overrideCond,
		"manual_override_at":       overrideAt,
		"manual_override_note":     overrideNote,
		"composite_red_alert_rules": mon.CompositeRedAlertRules,
		"last_composite_enter":     compositeSummary,
		"known_sensors":            getBrowardTGSensors(),
		"announce_timing":          LightningAnnounceTiming{},
	}
	if lightningConfig != nil {
		out["announcements"] = lightningConfig.LightningAnnouncements
		out["announce_timing"] = lightningConfig.AnnounceTiming
		out["displayname_overrides"] = lightningConfig.DisplaynameOverrides
	}
	return out
}

// Stop lightning trigger system
func stopLightningTrigger() {
	if lightningTrigger != nil {
		lightningTrigger.stopRedAlertReminders()
		lightningTrigger.Stop()
	}
}
