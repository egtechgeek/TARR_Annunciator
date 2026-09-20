package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "time/tzdata"
)

const (
	operatingHoursFile     = "operating_hours.json"
	defaultOperatingTZ     = "America/New_York"
	hoursCheckInterval     = 30 * time.Second
	ntpQueryTimeout        = 3 * time.Second
	ntpResyncInterval      = time.Hour
	systemClockDriftLimit  = 2 * time.Second
	ntpEpochOffsetSeconds  = 2208988800 // 1900 → 1970
)

var defaultNTPServers = []string{
	"pool.ntp.org",
	"time.google.com",
	"time.cloudflare.com",
}

type DayHours struct {
	Enabled bool   `json:"enabled"`
	Open    string `json:"open"`
	Close   string `json:"close"`
}

type TimeSyncConfig struct {
	Enabled        bool     `json:"enabled"`
	SyncOnStartup  bool     `json:"sync_on_startup"`
	SetSystemClock bool     `json:"set_system_clock"`
	NTPServers     []string `json:"ntp_servers"`
}

type OperatingHoursConfig struct {
	Enabled   bool                `json:"enabled"`
	Timezone  string              `json:"timezone"`
	OpenTime  string              `json:"open_time"`
	CloseTime string              `json:"close_time"`
	Days      map[string]DayHours `json:"days"`
	TimeSync  TimeSyncConfig      `json:"time_sync"`
}

type timeStatus struct {
	AppTime            string `json:"app_time"`
	SystemTime         string `json:"system_time"`
	Timezone           string `json:"timezone"`
	NTPEnabled         bool   `json:"ntp_enabled"`
	LastSync           string `json:"last_sync,omitempty"`
	LastServer         string `json:"last_server,omitempty"`
	OffsetMilliseconds int64  `json:"offset_ms"`
	SystemClockSet     bool   `json:"system_clock_set"`
	Error              string `json:"error,omitempty"`
}

var (
	clockMu            sync.RWMutex
	clockOffset        time.Duration
	lastNTPSync        time.Time
	lastNTPServer      string
	lastSystemClockSet bool
	lastNTPError       string

	hoursStateMu          sync.RWMutex
	schedulerWithinHours  = true
	hoursMonitorStarted   bool
	lastLoggedHoursOpen   *bool
)

func defaultOperatingHours() OperatingHoursConfig {
	days := map[string]DayHours{}
	for _, day := range weekdayKeys() {
		days[day] = DayHours{Enabled: true, Open: "09:00", Close: "16:00"}
	}
	return OperatingHoursConfig{
		Enabled:   false,
		Timezone:  defaultOperatingTZ,
		OpenTime:  "09:00",
		CloseTime: "16:00",
		Days:      days,
		TimeSync: TimeSyncConfig{
			Enabled:        true,
			SyncOnStartup:  true,
			SetSystemClock: true,
			NTPServers:     append([]string{}, defaultNTPServers...),
		},
	}
}

func weekdayKeys() []string {
	return []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
}

func operatingHoursPath() string {
	if app != nil && app.Config != nil && app.Config.JSONDir != "" {
		return filepath.Join(app.Config.JSONDir, operatingHoursFile)
	}
	return filepath.Join("json", operatingHoursFile)
}

func loadOperatingHours() OperatingHoursConfig {
	cfg := defaultOperatingHours()
	data, err := os.ReadFile(operatingHoursPath())
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Warning: invalid %s: %v", operatingHoursFile, err)
		return defaultOperatingHours()
	}
	return normalizeOperatingHours(cfg)
}

func normalizeOperatingHours(cfg OperatingHoursConfig) OperatingHoursConfig {
	if strings.TrimSpace(cfg.Timezone) == "" {
		cfg.Timezone = defaultOperatingTZ
	}
	if strings.TrimSpace(cfg.OpenTime) == "" {
		cfg.OpenTime = "09:00"
	}
	if strings.TrimSpace(cfg.CloseTime) == "" {
		cfg.CloseTime = "16:00"
	}
	if cfg.Days == nil {
		cfg.Days = map[string]DayHours{}
	}
	for _, day := range weekdayKeys() {
		entry, ok := cfg.Days[day]
		if !ok {
			cfg.Days[day] = DayHours{Enabled: true, Open: cfg.OpenTime, Close: cfg.CloseTime}
			continue
		}
		if strings.TrimSpace(entry.Open) == "" {
			entry.Open = cfg.OpenTime
		}
		if strings.TrimSpace(entry.Close) == "" {
			entry.Close = cfg.CloseTime
		}
		cfg.Days[day] = entry
	}
	if len(cfg.TimeSync.NTPServers) == 0 {
		cfg.TimeSync.NTPServers = append([]string{}, defaultNTPServers...)
	}
	return cfg
}

func saveOperatingHours(cfg OperatingHoursConfig) error {
	cfg = normalizeOperatingHours(cfg)
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	path := operatingHoursPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	applySchedulerHoursState(true)
	return nil
}

func initializeTimeAndHours() {
	cfg := loadOperatingHours()
	if cfg.TimeSync.Enabled && cfg.TimeSync.SyncOnStartup {
		if err := syncTimeFromNTP(cfg); err != nil {
			log.Printf("Warning: time sync failed: %v", err)
		}
	}

	applySchedulerHoursState(true)
	startOperatingHoursMonitor()

	if cfg.TimeSync.Enabled {
		go func() {
			ticker := time.NewTicker(ntpResyncInterval)
			defer ticker.Stop()
			for range ticker.C {
				current := loadOperatingHours()
				if !current.TimeSync.Enabled {
					continue
				}
				if err := syncTimeFromNTP(current); err != nil {
					log.Printf("Warning: periodic time sync failed: %v", err)
				}
			}
		}()
	}
}

func startOperatingHoursMonitor() {
	hoursStateMu.Lock()
	if hoursMonitorStarted {
		hoursStateMu.Unlock()
		return
	}
	hoursMonitorStarted = true
	hoursStateMu.Unlock()

	go func() {
		ticker := time.NewTicker(hoursCheckInterval)
		defer ticker.Stop()
		for range ticker.C {
			applySchedulerHoursState(false)
		}
	}()
}

func applySchedulerHoursState(forceLog bool) {
	cfg := loadOperatingHours()
	within := !cfg.Enabled || isWithinOperatingHours(cfg, appNow())

	hoursStateMu.Lock()
	changed := lastLoggedHoursOpen == nil || *lastLoggedHoursOpen != within
	schedulerWithinHours = within
	state := within
	lastLoggedHoursOpen = &state
	hoursStateMu.Unlock()

	if !forceLog && !changed {
		return
	}
	if !cfg.Enabled {
		log.Printf("Operating hours disabled — scheduler runs 24/7")
		return
	}
	if within {
		log.Printf("Operating hours OPEN (%s %s–%s) — scheduled announcements enabled",
			cfg.Timezone, cfg.OpenTime, cfg.CloseTime)
		return
	}
	log.Printf("Operating hours CLOSED (%s) — scheduled announcements paused", cfg.Timezone)
}

func scheduledJobsAllowed() bool {
	hoursStateMu.RLock()
	defer hoursStateMu.RUnlock()
	return schedulerWithinHours
}

func isWithinOperatingHours(cfg OperatingHoursConfig, now time.Time) bool {
	loc := loadHoursLocation(cfg.Timezone)
	local := now.In(loc)
	dayKey := strings.ToLower(local.Weekday().String())
	day, ok := cfg.Days[dayKey]
	if !ok || !day.Enabled {
		return false
	}

	openMins, openOK := parseHHMM(day.Open)
	closeMins, closeOK := parseHHMM(day.Close)
	if !openOK || !closeOK {
		return false
	}
	nowMins := local.Hour()*60 + local.Minute()

	if openMins == closeMins {
		return true
	}
	if openMins < closeMins {
		return nowMins >= openMins && nowMins < closeMins
	}
	// Overnight window, e.g. 22:00–02:00
	return nowMins >= openMins || nowMins < closeMins
}

func parseHHMM(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour := 0
	minute := 0
	if _, err := fmt.Sscanf(parts[0], "%d", &hour); err != nil {
		return 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minute); err != nil {
		return 0, false
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func loadHoursLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("Warning: unknown timezone %q, using UTC: %v", name, err)
		return time.UTC
	}
	return loc
}

func appNow() time.Time {
	clockMu.RLock()
	defer clockMu.RUnlock()
	return time.Now().Add(clockOffset)
}

func syncTimeFromNTP(cfg OperatingHoursConfig) error {
	if !cfg.TimeSync.Enabled {
		return fmt.Errorf("time sync is disabled")
	}

	enableSystemNTP()

	var lastErr error
	for _, server := range cfg.TimeSync.NTPServers {
		ntpTime, err := queryNTP(server)
		if err != nil {
			lastErr = err
			continue
		}

		offset := ntpTime.UTC().Sub(time.Now().UTC())
		clockMu.Lock()
		clockOffset = offset
		lastNTPSync = time.Now()
		lastNTPServer = server
		lastNTPError = ""
		clockMu.Unlock()

		systemSet := false
		if cfg.TimeSync.SetSystemClock && absDuration(offset) >= systemClockDriftLimit {
			if err := trySetSystemClock(ntpTime); err != nil {
				log.Printf("Note: could not set system clock (%v); using in-app NTP offset %s",
					err, offset.Round(time.Millisecond))
			} else {
				clockMu.Lock()
				clockOffset = 0
				lastSystemClockSet = true
				clockMu.Unlock()
				systemSet = true
			}
		}

		clockMu.Lock()
		lastSystemClockSet = systemSet || lastSystemClockSet
		clockMu.Unlock()

		log.Printf("✓ Time sync from %s (offset %s, app time %s)",
			server, offset.Round(time.Millisecond), appNow().In(loadHoursLocation(cfg.Timezone)).Format("2006-01-02 15:04:05 MST"))
		return nil
	}

	clockMu.Lock()
	if lastErr != nil {
		lastNTPError = lastErr.Error()
	}
	clockMu.Unlock()
	if lastErr == nil {
		lastErr = fmt.Errorf("no NTP servers configured")
	}
	return lastErr
}

func queryNTP(server string) (time.Time, error) {
	host := server
	if !strings.Contains(host, ":") {
		host += ":123"
	}

	conn, err := net.DialTimeout("udp", host, ntpQueryTimeout)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ntpQueryTimeout))

	var req [48]byte
	req[0] = 0x1B // LI=0, VN=3, Mode=3 (client)
	if _, err := conn.Write(req[:]); err != nil {
		return time.Time{}, err
	}

	var resp [48]byte
	if _, err := conn.Read(resp[:]); err != nil {
		return time.Time{}, err
	}

	sec := binary.BigEndian.Uint32(resp[40:44])
	frac := binary.BigEndian.Uint32(resp[44:48])
	if sec == 0 {
		return time.Time{}, fmt.Errorf("invalid NTP response from %s", server)
	}

	unixSec := int64(sec) - ntpEpochOffsetSeconds
	unixNsec := (int64(frac) * 1e9) >> 32
	return time.Unix(unixSec, unixNsec).UTC(), nil
}

func enableSystemNTP() {
	if runtime.GOOS != "linux" || !commandExists("timedatectl") {
		return
	}
	if err := exec.Command("timedatectl", "set-ntp", "true").Run(); err != nil {
		log.Printf("Note: timedatectl set-ntp true skipped (%v)", err)
	}
}

func trySetSystemClock(ntpTime time.Time) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("system clock set is only attempted on Linux")
	}
	stamp := ntpTime.Local().Format("2006-01-02 15:04:05")
	if commandExists("timedatectl") {
		if err := exec.Command("timedatectl", "set-time", stamp).Run(); err == nil {
			log.Printf("✓ System clock set via timedatectl to %s", stamp)
			return nil
		}
	}
	if commandExists("date") {
		if err := exec.Command("date", "-s", stamp).Run(); err == nil {
			log.Printf("✓ System clock set via date to %s", stamp)
			return nil
		} else {
			return err
		}
	}
	return fmt.Errorf("no timedatectl or date command available")
}

func getTimeStatus() timeStatus {
	cfg := loadOperatingHours()
	loc := loadHoursLocation(cfg.Timezone)

	clockMu.RLock()
	corrected := time.Now().Add(clockOffset)
	status := timeStatus{
		AppTime:            corrected.In(loc).Format("2006-01-02 15:04:05 MST"),
		SystemTime:         time.Now().In(loc).Format("2006-01-02 15:04:05 MST"),
		Timezone:           cfg.Timezone,
		NTPEnabled:         cfg.TimeSync.Enabled,
		OffsetMilliseconds: clockOffset.Milliseconds(),
		SystemClockSet:     lastSystemClockSet,
		Error:              lastNTPError,
	}
	if !lastNTPSync.IsZero() {
		status.LastSync = lastNTPSync.Format(time.RFC3339)
		status.LastServer = lastNTPServer
	}
	clockMu.RUnlock()
	return status
}

func getSchedulerHoursStatus() map[string]interface{} {
	cfg := loadOperatingHours()
	now := appNow()
	within := !cfg.Enabled || isWithinOperatingHours(cfg, now)
	loc := loadHoursLocation(cfg.Timezone)
	local := now.In(loc)
	dayKey := strings.ToLower(local.Weekday().String())
	day := cfg.Days[dayKey]

	hoursStateMu.RLock()
	active := schedulerWithinHours
	hoursStateMu.RUnlock()

	label := "Running (24/7)"
	if cfg.Enabled && within {
		label = "Running (within operating hours)"
	} else if cfg.Enabled {
		label = "Paused (outside operating hours)"
	}

	return map[string]interface{}{
		"hours_enabled":     cfg.Enabled,
		"within_hours":      within,
		"scheduler_active":  active,
		"label":             label,
		"timezone":          cfg.Timezone,
		"local_time":        local.Format("2006-01-02 15:04:05 MST"),
		"today":             dayKey,
		"today_enabled":     day.Enabled,
		"today_open":        day.Open,
		"today_close":       day.Close,
		"manual_exceptions": "Lightning, emergency, and manual announcements still run",
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
