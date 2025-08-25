package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

type Config struct {
	AdminUsername     string
	AdminPassword     string
	APIKey            string
	APIEnabled        bool
	BaseDir           string
	JSONDir           string
	MP3Dir            string
	CurrentVolume     float64
	SelectedAudioDevice string
}

type Train struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Direction struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Destination struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Track struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PromoAnnouncement struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SafetyLanguage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Emergency struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

type CronData struct {
	StationAnnouncements []StationCronJob `json:"station_announcements"`
	PromoAnnouncements   []PromoCronJob   `json:"promo_announcements"`
	SafetyAnnouncements  []SafetyCronJob  `json:"safety_announcements"`
}

type StationCronJob struct {
	Enabled      bool   `json:"enabled"`
	Cron         string `json:"cron"`
	TrainNumber  string `json:"train_number"`
	Direction    string `json:"direction"`
	Destination  string `json:"destination"`
	TrackNumber  string `json:"track_number"`
}

type PromoCronJob struct {
	Enabled bool   `json:"enabled"`
	Cron    string `json:"cron"`
	File    string `json:"file"`
}

type SafetyCronJob struct {
	Enabled  bool   `json:"enabled"`
	Cron     string `json:"cron"`
	Language string `json:"language"`
}

type App struct {
	Config       *Config
	Router       *gin.Engine
	Scheduler    *cron.Cron
	AudioEnabled bool
}

var app *App

func main() {
	fmt.Println("Starting TARR Annunciator...")
	app = &App{
		Config: &Config{
			AdminUsername: "admin",
			AdminPassword: "tarr2025",
			APIKey:        "tarr-api-2025",
			APIEnabled:    true,
			CurrentVolume: 0.7,
			SelectedAudioDevice: "default",
		},
		Scheduler:    cron.New(),
		AudioEnabled: true,
	}

	// Initialize paths
	baseDir, _ := os.Getwd()
	app.Config.BaseDir = baseDir
	app.Config.JSONDir = filepath.Join(baseDir, "json")
	app.Config.MP3Dir = filepath.Join(baseDir, "static", "mp3")

	// Initialize audio
	if err := initAudio(); err != nil {
		log.Printf("Audio initialization failed: %v", err)
		app.AudioEnabled = false
	} else {
		log.Println("✓ Audio system initialized successfully")
	}

	// Initialize announcement queue system
	InitializeAnnouncementManager()
	log.Println("✓ Announcement queue system initialized")

	// Setup router
	setupRouter()

	// Start scheduler
	app.Scheduler.Start()
	defer app.Scheduler.Stop()
	updateScheduler()

	// Start server
	log.Println("Starting TARR Annunciator Go Server...")
	log.Printf("Audio system: %s", audioStatus())
	log.Println("Access the application at: http://localhost:8080")
	log.Println("Admin interface at: http://localhost:8080/admin")

	app.Router.Run(":8080")
}

func initAudio() error {
	sr := beep.SampleRate(44100)
	return speaker.Init(sr, sr.N(time.Second/10))
}

func audioStatus() string {
	if app.AudioEnabled {
		return "Available"
	}
	return "Not Available"
}

func setupRouter() {
	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	app.Router = gin.Default()

	// Session store
	store := cookie.NewStore([]byte("2932d8c03fb85143293c803ff3f7f1c27923787e520ce335"))
	app.Router.Use(sessions.Sessions("session", store))

	// Add template functions
	app.Router.SetFuncMap(map[string]interface{}{
		"mul": func(a, b float64) float64 {
			return a * b
		},
	})
	
	// Load HTML templates
	app.Router.LoadHTMLGlob("templates/*")
	app.Router.Static("/static", "./static")

	// Routes
	setupWebRoutes()
	setupAPIRoutes()
}

func setupWebRoutes() {
	app.Router.GET("/", indexHandler)
	app.Router.POST("/play_announcement", playAnnouncementHandler)
	app.Router.POST("/play_promo", playPromoHandler)
	app.Router.POST("/play_safety_announcement", playSafetyHandler)
	app.Router.GET("/scheduler_status", schedulerStatusHandler)
	app.Router.GET("/audio_status", audioStatusHandler)

	// Admin routes
	app.Router.GET("/admin/login", adminLoginGetHandler)
	app.Router.POST("/admin/login", adminLoginPostHandler)
	app.Router.GET("/admin/logout", adminLogoutHandler)
	app.Router.GET("/admin", requireAuth(), adminHandler)
	app.Router.POST("/admin", requireAuth(), adminPostHandler)

	// Audio control routes (admin only)
	app.Router.GET("/audio/devices", requireAuth(), getAudioDevicesHandler)
	app.Router.POST("/audio/devices", requireAuth(), setAudioDeviceHandler)
	app.Router.POST("/audio/volume", requireAuth(), setVolumeHandler)
	app.Router.POST("/audio/test", requireAuth(), testAudioHandler)
}

func setupAPIRoutes() {
	api := app.Router.Group("/api")

	// Public endpoints
	api.GET("/status", apiStatusHandler)
	api.GET("/platform", apiPlatformInfoHandler)
	api.GET("/docs", apiDocsHandler)

	// Authenticated endpoints
	authAPI := api.Group("", requireAPIKey())
	{
		authAPI.POST("/announce/station", apiStationAnnouncementHandler)
		authAPI.POST("/announce/safety", apiSafetyAnnouncementHandler)
		authAPI.POST("/announce/promo", apiPromoAnnouncementHandler)
		authAPI.POST("/announce/emergency", apiEmergencyAnnouncementHandler)
		authAPI.GET("/audio/volume", apiGetVolumeHandler)
		authAPI.POST("/audio/volume", apiSetVolumeHandler)
		authAPI.GET("/audio/devices", apiGetAudioDevicesHandler)
		authAPI.POST("/audio/devices", apiSetAudioDeviceHandler)
		authAPI.GET("/config", apiGetConfigHandler)
		authAPI.GET("/schedule", apiGetScheduleHandler)
		authAPI.POST("/schedule", apiPostScheduleHandler)
		authAPI.GET("/queue/status", apiGetQueueStatusHandler)
		authAPI.GET("/queue/history", apiGetQueueHistoryHandler)
		authAPI.POST("/queue/cancel", apiCancelAnnouncementHandler)
	}
}

// Middleware
func requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		loggedIn := session.Get("admin_logged_in")
		if loggedIn == nil || !loggedIn.(bool) {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func requireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !app.Config.APIEnabled {
			c.JSON(503, gin.H{"error": "API is disabled"})
			c.Abort()
			return
		}

		// Check for API key in headers or query params
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}
		if apiKey == "" {
			apiKey = c.PostForm("api_key")
		}

		if apiKey == "" {
			c.JSON(401, gin.H{"error": "API key required. Use X-API-Key header or api_key parameter."})
			c.Abort()
			return
		}

		if apiKey != app.Config.APIKey {
			c.JSON(401, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Handlers
func indexHandler(c *gin.Context) {
	trains := loadJSON("trains", []Train{}).([]Train)
	directions := loadJSON("directions", []Direction{}).([]Direction)
	destinations := loadJSON("destinations", []Destination{}).([]Destination)
	tracks := loadJSON("tracks", []Track{}).([]Track)
	promoAnnouncements := loadJSON("promo", []PromoAnnouncement{}).([]PromoAnnouncement)
	safetyLanguages := loadJSON("safety", []SafetyLanguage{}).([]SafetyLanguage)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"trains":               trains,
		"directions":           directions,
		"destinations":         destinations,
		"tracks":               tracks,
		"promo_announcements":  promoAnnouncements,
		"safety_languages":     safetyLanguages,
	})
}

func playAnnouncementHandler(c *gin.Context) {
	trainNumber := c.PostForm("train_number")
	direction := c.PostForm("direction")
	destination := c.PostForm("destination")
	trackNumber := c.PostForm("track_number")

	// Queue the announcement through the proper queue system
	parameters := map[string]interface{}{
		"train_number": trainNumber,
		"direction":    direction,
		"destination":  destination,
		"track_number": trackNumber,
	}
	
	if announcementManager != nil {
		announcement, err := announcementManager.QueueAnnouncement(TypeStation, PriorityNormal, parameters, time.Now())
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to queue station announcement: "+err.Error())
			return
		}
		c.String(http.StatusOK, fmt.Sprintf("Station announcement queued successfully (ID: %s)", announcement.ID))
	} else {
		c.String(http.StatusInternalServerError, "Announcement system not available")
	}
}

func playPromoHandler(c *gin.Context) {
	file := c.PostForm("file")
	
	// Queue the announcement through the proper queue system
	parameters := map[string]interface{}{
		"file": file,
	}
	
	if announcementManager != nil {
		announcement, err := announcementManager.QueueAnnouncement(TypePromo, PriorityLow, parameters, time.Now())
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to queue promo announcement: "+err.Error())
			return
		}
		c.String(http.StatusOK, fmt.Sprintf("Promo announcement queued successfully (ID: %s)", announcement.ID))
	} else {
		c.String(http.StatusInternalServerError, "Announcement system not available")
	}
}

func playSafetyHandler(c *gin.Context) {
	language := c.PostForm("language")
	
	// Queue the announcement through the proper queue system
	parameters := map[string]interface{}{
		"language": language,
	}
	
	if announcementManager != nil {
		announcement, err := announcementManager.QueueAnnouncement(TypeSafety, PriorityHigh, parameters, time.Now())
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to queue safety announcement: "+err.Error())
			return
		}
		c.String(http.StatusOK, fmt.Sprintf("Safety announcement in %s queued successfully (ID: %s)", language, announcement.ID))
	} else {
		c.String(http.StatusInternalServerError, "Announcement system not available")
	}
}

func schedulerStatusHandler(c *gin.Context) {
	jobs := make([]gin.H, 0)
	for _, entry := range app.Scheduler.Entries() {
		jobs = append(jobs, gin.H{
			"next_run": entry.Next.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"scheduler_running": true,
		"jobs":              jobs,
		"audio_available":   app.AudioEnabled,
	})
}

func audioStatusHandler(c *gin.Context) {
	chimePath := filepath.Join(app.Config.MP3Dir, "chime.mp3")
	chimeExists := fileExists(chimePath)
	mp3DirExists := dirExists(app.Config.MP3Dir)

	c.JSON(http.StatusOK, gin.H{
		"audio_available":        app.AudioEnabled,
		"audio_backend":          "beep",
		"current_volume":         app.Config.CurrentVolume,
		"volume_percent":         int(app.Config.CurrentVolume * 100),
		"chime_exists":          chimeExists,
		"mp3_directory_exists":  mp3DirExists,
	})
}

// Admin handlers
func adminLoginGetHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "admin_login.html", nil)
}

func adminLoginPostHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == app.Config.AdminUsername && password == app.Config.AdminPassword {
		session := sessions.Default(c)
		session.Set("admin_logged_in", true)
		session.Save()
		c.Redirect(http.StatusFound, "/admin")
	} else {
		c.HTML(http.StatusOK, "admin_login.html", gin.H{
			"error": "Invalid username or password!",
		})
	}
}

func adminLogoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete("admin_logged_in")
	session.Save()
	c.Redirect(http.StatusFound, "/")
}

func adminHandler(c *gin.Context) {
	cronData := loadJSON("cron", CronData{}).(CronData)
	cronDataJSON, _ := json.MarshalIndent(cronData, "", "    ")
	
	trains := loadJSON("trains", []Train{}).([]Train)
	directions := loadJSON("directions", []Direction{}).([]Direction)
	destinations := loadJSON("destinations", []Destination{}).([]Destination)
	tracks := loadJSON("tracks", []Track{}).([]Track)
	promoAnnouncements := loadJSON("promo", []PromoAnnouncement{}).([]PromoAnnouncement)
	safetyLanguages := loadJSON("safety", []SafetyLanguage{}).([]SafetyLanguage)
	// DEBUG: Check before loading emergencies
	log.Printf("DEBUG: About to load emergencies JSON...")
	emergencies := loadJSON("emergencies", []Emergency{}).([]Emergency)
	log.Printf("DEBUG: loadJSON returned, type assertion complete")
	audioDevices := getAudioDevices()

	// DEBUG: Log emergencies data
	log.Printf("DEBUG: Admin handler - loaded %d emergencies", len(emergencies))
	for i, emergency := range emergencies {
		log.Printf("DEBUG: Emergency %d: %s (%s)", i+1, emergency.Name, emergency.ID)
	}

	c.HTML(http.StatusOK, "admin.html", gin.H{
		"cron_data":            string(cronDataJSON),
		"trains":               trains,
		"directions":           directions,
		"destinations":         destinations,
		"tracks":               tracks,
		"promo_announcements":  promoAnnouncements,
		"safety_languages":     safetyLanguages,
		"emergencies":          emergencies,
		"current_volume":       app.Config.CurrentVolume,
		"audio_devices":        audioDevices,
		"selected_audio_device": app.Config.SelectedAudioDevice,
	})
}

func adminPostHandler(c *gin.Context) {
	cronJSON := c.PostForm("cron_json")
	var cronData CronData

	if err := json.Unmarshal([]byte(cronJSON), &cronData); err != nil {
		cronDataDisplay := loadJSON("cron", CronData{}).(CronData)
		cronDataJSON, _ := json.MarshalIndent(cronDataDisplay, "", "    ")
		
		c.HTML(http.StatusBadRequest, "admin.html", gin.H{
			"error": fmt.Sprintf("Error parsing schedule: %v", err),
			"cron_data": string(cronDataJSON),
		})
		return
	}

	if err := saveJSON("cron", cronData); err != nil {
		cronDataJSON, _ := json.MarshalIndent(cronData, "", "    ")
		
		c.HTML(http.StatusInternalServerError, "admin.html", gin.H{
			"error": fmt.Sprintf("Error saving schedule: %v", err),
			"cron_data": string(cronDataJSON),
		})
		return
	}

	updateScheduler()
	c.Redirect(http.StatusFound, "/admin")
}

// Audio device handlers
func getAudioDevicesHandler(c *gin.Context) {
	devices := getAudioDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"current_device": app.Config.SelectedAudioDevice,
	})
}

func setAudioDeviceHandler(c *gin.Context) {
	deviceID := c.PostForm("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Device ID required"})
		return
	}

	// Validate device exists
	devices := getAudioDevices()
	validDevice := false
	var selectedDevice AudioDevice
	for _, device := range devices {
		if device.ID == deviceID {
			validDevice = true
			selectedDevice = device
			break
		}
	}

	if !validDevice {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid device ID"})
		return
	}

	// Set the device
	if err := setAudioDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to set audio device: " + err.Error()})
		return
	}

	app.Config.SelectedAudioDevice = deviceID

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"device": selectedDevice,
		"message": "Audio device set successfully",
	})
}

func setVolumeHandler(c *gin.Context) {
	volumeStr := c.PostForm("volume")
	volume, err := strconv.ParseFloat(volumeStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid volume value"})
		return
	}

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

func testAudioHandler(c *gin.Context) {
	chimePath := filepath.Join(app.Config.MP3Dir, "chime.mp3")
	if !fileExists(chimePath) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Test audio file not found"})
		return
	}

	if err := playAudio(chimePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Audio test failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Audio test played successfully"})
}