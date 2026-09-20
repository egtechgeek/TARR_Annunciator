package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	lowMixerThreshold  = 0.50
	defaultMixerVolume = 0.80
	audioSettingsName  = "audio_settings.json"
)

// SystemMixerStatus describes ALSA / system mixer state for the UI and API.
type SystemMixerStatus struct {
	Available    bool     `json:"available"`
	Backend      string   `json:"backend"`
	Summary      string   `json:"summary"`
	Percent      int      `json:"percent"`
	Applied      []string `json:"applied,omitempty"`
	Persisted    bool     `json:"persisted"`
	OwnsPlayback bool     `json:"owns_playback"`
	Error        string   `json:"error,omitempty"`
}

type audioSettings struct {
	Volume           float64 `json:"volume"`
	ApplyOnStartup   bool    `json:"apply_on_startup"`
	OutputDevice     string  `json:"output_device"`
	OutputDeviceName string  `json:"output_device_name,omitempty"`
	UpdatedAt        string  `json:"updated_at"`
}

type alsaControl struct {
	Card    string
	Name    string
	Percent int
}

var (
	systemMixerOwnsPlayback bool
	alsaControlLineRe       = regexp.MustCompile(`Simple mixer control '([^']+)',(\d+)`)
	alsaPlaybackPercentRe   = regexp.MustCompile(`Playback[^\[]*\[(\d+)%\]`)
	alsaAnyPercentRe        = regexp.MustCompile(`\[(\d+)%\]`)
)

func ensureLinuxAudioEnv() {
	if runtime.GOOS != "linux" {
		return
	}

	path := os.Getenv("PATH")
	for _, extra := range []string{"/usr/bin", "/usr/sbin", "/bin", "/sbin"} {
		if !pathContainsDir(path, extra) {
			if path == "" {
				path = extra
			} else {
				path = path + string(os.PathListSeparator) + extra
			}
		}
	}
	if err := os.Setenv("PATH", path); err != nil {
		log.Printf("Warning: could not expand PATH for audio tools: %v", err)
	}

	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		runtimeDir := fmt.Sprintf("/run/user/%d", os.Getuid())
		if info, err := os.Stat(runtimeDir); err == nil && info.IsDir() {
			if err := os.Setenv("XDG_RUNTIME_DIR", runtimeDir); err != nil {
				log.Printf("Warning: could not set XDG_RUNTIME_DIR: %v", err)
			}
		}
	}
}

func pathContainsDir(path, dir string) bool {
	for _, part := range strings.Split(path, string(os.PathListSeparator)) {
		if part == dir {
			return true
		}
	}
	return false
}

func initializeSystemVolume() {
	if runtime.GOOS != "linux" {
		return
	}
	if !commandExists("amixer") {
		log.Printf("amixer not found — in-app volume will use software gain only")
		return
	}

	if settings, ok := loadAudioSettings(); ok && settings.ApplyOnStartup {
		app.Config.CurrentVolume = clampVolume(settings.Volume)
		status := applySystemMixerVolume(app.Config.CurrentVolume)
		if status.Available {
			log.Printf("✓ Restored ALSA/system mixer volume to %d%% (%s)", status.Percent, status.Summary)
		} else {
			log.Printf("Warning: saved volume is %d%% but system mixer could not be set: %s",
				int(app.Config.CurrentVolume*100), status.Error)
		}
		return
	}

	if vol, status, ok := readPrimaryMixerVolume(); ok {
		if vol < lowMixerThreshold {
			log.Printf("ALSA/system mixer is low (%d%%) after boot; raising to %d%%",
				int(vol*100), int(defaultMixerVolume*100))
			app.Config.CurrentVolume = defaultMixerVolume
			applied := applySystemMixerVolume(defaultMixerVolume)
			if err := persistAudioSettings(defaultMixerVolume); err != nil {
				log.Printf("Warning: could not persist volume preference: %v", err)
			}
			if applied.Available {
				log.Printf("✓ Set ALSA/system mixer volume to %d%%", applied.Percent)
			}
			return
		}
		app.Config.CurrentVolume = vol
		systemMixerOwnsPlayback = true
		log.Printf("✓ Synced volume from %s: %d%%", status.Summary, int(vol*100))
		return
	}

	log.Printf("Could not read ALSA mixer; leaving software volume at %d%%", int(app.Config.CurrentVolume*100))
}

func applyVolumeChange(volume float64) SystemMixerStatus {
	volume = clampVolume(volume)
	app.Config.CurrentVolume = volume

	status := applySystemMixerVolume(volume)
	if err := persistAudioSettings(volume); err != nil {
		log.Printf("Warning: could not persist volume preference: %v", err)
	} else {
		status.Persisted = true
	}
	return status
}

func getSystemMixerStatus() SystemMixerStatus {
	if runtime.GOOS != "linux" {
		return SystemMixerStatus{
			Available:    false,
			Backend:      "software",
			Summary:      "Application software volume",
			Percent:      int(app.Config.CurrentVolume * 100),
			OwnsPlayback: false,
		}
	}

	if vol, status, ok := readPrimaryMixerVolume(); ok {
		status.Percent = int(vol * 100)
		status.OwnsPlayback = systemMixerOwnsPlayback
		return status
	}

	if !commandExists("amixer") {
		return SystemMixerStatus{
			Available: false,
			Backend:   "software",
			Summary:   "amixer not installed — software volume only",
			Percent:   int(app.Config.CurrentVolume * 100),
			Error:     "amixer not found in PATH",
		}
	}

	return SystemMixerStatus{
		Available:    false,
		Backend:      "alsa",
		Summary:      "ALSA mixer not readable",
		Percent:      int(app.Config.CurrentVolume * 100),
		OwnsPlayback: systemMixerOwnsPlayback,
	}
}

func applySystemMixerVolume(volume float64) SystemMixerStatus {
	status := SystemMixerStatus{
		Backend: "software",
		Percent: int(clampVolume(volume) * 100),
		Summary: "Application software volume",
	}

	if runtime.GOOS != "linux" {
		systemMixerOwnsPlayback = false
		return status
	}

	percent := status.Percent
	var applied []string
	hardwareSet := false

	if commandExists("amixer") {
		for _, card := range discoverALSACards() {
			for _, ctrl := range listALSAVolumeControls(&card) {
				if err := setALSAControl(&card, ctrl.Name, percent); err != nil {
					log.Printf("amixer -c %s sset %s failed: %v", card, ctrl.Name, err)
					continue
				}
				applied = append(applied, fmt.Sprintf("ALSA card %s %s → %d%%", card, ctrl.Name, percent))
				hardwareSet = true
			}
		}

		if !hardwareSet {
			if err := setALSAControl(nil, "Master", percent); err == nil {
				applied = append(applied, fmt.Sprintf("ALSA default Master → %d%%", percent))
			} else if err := setALSAControl(nil, "PCM", percent); err == nil {
				applied = append(applied, fmt.Sprintf("ALSA default PCM → %d%%", percent))
			}
		}
	}

	applied = append(applied, setPulsePipeVolume(percent)...)

	if len(applied) == 0 {
		systemMixerOwnsPlayback = false
		status.Error = "no ALSA/Pulse/PipeWire mixer controls could be set"
		status.Summary = "Software volume only (system mixer unavailable)"
		return status
	}

	systemMixerOwnsPlayback = true
	status.Available = true
	status.OwnsPlayback = true
	status.Applied = applied
	if hardwareSet {
		status.Backend = "alsa"
		status.Summary = fmt.Sprintf("ALSA mixer set to %d%%", percent)
	} else {
		status.Backend = "pulse/pipewire"
		status.Summary = fmt.Sprintf("System sink volume set to %d%%", percent)
	}

	persistALSAState()
	log.Printf("System mixer updated: %s", strings.Join(applied, "; "))
	return status
}

func readPrimaryMixerVolume() (float64, SystemMixerStatus, bool) {
	if runtime.GOOS != "linux" || !commandExists("amixer") {
		return 0, SystemMixerStatus{}, false
	}

	preferred := []string{"Master", "PCM", "Speaker", "Headphone", "HDMI", "Digital"}

	// Prefer the card implied by the selected hw:X,Y device.
	if app != nil && strings.HasPrefix(app.Config.SelectedAudioDevice, "hw:") {
		card := extractCardNumber(app.Config.SelectedAudioDevice)
		if ctrl, ok := pickPreferredControl(listALSAVolumeControls(&card), preferred); ok {
			return float64(ctrl.Percent) / 100.0, mixerStatusFromControl("alsa", ctrl), true
		}
	}

	for _, card := range discoverALSACards() {
		if ctrl, ok := pickPreferredControl(listALSAVolumeControls(&card), preferred); ok {
			return float64(ctrl.Percent) / 100.0, mixerStatusFromControl("alsa", ctrl), true
		}
	}

	if controls := listALSAVolumeControls(nil); len(controls) > 0 {
		if ctrl, ok := pickPreferredControl(controls, preferred); ok {
			return float64(ctrl.Percent) / 100.0, mixerStatusFromControl("alsa", ctrl), true
		}
	}

	if vol, ok := readWpctlVolume(); ok {
		return vol, SystemMixerStatus{
			Available: true,
			Backend:   "pipewire",
			Summary:   fmt.Sprintf("PipeWire default sink %d%%", int(vol*100)),
			Percent:   int(vol * 100),
		}, true
	}

	return 0, SystemMixerStatus{}, false
}

func mixerStatusFromControl(backend string, ctrl alsaControl) SystemMixerStatus {
	cardLabel := "default"
	if ctrl.Card != "" {
		cardLabel = "card " + ctrl.Card
	}
	return SystemMixerStatus{
		Available: true,
		Backend:   backend,
		Summary:   fmt.Sprintf("ALSA %s %s %d%%", cardLabel, ctrl.Name, ctrl.Percent),
		Percent:   ctrl.Percent,
	}
}

func pickPreferredControl(controls []alsaControl, preferred []string) (alsaControl, bool) {
	if len(controls) == 0 {
		return alsaControl{}, false
	}
	for _, name := range preferred {
		for _, ctrl := range controls {
			if strings.EqualFold(ctrl.Name, name) {
				return ctrl, true
			}
		}
	}
	return controls[0], true
}

func discoverALSACards() []string {
	var cards []string
	for i := 0; i <= 7; i++ {
		id := strconv.Itoa(i)
		cmd := exec.Command("amixer", "-c", id, "scontrols")
		if err := cmd.Run(); err == nil {
			cards = append(cards, id)
		}
	}
	return cards
}

func listALSAVolumeControls(card *string) []alsaControl {
	args := []string{"-M"}
	if card != nil {
		args = append(args, "-c", *card)
	}
	args = append(args, "scontrols")

	out, err := exec.Command("amixer", args...).Output()
	if err != nil {
		return nil
	}

	var controls []alsaControl
	seen := map[string]bool{}
	for _, match := range alsaControlLineRe.FindAllStringSubmatch(string(out), -1) {
		name := match[1]
		if isCaptureOnlyControl(name) {
			continue
		}
		key := name
		if card != nil {
			key = *card + ":" + name
		}
		if seen[key] {
			continue
		}
		percent, ok := readALSAControlPercent(card, name)
		if !ok {
			continue
		}
		seen[key] = true
		ctrl := alsaControl{Name: name, Percent: percent}
		if card != nil {
			ctrl.Card = *card
		}
		controls = append(controls, ctrl)
	}
	return controls
}

func readALSAControlPercent(card *string, name string) (int, bool) {
	args := []string{"-M"}
	if card != nil {
		args = append(args, "-c", *card)
	}
	args = append(args, "sget", name)

	out, err := exec.Command("amixer", args...).Output()
	if err != nil {
		return 0, false
	}
	text := string(out)
	if match := alsaPlaybackPercentRe.FindStringSubmatch(text); len(match) == 2 {
		pct, convErr := strconv.Atoi(match[1])
		return pct, convErr == nil
	}
	if !strings.Contains(text, "Playback") {
		return 0, false
	}
	if match := alsaAnyPercentRe.FindStringSubmatch(text); len(match) == 2 {
		pct, convErr := strconv.Atoi(match[1])
		return pct, convErr == nil
	}
	return 0, false
}

func setALSAControl(card *string, name string, percent int) error {
	base := []string{"-q", "-M"}
	if card != nil {
		base = append(base, "-c", *card)
	}
	pct := fmt.Sprintf("%d%%", percent)

	withUnmute := append(append([]string{}, base...), "sset", name, pct, "unmute")
	if out, err := exec.Command("amixer", withUnmute...).CombinedOutput(); err == nil {
		return nil
	} else {
		withoutUnmute := append(append([]string{}, base...), "sset", name, pct)
		if out2, err2 := exec.Command("amixer", withoutUnmute...).CombinedOutput(); err2 == nil {
			return nil
		} else {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)+string(out2)))
		}
	}
}

func setPulsePipeVolume(percent int) []string {
	var applied []string
	volArg := fmt.Sprintf("%d%%", percent)

	if commandExists("wpctl") {
		for _, sink := range []string{"@DEFAULT_AUDIO_SINK@", "@DEFAULT_SINK@"} {
			if exec.Command("wpctl", "set-volume", sink, volArg).Run() == nil {
				exec.Command("wpctl", "set-mute", sink, "0").Run()
				applied = append(applied, fmt.Sprintf("PipeWire %s → %s", sink, volArg))
				break
			}
		}
	}

	if commandExists("pactl") {
		if exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", volArg).Run() == nil {
			exec.Command("pactl", "set-sink-mute", "@DEFAULT_SINK@", "0").Run()
			applied = append(applied, fmt.Sprintf("Pulse default sink → %s", volArg))
		}
	}

	return applied
}

func readWpctlVolume() (float64, bool) {
	if !commandExists("wpctl") {
		return 0, false
	}
	for _, sink := range []string{"@DEFAULT_AUDIO_SINK@", "@DEFAULT_SINK@"} {
		out, err := exec.Command("wpctl", "get-volume", sink).Output()
		if err != nil {
			continue
		}
		// "Volume: 0.80" or "Volume: 0.80 [MUTED]"
		fields := strings.Fields(string(out))
		for i, field := range fields {
			if strings.EqualFold(field, "Volume:") && i+1 < len(fields) {
				vol, convErr := strconv.ParseFloat(fields[i+1], 64)
				if convErr == nil {
					return clampVolume(vol), true
				}
			}
		}
	}
	return 0, false
}

func persistALSAState() {
	if !commandExists("alsactl") {
		return
	}
	if app != nil && app.Config.BaseDir != "" {
		stateFile := filepath.Join(app.Config.BaseDir, "alsa.state")
		if err := exec.Command("alsactl", "--file", stateFile, "store").Run(); err == nil {
			return
		}
	}
	if err := exec.Command("alsactl", "store").Run(); err != nil {
		log.Printf("Note: alsactl store skipped (%v); app will still restore volume on startup", err)
	}
}

func loadAudioSettings() (audioSettings, bool) {
	path := audioSettingsPath()
	if path == "" {
		return audioSettings{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return audioSettings{}, false
	}
	var settings audioSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("Warning: invalid %s: %v", audioSettingsName, err)
		return audioSettings{}, false
	}
	settings.Volume = clampVolume(settings.Volume)
	return settings, true
}

func restoreAudioOutputDevice() {
	settings, ok := loadAudioSettings()
	if !ok || strings.TrimSpace(settings.OutputDevice) == "" {
		return
	}

	app.Config.SelectedAudioDevice = settings.OutputDevice
	if err := setAudioDevice(settings.OutputDevice); err != nil {
		log.Printf("Warning: could not restore audio output device %s: %v", settings.OutputDevice, err)
		return
	}
	if settings.OutputDeviceName != "" {
		log.Printf("✓ Restored audio output device: %s (%s)", settings.OutputDeviceName, settings.OutputDevice)
		return
	}
	log.Printf("✓ Restored audio output device: %s", settings.OutputDevice)
}

func applySelectedAudioDevice(deviceID, deviceName string) error {
	if err := setAudioDevice(deviceID); err != nil {
		return err
	}
	if app != nil && app.Config != nil {
		app.Config.SelectedAudioDevice = deviceID
	}
	if err := persistAudioDevice(deviceID, deviceName); err != nil {
		log.Printf("Warning: could not persist audio output device: %v", err)
	}
	applySystemMixerVolume(app.Config.CurrentVolume)
	return nil
}

func persistAudioDevice(deviceID, deviceName string) error {
	path := audioSettingsPath()
	if path == "" {
		return fmt.Errorf("JSON directory not available")
	}
	settings, _ := loadAudioSettings()
	if app != nil && app.Config != nil {
		settings.Volume = clampVolume(app.Config.CurrentVolume)
	}
	settings.ApplyOnStartup = true
	settings.OutputDevice = deviceID
	settings.OutputDeviceName = deviceName
	settings.UpdatedAt = time.Now().Format(time.RFC3339)
	return writeAudioSettings(path, settings)
}

func persistAudioSettings(volume float64) error {
	path := audioSettingsPath()
	if path == "" {
		return fmt.Errorf("JSON directory not available")
	}
	settings, _ := loadAudioSettings()
	settings.Volume = clampVolume(volume)
	settings.ApplyOnStartup = true
	settings.UpdatedAt = time.Now().Format(time.RFC3339)
	if app != nil && app.Config != nil && app.Config.SelectedAudioDevice != "" {
		settings.OutputDevice = app.Config.SelectedAudioDevice
	}
	return writeAudioSettings(path, settings)
}

func writeAudioSettings(path string, settings audioSettings) error {
	data, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func audioSettingsPath() string {
	if app == nil || app.Config.JSONDir == "" {
		return ""
	}
	return filepath.Join(app.Config.JSONDir, audioSettingsName)
}

func effectiveSoftwareVolume() float64 {
	if app.Config.CurrentVolume <= 0 {
		return 0
	}
	if systemMixerOwnsPlayback {
		return 1.0
	}
	return app.Config.CurrentVolume
}

func mixerHelpText() string {
	if runtime.GOOS == "linux" {
		return "On Raspberry Pi / Linux this slider sets ALSA mixer volume (same controls as alsamixer). Volume and the selected output device are saved and restored after reboot or power loss."
	}
	return "This slider controls application playback volume."
}

func isCaptureOnlyControl(name string) bool {
	n := strings.ToLower(name)
	for _, token := range []string{"capture", "mic", "microphone"} {
		if strings.Contains(n, token) {
			return true
		}
	}
	return false
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func clampVolume(volume float64) float64 {
	if volume < 0 {
		return 0
	}
	if volume > 1 {
		return 1
	}
	return volume
}
