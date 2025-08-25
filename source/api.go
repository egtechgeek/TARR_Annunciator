package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// API Status Handler
func apiStatusHandler(c *gin.Context) {
	platformInfo := getPlatformInfo()
	devices := getAudioDevices()
	
	c.JSON(http.StatusOK, gin.H{
		"status":               "online",
		"audio_available":      app.AudioEnabled,
		"audio_backend":        "beep",
		"api_enabled":          app.Config.APIEnabled,
		"scheduler_running":    true,
		"volume":              int(app.Config.CurrentVolume * 100),
		"selected_audio_device": app.Config.SelectedAudioDevice,
		"available_devices":    len(devices),
		"platform":            platformInfo,
		"timestamp":           time.Now().Format(time.RFC3339),
	})
}

// API Documentation Handler
func apiDocsHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "api_docs.html", nil)
}

// Station Announcement API
func apiStationAnnouncementHandler(c *gin.Context) {
	var data map[string]interface{}
	
	// Handle both JSON and form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	} else {
		data = make(map[string]interface{})
		data["train_number"] = c.PostForm("train_number")
		data["direction"] = c.PostForm("direction")
		data["destination"] = c.PostForm("destination")
		data["track_number"] = c.PostForm("track_number")
	}

	// Validate required fields
	requiredFields := []string{"train_number", "direction", "destination", "track_number"}
	for _, field := range requiredFields {
		if val, exists := data[field]; !exists || val == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Missing required field: " + field,
			})
			return
		}
	}

	// Extract values
	trainNumber := data["train_number"].(string)
	direction := data["direction"].(string)
	destination := data["destination"].(string)
	trackNumber := data["track_number"].(string)

	// Get priority from request or default to normal
	priorityStr := c.DefaultPostForm("priority", "normal")
	priority := ParsePriority(priorityStr)
	
	// Get scheduled time (default to immediate)
	scheduledAt := time.Now()
	if delayStr := c.PostForm("delay"); delayStr != "" {
		if delaySeconds, err := strconv.Atoi(delayStr); err == nil && delaySeconds > 0 {
			scheduledAt = scheduledAt.Add(time.Duration(delaySeconds) * time.Second)
		}
	}

	// Queue the announcement
	parameters := map[string]interface{}{
		"train_number": trainNumber,
		"direction":    direction,
		"destination":  destination,
		"track_number": trackNumber,
	}
	
	announcement, err := announcementManager.QueueAnnouncement(TypeStation, priority, parameters, scheduledAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to queue announcement: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Station announcement queued",
		"announcement": gin.H{
			"id":           announcement.ID,
			"type":         "station",
			"priority":     announcement.Priority.String(),
			"status":       string(announcement.Status),
			"train_number": trainNumber,
			"direction":    direction,
			"destination":  destination,
			"track_number": trackNumber,
			"scheduled_at": announcement.ScheduledAt.Format(time.RFC3339),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Safety Announcement API
func apiSafetyAnnouncementHandler(c *gin.Context) {
	var data map[string]interface{}
	
	// Handle both JSON and form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	} else {
		data = make(map[string]interface{})
		data["language"] = c.PostForm("language")
	}

	// Validate language field
	language, exists := data["language"]
	if !exists || language == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required field: language"})
		return
	}

	// Validate language exists
	safetyLanguages := loadJSON("safety", []SafetyLanguage{}).([]SafetyLanguage)
	validLanguage := false
	for _, lang := range safetyLanguages {
		if lang.ID == language.(string) {
			validLanguage = true
			break
		}
	}

	if !validLanguage {
		availableLanguages := make([]string, len(safetyLanguages))
		for i, lang := range safetyLanguages {
			availableLanguages[i] = lang.ID
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid language '" + language.(string) + "'. Available: " + joinStrings(availableLanguages, ", "),
		})
		return
	}

	// Get priority from request or default to high (safety is important)
	priorityStr := c.DefaultPostForm("priority", "high")
	priority := ParsePriority(priorityStr)
	
	// Get scheduled time (default to immediate)
	scheduledAt := time.Now()
	if delayStr := c.PostForm("delay"); delayStr != "" {
		if delaySeconds, err := strconv.Atoi(delayStr); err == nil && delaySeconds > 0 {
			scheduledAt = scheduledAt.Add(time.Duration(delaySeconds) * time.Second)
		}
	}

	// Queue the announcement
	parameters := map[string]interface{}{
		"language": language.(string),
	}
	
	announcement, err := announcementManager.QueueAnnouncement(TypeSafety, priority, parameters, scheduledAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to queue announcement: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Safety announcement queued",
		"announcement": gin.H{
			"id":           announcement.ID,
			"type":         "safety",
			"priority":     announcement.Priority.String(),
			"status":       string(announcement.Status),
			"language":     language,
			"scheduled_at": announcement.ScheduledAt.Format(time.RFC3339),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Promo Announcement API
func apiPromoAnnouncementHandler(c *gin.Context) {
	var data map[string]interface{}
	
	// Handle both JSON and form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	} else {
		data = make(map[string]interface{})
		data["file"] = c.PostForm("file")
	}

	// Validate file field
	file, exists := data["file"]
	if !exists || file == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required field: file"})
		return
	}

	// Validate promo file exists
	promoAnnouncements := loadJSON("promo", []PromoAnnouncement{}).([]PromoAnnouncement)
	validFile := false
	for _, promo := range promoAnnouncements {
		if promo.ID == file.(string) {
			validFile = true
			break
		}
	}

	if !validFile {
		availableFiles := make([]string, len(promoAnnouncements))
		for i, promo := range promoAnnouncements {
			availableFiles[i] = promo.ID
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid promo file '" + file.(string) + "'. Available: " + joinStrings(availableFiles, ", "),
		})
		return
	}

	// Get priority from request or default to low (promos are typically low priority)
	priorityStr := c.DefaultPostForm("priority", "low")
	priority := ParsePriority(priorityStr)
	
	// Get scheduled time (default to immediate)
	scheduledAt := time.Now()
	if delayStr := c.PostForm("delay"); delayStr != "" {
		if delaySeconds, err := strconv.Atoi(delayStr); err == nil && delaySeconds > 0 {
			scheduledAt = scheduledAt.Add(time.Duration(delaySeconds) * time.Second)
		}
	}

	// Queue the announcement
	parameters := map[string]interface{}{
		"file": file.(string),
	}
	
	announcement, err := announcementManager.QueueAnnouncement(TypePromo, priority, parameters, scheduledAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to queue announcement: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Promo announcement queued",
		"announcement": gin.H{
			"id":           announcement.ID,
			"type":         "promo",
			"priority":     announcement.Priority.String(),
			"status":       string(announcement.Status),
			"file":         file,
			"scheduled_at": announcement.ScheduledAt.Format(time.RFC3339),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Volume API handlers
func apiGetVolumeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"volume":         app.Config.CurrentVolume,
		"volume_percent": int(app.Config.CurrentVolume * 100),
	})
}

func apiSetVolumeHandler(c *gin.Context) {
	var data map[string]interface{}
	
	// Handle both JSON and form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	} else {
		data = make(map[string]interface{})
		data["volume"] = c.PostForm("volume")
	}

	volumeVal, exists := data["volume"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Volume parameter required (0.0 to 1.0 or 0 to 100)"})
		return
	}

	var volume float64
	var err error

	switch v := volumeVal.(type) {
	case string:
		volume, err = strconv.ParseFloat(v, 64)
	case float64:
		volume = v
	case int:
		volume = float64(v)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid volume value"})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid volume value"})
		return
	}

	// Handle both 0-1 and 0-100 ranges
	if volume > 1.0 {
		volume = volume / 100.0
	}

	// Clamp volume
	if volume < 0.0 {
		volume = 0.0
	} else if volume > 1.0 {
		volume = 1.0
	}

	app.Config.CurrentVolume = volume

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"volume":         app.Config.CurrentVolume,
		"volume_percent": int(app.Config.CurrentVolume * 100),
	})
}

// Audio Device API handlers
func apiGetAudioDevicesHandler(c *gin.Context) {
	devices := getAudioDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"current_device": app.Config.SelectedAudioDevice,
	})
}

func apiSetAudioDeviceHandler(c *gin.Context) {
	var data map[string]interface{}
	
	// Handle both JSON and form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	} else {
		data = make(map[string]interface{})
		data["device_id"] = c.PostForm("device_id")
	}

	deviceID, exists := data["device_id"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID parameter required"})
		return
	}

	deviceIDStr, ok := deviceID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	// Validate device exists
	devices := getAudioDevices()
	validDevice := false
	var selectedDevice AudioDevice
	for _, device := range devices {
		if device.ID == deviceIDStr {
			validDevice = true
			selectedDevice = device
			break
		}
	}

	if !validDevice {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	// Set the device
	if err := setAudioDevice(deviceIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set audio device: " + err.Error()})
		return
	}

	app.Config.SelectedAudioDevice = deviceIDStr

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"device": selectedDevice,
		"message": "Audio device set successfully",
	})
}

// Platform Information API
func apiPlatformInfoHandler(c *gin.Context) {
	platformInfo := getPlatformInfo()
	devices := getAudioDevices()
	
	c.JSON(http.StatusOK, gin.H{
		"platform_info":     platformInfo,
		"audio_devices":     devices,
		"current_device":    app.Config.SelectedAudioDevice,
		"audio_backend":     "beep (faiface/beep)",
		"cross_platform":    true,
	})
}

// Configuration API
func apiGetConfigHandler(c *gin.Context) {
	trains := loadJSON("trains", []Train{}).([]Train)
	directions := loadJSON("directions", []Direction{}).([]Direction)
	destinations := loadJSON("destinations", []Destination{}).([]Destination)
	tracks := loadJSON("tracks", []Track{}).([]Track)
	promoAnnouncements := loadJSON("promo", []PromoAnnouncement{}).([]PromoAnnouncement)
	safetyLanguages := loadJSON("safety", []SafetyLanguage{}).([]SafetyLanguage)
	emergencies := loadJSON("emergencies", []Emergency{}).([]Emergency)

	c.JSON(http.StatusOK, gin.H{
		"trains":               trains,
		"directions":           directions,
		"destinations":         destinations,
		"tracks":               tracks,
		"promo_announcements":  promoAnnouncements,
		"safety_languages":     safetyLanguages,
		"emergencies":          emergencies,
	})
}

// Schedule API handlers
func apiGetScheduleHandler(c *gin.Context) {
	schedule := loadJSON("cron", CronData{}).(CronData)
	c.JSON(http.StatusOK, gin.H{"schedule": schedule})
}

func apiPostScheduleHandler(c *gin.Context) {
	var data map[string]interface{}
	
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	scheduleData, exists := data["schedule"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Schedule data required"})
		return
	}

	// Convert interface{} to CronData
	scheduleJSON, _ := json.Marshal(scheduleData)
	var cronData CronData
	if err := json.Unmarshal(scheduleJSON, &cronData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule data"})
		return
	}

	if err := saveJSON("cron", cronData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update schedule: " + err.Error()})
		return
	}

	updateScheduler()

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Schedule updated successfully",
		"active_jobs": len(app.Scheduler.Entries()),
	})
}

// Queue Management API handlers
func apiGetQueueStatusHandler(c *gin.Context) {
	if announcementManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Announcement manager not initialized"})
		return
	}

	status := announcementManager.GetQueueStatus()
	c.JSON(http.StatusOK, status)
}

func apiGetQueueHistoryHandler(c *gin.Context) {
	if announcementManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Announcement manager not initialized"})
		return
	}

	// Get limit from query parameter (default to 20)
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	history := announcementManager.GetHistory(limit)
	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"count":   len(history),
	})
}

func apiCancelAnnouncementHandler(c *gin.Context) {
	if announcementManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Announcement manager not initialized"})
		return
	}

	var data map[string]interface{}
	
	// Handle both JSON and form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	} else {
		data = make(map[string]interface{})
		data["id"] = c.PostForm("id")
	}

	id, exists := data["id"]
	if !exists || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Announcement ID required"})
		return
	}

	err := announcementManager.CancelAnnouncement(id.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Announcement cancelled successfully",
		"id":      id,
	})
}

// Emergency announcement API (highest priority, audio files only)
func apiEmergencyAnnouncementHandler(c *gin.Context) {
	if announcementManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Announcement manager not initialized"})
		return
	}

	var data map[string]interface{}
	
	// Handle both JSON and form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	} else {
		data = make(map[string]interface{})
		data["file"] = c.PostForm("file")
	}

	// Emergency announcements require a file parameter
	file, hasFile := data["file"]
	if !hasFile || file == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Emergency announcement requires 'file' parameter"})
		return
	}

	// Validate emergency file exists in the emergency list
	emergencies := loadJSON("emergencies", []Emergency{}).([]Emergency)
	validFile := false
	var selectedEmergency Emergency
	for _, emergency := range emergencies {
		if emergency.ID == file.(string) {
			validFile = true
			selectedEmergency = emergency
			break
		}
	}

	if !validFile {
		availableFiles := make([]string, len(emergencies))
		for i, emergency := range emergencies {
			availableFiles[i] = emergency.ID
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid emergency file '%s'. Available: %s", file.(string), joinStrings(availableFiles, ", ")),
		})
		return
	}

	// Emergency announcements are always immediate and highest priority
	parameters := map[string]interface{}{
		"file": file.(string),
	}
	
	announcement, err := announcementManager.QueueAnnouncement(TypeEmergency, PriorityEmergency, parameters, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to queue emergency announcement: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Emergency announcement '%s' queued with highest priority", selectedEmergency.Name),
		"announcement": gin.H{
			"id":          announcement.ID,
			"type":        "emergency",
			"priority":    "emergency",
			"status":      string(announcement.Status),
			"file":        file,
			"name":        selectedEmergency.Name,
			"description": selectedEmergency.Description,
			"category":    selectedEmergency.Category,
			"scheduled_at": announcement.ScheduledAt.Format(time.RFC3339),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Utility function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}