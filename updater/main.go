package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	GITHUB_API_BASE = "https://api.github.com/repos/egtechgeek/TARR_Annunciator"
	GITHUB_RAW_BASE = "https://raw.githubusercontent.com/egtechgeek/TARR_Annunciator/main"
	USER_AGENT      = "TARR-Annunciator-Updater/1.0"
)

type UpdaterConfig struct {
	CurrentVersion string `json:"current_version"`
	LastCheck      string `json:"last_check"`
	AutoUpdate     bool   `json:"auto_update"`
}

type FileVersion struct {
	Path         string    `json:"path"`
	Version      string    `json:"version"`
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	Source       string    `json:"source"` // "local", "github", etc.
}

type VersionManifest struct {
	ApplicationVersion string                   `json:"application_version"`
	ManifestVersion    string                   `json:"manifest_version"`
	LastUpdated        time.Time                `json:"last_updated"`
	Files              map[string]FileVersion   `json:"files"`
	Platform           string                   `json:"platform"`
	Architecture       string                   `json:"architecture"`
}

type RemoteManifest struct {
	LatestVersion      string                   `json:"latest_version"`
	ManifestVersion    string                   `json:"manifest_version"`
	Files              map[string]FileVersion   `json:"files"`
	RequiredFiles      []string                 `json:"required_files"`
	OptionalFiles      []string                 `json:"optional_files"`
	PlatformSupport    map[string]bool          `json:"platform_support"`
}

type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GitURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
}

type SystemInfo struct {
	OS           string
	Architecture string
	ExecutableName string
	ExecutablePath string
}

func main() {
	fmt.Println("🔄 TARR Annunciator Updater v2.0")
	fmt.Println("Enhanced with Version Tracking & Efficient Updates")
	fmt.Println("===================================================")
	
	// Detect system information
	sysInfo := detectSystem()
	fmt.Printf("📱 Detected System: %s/%s\n", sysInfo.OS, sysInfo.Architecture)
	fmt.Printf("🎯 Target Executable: %s\n", sysInfo.ExecutableName)
	
	// Load updater configuration
	config := loadUpdaterConfig()
	fmt.Printf("📅 Last Check: %s\n", config.LastCheck)
	
	fmt.Println("\n🔍 Checking for updates...")
	
	// Try version-based update first (more efficient)
	if err := checkVersionBasedUpdate(); err != nil {
		log.Printf("❌ Error in version-based update: %v", err)
		fmt.Println("🔄 Falling back to traditional update method...")
		
		// Fallback to traditional update methods
		if err := checkExecutableUpdates(sysInfo, config); err != nil {
			log.Printf("❌ Error checking executable updates: %v", err)
		}
		
		if err := checkDataUpdates(config); err != nil {
			log.Printf("❌ Error checking data updates: %v", err)
		}
	} else {
		fmt.Println("📊 Version-based update completed successfully!")
	}
	
	// Update last check time
	config.LastCheck = time.Now().Format(time.RFC3339)
	saveUpdaterConfig(config)
	
	fmt.Println("\n✅ Update check complete!")
}

func detectSystem() SystemInfo {
	sysInfo := SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
	
	// Normalize OS names to match GitHub repository structure
	switch sysInfo.OS {
	case "windows":
		sysInfo.ExecutableName = "tarr-annunciator.exe"
		sysInfo.ExecutablePath = "tarr-annunciator.exe"
	case "linux":
		sysInfo.ExecutableName = "tarr-annunciator"
		sysInfo.ExecutablePath = "tarr-annunciator"
	case "darwin":
		sysInfo.ExecutableName = "tarr-annunciator"
		sysInfo.ExecutablePath = "tarr-annunciator"
	default:
		sysInfo.ExecutableName = "tarr-annunciator"
		sysInfo.ExecutablePath = "tarr-annunciator"
	}
	
	return sysInfo
}

func loadUpdaterConfig() UpdaterConfig {
	config := UpdaterConfig{
		CurrentVersion: "unknown",
		LastCheck:      "never",
		AutoUpdate:     false,
	}
	
	configPath := "updater_config.json"
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &config)
	}
	
	return config
}

func saveUpdaterConfig(config UpdaterConfig) error {
	configPath := "updater_config.json"
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}

func checkExecutableUpdates(sysInfo SystemInfo, config UpdaterConfig) error {
	fmt.Println("\n🔍 Checking for executable updates...")
	
	// Get directory listing from GitHub API
	url := fmt.Sprintf("%s/contents/compiled_packages", GITHUB_API_BASE)
	contents, err := getGitHubDirectoryContents(url)
	if err != nil {
		return fmt.Errorf("failed to get compiled packages directory: %v", err)
	}
	
	// Find the appropriate executable for our system
	var targetFile *GitHubContent
	expectedFilename := getExpectedExecutableFilename(sysInfo)
	
	fmt.Printf("📋 Looking for executable: %s\n", expectedFilename)
	fmt.Printf("📋 Available files in compiled_packages:\n")
	for _, content := range contents {
		if content.Type == "file" {
			fmt.Printf("   - %s\n", content.Name)
		}
	}
	
	for _, content := range contents {
		if content.Type == "file" && content.Name == expectedFilename {
			targetFile = &content
			break
		}
	}
	
	if targetFile == nil {
		fmt.Printf("⚠️  No executable found for %s/%s\n", sysInfo.OS, sysInfo.Architecture)
		return nil
	}
	
	fmt.Printf("📦 Found executable: %s (%d bytes)\n", targetFile.Name, targetFile.Size)
	
	// Check if we need to update (compare file size or SHA)
	needsUpdate, err := checkIfExecutableNeedsUpdate(sysInfo, targetFile)
	if err != nil {
		return fmt.Errorf("failed to check if update needed: %v", err)
	}
	
	if !needsUpdate {
		fmt.Println("✅ Executable is up to date")
		return nil
	}
	
	fmt.Println("⬇️  Downloading updated executable...")
	
	// Download and replace the executable
	if err := downloadAndReplaceExecutable(sysInfo, targetFile); err != nil {
		return fmt.Errorf("failed to download and replace executable: %v", err)
	}
	
	fmt.Println("✅ Executable updated successfully")
	return nil
}

func getExpectedExecutableFilename(sysInfo SystemInfo) string {
	// Map system info to expected filenames in the repository
	osArch := fmt.Sprintf("%s_%s", sysInfo.OS, sysInfo.Architecture)
	
	switch osArch {
	case "windows_amd64":
		return "tarr-annunciator.exe"  // Actual filename on GitHub
	case "linux_amd64":
		return "tarr-annunciator"      // Actual filename on GitHub
	case "linux_arm64":
		return "tarr-annunciator-raspberry-pi-arm64"
	case "linux_arm":
		return "tarr-annunciator-raspberry-pi-arm32"
	case "darwin_amd64":
		return "tarr-annunciator-macos-x64"
	case "darwin_arm64":
		return "tarr-annunciator-macos-arm64"
	default:
		return fmt.Sprintf("tarr-annunciator-%s-%s", sysInfo.OS, sysInfo.Architecture)
	}
}

func checkIfExecutableNeedsUpdate(sysInfo SystemInfo, remoteFile *GitHubContent) (bool, error) {
	localPath := sysInfo.ExecutablePath
	
	// Check if local file exists
	localInfo, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		// Local file doesn't exist, definitely needs update
		return true, nil
	}
	if err != nil {
		return false, err
	}
	
	// Compare file sizes first (quick check)
	if localInfo.Size() != remoteFile.Size {
		return true, nil
	}
	
	// If sizes match, could still be different files
	// For more thorough checking, we'd need to compare checksums
	// For now, we'll assume same size = same file
	return false, nil
}

func downloadAndReplaceExecutable(sysInfo SystemInfo, remoteFile *GitHubContent) error {
	// Download to temporary file first
	tempPath := sysInfo.ExecutablePath + ".update"
	
	if err := downloadFile(remoteFile.DownloadURL, tempPath); err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	
	// Set executable permissions on Unix systems
	if sysInfo.OS != "windows" {
		if err := os.Chmod(tempPath, 0755); err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("failed to set executable permissions: %v", err)
		}
	}
	
	// Create backup of existing executable
	backupPath := sysInfo.ExecutablePath + ".backup"
	if fileExists(sysInfo.ExecutablePath) {
		if err := os.Rename(sysInfo.ExecutablePath, backupPath); err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("failed to create backup: %v", err)
		}
	}
	
	// Move new executable into place
	if err := os.Rename(tempPath, sysInfo.ExecutablePath); err != nil {
		// Restore backup if move failed
		if fileExists(backupPath) {
			os.Rename(backupPath, sysInfo.ExecutablePath)
		}
		return fmt.Errorf("failed to replace executable: %v", err)
	}
	
	// Remove backup on success
	if fileExists(backupPath) {
		os.Remove(backupPath)
	}
	
	return nil
}

func checkDataUpdates(config UpdaterConfig) error {
	fmt.Println("\n🔍 Checking for data file updates...")
	
	// Get data directory contents from GitHub
	url := fmt.Sprintf("%s/contents/data", GITHUB_API_BASE)
	contents, err := getGitHubDirectoryContents(url)
	if err != nil {
		return fmt.Errorf("failed to get data directory: %v", err)
	}
	
	updatedFiles := 0
	
	for _, content := range contents {
		if content.Type == "file" {
			// Determine local path based on file type
			localPath := getLocalPathForDataFile(content.Name)
			
			needsUpdate, err := checkIfDataFileNeedsUpdate(localPath, &content)
			if err != nil {
				log.Printf("❌ Error checking %s: %v", content.Name, err)
				continue
			}
			
			if needsUpdate {
				fmt.Printf("⬇️  Updating: %s\n", content.Name)
				if err := downloadDataFile(&content, localPath); err != nil {
					log.Printf("❌ Failed to download %s: %v", content.Name, err)
					continue
				}
				updatedFiles++
			}
		} else if content.Type == "dir" {
			// Recursively check subdirectories
			fmt.Printf("🔍 Found subdirectory: %s\n", content.Name)
			if err := checkDataSubdirectory(content.Path); err != nil {
				log.Printf("❌ Error checking subdirectory %s: %v", content.Path, err)
			}
		}
	}
	
	if updatedFiles > 0 {
		fmt.Printf("✅ Updated %d data files\n", updatedFiles)
	} else {
		fmt.Println("✅ All data files are up to date")
	}
	
	return nil
}

func getLocalPathForDataFile(filename string) string {
	// Map GitHub data files to local paths
	ext := filepath.Ext(filename)
	
	switch ext {
	case ".json":
		return filepath.Join("json", filename)
	case ".html":
		return filepath.Join("templates", filename)
	case ".mp3", ".wav":
		return filepath.Join("static", "mp3", filename)
	default:
		// Default to current directory
		return filename
	}
}

func checkIfDataFileNeedsUpdate(localPath string, remoteFile *GitHubContent) (bool, error) {
	// Check if local file exists
	if !fileExists(localPath) {
		return true, nil // File missing, needs download
	}
	
	// Special handling for admin_config.json - check schema compatibility
	if strings.HasSuffix(localPath, "admin_config.json") {
		return checkAdminConfigCompatibility(localPath, remoteFile)
	}
	
	// Get local file info
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return false, err
	}
	
	// Compare file sizes
	if localInfo.Size() != remoteFile.Size {
		return true, nil
	}
	
	// For more thorough checking, we could compare checksums
	// For now, assume same size = same file
	return false, nil
}

func checkAdminConfigCompatibility(localPath string, remoteFile *GitHubContent) (bool, error) {
	// Read local config to check schema version
	localData, err := os.ReadFile(localPath)
	if err != nil {
		return true, nil // Can't read local, allow update
	}
	
	var localConfig map[string]interface{}
	if err := json.Unmarshal(localData, &localConfig); err != nil {
		return true, nil // Invalid JSON, allow update
	}
	
	// Check if local config has multi-user schema
	if metadata, exists := localConfig["metadata"].(map[string]interface{}); exists {
		if schemaVersion, exists := metadata["schema_version"].(string); exists && schemaVersion == "multi-user" {
			log.Printf("⚠️  Skipping admin_config.json update - local has newer multi-user schema")
			return false, nil // Don't overwrite multi-user config with single-user
		}
	}
	
	// If no multi-user schema detected, allow update
	return true, nil
}

func downloadDataFile(remoteFile *GitHubContent, localPath string) error {
	// Ensure local directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	
	// Download the file
	return downloadFile(remoteFile.DownloadURL, localPath)
}

func checkDataSubdirectory(remotePath string) error {
	// Get subdirectory contents
	url := fmt.Sprintf("%s/contents/%s", GITHUB_API_BASE, remotePath)
	contents, err := getGitHubDirectoryContents(url)
	if err != nil {
		return err
	}
	
	for _, content := range contents {
		if content.Type == "file" {
			// Map remote path to local path
			localPath := mapRemotePathToLocal(content.Path)
			
			needsUpdate, err := checkIfDataFileNeedsUpdate(localPath, &content)
			if err != nil {
				log.Printf("❌ Error checking %s: %v", content.Name, err)
				continue
			}
			
			if needsUpdate {
				fmt.Printf("⬇️  Updating: %s\n", content.Path)
				if err := downloadDataFile(&content, localPath); err != nil {
					log.Printf("❌ Failed to download %s: %v", content.Path, err)
				}
			}
		}
	}
	
	return nil
}

func mapRemotePathToLocal(remotePath string) string {
	// Remove "data/" prefix and map to local structure
	localPath := strings.TrimPrefix(remotePath, "data/")
	
	// Map specific subdirectories
	if strings.HasPrefix(localPath, "json/") {
		return localPath // Already correct
	} else if strings.HasPrefix(localPath, "templates/") {
		return localPath // Already correct
	} else if strings.HasPrefix(localPath, "static/") {
		return localPath // Already correct
	}
	
	return localPath
}

func getGitHubDirectoryContents(url string) ([]GitHubContent, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", USER_AGENT)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	var contents []GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}
	
	return contents, nil
}

func downloadFile(url, filepath string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("User-Agent", USER_AGENT)
	
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d when downloading %s", resp.StatusCode, url)
	}
	
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	_, err = io.Copy(file, resp.Body)
	return err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func calculateFileMD5(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// Version Tracking System Functions

// loadVersionManifest loads the local version manifest
func loadVersionManifest() VersionManifest {
	manifestPath := "version_manifest.json"
	manifest := VersionManifest{
		ApplicationVersion: "unknown",
		ManifestVersion:    "1.0.0",
		LastUpdated:        time.Now(),
		Files:              make(map[string]FileVersion),
		Platform:           runtime.GOOS,
		Architecture:       runtime.GOARCH,
	}
	
	if fileExists(manifestPath) {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			log.Printf("Warning: Could not read version manifest: %v", err)
			return manifest
		}
		
		if err := json.Unmarshal(data, &manifest); err != nil {
			log.Printf("Warning: Could not parse version manifest: %v", err)
			return manifest
		}
	}
	
	return manifest
}

// saveVersionManifest saves the local version manifest
func saveVersionManifest(manifest VersionManifest) error {
	manifestPath := "version_manifest.json"
	manifest.LastUpdated = time.Now()
	
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %v", err)
	}
	
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}
	
	return nil
}

// scanLocalFiles scans local files and updates the version manifest
func scanLocalFiles(manifest *VersionManifest) error {
	log.Printf("Scanning local files for version tracking...")
	
	// Define files to track
	trackFiles := []string{
		"tarr-annunciator.exe",
		"tarr-annunciator",
		"json/admin_config.json",
		"json/cron.json", 
		"json/trains_available.json",
		"json/destinations_available.json",
		"json/directions.json",
		"json/tracks.json",
		"json/safety.json",
		"json/promo.json",
		"json/emergencies.json",
		"templates/index.html",
		"templates/admin.html",
		"templates/admin_login.html",
		"templates/api_docs.html",
	}
	
	updatedCount := 0
	for _, filePath := range trackFiles {
		if fileExists(filePath) {
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				log.Printf("Warning: Could not stat file %s: %v", filePath, err)
				continue
			}
			
			hash, err := calculateFileMD5(filePath)
			if err != nil {
				log.Printf("Warning: Could not calculate hash for %s: %v", filePath, err)
				continue
			}
			
			// Check if file has changed
			existingFile, exists := manifest.Files[filePath]
			if !exists || existingFile.Hash != hash || existingFile.Size != fileInfo.Size() {
				manifest.Files[filePath] = FileVersion{
					Path:         filePath,
					Version:      manifest.ApplicationVersion,
					Hash:         hash,
					Size:         fileInfo.Size(),
					LastModified: fileInfo.ModTime(),
					Source:       "local",
				}
				updatedCount++
			}
		}
	}
	
	log.Printf("Scanned %d files, updated %d entries in manifest", len(trackFiles), updatedCount)
	return nil
}

// fetchRemoteManifest fetches the remote version manifest
func fetchRemoteManifest() (*RemoteManifest, error) {
	manifestURL := fmt.Sprintf("%s/version_manifest.json", GITHUB_RAW_BASE)
	
	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("User-Agent", USER_AGENT)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote manifest: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote manifest not found (HTTP %d)", resp.StatusCode)
	}
	
	var remoteManifest RemoteManifest
	if err := json.NewDecoder(resp.Body).Decode(&remoteManifest); err != nil {
		return nil, fmt.Errorf("failed to decode remote manifest: %v", err)
	}
	
	return &remoteManifest, nil
}

// compareVersions compares local and remote manifests to determine what needs updating
func compareVersions(local VersionManifest, remote *RemoteManifest) []string {
	var filesToUpdate []string
	
	log.Printf("Comparing versions - Local: %s, Remote: %s", 
		local.ApplicationVersion, remote.LatestVersion)
	
	// Check each file in the remote manifest
	for filePath, remoteFile := range remote.Files {
		needsUpdate := false
		
		localFile, exists := local.Files[filePath]
		if !exists {
			// File doesn't exist locally
			needsUpdate = true
			log.Printf("File missing locally: %s", filePath)
		} else if localFile.Hash != remoteFile.Hash {
			// File hash differs
			needsUpdate = true
			log.Printf("File hash differs: %s (local: %s, remote: %s)", 
				filePath, localFile.Hash[:8], remoteFile.Hash[:8])
		} else if localFile.Size != remoteFile.Size {
			// File size differs
			needsUpdate = true
			log.Printf("File size differs: %s (local: %d, remote: %d)", 
				filePath, localFile.Size, remoteFile.Size)
		}
		
		if needsUpdate {
			filesToUpdate = append(filesToUpdate, filePath)
		}
	}
	
	return filesToUpdate
}

// checkVersionBasedUpdate performs efficient version-based update checking
func checkVersionBasedUpdate() error {
	fmt.Println("\n🔍 Performing version-based update check...")
	
	// Load local manifest
	localManifest := loadVersionManifest()
	log.Printf("Local application version: %s", localManifest.ApplicationVersion)
	
	// Scan local files
	if err := scanLocalFiles(&localManifest); err != nil {
		return fmt.Errorf("failed to scan local files: %v", err)
	}
	
	// Fetch remote manifest
	remoteManifest, err := fetchRemoteManifest()
	if err != nil {
		log.Printf("Warning: Could not fetch remote manifest: %v", err)
		log.Printf("Falling back to traditional update method...")
		return nil // Fall back to existing update logic
	}
	
	// Compare versions
	filesToUpdate := compareVersions(localManifest, remoteManifest)
	
	if len(filesToUpdate) == 0 {
		fmt.Printf("✅ All files are up to date (v%s)\n", localManifest.ApplicationVersion)
		return nil
	}
	
	fmt.Printf("📦 Found %d files to update:\n", len(filesToUpdate))
	for _, file := range filesToUpdate {
		fmt.Printf("  - %s\n", file)
	}
	
	// Perform selective updates
	updatedCount := 0
	for _, filePath := range filesToUpdate {
		remoteFile := remoteManifest.Files[filePath]
		
		if err := downloadAndVerifyFile(filePath, remoteFile); err != nil {
			log.Printf("Error updating %s: %v", filePath, err)
		} else {
			// Update local manifest
			updatedFile := remoteFile
			updatedFile.Source = "github"
			localManifest.Files[filePath] = updatedFile
			updatedCount++
			fmt.Printf("✅ Updated: %s\n", filePath)
		}
	}
	
	// Update application version if any files were updated
	if updatedCount > 0 {
		localManifest.ApplicationVersion = remoteManifest.LatestVersion
		localManifest.ManifestVersion = remoteManifest.ManifestVersion
		
		if err := saveVersionManifest(localManifest); err != nil {
			log.Printf("Warning: Could not save updated manifest: %v", err)
		}
		
		fmt.Printf("🎉 Successfully updated %d files to version %s\n", 
			updatedCount, remoteManifest.LatestVersion)
	}
	
	return nil
}

// downloadAndVerifyFile downloads a file and verifies its integrity
func downloadAndVerifyFile(filePath string, expectedFile FileVersion) error {
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if dir != "." && !fileExists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}
	
	// Download file to temp location first
	tempPath := filePath + ".tmp"
	downloadURL := fmt.Sprintf("%s/%s", GITHUB_RAW_BASE, filePath)
	
	if err := downloadFile(downloadURL, tempPath); err != nil {
		return fmt.Errorf("failed to download: %v", err)
	}
	
	// Verify downloaded file
	actualHash, err := calculateFileMD5(tempPath)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to verify download: %v", err)
	}
	
	if actualHash != expectedFile.Hash {
		os.Remove(tempPath)
		return fmt.Errorf("hash mismatch - expected %s, got %s", expectedFile.Hash, actualHash)
	}
	
	// Move temp file to final location
	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to move file: %v", err)
	}
	
	// Set executable permissions if needed
	if strings.Contains(filePath, "tarr-annunciator") && !strings.Contains(filePath, ".exe") {
		if err := os.Chmod(filePath, 0755); err != nil {
			log.Printf("Warning: Could not set executable permissions on %s: %v", filePath, err)
		}
	}
	
	return nil
}