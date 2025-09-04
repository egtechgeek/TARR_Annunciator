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
	"strings"
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
	isRunning bool
	stopChan  chan bool
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
		Timeout:       30,  // 30 seconds timeout
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

// Load lightning configuration from JSON
func loadLightningConfig() error {
	configPath := filepath.Join("json", "lightning.json")
	
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("lightning.json not found at %s", configPath)
	}
	
	// Read file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read lightning.json: %v", err)
	}
	
	// Parse JSON
	lightningConfig = &LightningConfig{}
	if err := json.Unmarshal(data, lightningConfig); err != nil {
		return fmt.Errorf("failed to parse lightning.json: %v", err)
	}
	
	log.Printf("✓ Loaded lightning configuration with %d announcements", len(lightningConfig.LightningAnnouncements))
	return nil
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
		log.Printf("Lightning trigger fetch error: %v", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("Lightning trigger received status %d", resp.StatusCode)
		return
	}
	
	// Read response body
	xmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Lightning trigger read error: %v", err)
		return
	}
	
	// Save XML file locally
	if err := t.saveXMLFile(xmlData); err != nil {
		log.Printf("Lightning trigger failed to save XML file: %v", err)
		// Continue processing even if file save fails
	}
	
	// Convert XML from UTF-16 to UTF-8 if needed
	xmlString, err := t.convertXMLEncoding(xmlData)
	if err != nil {
		log.Printf("Lightning trigger encoding conversion error: %v", err)
		return
	}
	
	// Extract lightning alert value
	lightningAlert := t.extractLightningAlertFromString(xmlString)
	if lightningAlert == "" {
		log.Printf("No lightningalert tag found in XML")
		return
	}
	
	log.Printf("Lightning alert status: %s", lightningAlert)
	
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
			// Only play AllClear if previous condition was RedAlert or Warning
			prevCondition := strings.ToLower(t.LastCondition)
			if prevCondition != "redalert" && prevCondition != "warning" {
				log.Printf("AllClear condition ignored - previous condition was '%s' (not RedAlert or Warning)", t.LastCondition)
				// Update the condition but don't play announcement
				t.LastCondition = lightningAlert
				t.LastConditionTime = time.Now()
				return
			}
			log.Printf("AllClear condition accepted - previous condition was '%s'", t.LastCondition)
		}
		
		// Update condition state for valid (non-Unknown) conditions
		t.LastCondition = lightningAlert
		t.LastConditionTime = time.Now()
		
		// Play appropriate announcement for valid conditions
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
	
	log.Printf("Lightning XML saved to: %s (%d bytes)", filePath, len(xmlData))
	return nil
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
	// Debug: Log first 1000 characters of XML to see what we're parsing
	xmlPreview := xmlStr
	if len(xmlStr) > 1000 {
		xmlPreview = xmlStr[:1000] + "..."
	}
	log.Printf("Lightning XML preview (converted): %s", xmlPreview)
	
	// Look for <lightningalert>VALUE</lightningalert> (case sensitive)
	startTag := "<lightningalert>"
	endTag := "</lightningalert>"
	
	startIndex := strings.Index(xmlStr, startTag)
	if startIndex == -1 {
		// Try case-insensitive search for debugging
		lowerXML := strings.ToLower(xmlStr)
		if strings.Contains(lowerXML, "<lightningalert>") {
			log.Printf("Lightning: Found lightningalert tag in different case")
		} else {
			log.Printf("Lightning: No lightningalert tag found in XML")
		}
		return ""
	}
	
	startIndex += len(startTag)
	endIndex := strings.Index(xmlStr[startIndex:], endTag)
	if endIndex == -1 {
		log.Printf("Lightning: Found opening tag but no closing tag")
		return ""
	}
	
	value := strings.TrimSpace(xmlStr[startIndex : startIndex+endIndex])
	log.Printf("Lightning: Successfully extracted value: '%s'", value)
	return value
}

// Extract lightningalert value from XML (deprecated - use extractLightningAlertFromString)
func (t *LightningTrigger) extractLightningAlert(xmlData []byte) string {
	xmlStr := string(xmlData)
	
	// Debug: Log first 1000 characters of XML to see what we're parsing
	xmlPreview := xmlStr
	if len(xmlStr) > 1000 {
		xmlPreview = xmlStr[:1000] + "..."
	}
	log.Printf("Lightning XML preview: %s", xmlPreview)
	
	// Look for <lightningalert>VALUE</lightningalert> (case sensitive)
	startTag := "<lightningalert>"
	endTag := "</lightningalert>"
	
	startIndex := strings.Index(xmlStr, startTag)
	if startIndex == -1 {
		// Try case-insensitive search for debugging
		lowerXML := strings.ToLower(xmlStr)
		if strings.Contains(lowerXML, "<lightningalert>") {
			log.Printf("Lightning: Found lightningalert tag in different case")
		} else {
			log.Printf("Lightning: No lightningalert tag found in XML")
		}
		return ""
	}
	
	startIndex += len(startTag)
	endIndex := strings.Index(xmlStr[startIndex:], endTag)
	if endIndex == -1 {
		log.Printf("Lightning: Found opening tag but no closing tag")
		return ""
	}
	
	value := strings.TrimSpace(xmlStr[startIndex : startIndex+endIndex])
	log.Printf("Lightning: Successfully extracted value: '%s'", value)
	return value
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
				break
			}
		case "warning":
			if strings.Contains(strings.ToLower(announcement.ID), "warning") &&
			   !strings.Contains(strings.ToLower(announcement.ID), "red") {
				selectedAnnouncement = announcement
				break
			}
		case "allclear":
			if strings.Contains(strings.ToLower(announcement.ID), "allclear") ||
			   strings.Contains(strings.ToLower(announcement.ID), "all_clear") {
				selectedAnnouncement = announcement
				break
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

// TestCondition manually triggers a lightning announcement for testing
func (t *LightningTrigger) TestCondition(condition string) {
	log.Printf("DEBUG: Manual test for condition: %s", condition)
	// Fake a condition change
	t.LastCondition = "Testing"
	// Call the announcement function
	t.playLightningAnnouncement(condition)
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
			"enabled": false,
			"error":   "Lightning trigger not initialized",
		}
	}
	
	return map[string]interface{}{
		"id":                    lightningTrigger.ID,
		"name":                  lightningTrigger.Name,
		"enabled":               lightningTrigger.Enabled,
		"running":               lightningTrigger.isRunning,
		"url":                   lightningTrigger.URL,
		"fetch_interval":        lightningTrigger.FetchInterval,
		"timeout":               lightningTrigger.Timeout,
		"last_fetch":            lightningTrigger.LastFetch.Format("2006-01-02 15:04:05"),
		"last_condition":        lightningTrigger.LastCondition,
		"last_condition_time":   lightningTrigger.LastConditionTime.Format("2006-01-02 15:04:05"),
	}
}

// Stop lightning trigger system
func stopLightningTrigger() {
	if lightningTrigger != nil {
		lightningTrigger.Stop()
	}
}