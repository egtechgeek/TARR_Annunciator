package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"log"
	"regexp"
)

type AudioDevice struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Type      string `json:"type,omitempty"` // "pulse", "alsa", "windows"
}

// getAudioDevices retrieves available audio devices based on the current platform
func getAudioDevices() []AudioDevice {
	switch runtime.GOOS {
	case "windows":
		return getWindowsAudioDevices()
	case "linux":
		return getLinuxAudioDevices()
	case "darwin":
		return getDarwinAudioDevices()
	default:
		log.Printf("Unsupported platform: %s", runtime.GOOS)
		return getDefaultAudioDevice()
	}
}

// setAudioDevice sets the default audio device based on the current platform
func setAudioDevice(deviceID string) error {
	if deviceID == "default" || deviceID == "" {
		return nil // No change needed for default
	}

	switch runtime.GOOS {
	case "windows":
		return setWindowsAudioDevice(deviceID)
	case "linux":
		return setLinuxAudioDevice(deviceID)
	case "darwin":
		return setDarwinAudioDevice(deviceID)
	default:
		return fmt.Errorf("audio device setting not supported on %s", runtime.GOOS)
	}
}

// ============== WINDOWS IMPLEMENTATION ==============

func getWindowsAudioDevices() []AudioDevice {
	devices := []AudioDevice{}

	// First try with AudioDeviceCmdlets module
	psCommand := `if (Get-Module -ListAvailable -Name AudioDeviceCmdlets) {
		Import-Module AudioDeviceCmdlets -Force
		Get-AudioDevice -list | Where-Object {$_.Type -eq "Playback"} | Select-Object Name, ID, Default | ConvertTo-Json
	} else {
		throw "AudioDeviceCmdlets module not available"
	}`

	cmd := exec.Command("powershell", "-Command", psCommand)
	output, err := cmd.Output()

	if err != nil {
		log.Printf("AudioDeviceCmdlets not available, trying WMI: %v", err)
		return getWindowsAudioDevicesWMI()
	}

	// Parse JSON output
	var rawDevices interface{}
	if err := json.Unmarshal(output, &rawDevices); err != nil {
		log.Printf("Error parsing audio device JSON: %v", err)
		return getWindowsAudioDevicesWMI()
	}

	// Handle single device or array of devices
	switch v := rawDevices.(type) {
	case []interface{}:
		for _, deviceData := range v {
			if device, ok := deviceData.(map[string]interface{}); ok {
				audioDevice := AudioDevice{
					ID:        getString(device, "ID"),
					Name:      getString(device, "Name"),
					IsDefault: getBool(device, "Default"),
					Type:      "windows",
				}
				if audioDevice.Name != "" {
					devices = append(devices, audioDevice)
				}
			}
		}
	case map[string]interface{}:
		audioDevice := AudioDevice{
			ID:        getString(v, "ID"),
			Name:      getString(v, "Name"),
			IsDefault: getBool(v, "Default"),
			Type:      "windows",
		}
		if audioDevice.Name != "" {
			devices = append(devices, audioDevice)
		}
	}

	// Fallback if no devices found
	if len(devices) == 0 {
		return getDefaultAudioDevice()
	}

	return devices
}

func getWindowsAudioDevicesWMI() []AudioDevice {
	devices := []AudioDevice{}

	// Fallback PowerShell command using WMI
	psCommand := `Get-WmiObject -Class Win32_SoundDevice | Where-Object {$_.Status -eq "OK"} | Select-Object Name, DeviceID | ConvertTo-Json`

	cmd := exec.Command("powershell", "-Command", psCommand)
	output, err := cmd.Output()

	if err != nil {
		log.Printf("Error getting audio devices via WMI: %v", err)
		return getDefaultAudioDevice()
	}

	// Parse JSON output
	var rawDevices interface{}
	if err := json.Unmarshal(output, &rawDevices); err != nil {
		log.Printf("Error parsing WMI device JSON: %v", err)
		return getDefaultAudioDevice()
	}

	// Handle single device or array of devices
	switch v := rawDevices.(type) {
	case []interface{}:
		for i, deviceData := range v {
			if device, ok := deviceData.(map[string]interface{}); ok {
				audioDevice := AudioDevice{
					ID:        getString(device, "DeviceID"),
					Name:      getString(device, "Name"),
					IsDefault: i == 0, // First device as default
					Type:      "windows",
				}
				if audioDevice.Name != "" {
					devices = append(devices, audioDevice)
				}
			}
		}
	case map[string]interface{}:
		audioDevice := AudioDevice{
			ID:        getString(v, "DeviceID"),
			Name:      getString(v, "Name"),
			IsDefault: true,
			Type:      "windows",
		}
		if audioDevice.Name != "" {
			devices = append(devices, audioDevice)
		}
	}

	// Fallback if no devices found
	if len(devices) == 0 {
		return getDefaultAudioDevice()
	}

	return devices
}

func setWindowsAudioDevice(deviceID string) error {
	// PowerShell command to set audio device
	psCommand := fmt.Sprintf(`if (Get-Module -ListAvailable -Name AudioDeviceCmdlets) {
		Import-Module AudioDeviceCmdlets -Force
		Set-AudioDevice -ID "%s"
	} else {
		throw "AudioDeviceCmdlets module not available - cannot set audio device"
	}`, deviceID)

	cmd := exec.Command("powershell", "-Command", psCommand)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("Error setting Windows audio device (may need AudioDeviceCmdlets): %v, output: %s", err, string(output))
		return fmt.Errorf("failed to set Windows audio device - AudioDeviceCmdlets module may not be installed: %v", err)
	}

	log.Printf("Successfully set Windows audio device to: %s", deviceID)
	return nil
}

// ============== LINUX IMPLEMENTATION ==============

func getLinuxAudioDevices() []AudioDevice {
	// Check if we're on Raspberry Pi for specialized handling
	isRaspberryPi := detectRaspberryPi()
	
	// First try PulseAudio
	if devices := getPulseAudioDevices(); len(devices) > 0 {
		// On Raspberry Pi, enhance device names and add Pi-specific devices
		if isRaspberryPi {
			devices = enhanceRaspberryPiDevices(devices)
		}
		return devices
	}

	// Fallback to ALSA (common on Raspberry Pi)
	if devices := getALSAAudioDevices(); len(devices) > 0 {
		// On Raspberry Pi, enhance device names and add Pi-specific devices
		if isRaspberryPi {
			devices = enhanceRaspberryPiDevices(devices)
		}
		return devices
	}

	// Raspberry Pi specific fallback - add known Pi devices even if not detected
	if isRaspberryPi {
		return getRaspberryPiDefaultDevices()
	}

	// Ultimate fallback
	return getDefaultAudioDevice()
}

func getPulseAudioDevices() []AudioDevice {
	devices := []AudioDevice{}

	// Check if PulseAudio is available
	cmd := exec.Command("pactl", "info")
	if err := cmd.Run(); err != nil {
		log.Printf("PulseAudio not available: %v", err)
		return devices
	}

	// Get PulseAudio sinks (output devices)
	cmd = exec.Command("pactl", "list", "short", "sinks")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting PulseAudio sinks: %v", err)
		return devices
	}

	// Parse output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: index name driver sample_spec state
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			devices = append(devices, AudioDevice{
				ID:        parts[1], // sink name
				Name:      parts[1], // Use name as display name for now
				IsDefault: false,    // We'll check default separately
				Type:      "pulse",
			})
		}
	}

	// Get default sink
	cmd = exec.Command("pactl", "info")
	output, err = cmd.Output()
	if err == nil {
		re := regexp.MustCompile(`Default Sink: (.+)`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			defaultSink := strings.TrimSpace(matches[1])
			for i := range devices {
				if devices[i].ID == defaultSink {
					devices[i].IsDefault = true
					break
				}
			}
		}
	}

	// Try to get better device names
	for i := range devices {
		cmd = exec.Command("pactl", "list", "sinks")
		output, err := cmd.Output()
		if err == nil {
			// Parse detailed sink info to get description
			deviceInfo := string(output)
			sinkPattern := fmt.Sprintf(`Name: %s.*?Description: ([^\n\r]+)`, regexp.QuoteMeta(devices[i].ID))
			re := regexp.MustCompile(sinkPattern)
			matches := re.FindStringSubmatch(deviceInfo)
			if len(matches) > 1 {
				devices[i].Name = strings.TrimSpace(matches[1])
			}
		}
	}

	return devices
}

func getALSAAudioDevices() []AudioDevice {
	devices := []AudioDevice{}

	// Try aplay -l to list playback devices
	cmd := exec.Command("aplay", "-l")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("ALSA not available (aplay -l failed): %v", err)
		return devices
	}

	// Parse output
	// Format: card 0: PCH [HDA Intel PCH], device 0: ALC3246 Analog [ALC3246 Analog]
	lines := strings.Split(string(output), "\n")
	re := regexp.MustCompile(`card (\d+): (.+?) \[(.+?)\], device (\d+): (.+?) \[(.+?)\]`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 6 {
			cardNum := matches[1]
			deviceNum := matches[4]
			deviceName := matches[5]
			deviceID := fmt.Sprintf("hw:%s,%s", cardNum, deviceNum)

			devices = append(devices, AudioDevice{
				ID:        deviceID,
				Name:      deviceName,
				IsDefault: cardNum == "0" && deviceNum == "0", // First device as default
				Type:      "alsa",
			})
		}
	}

	return devices
}

func setLinuxAudioDevice(deviceID string) error {
	// Try PulseAudio first
	cmd := exec.Command("pactl", "info")
	if err := cmd.Run(); err == nil {
		// PulseAudio is available
		cmd = exec.Command("pactl", "set-default-sink", deviceID)
		if err := cmd.Run(); err != nil {
			log.Printf("Error setting PulseAudio default sink: %v", err)
			return fmt.Errorf("failed to set PulseAudio device: %v", err)
		}
		log.Printf("Successfully set PulseAudio default sink to: %s", deviceID)
		return nil
	}

	// For ALSA, we can't easily change the default device at runtime
	// ALSA defaults are typically configured in ~/.asoundrc or /etc/asound.conf
	log.Printf("ALSA device selection requires manual configuration in ~/.asoundrc")
	return fmt.Errorf("ALSA device selection not supported at runtime - please configure ~/.asoundrc manually")
}

// ============== MACOS IMPLEMENTATION ==============

func getDarwinAudioDevices() []AudioDevice {
	devices := []AudioDevice{}

	// Use system_profiler to get audio devices
	cmd := exec.Command("system_profiler", "SPAudioDataType", "-json")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting macOS audio devices: %v", err)
		return getDefaultAudioDevice()
	}

	// Parse JSON output (this is a simplified implementation)
	var data interface{}
	if err := json.Unmarshal(output, &data); err != nil {
		log.Printf("Error parsing macOS audio data: %v", err)
		return getDefaultAudioDevice()
	}

	// Add basic device (macOS audio device enumeration is complex)
	devices = append(devices, AudioDevice{
		ID:        "default",
		Name:      "Default Audio Device",
		IsDefault: true,
		Type:      "coreaudio",
	})

	return devices
}

func setDarwinAudioDevice(deviceID string) error {
	// macOS audio device setting would require more complex implementation
	// possibly using AppleScript or AudioUnit APIs
	log.Printf("macOS audio device selection not yet implemented")
	return fmt.Errorf("macOS audio device selection not yet implemented")
}

// ============== UTILITY FUNCTIONS ==============

func getDefaultAudioDevice() []AudioDevice {
	return []AudioDevice{{
		ID:        "default",
		Name:      "Default Audio Device",
		IsDefault: true,
		Type:      "default",
	}}
}

// Helper functions for parsing JSON
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// getPlatformInfo returns information about the current platform's audio system
func getPlatformInfo() map[string]interface{} {
	info := map[string]interface{}{
		"platform": runtime.GOOS,
		"arch":     runtime.GOARCH,
	}

	// Add ARM architecture detection
	isARM := runtime.GOARCH == "arm" || runtime.GOARCH == "arm64"
	info["is_arm"] = isARM
	
	// Detect if running on Raspberry Pi
	isRaspberryPi := detectRaspberryPi()
	info["is_raspberry_pi"] = isRaspberryPi
	
	if isRaspberryPi {
		info["pi_model"] = getRaspberryPiModel()
		info["pi_audio_config"] = getRaspberryPiAudioConfig()
	}

	switch runtime.GOOS {
	case "linux":
		// Check what audio systems are available
		pulseAvailable := false
		alsaAvailable := false
		jackAvailable := false

		if cmd := exec.Command("pactl", "info"); cmd.Run() == nil {
			pulseAvailable = true
		}
		if cmd := exec.Command("aplay", "--version"); cmd.Run() == nil {
			alsaAvailable = true
		}
		if cmd := exec.Command("jack_control", "status"); cmd.Run() == nil {
			jackAvailable = true
		}

		info["pulse_available"] = pulseAvailable
		info["alsa_available"] = alsaAvailable
		info["jack_available"] = jackAvailable
		
		// Raspberry Pi specific audio checks
		if isRaspberryPi {
			info["pi_audio_enabled"] = checkRaspberryPiAudio()
			info["pi_hdmi_audio"] = checkRaspberryPiHDMIAudio()
			info["pi_headphone_audio"] = checkRaspberryPiHeadphoneAudio()
		}

	case "windows":
		// Check if AudioDeviceCmdlets is available
		cmd := exec.Command("powershell", "-Command", "Get-Module -ListAvailable -Name AudioDeviceCmdlets")
		audioCmdletsAvailable := cmd.Run() == nil
		info["audiocmdlets_available"] = audioCmdletsAvailable
	}

	return info
}

// ============== RASPBERRY PI DETECTION ==============

// detectRaspberryPi checks if the system is running on a Raspberry Pi
func detectRaspberryPi() bool {
	// Check for Raspberry Pi specific files
	piFiles := []string{
		"/sys/firmware/devicetree/base/model",
		"/proc/device-tree/model",
		"/sys/class/dmi/id/board_name",
	}
	
	for _, file := range piFiles {
		if content, err := exec.Command("cat", file).Output(); err == nil {
			contentStr := strings.ToLower(string(content))
			if strings.Contains(contentStr, "raspberry pi") {
				return true
			}
		}
	}
	
	// Check /proc/cpuinfo for BCM2835/2836/2837/2711 (Pi processors)
	if content, err := exec.Command("cat", "/proc/cpuinfo").Output(); err == nil {
		contentStr := strings.ToLower(string(content))
		piProcessors := []string{"bcm2835", "bcm2836", "bcm2837", "bcm2711", "bcm2712"}
		for _, processor := range piProcessors {
			if strings.Contains(contentStr, processor) {
				return true
			}
		}
	}
	
	return false
}

// getRaspberryPiModel attempts to determine the Raspberry Pi model
func getRaspberryPiModel() string {
	// Try to read the model from device tree
	if content, err := exec.Command("cat", "/sys/firmware/devicetree/base/model").Output(); err == nil {
		model := strings.TrimSpace(string(content))
		// Remove null bytes that sometimes appear
		model = strings.ReplaceAll(model, "\x00", "")
		if model != "" {
			return model
		}
	}
	
	// Fallback to /proc/cpuinfo
	if content, err := exec.Command("grep", "Model", "/proc/cpuinfo").Output(); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Model") && strings.Contains(line, ":") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	
	return "Unknown Raspberry Pi"
}

// getRaspberryPiAudioConfig gets the current audio configuration
func getRaspberryPiAudioConfig() map[string]interface{} {
	config := make(map[string]interface{})
	
	// Check current audio output setting
	if output, err := exec.Command("amixer", "cget", "numid=3").Output(); err == nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "values=0") {
			config["output"] = "auto"
		} else if strings.Contains(outputStr, "values=1") {
			config["output"] = "headphone"
		} else if strings.Contains(outputStr, "values=2") {
			config["output"] = "hdmi"
		}
	}
	
	// Check if audio is enabled in config
	if content, err := exec.Command("grep", "-E", "^dtparam=audio", "/boot/config.txt").Output(); err == nil {
		if strings.Contains(string(content), "dtparam=audio=on") {
			config["config_enabled"] = true
		} else {
			config["config_enabled"] = false
		}
	}
	
	// Check for additional audio overlays
	if content, err := exec.Command("grep", "dtoverlay.*audio", "/boot/config.txt").Output(); err == nil {
		overlays := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(overlays) > 0 && overlays[0] != "" {
			config["audio_overlays"] = overlays
		}
	}
	
	return config
}

// checkRaspberryPiAudio checks if Raspberry Pi audio is properly configured
func checkRaspberryPiAudio() bool {
	// Check if the snd_bcm2835 module is loaded
	if err := exec.Command("lsmod").Run(); err == nil {
		if output, err := exec.Command("lsmod").Output(); err == nil {
			return strings.Contains(string(output), "snd_bcm2835")
		}
	}
	return false
}

// checkRaspberryPiHDMIAudio checks if HDMI audio is available
func checkRaspberryPiHDMIAudio() bool {
	// Check for HDMI audio device
	if output, err := exec.Command("aplay", "-l").Output(); err == nil {
		return strings.Contains(string(output), "HDMI") || strings.Contains(string(output), "hdmi")
	}
	return false
}

// checkRaspberryPiHeadphoneAudio checks if headphone audio is available  
func checkRaspberryPiHeadphoneAudio() bool {
	// Check for headphone/analog audio device
	if output, err := exec.Command("aplay", "-l").Output(); err == nil {
		outputStr := strings.ToLower(string(output))
		return strings.Contains(outputStr, "headphone") || 
			   strings.Contains(outputStr, "analog") ||
			   strings.Contains(outputStr, "bcm2835")
	}
	return false
}

// enhanceRaspberryPiDevices improves device names and adds Pi-specific information
func enhanceRaspberryPiDevices(devices []AudioDevice) []AudioDevice {
	enhanced := make([]AudioDevice, 0, len(devices))
	
	for _, device := range devices {
		enhancedDevice := device
		deviceName := strings.ToLower(device.Name)
		deviceID := strings.ToLower(device.ID)
		
		// Enhance names for common Raspberry Pi audio devices
		if strings.Contains(deviceName, "bcm2835") || strings.Contains(deviceID, "bcm2835") {
			if strings.Contains(deviceName, "hdmi") || strings.Contains(deviceID, "hdmi") {
				enhancedDevice.Name = "Raspberry Pi HDMI Audio"
			} else if strings.Contains(deviceName, "headphone") || 
					  strings.Contains(deviceName, "analog") ||
					  strings.Contains(deviceID, "analog") {
				enhancedDevice.Name = "Raspberry Pi Headphone/Analog Audio"
			} else {
				enhancedDevice.Name = "Raspberry Pi Audio (" + device.Name + ")"
			}
		}
		
		// Add Pi-specific type information
		if enhancedDevice.Type == "" {
			if strings.Contains(deviceID, "pulse") {
				enhancedDevice.Type = "pulse-pi"
			} else {
				enhancedDevice.Type = "alsa-pi"
			}
		}
		
		enhanced = append(enhanced, enhancedDevice)
	}
	
	return enhanced
}

// getRaspberryPiDefaultDevices returns default Raspberry Pi audio devices when detection fails
func getRaspberryPiDefaultDevices() []AudioDevice {
	devices := []AudioDevice{}
	
	// Add common Raspberry Pi audio devices
	devices = append(devices, AudioDevice{
		ID:        "hw:0,0",
		Name:      "Raspberry Pi Headphone/Analog Audio",
		IsDefault: true,
		Type:      "alsa-pi",
	})
	
	// Check if HDMI audio might be available
	if checkRaspberryPiHDMIAudio() {
		devices = append(devices, AudioDevice{
			ID:        "hw:0,1", 
			Name:      "Raspberry Pi HDMI Audio",
			IsDefault: false,
			Type:      "alsa-pi",
		})
	}
	
	// Add PulseAudio defaults if available
	if cmd := exec.Command("pactl", "info"); cmd.Run() == nil {
		devices = append(devices, AudioDevice{
			ID:        "alsa_output.platform-bcm2835_audio.analog-stereo",
			Name:      "Raspberry Pi Analog Audio (PulseAudio)",
			IsDefault: false,
			Type:      "pulse-pi",
		})
	}
	
	return devices
}

// setRaspberryPiAudioOutput sets the Raspberry Pi audio output mode
func setRaspberryPiAudioOutput(mode string) error {
	var value string
	switch strings.ToLower(mode) {
	case "auto", "0":
		value = "0"
	case "headphone", "analog", "1":
		value = "1"  
	case "hdmi", "2":
		value = "2"
	default:
		return fmt.Errorf("invalid audio output mode: %s (use auto, headphone, or hdmi)", mode)
	}
	
	// Use amixer to set the audio output
	cmd := exec.Command("amixer", "cset", "numid=3", value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set Raspberry Pi audio output: %v", err)
	}
	
	log.Printf("Successfully set Raspberry Pi audio output to mode %s", mode)
	return nil
}