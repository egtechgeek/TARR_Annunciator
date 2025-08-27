package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// System information structure
type SystemInfo struct {
	Uptime      string `json:"uptime"`
	MemoryUsage string `json:"memory_usage"`
	GoVersion   string `json:"go_version"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
}

// Bluetooth device structure
type BluetoothDevice struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	RSSI      int    `json:"rssi,omitempty"`
	Connected bool   `json:"connected"`
	Paired    bool   `json:"paired"`
}

// Global variables for system management
var (
	appStartTime    = time.Now()
	bluetoothScan   = make(chan bool, 1)
	bluetoothDevices = make([]BluetoothDevice, 0)
	pairedDevices   = make([]BluetoothDevice, 0)
)

// System Info Handler
func getSystemInfoHandler(c *gin.Context) {
	info := SystemInfo{
		Uptime:      getAppUptime(),
		MemoryUsage: getMemoryUsage(),
		GoVersion:   runtime.Version(),
		Platform:    runtime.GOOS,
		Arch:        runtime.GOARCH,
	}

	c.JSON(http.StatusOK, info)
}

// Get application uptime
func getAppUptime() string {
	uptime := time.Since(appStartTime)
	days := int(uptime.Hours()) / 24
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// Get memory usage
func getMemoryUsage() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Convert bytes to MB
	allocMB := bToMb(m.Alloc)
	sysMB := bToMb(m.Sys)
	
	return fmt.Sprintf("%.1f MB / %.1f MB", allocMB, sysMB)
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024 / 1024
}

// Restart Application Handler
func restartApplicationHandler(c *gin.Context) {
	log.Printf("Application restart requested by admin user")
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Application restart initiated",
	})

	// Use a goroutine to restart after a short delay
	go func() {
		time.Sleep(2 * time.Second)
		log.Printf("Restarting application...")
		
		if runtime.GOOS == "windows" {
			// On Windows, we'll use a batch script approach
			cmd := exec.Command("cmd", "/C", "timeout /T 3 && start", os.Args[0])
			cmd.Start()
		} else {
			// On Linux, use systemctl or direct restart
			if _, err := exec.LookPath("systemctl"); err == nil {
				// Try systemctl restart
				exec.Command("systemctl", "restart", "tarr-annunciator").Run()
			} else {
				// Direct restart
				cmd := exec.Command(os.Args[0])
				cmd.Start()
			}
		}
		
		// Exit current process
		os.Exit(0)
	}()
}

// Shutdown Application Handler
func shutdownApplicationHandler(c *gin.Context) {
	log.Printf("Application shutdown requested by admin user")
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Application shutdown initiated",
	})

	// Use a goroutine to shutdown after a short delay
	go func() {
		time.Sleep(2 * time.Second)
		log.Printf("Shutting down application...")
		os.Exit(0)
	}()
}

// Audio Device Redetection Handler
func redetectAudioDevicesHandler(c *gin.Context) {
	log.Printf("Audio device redetection requested")
	
	// Redetect audio devices
	devices := getAudioDevices()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"devices": devices,
		"count":   len(devices),
	})
}

// Bluetooth Scan Handler
func startBluetoothScanHandler(c *gin.Context) {
	log.Printf("Bluetooth scan requested")
	
	if runtime.GOOS == "windows" {
		// Try Windows Bluetooth scan
		go performWindowsBluetoothScan()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Windows Bluetooth scan started (limited functionality)",
		})
		return
	}

	// Clear previous scan results
	bluetoothDevices = make([]BluetoothDevice, 0)
	
	// Start Bluetooth scan
	go performBluetoothScan()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Bluetooth scan started",
	})
}

func stopBluetoothScanHandler(c *gin.Context) {
	// Signal scan to stop
	select {
	case bluetoothScan <- false:
	default:
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Bluetooth scan stopped",
	})
}

func getBluetoothDevicesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"devices": bluetoothDevices,
		"count":   len(bluetoothDevices),
	})
}

func getPairedBluetoothDevicesHandler(c *gin.Context) {
	loadPairedBluetoothDevices()
	
	c.JSON(http.StatusOK, gin.H{
		"devices": pairedDevices,
		"count":   len(pairedDevices),
	})
}

func pairBluetoothDeviceHandler(c *gin.Context) {
	var data struct {
		Address string `json:"address"`
		Name    string `json:"name"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": "Invalid JSON data",
		})
		return
	}

	if data.Address == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": "Device address is required",
		})
		return
	}

	// Perform Bluetooth pairing
	err := pairBluetoothDevice(data.Address, data.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": fmt.Sprintf("Failed to pair device: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device paired successfully",
	})
}

func unpairBluetoothDeviceHandler(c *gin.Context) {
	var data struct {
		Address string `json:"address"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": "Invalid JSON data",
		})
		return
	}

	// Perform Bluetooth unpairing
	err := unpairBluetoothDevice(data.Address)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": fmt.Sprintf("Failed to unpair device: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device unpaired successfully",
	})
}

// Bluetooth scan implementation
func performBluetoothScan() {
	if runtime.GOOS == "windows" {
		return
	}

	log.Printf("Starting Bluetooth device scan...")
	
	// Use hcitool or bluetoothctl depending on what's available
	var cmd *exec.Cmd
	if _, err := exec.LookPath("bluetoothctl"); err == nil {
		// Use bluetoothctl (more modern)
		cmd = exec.Command("bluetoothctl", "--timeout", "30", "scan", "on")
	} else if _, err := exec.LookPath("hcitool"); err == nil {
		// Use hcitool (legacy but widely available)  
		cmd = exec.Command("hcitool", "scan")
	} else {
		log.Printf("No Bluetooth tools available")
		return
	}

	output, err := cmd.Output()
	if err != nil {
		log.Printf("Bluetooth scan error: %v", err)
		return
	}

	parseBluetoothScanResults(string(output))
}

func parseBluetoothScanResults(output string) {
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		
		// Parse bluetooth scan results (format varies by tool)
		if strings.Contains(line, ":") && len(line) > 17 {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				address := parts[0]
				name := strings.Join(parts[1:], " ")
				
				// Check if it's a valid MAC address
				if isValidBluetoothAddress(address) {
					device := BluetoothDevice{
						Name:    name,
						Address: address,
						Paired:  false,
					}
					
					// Add to discovered devices if not already present
					found := false
					for _, existing := range bluetoothDevices {
						if existing.Address == address {
							found = true
							break
						}
					}
					
					if !found {
						bluetoothDevices = append(bluetoothDevices, device)
						log.Printf("Discovered Bluetooth device: %s (%s)", name, address)
					}
				}
			}
		}
	}
}

func isValidBluetoothAddress(addr string) bool {
	parts := strings.Split(addr, ":")
	return len(parts) == 6 && len(addr) == 17
}

func pairBluetoothDevice(address, name string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Bluetooth pairing not supported on Windows")
	}

	// Try to pair using bluetoothctl
	cmd := exec.Command("bluetoothctl", "pair", address)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("pairing failed: %v", err)
	}

	log.Printf("Paired with %s (%s): %s", name, address, string(output))
	
	// Try to connect after pairing
	connectCmd := exec.Command("bluetoothctl", "connect", address)
	connectCmd.Run() // Don't fail if connection fails
	
	return nil
}

func unpairBluetoothDevice(address string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Bluetooth unpairing not supported on Windows")
	}

	// Disconnect first
	disconnectCmd := exec.Command("bluetoothctl", "disconnect", address)
	disconnectCmd.Run()
	
	// Then remove/unpair
	cmd := exec.Command("bluetoothctl", "remove", address)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("unpairing failed: %v", err)
	}

	log.Printf("Unpaired device %s: %s", address, string(output))
	return nil
}

func loadPairedBluetoothDevices() {
	if runtime.GOOS == "windows" {
		pairedDevices = make([]BluetoothDevice, 0)
		return
	}

	pairedDevices = make([]BluetoothDevice, 0)
	
	// Get paired devices using bluetoothctl
	cmd := exec.Command("bluetoothctl", "paired-devices")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting paired devices: %v", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Device ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				address := parts[1]
				name := strings.Join(parts[2:], " ")
				
				// Check connection status
				statusCmd := exec.Command("bluetoothctl", "info", address)
				statusOutput, _ := statusCmd.Output()
				connected := strings.Contains(string(statusOutput), "Connected: yes")
				
				device := BluetoothDevice{
					Name:      name,
					Address:   address,
					Connected: connected,
					Paired:    true,
				}
				
				pairedDevices = append(pairedDevices, device)
			}
		}
	}
}

// ============== WINDOWS BLUETOOTH IMPLEMENTATION ==============

// performWindowsBluetoothScan performs Bluetooth device discovery on Windows
func performWindowsBluetoothScan() {
	log.Printf("Starting Windows Bluetooth device scan...")
	
	// Clear previous scan results
	bluetoothDevices = make([]BluetoothDevice, 0)
	
	// Use PowerShell to discover Bluetooth devices (simplified approach)
	psCommand := `
	Get-PnpDevice -Class Bluetooth | Where-Object {$_.Status -eq "OK"} | Select-Object FriendlyName, InstanceId | ConvertTo-Json`
	
	cmd := exec.Command("powershell", "-Command", psCommand)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Windows Bluetooth scan error: %v", err)
		
		// Fallback: Add a mock device to show functionality
		bluetoothDevices = append(bluetoothDevices, BluetoothDevice{
			Name:      "Windows Bluetooth Device (Mock)",
			Address:   "00:00:00:00:00:00",
			Paired:    false,
			Connected: false,
		})
		return
	}
	
	parseWindowsBluetoothResults(string(output))
}

// parseWindowsBluetoothResults parses Windows PowerShell Bluetooth scan results
func parseWindowsBluetoothResults(output string) {
	lines := strings.Split(output, "\n")
	deviceCount := 0
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		
		// Look for device names in the output
		if strings.Contains(line, "FriendlyName") {
			// Extract device name (simplified parsing)
			if name := extractSimpleJsonValue(line, "FriendlyName"); name != "" {
				deviceCount++
				device := BluetoothDevice{
					Name:      name,
					Address:   fmt.Sprintf("WINDOWS-BT-%03d", deviceCount),
					Paired:    false,
					Connected: false,
				}
				
				bluetoothDevices = append(bluetoothDevices, device)
				log.Printf("Discovered Windows Bluetooth device: %s", name)
			}
		}
	}
	
	// If no devices found, add informational entry
	if len(bluetoothDevices) == 0 {
		bluetoothDevices = append(bluetoothDevices, BluetoothDevice{
			Name:      "Windows Bluetooth (Limited Support)",
			Address:   "WINDOWS-INFO",
			Paired:    false,
			Connected: false,
		})
	}
}

// extractSimpleJsonValue extracts a value from JSON output (simplified)
func extractSimpleJsonValue(jsonStr, key string) string {
	// Very simple extraction for PowerShell JSON output
	pattern := `"` + key + `"\s*:\s*"([^"]*)"` 
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(jsonStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ============== AUDIO SYSTEM OVERRIDE HANDLERS ==============

// Global variable to store audio system override
var audioSystemOverride = "auto"

// audioSystemOverrideHandler handles requests to force a specific audio system
func audioSystemOverrideHandler(c *gin.Context) {
	var data struct {
		System string `json:"system"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid JSON data",
		})
		return
	}

	// Validate the system selection
	validSystems := []string{"auto", "pipewire", "pulseaudio", "alsa"}
	isValid := false
	for _, system := range validSystems {
		if data.System == system {
			isValid = true
			break
		}
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid audio system. Must be one of: auto, pipewire, pulseaudio, alsa",
		})
		return
	}

	// Set the override
	audioSystemOverride = data.System
	log.Printf("Audio system override set to: %s", data.System)

	// Get audio devices with the new override
	devices := getAudioDevicesWithOverride(data.System)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Audio system override applied: %s", data.System),
		"system":  data.System,
		"devices": devices,
	})
}

// getPlatformInfoHandler returns platform information for the admin UI
func getPlatformInfoHandler(c *gin.Context) {
	platformInfo := getPlatformInfo()
	
	// Add detailed PipeWire diagnostics for troubleshooting
	pipeWireDiagnostics := getPipeWireDiagnostics()
	
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"platform": platformInfo["platform"],
		"arch":     platformInfo["arch"],
		"is_arm":   platformInfo["is_arm"],
		"is_raspberry_pi": platformInfo["is_raspberry_pi"],
		"pipewire_available":  platformInfo["pipewire_available"],
		"pulse_available":     platformInfo["pulse_available"],
		"alsa_available":      platformInfo["alsa_available"],
		"preferred_audio_system": platformInfo["preferred_audio_system"],
		"pipewire_diagnostics": pipeWireDiagnostics,
	})
}

// getPipeWireDiagnostics provides detailed PipeWire diagnostic information
func getPipeWireDiagnostics() map[string]interface{} {
	diagnostics := make(map[string]interface{})
	
	// Check for PipeWire processes
	cmd := exec.Command("pgrep", "-f", "pipewire")
	if err := cmd.Run(); err == nil {
		diagnostics["pipewire_process_running"] = true
	} else {
		diagnostics["pipewire_process_running"] = false
	}
	
	// Check for WirePlumber
	cmd = exec.Command("pgrep", "-f", "wireplumber")
	if err := cmd.Run(); err == nil {
		diagnostics["wireplumber_running"] = true
	} else {
		diagnostics["wireplumber_running"] = false
	}
	
	// Check pw-cli availability
	cmd = exec.Command("pw-cli", "--version")
	if output, err := cmd.Output(); err == nil {
		diagnostics["pw_cli_available"] = true
		diagnostics["pw_cli_version"] = strings.TrimSpace(string(output))
	} else {
		diagnostics["pw_cli_available"] = false
		diagnostics["pw_cli_error"] = err.Error()
	}
	
	// Check wpctl availability
	cmd = exec.Command("wpctl", "--version")
	if output, err := cmd.Output(); err == nil {
		diagnostics["wpctl_available"] = true
		diagnostics["wpctl_version"] = strings.TrimSpace(string(output))
	} else {
		diagnostics["wpctl_available"] = false
		diagnostics["wpctl_error"] = err.Error()
	}
	
	// Check pactl availability (PulseAudio compatibility)
	cmd = exec.Command("pactl", "--version")
	if output, err := cmd.Output(); err == nil {
		diagnostics["pactl_available"] = true
		diagnostics["pactl_version"] = strings.TrimSpace(string(output))
		
		// Check if pactl can connect (indicates PipeWire or PulseAudio is running)
		cmd = exec.Command("pactl", "info")
		if _, err := cmd.Output(); err == nil {
			diagnostics["pactl_can_connect"] = true
		} else {
			diagnostics["pactl_can_connect"] = false
			diagnostics["pactl_connect_error"] = err.Error()
		}
	} else {
		diagnostics["pactl_available"] = false
		diagnostics["pactl_error"] = err.Error()
	}
	
	return diagnostics
}