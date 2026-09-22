package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	"unicode/utf16"

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
	Enabled     bool   `json:"enabled"`
	Cron        string `json:"cron"`
	TrainNumber string `json:"train_number"`
	Direction   string `json:"direction"`
	Destination string `json:"destination"`
	TrackNumber string `json:"track_number"`
}

type PromoCronJob struct {
	Enabled bool   `json:"enabled"`
	Cron    string `json:"cron"`
	File    string `json:"file"`
}

type SafetyCronJob struct {
	Enabled   bool     `json:"enabled"`
	Cron      string   `json:"cron"`
	Language  string   `json:"language"`            // Legacy single language support
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

// resolveAppBaseDir picks the install root that contains json/ + static/.
// Prefer the executable's directory (Windows double-click / shortcut safe),
// then fall back to the process working directory.
func resolveAppBaseDir() string {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			candidates = append(candidates, filepath.Dir(resolved))
		} else {
			candidates = append(candidates, filepath.Dir(exe))
		}
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		mp3 := filepath.Join(dir, "static", "mp3")
		json := filepath.Join(dir, "json")
		if dirExists(mp3) || dirExists(json) {
			return dir
		}
	}
	if len(candidates) > 0 && candidates[0] != "" {
		return candidates[0]
	}
	wd, _ := os.Getwd()
	return wd
}

func main() {
	fmt.Println("Starting TARR Annunciator...")

	// Prefer directory of the executable so double-click / IDE launches still find
	// json/ and static/ next to the binary (Getwd alone breaks Windows testing).
	baseDir := resolveAppBaseDir()
	jsonDir := filepath.Join(baseDir, "json")
	mp3Dir := filepath.Join(baseDir, "static", "mp3")
	logDir := filepath.Join(baseDir, "logs")

	log.Printf("Base directory: %s", baseDir)

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

	// Screen/cron/boot sessions often have a short PATH and no XDG_RUNTIME_DIR.
	// amixer still talks to ALSA without a TTY; this just makes the tools findable.
	ensureLinuxAudioEnv()

	// Restore saved output device before the speaker opens the default ALSA/Pulse sink.
	restoreAudioOutputDevice()

	// Initialize audio
	if err := initAudio(); err != nil {
		log.Printf("Audio initialization failed: %v", err)
		app.AudioEnabled = false
	} else {
		log.Println("✓ Audio system initialized successfully")
	}

	// Sync/restore ALSA (alsamixer) volume on Linux so reboot defaults don't stay quiet
	initializeSystemVolume()

	// NTP/time sync and operating-hours gate for scheduled announcements
	initializeTimeAndHours()

	// Embedded catalog safety net (runs even when install_version already matches AppVersion)
	ensureEmbeddedJSONSeeds()

	// Pending update handoff from previous apply (deferred install_version finalize)
	pendingFailed := false
	if err := processUpdatePendingIfAny(); err != nil {
		pendingFailed = true
		log.Printf("ERROR: update_pending migrations failed: %v (leaving update_pending in place)", err)
	}

	// Additive schema migrations (never overwrite existing settings)
	fromVer := getInstalledVersion()
	if err := runSchemaMigrations(fromVer); err != nil {
		log.Printf("Warning: schema migrations failed: %v", err)
	} else if !pendingFailed {
		// Do not stamp success if pending update is still broken
		if _, err := readUpdatePendingMeta(); err == nil {
			log.Printf("Note: update_pending still present — deferring install_version stamp")
		} else if err := writeInstallVersion(AppVersion, "startup"); err != nil {
			log.Printf("Warning: could not update install_version.json: %v", err)
		}
	}

	// Re-ensure catalogs after migrations (covers from==to early exit path)
	ensureEmbeddedJSONSeeds()

	// Initialize announcement queue system
	InitializeAnnouncementManager()
	log.Println("✓ Announcement queue system initialized")

	// Initialize lightning trigger system
	if err := initializeLightningTrigger(); err != nil {
		log.Printf("Warning: Lightning trigger initialization failed: %v", err)
	}

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

		// Stop lightning trigger
		stopLightningTrigger()
		log.Println("Lightning trigger stopped")

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
	app.Router.GET("/lightning_status", publicLightningStatusHandler)

	// Admin routes
	app.Router.GET("/admin/login", adminLoginGetHandler)
	app.Router.POST("/admin/login", adminLoginPostHandler)
	app.Router.GET("/admin/logout", adminLogoutHandler)
	app.Router.GET("/admin", requireAuth(), adminHandler)
	app.Router.POST("/admin", requireAuth(), adminPostHandler)
	app.Router.GET("/admin/api-docs", requireAuth(), apiDocsHandler)
	app.Router.GET("/api/docs", requireAuth(), apiDocsHandler) // legacy URL → same auth gate

	// Audio control routes (admin only)
	app.Router.GET("/audio/devices", requireAuth(), getAudioDevicesHandler)
	app.Router.POST("/audio/devices", requireAuth(), setAudioDeviceHandler)
	app.Router.GET("/audio/volume", requireAuth(), getVolumeHandler)
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

	app.Router.GET("/admin/updates/status", requireAuth(), getUpdateStatusHandler)
	app.Router.GET("/admin/updates/check", requireAuth(), checkUpdatesHandler)
	app.Router.POST("/admin/updates/install", requireAuth(), installUpdateHandler)

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

	// Lightning trigger management routes (admin only)
	app.Router.GET("/admin/lightning/status", requireAuth(), getLightningTriggerStatusHandler)
	app.Router.POST("/admin/lightning/config", requireAuth(), updateLightningTriggerConfigHandler)
	app.Router.POST("/admin/lightning/test", requireAuth(), testLightningFetchHandler)
	app.Router.POST("/admin/lightning/test-condition/:condition", requireAuth(), testLightningConditionHandler)
	app.Router.POST("/admin/lightning/test-reminder", requireAuth(), testRedAlertReminderHandler)
	app.Router.POST("/admin/lightning/reset", requireAuth(), resetLightningStateHandler)
	app.Router.GET("/admin/lightning/sensors", requireAuth(), getLightningSensorsHandler)
	app.Router.POST("/admin/lightning/active-feed", requireAuth(), pinLightningActiveFeedHandler)
	app.Router.POST("/admin/lightning/test-feed-switch-audio", requireAuth(), testFeedSwitchAudioHandler)
	app.Router.POST("/admin/lightning/override", requireAuth(), lightningManualOverrideHandler)

	app.Router.GET("/admin/time-status", requireAuth(), getTimeStatusHandler)
	app.Router.POST("/admin/time-sync", requireAuth(), syncTimeHandler)
	app.Router.GET("/admin/operating-hours", requireAuth(), getOperatingHoursHandler)
	app.Router.POST("/admin/operating-hours", requireAuth(), saveOperatingHoursHandler)
}

func setupAPIRoutes() {
	api := app.Router.Group("/api")

	// Public endpoints
	api.GET("/status", apiStatusHandler)
	api.GET("/platform", apiPlatformInfoHandler)
	// /api/docs is not public — use /admin/api-docs (session auth)

	// Authenticated endpoints
	authAPI := api.Group("", requireAPIKey())
	{
		authAPI.POST("/announce/station", apiStationAnnouncementHandler)
		authAPI.POST("/announce/safety", apiSafetyAnnouncementHandler)
		authAPI.POST("/announce/promo", apiPromoAnnouncementHandler)
		authAPI.POST("/announce/emergency", apiEmergencyAnnouncementHandler)
		authAPI.POST("/lightning/test/:condition", apiTestLightningConditionHandler)
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
		authAPI.GET("/lightning/status", apiGetLightningStatusHandler)
		authAPI.POST("/lightning/config", apiUpdateLightningConfigHandler)
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
		"trains":              trains,
		"directions":          directions,
		"destinations":        destinations,
		"tracks":              tracks,
		"promo_announcements": promoAnnouncements,
		"safety_languages":    safetyLanguages,
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
			c.String(playQueueHTTPStatus(err), "Failed to queue station announcement: "+err.Error())
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
			c.String(playQueueHTTPStatus(err), "Failed to queue promo announcement: "+err.Error())
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
			c.String(playQueueHTTPStatus(err), "Failed to queue safety announcement: "+err.Error())
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

	hours := getSchedulerHoursStatus()
	c.JSON(http.StatusOK, gin.H{
		"scheduler_running": hours["scheduler_active"],
		"jobs":              jobs,
		"audio_available":   app.AudioEnabled,
		"operating_hours":   hours,
		"time":              getTimeStatus(),
	})
}

func audioStatusHandler(c *gin.Context) {
	chimePath := stationChimePath()
	chimeExists := fileExists(chimePath)
	mp3DirExists := dirExists(app.Config.MP3Dir)

	c.JSON(http.StatusOK, gin.H{
		"audio_available":      app.AudioEnabled,
		"audio_backend":        "beep",
		"current_volume":       app.Config.CurrentVolume,
		"volume_percent":       int(app.Config.CurrentVolume * 100),
		"chime_exists":         chimeExists,
		"mp3_directory_exists": mp3DirExists,
		"system_mixer":         getSystemMixerStatus(),
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
		"promo_announcements":    promoAnnouncements,
		"safety_languages":       safetyLanguages,
		"emergencies":            emergencies,
		"current_volume":         app.Config.CurrentVolume,
		"audio_devices":          audioDevices,
		"selected_audio_device":  app.Config.SelectedAudioDevice,
		"mixer_summary":          getSystemMixerStatus().Summary,
		"mixer_help":             mixerHelpText(),
	})
}

func adminPostHandler(c *gin.Context) {
	cronJSON := c.PostForm("cron_json")
	var cronData CronData

	if err := json.Unmarshal([]byte(cronJSON), &cronData); err != nil {
		cronDataDisplay := loadJSON("cron", CronData{}).(CronData)
		cronDataJSON, _ := json.MarshalIndent(cronDataDisplay, "", "    ")

		c.HTML(http.StatusBadRequest, "admin.html", gin.H{
			"error":     fmt.Sprintf("Error parsing schedule: %v", err),
			"cron_data": string(cronDataJSON),
		})
		return
	}

	if err := saveJSON("cron", cronData); err != nil {
		cronDataJSON, _ := json.MarshalIndent(cronData, "", "    ")

		c.HTML(http.StatusInternalServerError, "admin.html", gin.H{
			"error":     fmt.Sprintf("Error saving schedule: %v", err),
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
		"devices":        devices,
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
	if err := applySelectedAudioDevice(deviceID, selectedDevice.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to set audio device: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"device":    selectedDevice,
		"persisted": true,
		"message":   "Audio device set and saved for reboot",
	})
}

func getVolumeHandler(c *gin.Context) {
	status := getSystemMixerStatus()
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"volume":         app.Config.CurrentVolume,
		"volume_percent": int(app.Config.CurrentVolume * 100),
		"system_mixer":   status,
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

	status := applyVolumeChange(volume)
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"volume":         app.Config.CurrentVolume,
		"volume_percent": int(app.Config.CurrentVolume * 100),
		"system_mixer":   status,
	})
}

func testAudioHandler(c *gin.Context) {
	chimePath := stationChimePath()
	if !fileExists(chimePath) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Test audio file not found: %s (base=%s mp3=%s)", chimePath, app.Config.BaseDir, app.Config.MP3Dir),
		})
		return
	}

	if err := playAudio(chimePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("Audio test failed: %v", err)})
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
			"id":          key.ID,
			"name":        key.Name,
			"key":         key.Key, // Include key for frontend masking
			"enabled":     key.Enabled,
			"permanent":   key.Permanent,
			"expires_at":  key.ExpiresAt,
			"created_at":  key.CreatedAt,
			"created_by":  key.CreatedBy,
			"last_used":   key.LastUsed,
			"permissions": key.Permissions,
			"rate_limit":  key.RateLimit,
		}
	}

	// Return safe data
	c.JSON(http.StatusOK, gin.H{
		"admin_users":           safeUsers,
		"api_keys":              safeAPIKeys,
		"session_timeout":       adminConfig.Security.SessionTimeoutMinutes,
		"require_admin_login":   adminConfig.Security.RequireAdminLogin,
		"password_policy":       adminConfig.Security.PasswordPolicy,
		"failed_login_attempts": adminConfig.Security.FailedLoginAttempts,
		"last_modified":         adminConfig.Metadata.LastModified,
		"schema_version":        adminConfig.Metadata.SchemaVersion,
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
		"success":    true,
		"message":    "API key created successfully",
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
	logFile   *rotatingFileWriter
	logWriter io.Writer
)

// initializeLogging sets up file logging with size-based rotation and cleanup
func initializeLogging(logDir string) error {
	file, err := newRotatingFileWriter(logDir)
	if err != nil {
		return err
	}

	logFile = file
	logWriter = io.MultiWriter(os.Stdout, file)
	log.SetOutput(logWriter)

	log.Printf("=== TARR Annunciator Started ===")
	log.Printf("Version: %s (installed record: %s)", AppVersion, getInstalledVersion())
	log.Printf("Platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("Log file: %s", file.Path())
	log.Printf("Log rotation: max %d MB per file, keep %d files, delete after %d days",
		maxLogFileSize/1024/1024, maxLogFiles, logRetentionDays)
	log.Printf("Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))
	log.Printf("=====================================")
	reportLogCleanup(logDir)

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := pruneLogFiles(logDir, file.Path()); err != nil {
				log.Printf("Warning: Failed to cleanup old logs: %v", err)
				continue
			}
			reportLogCleanup(logDir)
		}
	}()

	return nil
}

// Lightning trigger handler functions
func getLightningTriggerStatusHandler(c *gin.Context) {
	status := getLightningTriggerStatus()
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   status,
	})
}

func playQueueHTTPStatus(err error) int {
	if errors.Is(err, ErrRedAlertSuppressed) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

func publicLightningStatusHandler(c *gin.Context) {
	status := getLightningTriggerStatus()
	c.JSON(http.StatusOK, gin.H{
		"red_alert_active":          status["red_alert_active"],
		"last_condition":            status["last_condition"],
		"red_alert_since":           status["red_alert_since"],
		"next_reminder":             status["next_reminder"],
		"reminder_count":            status["reminder_count"],
		"active_displayname":        status["active_displayname"],
		"active_feed_id":            status["active_feed_id"],
		"on_failover":               status["on_failover"],
		"feed_health":               status["feed_health"],
		"feeds":                     status["feeds"],
		"allclear_release_mode":     status["allclear_release_mode"],
		"entered_red_alert_on_feed": status["entered_red_alert_on_feed"],
		"manual_override_active":    status["manual_override_active"],
		"manual_override_condition": status["manual_override_condition"],
	})
}

func testRedAlertReminderHandler(c *gin.Context) {
	if lightningTrigger == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Lightning trigger not available",
		})
		return
	}
	if !isRedAlertActive() {
		c.JSON(http.StatusConflict, gin.H{
			"status":  "error",
			"message": "Red Alert is not active. Trigger a Red Alert first, then play a reminder.",
		})
		return
	}
	lightningTrigger.playRedAlertReminder()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Red Alert reminder queued",
	})
}

func updateLightningTriggerConfigHandler(c *gin.Context) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Invalid request format: " + err.Error()})
		return
	}

	section := "monitor"
	if sRaw, ok := raw["section"]; ok {
		_ = json.Unmarshal(sRaw, &section)
	}

	if lightningTrigger == nil || lightningConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "Lightning trigger not available"})
		return
	}

	switch section {
	case "condition_announce":
		var body struct {
			ConditionAnnounce map[string]bool `json:"condition_announce"`
		}
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &body); err != nil || body.ConditionAnnounce == nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "condition_announce required"})
			return
		}
		for cond, enabled := range body.ConditionAnnounce {
			setConditionAnnounceEnabled(cond, enabled)
		}
		if err := saveLightningConfig(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Condition announcements saved", "data": getLightningTriggerStatus()})
		return

	case "condition_audio":
		var body struct {
			ConditionAudio *ConditionAudioConfig `json:"condition_audio"`
		}
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &body); err != nil || body.ConditionAudio == nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "condition_audio required"})
			return
		}
		lightningConfig.ConditionAudio = *body.ConditionAudio
		lightningConfig.ensurePolicyDefaults()
		if err := saveLightningConfig(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Condition audio (horn + announce) saved", "data": getLightningTriggerStatus()})
		return

	case "red_alert_policy":
		var body struct {
			RedAlertPolicy *RedAlertPolicy `json:"red_alert_policy"`
		}
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &body); err != nil || body.RedAlertPolicy == nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "red_alert_policy required"})
			return
		}
		if err := applyRedAlertPolicy(*body.RedAlertPolicy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Red Alert policy saved", "data": getLightningTriggerStatus()})
		return

	default: // monitor
		var body struct {
			URL                     string                   `json:"url"`
			FetchInterval           int                      `json:"fetch_interval"`
			Timeout                 int                      `json:"timeout"`
			Enabled                 bool                     `json:"enabled"`
			Failover                *LightningFailoverPolicy `json:"failover"`
			Feeds                   []LightningFeedConfig    `json:"feeds"`
			FeedSwitchAnnouncements []FeedSwitchAnnouncement `json:"feed_switch_announcements"`
			CompositeRedAlertRules  []CompositeRedAlertRule  `json:"composite_red_alert_rules"`
			AnnounceTiming          *LightningAnnounceTiming `json:"announce_timing"`
			DisplaynameOverrides    []DisplaynameOverride    `json:"displayname_overrides"`
		}
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
			return
		}

		mon := getLightningMonitorConfig()
		if body.FetchInterval > 0 {
			mon.FetchInterval = body.FetchInterval
		}
		if body.Timeout > 0 {
			mon.Timeout = body.Timeout
		}
		mon.Enabled = body.Enabled
		if body.Failover != nil {
			mon.Failover = *body.Failover
		}
		if len(body.Feeds) > 0 {
			mon.Feeds = body.Feeds
		} else if strings.TrimSpace(body.URL) != "" {
			// Legacy flat URL update
			if len(mon.Feeds) == 0 {
				mon.Feeds = defaultFeedSlots()
			}
			mon.Feeds[0].URL = strings.TrimSpace(body.URL)
			mon.Feeds[0].Enabled = body.Enabled
			mon.URL = mon.Feeds[0].URL
		}
		if body.FeedSwitchAnnouncements != nil {
			mon.FeedSwitchAnnouncements = body.FeedSwitchAnnouncements
		}
		if body.CompositeRedAlertRules != nil {
			mon.CompositeRedAlertRules = body.CompositeRedAlertRules
		}
		mon.URL = primaryFeedURL(mon)

		var timing *LightningAnnounceTiming
		if body.AnnounceTiming != nil {
			timing = body.AnnounceTiming
		}
		var overrides []DisplaynameOverride
		if body.DisplaynameOverrides != nil {
			overrides = body.DisplaynameOverrides
		}

		if err := lightningTrigger.ApplyMonitorConfig(mon, timing, overrides); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Lightning monitor configuration saved", "data": getLightningTriggerStatus()})
		return
	}
}

func getLightningSensorsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"sensors": getBrowardTGSensors(),
		"catalog": getBrowardTGCatalog(),
	})
}

func pinLightningActiveFeedHandler(c *gin.Context) {
	var body struct {
		FeedID string `json:"feed_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
		return
	}
	if lightningTrigger == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "Lightning trigger not available"})
		return
	}
	if err := lightningTrigger.PinActiveFeed(body.FeedID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Active feed pin updated", "data": getLightningTriggerStatus()})
}

func testFeedSwitchAudioHandler(c *gin.Context) {
	var body struct {
		AudioFile string `json:"audio_file"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.AudioFile) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "audio_file required"})
		return
	}
	if announcementManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "Announcement manager not available"})
		return
	}
	params := map[string]interface{}{
		"condition":      "feed_switch",
		"audio_files":    []string{body.AudioFile},
		"trigger_source": "FEED_SWITCH_TEST",
	}
	if _, err := announcementManager.QueueAnnouncement(TypeLightning, AnnouncementPriority(10), params, time.Now()); err != nil {
		c.JSON(playQueueHTTPStatus(err), gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Feed-switch test audio queued"})
}

// API handlers for lightning trigger
func apiGetLightningStatusHandler(c *gin.Context) {
	status := getLightningTriggerStatus()
	c.JSON(http.StatusOK, status)
}

func apiUpdateLightningConfigHandler(c *gin.Context) {
	updateLightningTriggerConfigHandler(c)
}

// Test lightning XML fetch handler
func testLightningFetchHandler(c *gin.Context) {
	var config struct {
		URL     string `json:"url"`
		FeedID  string `json:"feed_id"`
		Timeout int    `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format: " + err.Error(),
		})
		return
	}

	if config.URL == "" && config.FeedID != "" {
		mon := getLightningMonitorConfig()
		if f := feedByID(mon, config.FeedID); f != nil {
			config.URL = f.URL
			if config.Timeout == 0 && f.TimeoutSeconds >= 5 {
				config.Timeout = f.TimeoutSeconds
			}
		}
	}

	if config.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "URL or feed_id is required",
		})
		return
	}

	if config.Timeout == 0 {
		config.Timeout = 30
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	resp, err := client.Get(config.URL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "Failed to fetch XML: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
		})
		return
	}

	xmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "Failed to read response: " + err.Error(),
		})
		return
	}

	xmlStr, err := convertXMLEncodingTest(xmlData)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "Failed to convert XML encoding: " + err.Error(),
		})
		return
	}

	lightningAlert := extractXMLTag(xmlStr, "lightningalert")
	displayname := extractXMLTag(xmlStr, "displayname")
	uniqueid := extractXMLTag(xmlStr, "uniqueid")

	if lightningAlert != "" {
		c.JSON(http.StatusOK, gin.H{
			"status":          "success",
			"message":         "Test successful! Lightning alert tag found in XML.",
			"lightningalert":  lightningAlert,
			"displayname":     displayname,
			"uniqueid":        uniqueid,
			"xml_size":        len(xmlData),
			"response_status": resp.Status,
			"url":             config.URL,
			"feed_id":         config.FeedID,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status":          "warning",
			"message":         "Test completed, but no lightningalert tag found in XML.",
			"displayname":     displayname,
			"uniqueid":        uniqueid,
			"xml_size":        len(xmlData),
			"response_status": resp.Status,
			"url":             config.URL,
			"feed_id":         config.FeedID,
		})
	}
}

// Convert XML encoding from UTF-16 to UTF-8 if needed (for test handler)
func convertXMLEncodingTest(xmlData []byte) (string, error) {
	// Check if the data starts with a UTF-16 BOM
	if len(xmlData) >= 2 {
		// UTF-16 LE BOM
		if xmlData[0] == 0xFF && xmlData[1] == 0xFE {
			return decodeUTF16LETest(xmlData[2:])
		}
		// UTF-16 BE BOM
		if xmlData[0] == 0xFE && xmlData[1] == 0xFF {
			return decodeUTF16BETest(xmlData[2:])
		}
	}

	// Check if it looks like UTF-16 by checking for null bytes in even positions
	xmlStr := string(xmlData)
	if len(xmlData) > 20 && strings.Contains(xmlStr[:100], "\x00") {
		// Looks like UTF-16, try to decode as UTF-16 LE
		decoded, err := decodeUTF16LETest(xmlData)
		if err == nil && strings.Contains(decoded, "<?xml") {
			return decoded, nil
		}
	}

	// Already UTF-8 or ASCII
	return string(xmlData), nil
}

// Decode UTF-16 Little Endian (for test handler)
func decodeUTF16LETest(data []byte) (string, error) {
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

// Decode UTF-16 Big Endian (for test handler)
func decodeUTF16BETest(data []byte) (string, error) {
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

// Test lightning condition for debugging
// API Test lightning condition handler
func apiTestLightningConditionHandler(c *gin.Context) {
	condition := c.Param("condition")

	// Validate condition
	validConditions := []string{"RedAlert", "AllClear", "Warning", "Caution", "Unknown"}
	valid := false
	for _, v := range validConditions {
		if strings.EqualFold(condition, v) {
			condition = v // Use proper case
			valid = true
			break
		}
	}

	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid condition. Valid options: RedAlert, AllClear, Warning, Caution, Unknown",
		})
		return
	}

	if lightningTrigger != nil {
		log.Printf("API: Manual %s test triggered", condition)
		message, ok := lightningTrigger.TestCondition(condition)
		c.JSON(http.StatusOK, gin.H{
			"success":   ok,
			"message":   message,
			"condition": condition,
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Lightning trigger not available",
		})
	}
}

func testLightningConditionHandler(c *gin.Context) {
	condition := c.Param("condition")

	// Validate condition
	validConditions := []string{"RedAlert", "AllClear", "Warning", "Caution", "Unknown"}
	valid := false
	for _, v := range validConditions {
		if strings.EqualFold(condition, v) {
			condition = v // Use proper case
			valid = true
			break
		}
	}

	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid condition. Valid options: RedAlert, AllClear, Warning, Caution, Unknown",
		})
		return
	}

	if lightningTrigger != nil {
		log.Printf("DEBUG: Manual %s test triggered", condition)
		message, ok := lightningTrigger.TestCondition(condition)
		status := "success"
		if !ok {
			status = "warning"
		}
		c.JSON(http.StatusOK, gin.H{
			"status":    status,
			"message":   message,
			"condition": condition,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "Lightning trigger not available",
		})
	}
}

func resetLightningStateHandler(c *gin.Context) {
	if lightningTrigger == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Lightning trigger not available",
		})
		return
	}
	lightningTrigger.ResetState()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "THOR Guard cached state reset",
		"data":    getLightningTriggerStatus(),
	})
}

func lightningManualOverrideHandler(c *gin.Context) {
	if lightningTrigger == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Lightning trigger not available",
		})
		return
	}
	var body struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "action required"})
		return
	}
	msg, err := lightningTrigger.ApplyManualOverride(body.Action, body.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": msg,
		"data":    getLightningTriggerStatus(),
	})
}

// closeLogging properly closes the log file
func getTimeStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"time":            getTimeStatus(),
		"operating_hours": getSchedulerHoursStatus(),
	})
}

func syncTimeHandler(c *gin.Context) {
	cfg := loadOperatingHours()
	if err := syncTimeFromNTP(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
			"time":    getTimeStatus(),
		})
		return
	}
	applySchedulerHoursState(true)
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "Time synchronized",
		"time":            getTimeStatus(),
		"operating_hours": getSchedulerHoursStatus(),
	})
}

func getOperatingHoursHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  loadOperatingHours(),
		"status":  getSchedulerHoursStatus(),
		"time":    getTimeStatus(),
	})
}

func saveOperatingHoursHandler(c *gin.Context) {
	var cfg OperatingHoursConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid operating hours JSON: " + err.Error()})
		return
	}
	if err := saveOperatingHours(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save operating hours: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Operating hours saved",
		"config":  loadOperatingHours(),
		"status":  getSchedulerHoursStatus(),
		"time":    getTimeStatus(),
	})
}

func closeLogging() {
	if logFile != nil {
		log.Printf("=== TARR Annunciator Shutting Down ===")
		log.Printf("Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))
		log.Printf("=======================================")
		logFile.Close()
		logFile = nil
	}
}
