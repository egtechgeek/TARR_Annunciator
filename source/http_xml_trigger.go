package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPXMLTrigger represents an HTTP XML monitoring trigger
type HTTPXMLTrigger struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Type     string              `json:"type"`
	Enabled  bool                `json:"enabled"`
	Config   HTTPXMLTriggerConfig `json:"config"`
	
	// Internal state
	isRunning bool
	stopChan  chan bool
	lastFetch time.Time
}

// HTTPXMLTriggerConfig defines the configuration for HTTP XML triggers
type HTTPXMLTriggerConfig struct {
	URL           string                    `json:"url"`
	FetchInterval int                       `json:"fetch_interval"` // seconds
	Timeout       int                       `json:"timeout"`        // seconds
	Monitors      []HTTPXMLMonitor          `json:"monitors"`
	Actions       []HTTPXMLTriggerAction    `json:"actions"`
}

// HTTPXMLMonitor defines what to monitor in the XML
type HTTPXMLMonitor struct {
	ID             string   `json:"id"`
	XPath          string   `json:"xpath"`
	TriggerValues  []string `json:"trigger_values"`
	Comparison     string   `json:"comparison"` // "equals", "contains", "greater_than", "less_than"
	LastValue      string   `json:"-"` // Internal state
	TriggeredCount int      `json:"-"` // Internal counter
}

// HTTPXMLTriggerAction defines what action to take when triggered
type HTTPXMLTriggerAction struct {
	AnnouncementType string            `json:"announcement_type"`
	Message          string            `json:"message"`
	Parameters       map[string]string `json:"parameters,omitempty"`
}

// Global HTTP XML triggers
var httpXMLTriggers []*HTTPXMLTrigger

// Initialize HTTP XML trigger system
func initializeHTTPXMLTriggers() error {
	// NOTE: systemConfig is not defined in this codebase
	// This function is preserved but disabled to avoid compilation errors
	log.Println("HTTP XML triggers disabled - systemConfig not available in this implementation")
	return nil
	
	/* Original code commented out to avoid compilation errors:
	if systemConfig == nil || !systemConfig.TriggerConfig.Enabled {
		log.Println("HTTP XML triggers disabled or not configured")
		return nil
	}
	
	// Load HTTP XML triggers from configuration
	for _, triggerConfig := range systemConfig.TriggerConfig.TriggerTypes {
		if triggerConfig.Type == "http_xml" && triggerConfig.Enabled {
			trigger := &HTTPXMLTrigger{
				ID:      triggerConfig.ID,
				Name:    triggerConfig.Name,
				Type:    triggerConfig.Type,
				Enabled: triggerConfig.Enabled,
				stopChan: make(chan bool),
			}
			
			// Parse config from Settings map
			if configData, ok := triggerConfig.Settings["config"].(map[string]interface{}); ok {
				trigger.Config = HTTPXMLTriggerConfig{
					URL:           getStringValue(configData, "url"),
					FetchInterval: getIntValue(configData, "fetch_interval"),
					Timeout:       getIntValue(configData, "timeout"),
				}
			} else {
				// Try direct access to settings
				trigger.Config = HTTPXMLTriggerConfig{
					URL:           getStringValue(triggerConfig.Settings, "url"),
					FetchInterval: getIntValue(triggerConfig.Settings, "fetch_interval"),
					Timeout:       getIntValue(triggerConfig.Settings, "timeout"),
				}
			}
			
			// Parse monitors and actions from the trigger settings
			// For now, use defaults since the JSON structure may not match perfectly
			// This can be configured properly through the admin interface later
			trigger.Config.Monitors = []HTTPXMLMonitor{
				{
					ID:            "default_monitor",
					XPath:         "//status/text()",
					TriggerValues: []string{"alert", "emergency"},
					Comparison:    "equals",
				},
			}
			
			trigger.Config.Actions = []HTTPXMLTriggerAction{
				{
					AnnouncementType: "safety",
					Message:          "System alert detected from {trigger}",
				},
			}
			
			httpXMLTriggers = append(httpXMLTriggers, trigger)
			
			// Start the trigger
			if trigger.Enabled {
				go trigger.Start()
				log.Printf("Started HTTP XML trigger: %s (%s)", trigger.Name, trigger.Config.URL)
			}
		}
	}
	
	log.Printf("✓ HTTP XML trigger system initialized with %d triggers", len(httpXMLTriggers))
	return nil
	*/
}

// Start the HTTP XML trigger monitoring
func (t *HTTPXMLTrigger) Start() {
	if t.isRunning {
		return
	}
	
	t.isRunning = true
	ticker := time.NewTicker(time.Duration(t.Config.FetchInterval) * time.Second)
	defer ticker.Stop()
	
	log.Printf("HTTP XML trigger '%s' started with %d second interval", t.Name, t.Config.FetchInterval)
	
	for {
		select {
		case <-ticker.C:
			t.fetchAndCheck()
		case <-t.stopChan:
			t.isRunning = false
			log.Printf("HTTP XML trigger '%s' stopped", t.Name)
			return
		}
	}
}

// Stop the HTTP XML trigger
func (t *HTTPXMLTrigger) Stop() {
	if t.isRunning {
		close(t.stopChan)
	}
}

// Fetch XML and check for trigger conditions
func (t *HTTPXMLTrigger) fetchAndCheck() {
	defer func() {
		t.lastFetch = time.Now()
	}()
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(t.Config.Timeout) * time.Second,
	}
	
	// Fetch XML
	resp, err := client.Get(t.Config.URL)
	if err != nil {
		log.Printf("HTTP XML trigger '%s' fetch error: %v", t.Name, err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("HTTP XML trigger '%s' received status %d", t.Name, resp.StatusCode)
		return
	}
	
	// Read response body
	xmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("HTTP XML trigger '%s' read error: %v", t.Name, err)
		return
	}
	
	// Parse and check each monitor
	for i, monitor := range t.Config.Monitors {
		value := t.extractValueFromXML(xmlData, monitor.XPath)
		if value == "" {
			continue
		}
		
		// Store the current value
		t.Config.Monitors[i].LastValue = value
		
		// Check if trigger condition is met
		if t.checkTriggerCondition(monitor, value) {
			t.Config.Monitors[i].TriggeredCount++
			log.Printf("HTTP XML trigger '%s' monitor '%s' triggered: %s", t.Name, monitor.ID, value)
			t.executeActions(monitor, value)
		}
	}
}

// Extract value from XML using simple string matching (simplified XPath)
func (t *HTTPXMLTrigger) extractValueFromXML(xmlData []byte, xpath string) string {
	// This is a simplified XPath implementation
	// For production, consider using a proper XPath library like gokogiri or xmlpath
	
	xmlStr := string(xmlData)
	
	// Handle simple cases like "//status/text()" or "//temperature"
	if strings.Contains(xpath, "//") && strings.Contains(xpath, "/text()") {
		// Extract tag name
		xpath = strings.Replace(xpath, "//", "", 1)
		xpath = strings.Replace(xpath, "/text()", "", 1)
		
		// Find the tag content
		startTag := fmt.Sprintf("<%s>", xpath)
		endTag := fmt.Sprintf("</%s>", xpath)
		
		startIndex := strings.Index(xmlStr, startTag)
		if startIndex == -1 {
			return ""
		}
		
		startIndex += len(startTag)
		endIndex := strings.Index(xmlStr[startIndex:], endTag)
		if endIndex == -1 {
			return ""
		}
		
		return strings.TrimSpace(xmlStr[startIndex : startIndex+endIndex])
	}
	
	return ""
}

// Check if trigger condition is met
func (t *HTTPXMLTrigger) checkTriggerCondition(monitor HTTPXMLMonitor, value string) bool {
	switch monitor.Comparison {
	case "equals":
		for _, triggerValue := range monitor.TriggerValues {
			if value == triggerValue {
				return true
			}
		}
	case "contains":
		for _, triggerValue := range monitor.TriggerValues {
			if strings.Contains(value, triggerValue) {
				return true
			}
		}
	case "not_equals":
		for _, triggerValue := range monitor.TriggerValues {
			if value == triggerValue {
				return false
			}
		}
		return len(monitor.TriggerValues) > 0 // Only trigger if we have values to compare against
	}
	
	return false
}

// Execute actions when trigger condition is met
func (t *HTTPXMLTrigger) executeActions(monitor HTTPXMLMonitor, triggerValue string) {
	for _, action := range t.Config.Actions {
		// Create announcement based on action
		message := strings.Replace(action.Message, "{value}", triggerValue, -1)
		message = strings.Replace(message, "{monitor}", monitor.ID, -1)
		message = strings.Replace(message, "{trigger}", t.Name, -1)
		
		// Queue announcement
		if announcementManager != nil {
			// Convert string to AnnouncementType
			var announcementType AnnouncementType
			switch action.AnnouncementType {
			case "station":
				announcementType = TypeStation
			case "safety":
				announcementType = TypeSafety
			case "promo":
				announcementType = TypePromo
			case "emergency":
				announcementType = TypeEmergency
			default:
				announcementType = TypeStation
			}
			
			// Create parameters map
			parameters := map[string]interface{}{
				"message":        message,
				"trigger_source": fmt.Sprintf("HTTP_XML_TRIGGER:%s", t.Name),
				"monitor_id":     monitor.ID,
				"trigger_value":  triggerValue,
			}
			
			// Get priority based on announcement type
			priority := AnnouncementPriority(getAnnouncementTypePriority(action.AnnouncementType))
			
			announcement, err := announcementManager.QueueAnnouncement(announcementType, priority, parameters, time.Now())
			if err != nil {
				log.Printf("Failed to queue HTTP XML trigger announcement: %v", err)
			} else {
				log.Printf("Queued HTTP XML trigger announcement: %s (ID: %s)", message, announcement.ID)
			}
		}
	}
}

// Get announcement type priority
func getAnnouncementTypePriority(announcementType string) int {
	// NOTE: systemConfig is not defined in this codebase
	// Return appropriate default priorities for different announcement types
	switch announcementType {
	case "emergency":
		return 10
	case "safety":
		return 8
	case "station":
		return 5
	case "promo":
		return 3
	default:
		return 5 // Default priority
	}
}

// Stop all HTTP XML triggers
func stopHTTPXMLTriggers() {
	for _, trigger := range httpXMLTriggers {
		trigger.Stop()
	}
	httpXMLTriggers = nil
}

// Get HTTP XML trigger status for API
func getHTTPXMLTriggerStatus() []map[string]interface{} {
	status := make([]map[string]interface{}, 0)
	
	for _, trigger := range httpXMLTriggers {
		triggerStatus := map[string]interface{}{
			"id":             trigger.ID,
			"name":           trigger.Name,
			"enabled":        trigger.Enabled,
			"running":        trigger.isRunning,
			"url":            trigger.Config.URL,
			"fetch_interval": trigger.Config.FetchInterval,
			"last_fetch":     trigger.lastFetch.Format("2006-01-02 15:04:05"),
			"monitors":       make([]map[string]interface{}, 0),
		}
		
		for _, monitor := range trigger.Config.Monitors {
			monitorStatus := map[string]interface{}{
				"id":               monitor.ID,
				"xpath":            monitor.XPath,
				"last_value":       monitor.LastValue,
				"triggered_count":  monitor.TriggeredCount,
				"trigger_values":   monitor.TriggerValues,
				"comparison":       monitor.Comparison,
			}
			triggerStatus["monitors"] = append(triggerStatus["monitors"].([]map[string]interface{}), monitorStatus)
		}
		
		status = append(status, triggerStatus)
	}
	
	return status
}

// Helper functions to safely extract values from configuration maps
func getStringValue(config map[string]interface{}, key string) string {
	if val, ok := config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntValue(config map[string]interface{}, key string) int {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// Try to parse string as int
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	// Return sensible defaults
	switch key {
	case "fetch_interval":
		return 30
	case "timeout":
		return 10
	default:
		return 0
	}
}