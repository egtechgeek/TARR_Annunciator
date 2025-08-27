package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// JSON file handling
func loadJSON(name string, defaultValue interface{}) interface{} {
	var filePath string
	
	switch name {
	case "trains":
		filePath = filepath.Join(app.Config.JSONDir, "trains_selected.json")
	case "trains_available":
		filePath = filepath.Join(app.Config.JSONDir, "trains_available.json")
	case "directions":
		filePath = filepath.Join(app.Config.JSONDir, "directions.json")
	case "destinations":
		filePath = filepath.Join(app.Config.JSONDir, "destinations_selected.json")
	case "destinations_available":
		filePath = filepath.Join(app.Config.JSONDir, "destinations_available.json")
	case "tracks":
		filePath = filepath.Join(app.Config.JSONDir, "tracks.json")
	case "promo":
		filePath = filepath.Join(app.Config.JSONDir, "promo.json")
	case "safety":
		filePath = filepath.Join(app.Config.JSONDir, "safety.json")
	case "emergencies":
		filePath = filepath.Join(app.Config.JSONDir, "emergencies.json")
	case "cron":
		filePath = filepath.Join(app.Config.JSONDir, "cron.json")
	default:
		return defaultValue
	}

	if !fileExists(filePath) {
		return defaultValue
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading JSON file %s: %v", filePath, err)
		return defaultValue
	}

	// Parse based on expected type
	switch name {
	case "trains":
		var wrapper struct {
			Trains []Train `json:"trains"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Trains) > 0 {
			return wrapper.Trains
		}
		// Try direct array format
		var trains []Train
		if err := json.Unmarshal(data, &trains); err == nil {
			return trains
		}
		
	case "directions":
		var wrapper struct {
			Directions []Direction `json:"directions"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Directions) > 0 {
			return wrapper.Directions
		}
		var directions []Direction
		if err := json.Unmarshal(data, &directions); err == nil {
			return directions
		}
		
	case "destinations":
		var wrapper struct {
			Destinations []Destination `json:"destinations"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Destinations) > 0 {
			return wrapper.Destinations
		}
		var destinations []Destination
		if err := json.Unmarshal(data, &destinations); err == nil {
			return destinations
		}
		
	case "tracks":
		var wrapper struct {
			Tracks []Track `json:"tracks"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Tracks) > 0 {
			return wrapper.Tracks
		}
		var tracks []Track
		if err := json.Unmarshal(data, &tracks); err == nil {
			return tracks
		}
		
	case "promo":
		var wrapper struct {
			Promo []PromoAnnouncement `json:"promo"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Promo) > 0 {
			return wrapper.Promo
		}
		var promo []PromoAnnouncement
		if err := json.Unmarshal(data, &promo); err == nil {
			return promo
		}
		
	case "safety":
		var wrapper struct {
			Safety []SafetyLanguage `json:"safety"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Safety) > 0 {
			return wrapper.Safety
		}
		var safety []SafetyLanguage
		if err := json.Unmarshal(data, &safety); err == nil {
			return safety
		}
		
	case "emergencies":
		var wrapper struct {
			Emergencies []Emergency `json:"emergencies"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Emergencies) > 0 {
			return wrapper.Emergencies
		}
		var emergencies []Emergency
		if err := json.Unmarshal(data, &emergencies); err == nil {
			return emergencies
		}
		
	case "cron":
		var cronData CronData
		if err := json.Unmarshal(data, &cronData); err == nil {
			return cronData
		}
	}

	log.Printf("Error parsing JSON file %s, using default", filePath)
	return defaultValue
}

func saveJSON(name string, data interface{}) error {
	var filePath string
	
	switch name {
	case "trains":
		filePath = filepath.Join(app.Config.JSONDir, "trains_selected.json")
	case "trains_available":
		filePath = filepath.Join(app.Config.JSONDir, "trains_available.json")
	case "directions":
		filePath = filepath.Join(app.Config.JSONDir, "directions.json")
	case "destinations":
		filePath = filepath.Join(app.Config.JSONDir, "destinations_selected.json")
	case "destinations_available":
		filePath = filepath.Join(app.Config.JSONDir, "destinations_available.json")
	case "tracks":
		filePath = filepath.Join(app.Config.JSONDir, "tracks.json")
	case "promo":
		filePath = filepath.Join(app.Config.JSONDir, "promo.json")
	case "safety":
		filePath = filepath.Join(app.Config.JSONDir, "safety.json")
	case "emergencies":
		filePath = filepath.Join(app.Config.JSONDir, "emergencies.json")
	case "cron":
		filePath = filepath.Join(app.Config.JSONDir, "cron.json")
	default:
		return fmt.Errorf("unknown JSON file: %s", name)
	}

	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, jsonData, 0644)
}

// Scheduler functions
func updateScheduler() {
	log.Println("Updating scheduler...")
	
	// Remove all existing jobs
	entries := app.Scheduler.Entries()
	for _, entry := range entries {
		app.Scheduler.Remove(entry.ID)
	}

	cronData := loadJSON("cron", CronData{}).(CronData)

	// Station announcements
	for i, item := range cronData.StationAnnouncements {
		if item.Enabled {
			// Capture variables for closure
			trainNum, direction, destination, trackNum := item.TrainNumber, item.Direction, item.Destination, item.TrackNumber
			_, err := app.Scheduler.AddFunc(item.Cron, func() {
				log.Printf("🕐 Scheduled station announcement triggered: Train %s", trainNum)
				if announcementManager != nil {
					parameters := map[string]interface{}{
						"train_number": trainNum,
						"direction":    direction,
						"destination":  destination,
						"track_number": trackNum,
					}
					announcement, queueErr := announcementManager.QueueAnnouncement(TypeStation, PriorityNormal, parameters, time.Now())
					if queueErr != nil {
						log.Printf("Error queuing scheduled station announcement: %v", queueErr)
					} else {
						log.Printf("Scheduled station announcement queued successfully (ID: %s)", announcement.ID)
					}
				} else {
					log.Printf("⚠️  Announcement manager not available for scheduled announcement")
				}
			})
			if err != nil {
				log.Printf("Error scheduling station announcement %d: %v", i, err)
			} else {
				log.Printf("Scheduled: %s - Train %s", item.Cron, item.TrainNumber)
			}
		}
	}

	// Promo announcements
	for i, item := range cronData.PromoAnnouncements {
		if item.Enabled {
			// Capture variables for closure
			file := item.File
			_, err := app.Scheduler.AddFunc(item.Cron, func() {
				log.Printf("🕐 Scheduled promo announcement triggered: %s", file)
				if announcementManager != nil {
					parameters := map[string]interface{}{
						"file": file,
					}
					announcement, queueErr := announcementManager.QueueAnnouncement(TypePromo, PriorityLow, parameters, time.Now())
					if queueErr != nil {
						log.Printf("Error queuing scheduled promo announcement: %v", queueErr)
					} else {
						log.Printf("Scheduled promo announcement queued successfully (ID: %s)", announcement.ID)
					}
				} else {
					log.Printf("⚠️  Announcement manager not available for scheduled announcement")
				}
			})
			if err != nil {
				log.Printf("Error scheduling promo announcement %d: %v", i, err)
			} else {
				log.Printf("Scheduled: %s - %s", item.Cron, item.File)
			}
		}
	}

	// Safety announcements
	for i, item := range cronData.SafetyAnnouncements {
		if item.Enabled {
			// Capture variables for closure
			language := item.Language
			_, err := app.Scheduler.AddFunc(item.Cron, func() {
				log.Printf("🕐 Scheduled safety announcement triggered: %s", language)
				if announcementManager != nil {
					parameters := map[string]interface{}{
						"language": language,
					}
					announcement, queueErr := announcementManager.QueueAnnouncement(TypeSafety, PriorityHigh, parameters, time.Now())
					if queueErr != nil {
						log.Printf("Error queuing scheduled safety announcement: %v", queueErr)
					} else {
						log.Printf("Scheduled safety announcement queued successfully (ID: %s)", announcement.ID)
					}
				} else {
					log.Printf("⚠️  Announcement manager not available for scheduled announcement")
				}
			})
			if err != nil {
				log.Printf("Error scheduling safety announcement %d: %v", i, err)
			} else {
				log.Printf("Scheduled: %s - %s", item.Cron, item.Language)
			}
		}
	}

	log.Printf("Scheduler updated with %d active jobs.", len(app.Scheduler.Entries()))
}

// File system utilities
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// Cron validation function
func validateCronExpression(cronExpr string) error {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return fmt.Errorf("cron expression must have exactly 5 fields")
	}
	
	// Try to parse with cron library
	_, err := cron.ParseStandard(cronExpr)
	return err
}