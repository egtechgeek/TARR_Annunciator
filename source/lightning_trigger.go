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
	URL               string    `json:"url"`
	FetchInterval     int       `json:"fetch_interval"` // seconds
	Timeout           int       `json:"timeout"`        // seconds
	LastCondition     string    `json:"last_condition"`
	LastFetch         time.Time `json:"last_fetch"`
	LastConditionTime time.Time `json:"last_condition_time"`

	// Internal state
	isRunning          bool
	stopChan           chan bool
	mu                 sync.Mutex
	redAlertActive     bool
	redAlertSince      time.Time
	reminderCount      int
	lastReminderAt     time.Time
	nextReminderAt     time.Time
	reminderStop       chan struct{}
	lastFetchError     string
	lastFetchErrorLog  time.Time
	loggedFetchRecover bool
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

// LightningConfig represents the lightning.json configuration
type LightningConfig struct {
	LightningAnnouncements []LightningAnnouncement `json:"lightning_announcements"`
	RedAlertPolicy         RedAlertPolicy          `json:"red_alert_policy"`
	Metadata               json.RawMessage         `json:"metadata,omitempty"`
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
	// Load lightning configuration
	if err := loadLightningConfig(); err != nil {
		log.Printf("Warning: Failed to load lightning configuration: %v", err)
		return err
	}

	// Create lightning trigger with default settings
	lightningTrigger = &LightningTrigger{
		ID:            "lightning_monitor",
		Name:          "Lightning Alert Monitor",
		Enabled:       true,
		URL:           "https://broward.thormobile4.net/tp/FL0115.xml",
		FetchInterval: 30, // 30 seconds default
		Timeout:       30, // 30 seconds timeout
		LastCondition: "Reset",
		stopChan:      make(chan bool),
	}

	// Start the lightning trigger if enabled
	if lightningTrigger.Enabled {
		go lightningTrigger.Start()
		log.Printf("✓ Lightning trigger system initialized and started")
		log.Printf("  - Monitoring URL: %s", lightningTrigger.URL)
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
		ReminderAudioFile:       "thor_repeat1.mp3",
		ReminderIncludeHorn:     false,
		HornAudioFile:           "thor_red_alert.mp3",
	}
}

func (c *LightningConfig) ensurePolicyDefaults() {
	if c.RedAlertPolicy.ReminderIntervalMinutes <= 0 {
		c.RedAlertPolicy = defaultRedAlertPolicy()
		return
	}
	if c.RedAlertPolicy.ReminderAudioFile == "" {
		c.RedAlertPolicy.ReminderAudioFile = "thor_repeat1.mp3"
	}
	if c.RedAlertPolicy.HornAudioFile == "" {
		c.RedAlertPolicy.HornAudioFile = "thor_red_alert.mp3"
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
	name = filepath.Base(name)
	if name == "" {
		name = "redalert.mp3"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".mp3") {
		name += ".mp3"
	}
	if app != nil && app.Config != nil && app.Config.MP3Dir != "" {
		return filepath.Join(app.Config.MP3Dir, "lightning", name)
	}
	return filepath.Join("static", "mp3", "lightning", name)
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
	if t.isRunning {
		return
	}

	t.isRunning = true
	ticker := time.NewTicker(time.Duration(t.FetchInterval) * time.Second)
	defer ticker.Stop()

	log.Printf("Lightning trigger '%s' started with %d second interval", t.Name, t.FetchInterval)

	// Do initial fetch
	t.fetchAndCheck()

	for {
		select {
		case <-ticker.C:
			t.fetchAndCheck()
		case <-t.stopChan:
			t.isRunning = false
			log.Printf("Lightning trigger '%s' stopped", t.Name)
			return
		}
	}
}

// Stop the lightning trigger
func (t *LightningTrigger) Stop() {
	if t.isRunning {
		close(t.stopChan)
	}
}

// Fetch XML and check for lightning conditions
func (t *LightningTrigger) fetchAndCheck() {
	defer func() {
		t.LastFetch = time.Now()
	}()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(t.Timeout) * time.Second,
	}

	// Fetch XML
	resp, err := client.Get(t.URL)
	if err != nil {
		t.logFetchProblem(fmt.Sprintf("Lightning trigger fetch error: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.logFetchProblem(fmt.Sprintf("Lightning trigger received status %d", resp.StatusCode))
		return
	}

	// Read response body
	xmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.logFetchProblem(fmt.Sprintf("Lightning trigger read error: %v", err))
		return
	}

	// Save XML file locally
	if err := t.saveXMLFile(xmlData); err != nil {
		t.logFetchProblem(fmt.Sprintf("Lightning trigger failed to save XML file: %v", err))
		// Continue processing even if file save fails
	}

	// Convert XML from UTF-16 to UTF-8 if needed
	xmlString, err := t.convertXMLEncoding(xmlData)
	if err != nil {
		t.logFetchProblem(fmt.Sprintf("Lightning trigger encoding conversion error: %v", err))
		return
	}

	// Extract lightning alert value
	lightningAlert := t.extractLightningAlertFromString(xmlString)
	if lightningAlert == "" {
		return
	}

	t.noteFetchSuccess()

	// Check if condition has changed
	if lightningAlert != t.LastCondition {
		log.Printf("Lightning condition changed from '%s' to '%s'", t.LastCondition, lightningAlert)

		// Handle different lightning conditions
		if strings.ToLower(lightningAlert) == "unknown" {
			log.Printf("Lightning status 'Unknown' - treating as XML error, ignoring condition change")
			// Don't update LastCondition for Unknown - treat as XML parsing error
			return
		}

		// Check if this is an AllClear condition
		if strings.ToLower(lightningAlert) == "allclear" {
			if !t.shouldAcceptAllClear() {
				log.Printf("AllClear condition ignored - previous condition was '%s' (not RedAlert)", t.LastCondition)
				t.LastCondition = lightningAlert
				t.LastConditionTime = time.Now()
				return
			}
			log.Printf("AllClear condition accepted - previous condition was '%s'", t.LastCondition)
		}

		// Update condition state for valid (non-Unknown) conditions
		t.LastCondition = lightningAlert
		t.LastConditionTime = time.Now()

		t.applyConditionEffects(lightningAlert)
		t.playLightningAnnouncement(lightningAlert)
	}
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
	if lightningConfig == nil {
		log.Printf("Lightning configuration not loaded, cannot play announcement")
		return
	}

	var selectedAnnouncement *LightningAnnouncement

	// Find appropriate announcement based on condition
	// First try to match exact condition names
	for i := range lightningConfig.LightningAnnouncements {
		announcement := &lightningConfig.LightningAnnouncements[i]
		if !announcement.Enabled {
			continue
		}

		// Check for direct matches or pattern matches
		switch strings.ToLower(condition) {
		case "redalert":
			if strings.Contains(strings.ToLower(announcement.ID), "redalert") ||
				strings.Contains(strings.ToLower(announcement.ID), "red_alert") {
				selectedAnnouncement = announcement
			}
		case "warning":
			if strings.Contains(strings.ToLower(announcement.ID), "warning") &&
				!strings.Contains(strings.ToLower(announcement.ID), "red") {
				selectedAnnouncement = announcement
			}
		case "caution":
			if strings.Contains(strings.ToLower(announcement.ID), "caution") {
				selectedAnnouncement = announcement
			}
		case "allclear":
			if strings.Contains(strings.ToLower(announcement.ID), "allclear") ||
				strings.Contains(strings.ToLower(announcement.ID), "all_clear") {
				selectedAnnouncement = announcement
			}
		}

		if selectedAnnouncement != nil {
			break
		}
	}

	// If no specific match found, try generic matches
	if selectedAnnouncement == nil {
		for i := range lightningConfig.LightningAnnouncements {
			announcement := &lightningConfig.LightningAnnouncements[i]
			if !announcement.Enabled {
				continue
			}

			switch strings.ToLower(condition) {
			case "redalert":
				if strings.Contains(strings.ToLower(announcement.ID), "generic_redalert") {
					selectedAnnouncement = announcement
				}
			case "warning":
				if strings.Contains(strings.ToLower(announcement.ID), "generic_warning") {
					selectedAnnouncement = announcement
				}
			case "caution":
				if strings.Contains(strings.ToLower(announcement.ID), "generic_caution") {
					selectedAnnouncement = announcement
				}
			case "allclear":
				if strings.Contains(strings.ToLower(announcement.ID), "generic_allclear") {
					selectedAnnouncement = announcement
				}
			}

			if selectedAnnouncement != nil {
				break
			}
		}
	}

	if selectedAnnouncement == nil {
		log.Printf("No matching lightning announcement found for condition: %s", condition)
		return
	}

	log.Printf("Playing lightning announcement: %s", selectedAnnouncement.Name)

	// Queue announcement using the existing announcement system
	if announcementManager != nil {
		// Lightning alerts use their own type but with emergency priority
		announcementType := TypeLightning

		parameters := map[string]interface{}{
			"condition":      condition,
			"message":        selectedAnnouncement.TTSText,
			"trigger_source": "LIGHTNING_TRIGGER",
		}

		log.Printf("DEBUG: Lightning parameters being sent: %+v", parameters)

		// Lightning alerts always get the highest priority (10)
		priority := AnnouncementPriority(10)

		announcement, err := announcementManager.QueueAnnouncement(announcementType, priority, parameters, time.Now())
		if err != nil {
			log.Printf("Failed to queue lightning announcement: %v", err)
		} else {
			log.Printf("Queued HIGHEST PRIORITY lightning announcement: %s (ID: %s)", selectedAnnouncement.Name, announcement.ID)
			log.Printf("DEBUG: Audio files queued: %v", announcement.AudioFiles)
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
	t.mu.Unlock()

	t.LastCondition = "Reset"
	t.LastConditionTime = time.Time{}
	log.Printf("THOR Guard cached state reset to Reset")
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
		policy.ReminderAudioFile = "thor_repeat1.mp3"
	}
	if policy.HornAudioFile == "" {
		policy.HornAudioFile = "thor_red_alert.mp3"
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

func listLightningAudioFiles() []string {
	known := []string{
		"thor_repeat1.mp3",
		"redalert.mp3",
		"thor_red_alert.mp3",
		"thor_warning.mp3",
		"thor_caution.mp3",
		"thor_all_clear.mp3",
		"warning.mp3",
		"all_clear.mp3",
	}
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
	for _, name := range known {
		add(name)
	}
	if lightningConfig != nil {
		for _, announcement := range lightningConfig.LightningAnnouncements {
			add(announcement.AudioFile)
		}
		add(lightningConfig.RedAlertPolicy.ReminderAudioFile)
		add(lightningConfig.RedAlertPolicy.HornAudioFile)
	}
	if app != nil && app.Config != nil && app.Config.MP3Dir != "" {
		entries, err := os.ReadDir(filepath.Join(app.Config.MP3Dir, "lightning"))
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mp3") {
					add(entry.Name())
				}
			}
		}
	}
	sort.Strings(files)
	return files
}

// Update lightning trigger configuration
func (t *LightningTrigger) UpdateConfig(url string, fetchInterval int, timeout int) error {
	wasRunning := t.isRunning

	// Stop if running
	if wasRunning {
		t.Stop()
		// Wait a moment for the goroutine to stop
		time.Sleep(100 * time.Millisecond)
	}

	// Update configuration
	t.URL = url
	t.FetchInterval = fetchInterval
	t.Timeout = timeout

	// Restart if it was running
	if wasRunning {
		t.stopChan = make(chan bool) // Create new channel
		go t.Start()
	}

	log.Printf("Lightning trigger configuration updated - URL: %s, Interval: %ds", url, fetchInterval)
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
			"available_reminder_files": listLightningAudioFiles(),
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
	lightningTrigger.mu.Unlock()

	return map[string]interface{}{
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
		"available_reminder_files": listLightningAudioFiles(),
	}
}

// Stop lightning trigger system
func stopLightningTrigger() {
	if lightningTrigger != nil {
		lightningTrigger.stopRedAlertReminders()
		lightningTrigger.Stop()
	}
}
