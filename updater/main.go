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
	fmt.Println("🔄 TARR Annunciator Updater v1.0")
	fmt.Println("=====================================")
	
	// Detect system information
	sysInfo := detectSystem()
	fmt.Printf("📱 Detected System: %s/%s\n", sysInfo.OS, sysInfo.Architecture)
	fmt.Printf("🎯 Target Executable: %s\n", sysInfo.ExecutableName)
	
	// Load updater configuration
	config := loadUpdaterConfig()
	fmt.Printf("📅 Last Check: %s\n", config.LastCheck)
	
	fmt.Println("\n🔍 Checking for updates...")
	
	// Check for executable updates
	if err := checkExecutableUpdates(sysInfo, config); err != nil {
		log.Printf("❌ Error checking executable updates: %v", err)
	}
	
	// Check for data file updates
	if err := checkDataUpdates(config); err != nil {
		log.Printf("❌ Error checking data updates: %v", err)
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
		return "tarr-annunciator-windows-x64.exe"
	case "linux_amd64":
		return "tarr-annunciator-linux-x64"
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