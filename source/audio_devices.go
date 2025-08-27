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
	// Detect hardware platform for better audio support
	platform := detectLinuxPlatform()
	log.Printf("Detected platform: %s", platform)
	
	var devices []AudioDevice
	
	// For Raspberry Pi and OrangePi, use optimized audio detection order
	if platform == "raspberrypi" || platform == "orangepi" {
		log.Printf("Using Pi-optimized audio detection")
		
		// Try PipeWire first (modern Pi distributions)
		if pipeWireDevices := getPipeWireDevices(); len(pipeWireDevices) > 0 {
			log.Printf("Found %d PipeWire devices on Pi platform", len(pipeWireDevices))
			devices = append(devices, pipeWireDevices...)
		}
		
		// Try ALSA next for Pi systems (traditional approach)
		if len(devices) == 0 {
			if alsaDevices := getALSAAudioDevicesEnhanced(); len(alsaDevices) > 0 {
				log.Printf("Found %d ALSA devices on Pi platform", len(alsaDevices))
				devices = append(devices, alsaDevices...)
			}
		}
		
		// Only use PulseAudio if others don't work or user specifically wants it
		if len(devices) == 0 || isPulseAudioPreferred() {
			if pulseDevices := getPulseAudioDevices(); len(pulseDevices) > 0 {
				log.Printf("Found %d PulseAudio devices on Pi platform", len(pulseDevices))
				devices = append(devices, pulseDevices...)
			}
		}
		
		// Pi-specific device detection as fallback
		if len(devices) == 0 {
			devices = getPiAudioDevices(platform)
		}
		
		// Enhance device names for Pi platforms
		devices = enhancePiDevices(devices, platform)
	} else {
		// For regular Linux systems, try modern audio systems first
		
		// Try PipeWire first (modern Linux distributions)
		if pipeWireDevices := getPipeWireDevices(); len(pipeWireDevices) > 0 {
			log.Printf("Found %d PipeWire devices", len(pipeWireDevices))
			devices = append(devices, pipeWireDevices...)
		}
		
		// Try PulseAudio next (traditional approach)
		if len(devices) == 0 {
			if pulseDevices := getPulseAudioDevices(); len(pulseDevices) > 0 {
				log.Printf("Found %d PulseAudio devices", len(pulseDevices))
				devices = append(devices, pulseDevices...)
			}
		}
		
		// Try enhanced ALSA detection as fallback
		if len(devices) == 0 {
			if alsaDevices := getALSAAudioDevicesEnhanced(); len(alsaDevices) > 0 {
				log.Printf("Found %d ALSA devices", len(alsaDevices))
				devices = append(devices, alsaDevices...)
			}
		}
	}
	
	// Add default device if no devices found
	if len(devices) == 0 {
		log.Printf("No audio devices detected, using default")
		devices = getDefaultAudioDevice()
	}
	
	return devices
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

// getPipeWireDevices retrieves audio devices from PipeWire
func getPipeWireDevices() []AudioDevice {
	devices := []AudioDevice{}

	// Check if PipeWire is available using pw-cli
	cmd := exec.Command("pw-cli", "info")
	if err := cmd.Run(); err != nil {
		log.Printf("PipeWire not available (pw-cli): %v", err)
		
		// Try alternative PipeWire detection using wpctl (WirePlumber)
		cmd = exec.Command("wpctl", "status")
		if err := cmd.Run(); err != nil {
			log.Printf("PipeWire not available (wpctl): %v", err)
			
			// Try PipeWire through PulseAudio compatibility layer
			log.Printf("Trying PipeWire through PulseAudio compatibility layer")
			return getPipeWireDevicesThroughPulse()
		}
		
		// Use wpctl to get devices
		return getPipeWireDevicesWithWpctl()
	}

	// Get PipeWire nodes (sinks/outputs)
	cmd = exec.Command("pw-cli", "ls", "Node")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting PipeWire nodes: %v", err)
		// Try wpctl as fallback
		wpctlDevices := getPipeWireDevicesWithWpctl()
		if len(wpctlDevices) == 0 {
			// Try PulseAudio compatibility as final fallback
			log.Printf("Trying PipeWire through PulseAudio compatibility as fallback")
			return getPipeWireDevicesThroughPulse()
		}
		return wpctlDevices
	}

	devices = parsePipeWireNodes(string(output))
	
	// If no devices found with native PipeWire, try PulseAudio compatibility
	if len(devices) == 0 {
		log.Printf("No devices found with native PipeWire, trying PulseAudio compatibility")
		return getPipeWireDevicesThroughPulse()
	}
	
	// Enhance device information with additional details
	if len(devices) > 0 {
		enhancePipeWireDevices(devices)
	}

	return devices
}

// getPipeWireDevicesWithWpctl uses wpctl (WirePlumber) to get PipeWire devices
func getPipeWireDevicesWithWpctl() []AudioDevice {
	devices := []AudioDevice{}

	// Get audio sinks using wpctl
	cmd := exec.Command("wpctl", "status")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting PipeWire devices with wpctl: %v", err)
		return devices
	}

	// Parse wpctl output for audio sinks
	devices = parseWpctlOutput(string(output))
	
	return devices
}

// parsePipeWireNodes parses pw-cli Node output
func parsePipeWireNodes(output string) []AudioDevice {
	devices := []AudioDevice{}
	
	lines := strings.Split(output, "\n")
	var currentNode map[string]string
	var nodeID string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Look for node start (id followed by type)
		if strings.Contains(line, "id") && strings.Contains(line, "type PipeWire:Interface:Node") {
			// Extract node ID
			re := regexp.MustCompile(`id (\d+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				nodeID = matches[1]
				currentNode = make(map[string]string)
				currentNode["id"] = nodeID
			}
		}
		
		// Parse properties within a node
		if currentNode != nil && strings.Contains(line, "=") {
			// Look for relevant properties
			if strings.Contains(line, "node.description") {
				currentNode["description"] = extractPipeWireProperty(line)
			} else if strings.Contains(line, "node.name") {
				currentNode["name"] = extractPipeWireProperty(line)
			} else if strings.Contains(line, "media.class") {
				currentNode["class"] = extractPipeWireProperty(line)
			} else if strings.Contains(line, "node.nick") {
				currentNode["nick"] = extractPipeWireProperty(line)
			}
		}
		
		// End of node - process if it's an audio sink
		if currentNode != nil && (line == "" || strings.HasPrefix(line, "id")) && len(currentNode) > 1 {
			if class, exists := currentNode["class"]; exists && strings.Contains(class, "Audio/Sink") {
				device := AudioDevice{
					ID:        currentNode["id"],
					Name:      getPipeWireDisplayName(currentNode),
					IsDefault: false, // We'll determine default separately
					Type:      "pipewire",
				}
				devices = append(devices, device)
			}
			
			// Start new node if we see another ID line
			if strings.Contains(line, "id") && strings.Contains(line, "type PipeWire:Interface:Node") {
				re := regexp.MustCompile(`id (\d+)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					nodeID = matches[1]
					currentNode = make(map[string]string)
					currentNode["id"] = nodeID
				}
			} else {
				currentNode = nil
			}
		}
	}
	
	return devices
}

// parseWpctlOutput parses wpctl status output
func parseWpctlOutput(output string) []AudioDevice {
	devices := []AudioDevice{}
	
	lines := strings.Split(output, "\n")
	inAudioSection := false
	inSinksSection := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Track sections
		if strings.Contains(line, "Audio") {
			inAudioSection = true
			continue
		} else if inAudioSection && strings.Contains(line, "Sinks:") {
			inSinksSection = true
			continue
		} else if inAudioSection && (strings.Contains(line, "Sources:") || strings.Contains(line, "Sink endpoints:")) {
			inSinksSection = false
			continue
		} else if !inAudioSection || line == "" {
			continue
		}
		
		// Parse sink lines
		if inSinksSection && (strings.Contains(line, "*.") || strings.Contains(line, " ")) {
			device := parseWpctlSinkLine(line)
			if device.Name != "" {
				devices = append(devices, device)
			}
		}
	}
	
	return devices
}

// parseWpctlSinkLine parses a single wpctl sink line
func parseWpctlSinkLine(line string) AudioDevice {
	// Format examples:
	// │  ├─ 43. Built-in Audio Analog Stereo               [vol: 1.00]
	// │  ├─ *44. HDMI / DisplayPort - Built-in Audio       [vol: 0.65]
	
	device := AudioDevice{Type: "pipewire"}
	
	// Check if it's the default device (marked with *)
	device.IsDefault = strings.Contains(line, "*")
	
	// Extract device ID and name
	re := regexp.MustCompile(`\*?(\d+)\.\s+([^[]+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 2 {
		device.ID = strings.TrimSpace(matches[1])
		device.Name = strings.TrimSpace(matches[2])
	}
	
	return device
}

// extractPipeWireProperty extracts property value from PipeWire property line
func extractPipeWireProperty(line string) string {
	// Extract quoted values: property = "value"
	re := regexp.MustCompile(`=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// getPipeWireDisplayName gets the best display name for a PipeWire device
func getPipeWireDisplayName(nodeProps map[string]string) string {
	// Prefer description, then nick, then name
	if desc, exists := nodeProps["description"]; exists && desc != "" {
		return desc
	}
	if nick, exists := nodeProps["nick"]; exists && nick != "" {
		return nick
	}
	if name, exists := nodeProps["name"]; exists && name != "" {
		return name
	}
	return "PipeWire Audio Device"
}

// enhancePipeWireDevices adds additional information to PipeWire devices
func enhancePipeWireDevices(devices []AudioDevice) {
	// Try to determine the default device
	cmd := exec.Command("wpctl", "get-volume", "@DEFAULT_SINK@")
	if _, err := cmd.Output(); err == nil && len(devices) > 0 {
		// If we can get default sink volume, mark first device as default
		// This is a simplified approach - could be enhanced with better detection
		devices[0].IsDefault = true
	}
	
	// Add platform-specific enhancements
	platform := detectLinuxPlatform()
	if platform == "raspberrypi" || platform == "orangepi" {
		for i := range devices {
			// Enhance Pi device names
			if strings.Contains(strings.ToLower(devices[i].Name), "bcm") {
				if strings.Contains(strings.ToLower(devices[i].Name), "hdmi") {
					devices[i].Name = platform + " HDMI Audio (PipeWire)"
				} else {
					devices[i].Name = platform + " Analog Audio (PipeWire)"
				}
				devices[i].Type = "pipewire-" + platform
			}
		}
	}
}

// getPipeWireDevicesThroughPulse uses PulseAudio compatibility to detect PipeWire devices
func getPipeWireDevicesThroughPulse() []AudioDevice {
	devices := []AudioDevice{}
	
	// Check if PipeWire is running by looking for PipeWire processes
	isPipeWireRunning := false
	
	// Check for PipeWire processes
	cmd := exec.Command("pgrep", "-f", "pipewire")
	if err := cmd.Run(); err == nil {
		isPipeWireRunning = true
		log.Printf("PipeWire processes detected, using PulseAudio compatibility layer")
	} else {
		// Also check for wireplumber
		cmd = exec.Command("pgrep", "-f", "wireplumber")
		if err := cmd.Run(); err == nil {
			isPipeWireRunning = true
			log.Printf("WirePlumber detected, using PulseAudio compatibility layer")
		}
	}
	
	// Check if PulseAudio/PipeWire compatibility is available
	cmd = exec.Command("pactl", "info")
	if err := cmd.Run(); err != nil {
		log.Printf("PulseAudio compatibility layer not available: %v", err)
		return devices
	}
	
	// Get sinks using pactl (works with PipeWire's PulseAudio compatibility)
	cmd = exec.Command("pactl", "list", "short", "sinks")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting sinks via PulseAudio compatibility: %v", err)
		return devices
	}
	
	log.Printf("PulseAudio compatibility layer output: %s", string(output))
	
	// Parse output - similar to PulseAudio but mark as PipeWire devices
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: index name driver sample_spec state
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			device := AudioDevice{
				ID:        parts[1], // sink name
				Name:      parts[1], // Use name as display name initially
				IsDefault: false,    // We'll check default separately
				Type:      "pipewire-pulse", // Mark as PipeWire via PulseAudio compatibility
			}
			devices = append(devices, device)
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

	// Get better device names using pactl list sinks
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
				
				// Add PipeWire identifier to the name if PipeWire is detected
				if isPipeWireRunning && !strings.Contains(devices[i].Name, "PipeWire") {
					devices[i].Name += " (PipeWire)"
				}
			}
		}
	}

	// Enhance for Raspberry Pi
	if detectLinuxPlatform() == "raspberrypi" {
		devices = enhancePiPipeWireDevices(devices)
	}

	return devices
}

// enhancePiPipeWireDevices enhances PipeWire device names specifically for Raspberry Pi
func enhancePiPipeWireDevices(devices []AudioDevice) []AudioDevice {
	enhanced := make([]AudioDevice, 0, len(devices))
	
	for _, device := range devices {
		enhancedDevice := device
		deviceName := strings.ToLower(device.Name)
		
		// Enhance common Raspberry Pi audio device names
		if strings.Contains(deviceName, "bcm2835") || strings.Contains(deviceName, "vc4-hdmi") {
			if strings.Contains(deviceName, "hdmi") || strings.Contains(deviceName, "vc4") {
				enhancedDevice.Name = "Raspberry Pi HDMI Audio (PipeWire)"
				enhancedDevice.Type = "pipewire-pi-hdmi"
			} else {
				enhancedDevice.Name = "Raspberry Pi Headphone/Analog Audio (PipeWire)"  
				enhancedDevice.Type = "pipewire-pi-analog"
			}
		} else if strings.Contains(deviceName, "built-in") || strings.Contains(deviceName, "analog") {
			enhancedDevice.Name = "Raspberry Pi " + device.Name
			enhancedDevice.Type = "pipewire-pi"
		}
		
		enhanced = append(enhanced, enhancedDevice)
	}
	
	// If no enhanced devices and we're on Pi, add some defaults
	if len(enhanced) == 0 {
		enhanced = append(enhanced, AudioDevice{
			ID:        "alsa_output.platform-bcm2835_audio.analog-stereo",
			Name:      "Raspberry Pi Analog Audio (PipeWire/Pulse Compat)",
			IsDefault: true,
			Type:      "pipewire-pi",
		})
	}
	
	return enhanced
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
	// Try PipeWire first (most modern)
	cmd := exec.Command("wpctl", "set-default", deviceID)
	if err := cmd.Run(); err == nil {
		log.Printf("Successfully set PipeWire default sink to: %s", deviceID)
		return nil
	}

	// Try PulseAudio next
	cmd = exec.Command("pactl", "info")
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
		pipeWireAvailable := false
		pulseAvailable := false
		alsaAvailable := false
		jackAvailable := false

		// Check PipeWire (native tools)
		if cmd := exec.Command("wpctl", "status"); cmd.Run() == nil {
			pipeWireAvailable = true
		} else if cmd := exec.Command("pw-cli", "info"); cmd.Run() == nil {
			pipeWireAvailable = true
		} else {
			// Check PipeWire via PulseAudio compatibility layer
			if cmd := exec.Command("pgrep", "-f", "pipewire"); cmd.Run() == nil {
				if cmd := exec.Command("pactl", "info"); cmd.Run() == nil {
					pipeWireAvailable = true
					log.Printf("PipeWire detected via PulseAudio compatibility layer")
				}
			}
		}

		if cmd := exec.Command("pactl", "info"); cmd.Run() == nil {
			pulseAvailable = true
		}
		if cmd := exec.Command("aplay", "--version"); cmd.Run() == nil {
			alsaAvailable = true
		}
		if cmd := exec.Command("jack_control", "status"); cmd.Run() == nil {
			jackAvailable = true
		}

		info["pipewire_available"] = pipeWireAvailable
		info["pulse_available"] = pulseAvailable
		info["alsa_available"] = alsaAvailable
		info["jack_available"] = jackAvailable
		
		// Determine the preferred audio system
		if pipeWireAvailable {
			info["preferred_audio_system"] = "pipewire"
		} else if pulseAvailable {
			info["preferred_audio_system"] = "pulseaudio"
		} else if alsaAvailable {
			info["preferred_audio_system"] = "alsa"
		} else {
			info["preferred_audio_system"] = "none"
		}
		
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
			if strings.Contains(deviceID, "pipewire") {
				enhancedDevice.Type = "pipewire-pi"
			} else if strings.Contains(deviceID, "pulse") {
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
	
	// Add PipeWire defaults if available
	if cmd := exec.Command("wpctl", "status"); cmd.Run() == nil {
		devices = append(devices, AudioDevice{
			ID:        "alsa_output.platform-bcm2835_audio.analog-stereo",
			Name:      "Raspberry Pi Analog Audio (PipeWire)",
			IsDefault: false,
			Type:      "pipewire-pi",
		})
	} else if cmd := exec.Command("pactl", "info"); cmd.Run() == nil {
		// Fallback to PulseAudio if PipeWire not available
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

// ============== ENHANCED PI SUPPORT FUNCTIONS ==============

// detectLinuxPlatform detects specific Linux platform (Raspberry Pi, OrangePi, etc.)
func detectLinuxPlatform() string {
	// Check for Raspberry Pi first
	if detectRaspberryPi() {
		return "raspberrypi"
	}
	
	// Check for OrangePi
	if detectOrangePi() {
		return "orangepi"
	}
	
	// Check for other ARM-based boards
	if detectARMBoard() {
		return "armboard"
	}
	
	return "linux"
}

// detectOrangePi checks if the system is running on an OrangePi
func detectOrangePi() bool {
	// Check for OrangePi specific files and identifiers
	piFiles := []string{
		"/sys/firmware/devicetree/base/model",
		"/proc/device-tree/model",
		"/sys/class/dmi/id/board_name",
	}
	
	for _, file := range piFiles {
		if content, err := exec.Command("cat", file).Output(); err == nil {
			contentStr := strings.ToLower(string(content))
			if strings.Contains(contentStr, "orange pi") || 
			   strings.Contains(contentStr, "orangepi") {
				return true
			}
		}
	}
	
	// Check /proc/cpuinfo for Allwinner processors (common in OrangePi)
	if content, err := exec.Command("cat", "/proc/cpuinfo").Output(); err == nil {
		contentStr := strings.ToLower(string(content))
		orangeProcessors := []string{"allwinner", "sun8i", "sun50i", "h3", "h5", "h6"}
		for _, processor := range orangeProcessors {
			if strings.Contains(contentStr, processor) {
				return true
			}
		}
	}
	
	// Check for OrangePi in hostname or other system files
	if content, err := exec.Command("hostname").Output(); err == nil {
		contentStr := strings.ToLower(string(content))
		if strings.Contains(contentStr, "orange") {
			return true
		}
	}
	
	return false
}

// detectARMBoard detects other ARM-based single board computers
func detectARMBoard() bool {
	// Check if we're on ARM architecture
	isARM := runtime.GOARCH == "arm" || runtime.GOARCH == "arm64"
	if !isARM {
		return false
	}
	
	// Check for common ARM board indicators
	if content, err := exec.Command("cat", "/proc/cpuinfo").Output(); err == nil {
		contentStr := strings.ToLower(string(content))
		armBoards := []string{"rockchip", "amlogic", "broadcom", "qualcomm"}
		for _, board := range armBoards {
			if strings.Contains(contentStr, board) {
				return true
			}
		}
	}
	
	return false
}

// getALSAAudioDevicesEnhanced provides enhanced ALSA device detection
func getALSAAudioDevicesEnhanced() []AudioDevice {
	devices := []AudioDevice{}
	
	// First try the basic ALSA detection
	basicDevices := getALSAAudioDevices()
	devices = append(devices, basicDevices...)
	
	// Try alternative ALSA detection methods
	if len(devices) == 0 {
		// Try using /proc/asound/cards
		if procDevices := getALSADevicesFromProc(); len(procDevices) > 0 {
			devices = append(devices, procDevices...)
		}
	}
	
	// Try using amixer to get more detailed info
	if len(devices) > 0 {
		enhanceALSADevicesWithAmixer(devices)
	}
	
	return devices
}

// getALSADevicesFromProc reads ALSA devices from /proc/asound/cards
func getALSADevicesFromProc() []AudioDevice {
	devices := []AudioDevice{}
	
	if content, err := exec.Command("cat", "/proc/asound/cards").Output(); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			// Format: 0 [PCH           ]: HDA-Intel - HDA Intel PCH
			re := regexp.MustCompile(`^(\d+)\s+\[([^\]]+)\]\s*:\s*(.+?)\s*-\s*(.+)$`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 4 {
				cardNum := matches[1]
				deviceName := strings.TrimSpace(matches[4])
				
				// Create device ID
				deviceID := fmt.Sprintf("hw:%s,0", cardNum)
				
				devices = append(devices, AudioDevice{
					ID:        deviceID,
					Name:      deviceName,
					IsDefault: cardNum == "0",
					Type:      "alsa",
				})
			}
		}
	}
	
	return devices
}

// enhanceALSADevicesWithAmixer uses amixer to get better device information
func enhanceALSADevicesWithAmixer(devices []AudioDevice) {
	for i := range devices {
		// Try to get volume controls for this device
		cmd := exec.Command("amixer", "-c", extractCardNumber(devices[i].ID), "scontrols")
		if output, err := cmd.Output(); err == nil {
			controls := string(output)
			if strings.Contains(controls, "Master") {
				devices[i].Name += " (Master Volume)"
			} else if strings.Contains(controls, "PCM") {
				devices[i].Name += " (PCM)"
			}
		}
	}
}

// extractCardNumber extracts card number from device ID like "hw:0,0"
func extractCardNumber(deviceID string) string {
	parts := strings.Split(deviceID, ":")
	if len(parts) > 1 {
		cardParts := strings.Split(parts[1], ",")
		if len(cardParts) > 0 {
			return cardParts[0]
		}
	}
	return "0"
}

// isPulseAudioPreferred checks if user prefers PulseAudio over ALSA
func isPulseAudioPreferred() bool {
	// Check environment variable
	if preference := strings.ToLower(strings.TrimSpace(exec.Command("echo", "$TARR_AUDIO_PREFERENCE").String())); preference == "pulse" {
		return true
	}
	
	// Check if PulseAudio is running and has active sinks
	if cmd := exec.Command("pactl", "list", "short", "sinks"); cmd.Run() == nil {
		if output, err := cmd.Output(); err == nil && len(strings.TrimSpace(string(output))) > 0 {
			return true
		}
	}
	
	return false
}

// getPiAudioDevices returns platform-specific default audio devices for Pi systems
func getPiAudioDevices(platform string) []AudioDevice {
	switch platform {
	case "raspberrypi":
		return getRaspberryPiDefaultDevices()
	case "orangepi":
		return getOrangePiDefaultDevices()
	default:
		return getARMBoardDefaultDevices()
	}
}

// getOrangePiDefaultDevices returns default audio devices for OrangePi
func getOrangePiDefaultDevices() []AudioDevice {
	devices := []AudioDevice{}
	
	// Common OrangePi audio devices
	devices = append(devices, AudioDevice{
		ID:        "hw:0,0",
		Name:      "OrangePi Analog Audio",
		IsDefault: true,
		Type:      "alsa-orangepi",
	})
	
	// Check for HDMI audio (common on OrangePi boards)
	if output, err := exec.Command("aplay", "-l").Output(); err == nil {
		if strings.Contains(strings.ToLower(string(output)), "hdmi") {
			devices = append(devices, AudioDevice{
				ID:        "hw:1,0",
				Name:      "OrangePi HDMI Audio",
				IsDefault: false,
				Type:      "alsa-orangepi",
			})
		}
	}
	
	// Add PipeWire defaults if available
	if cmd := exec.Command("wpctl", "status"); cmd.Run() == nil {
		devices = append(devices, AudioDevice{
			ID:        "alsa_output.platform-snd_soc_dummy.analog-stereo",
			Name:      "OrangePi Audio (PipeWire)",
			IsDefault: false,
			Type:      "pipewire-orangepi",
		})
	} else if cmd := exec.Command("pactl", "info"); cmd.Run() == nil {
		// Fallback to PulseAudio if PipeWire not available
		devices = append(devices, AudioDevice{
			ID:        "alsa_output.platform-snd_soc_dummy.analog-stereo",
			Name:      "OrangePi Audio (PulseAudio)",
			IsDefault: false,
			Type:      "pulse-orangepi",
		})
	}
	
	return devices
}

// getARMBoardDefaultDevices returns default audio devices for other ARM boards
func getARMBoardDefaultDevices() []AudioDevice {
	return []AudioDevice{{
		ID:        "hw:0,0",
		Name:      "ARM Board Audio",
		IsDefault: true,
		Type:      "alsa-arm",
	}}
}

// enhancePiDevices enhances device names and information for Pi platforms
func enhancePiDevices(devices []AudioDevice, platform string) []AudioDevice {
	switch platform {
	case "raspberrypi":
		return enhanceRaspberryPiDevices(devices)
	case "orangepi":
		return enhanceOrangePiDevices(devices)
	default:
		return enhanceARMBoardDevices(devices)
	}
}

// enhanceOrangePiDevices improves device names for OrangePi systems
func enhanceOrangePiDevices(devices []AudioDevice) []AudioDevice {
	enhanced := make([]AudioDevice, 0, len(devices))
	
	for _, device := range devices {
		enhancedDevice := device
		deviceName := strings.ToLower(device.Name)
		deviceID := strings.ToLower(device.ID)
		
		// Enhance names for common OrangePi audio devices
		if strings.Contains(deviceName, "sun") || strings.Contains(deviceID, "sun") ||
		   strings.Contains(deviceName, "allwinner") {
			if strings.Contains(deviceName, "hdmi") || strings.Contains(deviceID, "hdmi") {
				enhancedDevice.Name = "OrangePi HDMI Audio"
			} else {
				enhancedDevice.Name = "OrangePi Analog Audio"
			}
		} else if !strings.Contains(deviceName, "orangepi") {
			// Add OrangePi prefix if not already present
			enhancedDevice.Name = "OrangePi " + device.Name
		}
		
		// Add platform-specific type information
		if enhancedDevice.Type == "" {
			if strings.Contains(deviceID, "pipewire") {
				enhancedDevice.Type = "pipewire-orangepi"
			} else if strings.Contains(deviceID, "pulse") {
				enhancedDevice.Type = "pulse-orangepi"
			} else {
				enhancedDevice.Type = "alsa-orangepi"
			}
		}
		
		enhanced = append(enhanced, enhancedDevice)
	}
	
	return enhanced
}

// enhanceARMBoardDevices improves device names for generic ARM boards
func enhanceARMBoardDevices(devices []AudioDevice) []AudioDevice {
	enhanced := make([]AudioDevice, 0, len(devices))
	
	for _, device := range devices {
		enhancedDevice := device
		
		// Add ARM board prefix if not already descriptive
		if !strings.Contains(strings.ToLower(device.Name), "arm") && 
		   !strings.Contains(strings.ToLower(device.Name), "board") {
			enhancedDevice.Name = "ARM Board " + device.Name
		}
		
		// Add type information
		if enhancedDevice.Type == "" {
			if strings.Contains(strings.ToLower(device.ID), "pipewire") {
				enhancedDevice.Type = "pipewire-arm"
			} else if strings.Contains(strings.ToLower(device.ID), "pulse") {
				enhancedDevice.Type = "pulse-arm"
			} else {
				enhancedDevice.Type = "alsa-arm"
			}
		}
		
		enhanced = append(enhanced, enhancedDevice)
	}
	
	return enhanced
}

// ============== AUDIO SYSTEM OVERRIDE FUNCTIONS ==============

// getAudioDevicesWithOverride gets audio devices using a specific audio system override
func getAudioDevicesWithOverride(systemOverride string) []AudioDevice {
	if systemOverride == "auto" {
		return getAudioDevices()
	}
	
	log.Printf("Using audio system override: %s", systemOverride)
	
	switch runtime.GOOS {
	case "windows":
		// Windows doesn't support audio system overrides
		return getAudioDevices()
	case "linux":
		return getLinuxAudioDevicesWithOverride(systemOverride)
	case "darwin":
		// macOS doesn't support audio system overrides
		return getAudioDevices()
	default:
		return getDefaultAudioDevice()
	}
}

// getLinuxAudioDevicesWithOverride gets Linux audio devices using a specific system
func getLinuxAudioDevicesWithOverride(systemOverride string) []AudioDevice {
	platform := detectLinuxPlatform()
	var devices []AudioDevice
	
	log.Printf("Audio system override: %s on platform: %s", systemOverride, platform)
	
	switch systemOverride {
	case "pipewire":
		if pipeWireDevices := getPipeWireDevices(); len(pipeWireDevices) > 0 {
			log.Printf("Found %d PipeWire devices (forced)", len(pipeWireDevices))
			devices = append(devices, pipeWireDevices...)
		} else {
			log.Printf("No PipeWire devices found (forced)")
		}
		
	case "pulseaudio":
		if pulseDevices := getPulseAudioDevices(); len(pulseDevices) > 0 {
			log.Printf("Found %d PulseAudio devices (forced)", len(pulseDevices))
			devices = append(devices, pulseDevices...)
		} else {
			log.Printf("No PulseAudio devices found (forced)")
		}
		
	case "alsa":
		if alsaDevices := getALSAAudioDevicesEnhanced(); len(alsaDevices) > 0 {
			log.Printf("Found %d ALSA devices (forced)", len(alsaDevices))
			devices = append(devices, alsaDevices...)
		} else {
			log.Printf("No ALSA devices found, trying Pi-specific detection (forced)")
			// For Pi systems, try the Pi-specific ALSA detection
			if platform == "raspberrypi" || platform == "orangepi" {
				devices = getPiAudioDevices(platform)
			}
		}
	}
	
	// If no devices found, provide fallback based on platform
	if len(devices) == 0 {
		log.Printf("No devices found with override %s, using platform fallback", systemOverride)
		if platform == "raspberrypi" || platform == "orangepi" {
			devices = getPiAudioDevices(platform)
		} else {
			devices = getDefaultAudioDevice()
		}
	}
	
	// Enhance device names for Pi platforms
	if platform == "raspberrypi" || platform == "orangepi" {
		devices = enhancePiDevices(devices, platform)
	}
	
	return devices
}