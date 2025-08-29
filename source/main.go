package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

type Config struct {
	AdminUsername       string
	AdminPassword       string
	APIKey              string
	APIEnabled          bool
	BaseDir             string
	JSONDir             string
	MP3Dir              string
	LogDir              string
	CurrentVolume       float64
	SelectedAudioDevice string
	SessionSecret       string
}

type AdminUser struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Role        string   `json:"role"`
	Enabled     bool     `json:"enabled"`
	CreatedAt   string   `json:"created_at"`
	LastLogin   string   `json:"last_login"`
	Permissions []string `json:"permissions"`
}

type APIKey struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Enabled     bool     `json:"enabled"`
	Permanent   bool     `json:"permanent"`
	ExpiresAt   string   `json:"expires_at"`
	CreatedAt   string   `json:"created_at"`
	CreatedBy   string   `json:"created_by"`
	LastUsed    string   `json:"last_used"`
	Permissions []string `json:"permissions"`
	RateLimit   struct {
		RequestsPerHour int  `json:"requests_per_hour"`
		Enabled         bool `json:"enabled"`
	} `json:"rate_limit"`
}

type AdminConfig struct {
	AdminUsers []AdminUser `json:"admin_users"`
	APIKeys    []APIKey    `json:"api_keys"`
	Security   struct {
		SessionTimeoutMinutes  int    `json:"session_timeout_minutes"`
		RequireAdminLogin      bool   `json:"require_admin_login"`
		ShowDefaultCredentials bool   `json:"show_default_credentials"`
		SessionSecret          string `json:"session_secret"`
		PasswordPolicy         struct {
			MinLength           int  `json:"min_length"`
			RequireSpecialChars bool `json:"require_special_chars"`
			RequireNumbers      bool `json:"require_numbers"`
		} `json:"password_policy"`
		FailedLoginAttempts struct {
			MaxAttempts            int  `json:"max_attempts"`
			LockoutDurationMinutes int  `json:"lockout_duration_minutes"`
			Enabled                bool `json:"enabled"`
		} `json:"failed_login_attempts"`
	} `json:"security"`
	Metadata struct {
		CreatedAt     string `json:"created_at"`
		LastModified  string `json:"last_modified"`
		Version       string `json:"version"`
		SchemaVersion string `json:"schema_version"`
	} `json:"metadata"`
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
	Enabled   bool     `json:"enabled"`
	Cron      string   `json:"cron"`
	Language  string   `json:"language"`           // Legacy single language support
	Languages []string `json:"languages,omitempty"` // New multi-language support
	Delay     int      `json:"delay,omitempty"`     // Optional delay between languages in seconds (default: 2)
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
	
	// Initialize paths first
	baseDir, _ := os.Getwd()
	jsonDir := filepath.Join(baseDir, "json")
	mp3Dir := filepath.Join(baseDir, "static", "mp3")
	logDir := filepath.Join(baseDir, "logs")
	
	// Initialize logging system
	if err := initializeLogging(logDir); err != nil {
		log.Printf("Warning: Failed to initialize file logging: %v", err)
	}

	// Load admin configuration
	adminConfig, err := loadAdminConfig(filepath.Join(jsonDir, "admin_config.json"))
	if err != nil {
		log.Printf("Warning: Could not load admin config, using defaults: %v", err)
		adminConfig = getDefaultAdminConfig()
	}

	// Get first admin user for backward compatibility
	firstAdmin := getFirstAdminUser(adminConfig)
	firstAPIKey := getFirstAPIKey(adminConfig)

	app = &App{
		Config: &Config{
			AdminUsername:       firstAdmin.Username,
			AdminPassword:       firstAdmin.Password,
			APIKey:              firstAPIKey.Key,
			APIEnabled:          len(adminConfig.APIKeys) > 0 && firstAPIKey.Enabled,
			CurrentVolume:       0.7,
			SelectedAudioDevice: "default",
			SessionSecret:       adminConfig.Security.SessionSecret,
			BaseDir:             baseDir,
			JSONDir:             jsonDir,
			MP3Dir:              mp3Dir,
			LogDir:              logDir,
		},
		Scheduler:    cron.New(),
		AudioEnabled: true,
	}

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
	setupRouter(adminConfig)

	// Start scheduler
	app.Scheduler.Start()
	defer app.Scheduler.Stop()
	updateScheduler()

	// Start server
	log.Println("Starting TARR Annunciator Go Server...")
	log.Printf("Audio system: %s", audioStatus())
	log.Println("Access the application at: http://localhost:8080")
	log.Println("Admin interface at: http://localhost:8080/admin")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, cleaning up...")
		
		// Stop scheduler
		if app.Scheduler != nil {
			app.Scheduler.Stop()
			log.Println("Scheduler stopped")
		}
		
		// Close logging
		closeLogging()
		
		os.Exit(0)
	}()

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

func setupRouter(adminConfig *AdminConfig) {
	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	app.Router = gin.Default()

	// Session store - use session secret from admin config
	sessionSecret := adminConfig.Security.SessionSecret
	if sessionSecret == "" {
		sessionSecret = "2932d8c03fb85143293c803ff3f7f1c27923787e520ce335"
	}
	store := cookie.NewStore([]byte(sessionSecret))
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
	
	// Credential management routes (admin only)
	app.Router.GET("/admin/credentials", requireAuth(), getCredentialsHandler)
	app.Router.POST("/admin/credentials", requireAuth(), updateCredentialsHandler)
	
	// User management routes (admin only)
	app.Router.POST("/admin/users", requireAuth(), createUserHandler)
	app.Router.PUT("/admin/users/:id", requireAuth(), updateUserHandler)
	app.Router.DELETE("/admin/users/:id", requireAuth(), deleteUserHandler)
	
	// API Key management routes (admin only)
	app.Router.POST("/admin/api-keys", requireAuth(), createAPIKeyHandler)
	app.Router.PUT("/admin/api-keys/:id", requireAuth(), updateAPIKeyHandler)
	app.Router.DELETE("/admin/api-keys/:id", requireAuth(), deleteAPIKeyHandler)
	
	// Track Layout Routes (Authenticated)
	app.Router.GET("/admin/track-layout", requireAuth(), getTrackLayoutHandler)
	app.Router.POST("/admin/track-layout", requireAuth(), postTrackLayoutHandler)
	
	// System Control Routes (Authenticated)
	app.Router.GET("/admin/system/info", requireAuth(), getSystemInfoHandler)
	app.Router.POST("/admin/system/restart", requireAuth(), restartApplicationHandler)
	app.Router.POST("/admin/system/shutdown", requireAuth(), shutdownApplicationHandler)
	
	// Audio Management Routes (Authenticated)
	app.Router.POST("/admin/audio/redetect", requireAuth(), redetectAudioDevicesHandler)
	app.Router.POST("/admin/audio/system-override", requireAuth(), audioSystemOverrideHandler)
	app.Router.GET("/admin/system/platform-info", requireAuth(), getPlatformInfoHandler)
	
	// Bluetooth Management Routes (Authenticated)
	app.Router.POST("/admin/bluetooth/scan", requireAuth(), startBluetoothScanHandler)
	app.Router.POST("/admin/bluetooth/scan/stop", requireAuth(), stopBluetoothScanHandler)
	app.Router.GET("/admin/bluetooth/devices", requireAuth(), getBluetoothDevicesHandler)
	app.Router.GET("/admin/bluetooth/paired", requireAuth(), getPairedBluetoothDevicesHandler)
	app.Router.POST("/admin/bluetooth/pair", requireAuth(), pairBluetoothDeviceHandler)
	app.Router.POST("/admin/bluetooth/unpair", requireAuth(), unpairBluetoothDeviceHandler)
	
	// Queue management routes (admin only) - session authenticated versions
	app.Router.GET("/api/queue/status", requireAuth(), apiGetQueueStatusHandler)
	app.Router.GET("/api/queue/history", requireAuth(), apiGetQueueHistoryHandler)
	app.Router.POST("/api/queue/cancel", requireAuth(), apiCancelAnnouncementHandler)
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
		authAPI.POST("/announcements/pause", apiPauseAnnouncementsHandler)
		authAPI.POST("/announcements/resume", apiResumeAnnouncementsHandler)
		authAPI.POST("/announcements/stop-current", apiStopCurrentAnnouncementHandler)
		authAPI.GET("/audio/volume", apiGetVolumeHandler)
		authAPI.POST("/audio/volume", apiSetVolumeHandler)
		authAPI.GET("/audio/devices", apiGetAudioDevicesHandler)
		authAPI.POST("/audio/devices", apiSetAudioDeviceHandler)
		authAPI.GET("/config", apiGetConfigHandler)
		authAPI.GET("/schedule", apiGetScheduleHandler)
		authAPI.POST("/schedule", apiPostScheduleHandler)
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

		// Load admin config to check against multiple API keys
		configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
		adminConfig, err := loadAdminConfig(configPath)
		if err != nil {
			// Fall back to single API key check
			if apiKey != app.Config.APIKey {
				c.JSON(401, gin.H{"error": "Invalid API key"})
				c.Abort()
				return
			}
		} else {
			// Check against multi-API key system
			apiKeyData := findAPIKeyByKey(adminConfig, apiKey)
			if apiKeyData == nil {
				c.JSON(401, gin.H{"error": "Invalid API key"})
				c.Abort()
				return
			}
			
			// Update last used time
			apiKeyData.LastUsed = time.Now().Format(time.RFC3339)
			saveAdminConfig(configPath, adminConfig)
			
			// Store API key info in context for permission checks
			c.Set("api_key_data", apiKeyData)
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

	// Load admin config to verify credentials against multi-user system
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		// Fall back to single user check if config load fails
		if username == app.Config.AdminUsername && password == app.Config.AdminPassword {
			session := sessions.Default(c)
			session.Set("admin_logged_in", true)
			session.Set("admin_user_id", "admin-001")
			session.Save()
			c.Redirect(http.StatusFound, "/admin")
			return
		}
	} else {
		// Check against multi-user system
		user := findUserByUsername(adminConfig, username)
		if user != nil && user.Password == password {
			// Update last login time
			user.LastLogin = time.Now().Format(time.RFC3339)
			saveAdminConfig(configPath, adminConfig)
			
			session := sessions.Default(c)
			session.Set("admin_logged_in", true)
			session.Set("admin_user_id", user.ID)
			session.Save()
			c.Redirect(http.StatusFound, "/admin")
			return
		}
	}

	c.HTML(http.StatusOK, "admin_login.html", gin.H{
		"error": "Invalid username or password!",
	})
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
	trainsAvailable := loadJSON("trains_available", []Train{}).([]Train)
	directions := loadJSON("directions", []Direction{}).([]Direction)
	destinations := loadJSON("destinations", []Destination{}).([]Destination)
	destinationsAvailable := loadJSON("destinations_available", []Destination{}).([]Destination)
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
		"cron_data":              string(cronDataJSON),
		"trains":                 trains,
		"trains_available":       trainsAvailable,
		"directions":             directions,
		"destinations":           destinations,
		"destinations_available": destinationsAvailable,
		"tracks":                 tracks,
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

// Admin configuration management functions
func loadAdminConfig(configPath string) (*AdminConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config AdminConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func saveAdminConfig(configPath string, config *AdminConfig) error {
	config.Metadata.LastModified = time.Now().Format(time.RFC3339)
	
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600) // Restrict permissions for security
}

func getDefaultAdminConfig() *AdminConfig {
	config := &AdminConfig{}
	
	// Create default admin user
	defaultUser := AdminUser{
		ID:          "admin-001",
		Username:    "admin", 
		Password:    "tarr2025",
		Role:        "admin",
		Enabled:     true,
		CreatedAt:   time.Now().Format(time.RFC3339),
		LastLogin:   "",
		Permissions: []string{"system_config", "user_management", "api_management", "audio_control", "announcements"},
	}
	config.AdminUsers = []AdminUser{defaultUser}
	
	// Create default API key
	defaultAPIKey := APIKey{
		ID:          "api-001",
		Name:        "Default API Key",
		Key:         "tarr-api-2025", 
		Enabled:     true,
		Permanent:   false,
		ExpiresAt:   "",
		CreatedAt:   time.Now().Format(time.RFC3339),
		CreatedBy:   "admin-001",
		LastUsed:    "",
		Permissions: []string{"announce", "status", "config"},
	}
	defaultAPIKey.RateLimit.RequestsPerHour = 1000
	defaultAPIKey.RateLimit.Enabled = false
	config.APIKeys = []APIKey{defaultAPIKey}
	
	// Security settings
	config.Security.SessionTimeoutMinutes = 60
	config.Security.RequireAdminLogin = true
	config.Security.ShowDefaultCredentials = false
	config.Security.SessionSecret = "tarr-session-secret-change-this"
	config.Security.PasswordPolicy.MinLength = 8
	config.Security.PasswordPolicy.RequireSpecialChars = true
	config.Security.PasswordPolicy.RequireNumbers = true
	config.Security.FailedLoginAttempts.MaxAttempts = 5
	config.Security.FailedLoginAttempts.LockoutDurationMinutes = 15
	config.Security.FailedLoginAttempts.Enabled = true
	
	// Metadata
	config.Metadata.CreatedAt = time.Now().Format(time.RFC3339)
	config.Metadata.LastModified = time.Now().Format(time.RFC3339)
	config.Metadata.Version = "2.0"
	config.Metadata.SchemaVersion = "multi-user"
	
	return config
}

func getFirstAdminUser(config *AdminConfig) AdminUser {
	if len(config.AdminUsers) > 0 {
		return config.AdminUsers[0]
	}
	// Return default if no users
	return AdminUser{
		Username: "admin",
		Password: "tarr2025",
		Role:     "admin",
		Enabled:  true,
	}
}

func getFirstAPIKey(config *AdminConfig) APIKey {
	if len(config.APIKeys) > 0 {
		return config.APIKeys[0]
	}
	// Return default if no API keys
	return APIKey{
		Key:     "tarr-api-2025",
		Enabled: true,
	}
}

func findUserByUsername(config *AdminConfig, username string) *AdminUser {
	for i, user := range config.AdminUsers {
		if user.Username == username && user.Enabled {
			return &config.AdminUsers[i]
		}
	}
	return nil
}

func findAPIKeyByKey(config *AdminConfig, apiKey string) *APIKey {
	for i, key := range config.APIKeys {
		if key.Key == apiKey && key.Enabled {
			return &config.APIKeys[i]
		}
	}
	return nil
}

func hasPermission(user *AdminUser, permission string) bool {
	for _, perm := range user.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

func hasAPIPermission(apiKey *APIKey, permission string) bool {
	for _, perm := range apiKey.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// Credential management API endpoints
func getCredentialsHandler(c *gin.Context) {
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	// Prepare safe user data (no passwords)
	safeUsers := make([]gin.H, len(adminConfig.AdminUsers))
	for i, user := range adminConfig.AdminUsers {
		safeUsers[i] = gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"role":        user.Role,
			"enabled":     user.Enabled,
			"created_at":  user.CreatedAt,
			"last_login":  user.LastLogin,
			"permissions": user.Permissions,
		}
	}

	// Prepare safe API key data (with keys for frontend display)
	safeAPIKeys := make([]gin.H, len(adminConfig.APIKeys))
	for i, key := range adminConfig.APIKeys {
		safeAPIKeys[i] = gin.H{
			"id":         key.ID,
			"name":       key.Name,
			"key":        key.Key, // Include key for frontend masking
			"enabled":    key.Enabled,
			"permanent":  key.Permanent,
			"expires_at": key.ExpiresAt,
			"created_at": key.CreatedAt,
			"created_by": key.CreatedBy,
			"last_used":  key.LastUsed,
			"permissions": key.Permissions,
			"rate_limit": key.RateLimit,
		}
	}

	// Return safe data
	c.JSON(http.StatusOK, gin.H{
		"admin_users":          safeUsers,
		"api_keys":             safeAPIKeys,
		"session_timeout":      adminConfig.Security.SessionTimeoutMinutes,
		"require_admin_login":  adminConfig.Security.RequireAdminLogin,
		"password_policy":      adminConfig.Security.PasswordPolicy,
		"failed_login_attempts": adminConfig.Security.FailedLoginAttempts,
		"last_modified":        adminConfig.Metadata.LastModified,
		"schema_version":       adminConfig.Metadata.SchemaVersion,
	})
}

func updateCredentialsHandler(c *gin.Context) {
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	var updateData struct {
		SessionTimeout *int `json:"session_timeout,omitempty"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Update global settings
	if updateData.SessionTimeout != nil {
		adminConfig.Security.SessionTimeoutMinutes = *updateData.SessionTimeout
	}

	// Save updated config
	if err := saveAdminConfig(configPath, adminConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save admin config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Settings updated successfully",
	})
}

// User management handlers
func createUserHandler(c *gin.Context) {
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	var newUser AdminUser
	if err := c.ShouldBindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user data"})
		return
	}

	// Generate unique ID if not provided
	if newUser.ID == "" {
		newUser.ID = fmt.Sprintf("admin-%03d", len(adminConfig.AdminUsers)+1)
	}

	// Check if username already exists
	for _, user := range adminConfig.AdminUsers {
		if user.Username == newUser.Username {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
	}

	// Set defaults
	if newUser.Role == "" {
		newUser.Role = "admin"
	}
	if newUser.Permissions == nil {
		newUser.Permissions = []string{"announcements"}
	}
	newUser.CreatedAt = time.Now().Format(time.RFC3339)
	newUser.Enabled = true

	// Add user to config
	adminConfig.AdminUsers = append(adminConfig.AdminUsers, newUser)

	// Save config
	if err := saveAdminConfig(configPath, adminConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save admin config"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User created successfully",
		"user_id": newUser.ID,
	})
}

func updateUserHandler(c *gin.Context) {
	userID := c.Param("id")
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	// Find user
	userIndex := -1
	for i, user := range adminConfig.AdminUsers {
		if user.ID == userID {
			userIndex = i
			break
		}
	}

	if userIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var updateData AdminUser
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user data"})
		return
	}

	// Update user fields
	user := &adminConfig.AdminUsers[userIndex]
	if updateData.Username != "" {
		// Check if new username already exists (excluding current user)
		for i, existingUser := range adminConfig.AdminUsers {
			if i != userIndex && existingUser.Username == updateData.Username {
				c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
				return
			}
		}
		user.Username = updateData.Username
	}
	if updateData.Password != "" {
		user.Password = updateData.Password
	}
	if updateData.Role != "" {
		user.Role = updateData.Role
	}
	if updateData.Permissions != nil {
		user.Permissions = updateData.Permissions
	}
	user.Enabled = updateData.Enabled

	// Save config
	if err := saveAdminConfig(configPath, adminConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save admin config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully",
	})
}

func deleteUserHandler(c *gin.Context) {
	userID := c.Param("id")
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	// Find user
	userIndex := -1
	for i, user := range adminConfig.AdminUsers {
		if user.ID == userID {
			userIndex = i
			break
		}
	}

	if userIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Don't allow deleting the last admin user
	if len(adminConfig.AdminUsers) <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last admin user"})
		return
	}

	// Remove user
	adminConfig.AdminUsers = append(adminConfig.AdminUsers[:userIndex], adminConfig.AdminUsers[userIndex+1:]...)

	// Save config
	if err := saveAdminConfig(configPath, adminConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save admin config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// API Key management handlers
func createAPIKeyHandler(c *gin.Context) {
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	var newAPIKey APIKey
	if err := c.ShouldBindJSON(&newAPIKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key data"})
		return
	}

	// Generate unique ID if not provided
	if newAPIKey.ID == "" {
		newAPIKey.ID = fmt.Sprintf("api-%03d", len(adminConfig.APIKeys)+1)
	}

	// Check if key already exists
	for _, key := range adminConfig.APIKeys {
		if key.Key == newAPIKey.Key {
			c.JSON(http.StatusConflict, gin.H{"error": "API key already exists"})
			return
		}
	}

	// Set defaults
	if newAPIKey.Name == "" {
		newAPIKey.Name = "New API Key"
	}
	if newAPIKey.Permissions == nil {
		newAPIKey.Permissions = []string{"announce", "status"}
	}
	newAPIKey.CreatedAt = time.Now().Format(time.RFC3339)
	newAPIKey.Enabled = true

	// Get current user ID from session
	session := sessions.Default(c)
	createdBy := session.Get("admin_user_id")
	if createdBy != nil {
		newAPIKey.CreatedBy = createdBy.(string)
	}

	// Set rate limit defaults
	if newAPIKey.RateLimit.RequestsPerHour == 0 {
		newAPIKey.RateLimit.RequestsPerHour = 1000
	}

	// Add API key to config
	adminConfig.APIKeys = append(adminConfig.APIKeys, newAPIKey)

	// Save config
	if err := saveAdminConfig(configPath, adminConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save admin config"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "API key created successfully",
		"api_key_id": newAPIKey.ID,
	})
}

func updateAPIKeyHandler(c *gin.Context) {
	keyID := c.Param("id")
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	// Find API key
	keyIndex := -1
	for i, key := range adminConfig.APIKeys {
		if key.ID == keyID {
			keyIndex = i
			break
		}
	}

	if keyIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	var updateData APIKey
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key data"})
		return
	}

	// Update API key fields
	key := &adminConfig.APIKeys[keyIndex]
	if updateData.Name != "" {
		key.Name = updateData.Name
	}
	if updateData.Key != "" {
		// Check if new key already exists (excluding current key)
		for i, existingKey := range adminConfig.APIKeys {
			if i != keyIndex && existingKey.Key == updateData.Key {
				c.JSON(http.StatusConflict, gin.H{"error": "API key already exists"})
				return
			}
		}
		key.Key = updateData.Key
	}
	if updateData.Permissions != nil {
		key.Permissions = updateData.Permissions
	}
	if updateData.ExpiresAt != "" {
		key.ExpiresAt = updateData.ExpiresAt
	}
	key.Enabled = updateData.Enabled
	key.Permanent = updateData.Permanent

	// Update rate limiting
	if updateData.RateLimit.RequestsPerHour > 0 {
		key.RateLimit.RequestsPerHour = updateData.RateLimit.RequestsPerHour
	}
	key.RateLimit.Enabled = updateData.RateLimit.Enabled

	// Save config
	if err := saveAdminConfig(configPath, adminConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save admin config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key updated successfully",
	})
}

func deleteAPIKeyHandler(c *gin.Context) {
	keyID := c.Param("id")
	configPath := filepath.Join(app.Config.JSONDir, "admin_config.json")
	adminConfig, err := loadAdminConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load admin config"})
		return
	}

	// Find API key
	keyIndex := -1
	for i, key := range adminConfig.APIKeys {
		if key.ID == keyID {
			keyIndex = i
			break
		}
	}

	if keyIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	// Check if it's a permanent key
	if adminConfig.APIKeys[keyIndex].Permanent {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete permanent API key"})
		return
	}

	// Remove API key
	adminConfig.APIKeys = append(adminConfig.APIKeys[:keyIndex], adminConfig.APIKeys[keyIndex+1:]...)

	// Save config
	if err := saveAdminConfig(configPath, adminConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save admin config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key deleted successfully",
	})
}

// Logging system variables
var (
	logFile   *os.File
	logWriter io.Writer
)

// initializeLogging sets up file logging with automatic rotation and cleanup
func initializeLogging(logDir string) error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}
	
	// Generate log filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := fmt.Sprintf("tarr-annunciator_%s.log", timestamp)
	logFilePath := filepath.Join(logDir, logFileName)
	
	// Open log file
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	
	logFile = file
	
	// Create multi-writer to write to both console and file
	logWriter = io.MultiWriter(os.Stdout, file)
	log.SetOutput(logWriter)
	
	// Add log header
	log.Printf("=== TARR Annunciator Started ===")
	log.Printf("Version: Go Application")
	log.Printf("Platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("Log file: %s", logFilePath)
	log.Printf("Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))
	log.Printf("=====================================")
	
	// Start log cleanup routine
	go func() {
		if err := cleanupOldLogs(logDir); err != nil {
			log.Printf("Warning: Failed to cleanup old logs: %v", err)
		}
		
		// Setup periodic cleanup (every 24 hours)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		
		for range ticker.C {
			if err := cleanupOldLogs(logDir); err != nil {
				log.Printf("Warning: Failed to cleanup old logs: %v", err)
			}
		}
	}()
	
	return nil
}

// cleanupOldLogs removes log files older than 30 days
func cleanupOldLogs(logDir string) error {
	log.Printf("Starting log cleanup routine...")
	
	// Read directory contents
	files, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read logs directory: %v", err)
	}
	
	cutoffTime := time.Now().AddDate(0, 0, -30) // 30 days ago
	deletedCount := 0
	totalSize := int64(0)
	
	for _, file := range files {
		// Only process .log files
		if !strings.HasSuffix(file.Name(), ".log") {
			continue
		}
		
		// Get file info
		info, err := file.Info()
		if err != nil {
			log.Printf("Warning: Could not get info for log file %s: %v", file.Name(), err)
			continue
		}
		
		totalSize += info.Size()
		
		// Check if file is older than 30 days
		if info.ModTime().Before(cutoffTime) {
			filePath := filepath.Join(logDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("Warning: Could not delete old log file %s: %v", file.Name(), err)
			} else {
				log.Printf("Deleted old log file: %s (%.2f MB, %s old)", 
					file.Name(), 
					float64(info.Size())/1024/1024,
					time.Since(info.ModTime()).Round(24*time.Hour))
				deletedCount++
			}
		}
	}
	
	log.Printf("Log cleanup completed: %d files deleted, total log size: %.2f MB", 
		deletedCount, float64(totalSize)/1024/1024)
	
	return nil
}

// closeLogging properly closes the log file
func closeLogging() {
	if logFile != nil {
		log.Printf("=== TARR Annunciator Shutting Down ===")
		log.Printf("Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))
		log.Printf("=======================================")
		logFile.Close()
	}
}