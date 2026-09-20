package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	githubOwner = "egtechgeek"
	githubRepo  = "TARR_Annunciator"
	updateUA    = "TARR-Annunciator-Updater/1.1"
)

type updatePackageMeta struct {
	SchemaVersion  int    `json:"schema_version"`
	AppVersion     string `json:"app_version"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	MinAppVersion  string `json:"min_app_version"`
	Binary         struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"binary"`
	CreatedAt    string   `json:"created_at"`
	ReleaseNotes string   `json:"release_notes"`
	Migrations   []string `json:"migrations"`
}

type remoteUpdateInfo struct {
	TagName      string `json:"tag_name"`
	AppVersion   string `json:"app_version"`
	Name         string `json:"name"`
	Body         string `json:"body"`
	AssetName    string `json:"asset_name"`
	DownloadURL  string `json:"download_url"`
	PublishedAt  string `json:"published_at"`
	UpdateAvail  bool   `json:"update_available"`
	LocalVersion string `json:"local_version"`
}

type updateJobState struct {
	mu      sync.Mutex
	Status  string   `json:"status"` // idle, checking, available, downloading, applying, restarting, done, error
	Message string   `json:"message"`
	Log     []string `json:"log"`
	Remote  *remoteUpdateInfo `json:"remote,omitempty"`
}

var updateJob = updateJobState{Status: "idle", Message: "No update in progress", Log: []string{}}

func (s *updateJobState) set(status, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.Message = msg
	s.Log = append(s.Log, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	if len(s.Log) > 200 {
		s.Log = s.Log[len(s.Log)-200:]
	}
	log.Printf("Update: %s", msg)
}

type updateJobSnapshot struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Log     []string         `json:"log"`
	Remote  *remoteUpdateInfo `json:"remote,omitempty"`
}

func (s *updateJobState) snapshot() updateJobSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := updateJobSnapshot{
		Status:  s.Status,
		Message: s.Message,
		Log:     append([]string{}, s.Log...),
	}
	if s.Remote != nil {
		r := *s.Remote
		cp.Remote = &r
	}
	return cp
}

func getUpdateStatusHandler(c *gin.Context) {
	snap := updateJob.snapshot()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"version": versionStatusMap(),
		"job":     snap,
	})
}

func checkUpdatesHandler(c *gin.Context) {
	if runtime.GOOS == "windows" {
		// Still allow check for testing discovery; install may be blocked later on non-linux.
	}
	updateJob.set("checking", "Checking GitHub Releases for updates…")
	info, err := fetchLatestPiRelease()
	if err != nil {
		updateJob.set("error", err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": err.Error(), "version": versionStatusMap()})
		return
	}
	updateJob.mu.Lock()
	updateJob.Remote = info
	updateJob.mu.Unlock()
	if info.UpdateAvail {
		updateJob.set("available", fmt.Sprintf("Update available: %s → %s", info.LocalVersion, info.AppVersion))
	} else {
		updateJob.set("idle", fmt.Sprintf("Up to date at %s", info.LocalVersion))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"version": versionStatusMap(),
		"remote":  info,
		"job":     updateJob.snapshot(),
	})
}

func installUpdateHandler(c *gin.Context) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "arm64" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "In-app install is only supported on linux/arm64 (Raspberry Pi). Use the packaging script on your build machine to publish releases.",
		})
		return
	}

	snap := updateJob.snapshot()
	if snap.Status == "downloading" || snap.Status == "applying" || snap.Status == "restarting" {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "An update is already in progress", "job": snap})
		return
	}

	info := snap.Remote
	if info == nil || !info.UpdateAvail || info.DownloadURL == "" {
		var err error
		info, err = fetchLatestPiRelease()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": err.Error()})
			return
		}
		if !info.UpdateAvail {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Already up to date", "remote": info})
			return
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Update started",
		"remote":  info,
	})

	go func(remote *remoteUpdateInfo) {
		if err := performUpdate(remote); err != nil {
			updateJob.set("error", err.Error())
			return
		}
		updateJob.set("restarting", "Update applied — restarting application…")
		time.Sleep(2 * time.Second)
		if isRaspberryPi() && isRunningInScreen() {
			restartInScreen()
			return
		}
		home := os.Getenv("HOME")
		if home != "" {
			stop := filepath.Join(home, "tarr-stop.sh")
			start := filepath.Join(home, "tarr-start.sh")
			if fileExists(stop) && fileExists(start) {
				_ = exec.Command(stop).Start()
				time.Sleep(2 * time.Second)
				_ = exec.Command(start).Start()
				os.Exit(0)
			}
		}
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		_ = exec.Command(exe).Start()
		os.Exit(0)
	}(info)
}

func fetchLatestPiRelease() (*remoteUpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOwner, githubRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", updateUA)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub Releases request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub Releases returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	var assetName, downloadURL string
	for _, a := range release.Assets {
		name := a.Name
		if strings.HasPrefix(name, "TARR_Annunciator_Pi_arm64_") && strings.HasSuffix(name, ".tar.gz") {
			assetName = name
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("latest release %s has no TARR_Annunciator_Pi_arm64_*.tar.gz asset", release.TagName)
	}

	remoteVer := normalizeVersion(release.TagName)
	localVer := normalizeVersion(AppVersion)
	info := &remoteUpdateInfo{
		TagName:      release.TagName,
		AppVersion:   remoteVer,
		Name:         release.Name,
		Body:         release.Body,
		AssetName:    assetName,
		DownloadURL:  downloadURL,
		PublishedAt:  release.PublishedAt,
		LocalVersion: localVer,
		UpdateAvail:  compareSemver(localVer, remoteVer) < 0,
	}
	return info, nil
}

func performUpdate(remote *remoteUpdateInfo) error {
	updateJob.set("downloading", "Downloading "+remote.AssetName)
	tmpDir, err := os.MkdirTemp("", "tarr-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, remote.AssetName)
	if err := downloadFileHTTP(remote.DownloadURL, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return err
	}
	updateJob.set("applying", "Extracting package")
	root, err := extractTarGz(archivePath, extractDir)
	if err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	metaPath := filepath.Join(root, "UPDATE_PACKAGE.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("UPDATE_PACKAGE.json missing: %w", err)
	}
	var meta updatePackageMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return fmt.Errorf("invalid UPDATE_PACKAGE.json: %w", err)
	}
	if meta.Platform != "" && meta.Platform != "linux" {
		return fmt.Errorf("package platform %q is not linux", meta.Platform)
	}
	if meta.Arch != "" && meta.Arch != "arm64" {
		return fmt.Errorf("package arch %q is not arm64", meta.Arch)
	}

	binRel := meta.Binary.Path
	if binRel == "" {
		binRel = "tarr-annunciator"
	}
	binPath := filepath.Join(root, binRel)
	sum, err := fileSHA256(binPath)
	if err != nil {
		return err
	}
	if meta.Binary.SHA256 != "" && !strings.EqualFold(sum, meta.Binary.SHA256) {
		return fmt.Errorf("binary sha256 mismatch: got %s want %s", sum, meta.Binary.SHA256)
	}

	base := app.Config.BaseDir
	updateJob.set("applying", "Backing up json and current binary")
	backupDir := filepath.Join(base, fmt.Sprintf("update_backup_%s", time.Now().Format("20060102-150405")))
	_ = copyDir(filepath.Join(base, "json"), filepath.Join(backupDir, "json"))
	curBin := filepath.Join(base, "tarr-annunciator")
	if fileExists(curBin) {
		_ = copyFile(curBin, filepath.Join(backupDir, "tarr-annunciator"))
	}

	updateJob.set("applying", "Updating templates and static assets")
	if err := copyDir(filepath.Join(root, "templates"), filepath.Join(base, "templates")); err != nil {
		return fmt.Errorf("templates: %w", err)
	}
	if err := mergeCopyDir(filepath.Join(root, "static"), filepath.Join(base, "static")); err != nil {
		return fmt.Errorf("static: %w", err)
	}

	updateJob.set("applying", "Running additive JSON migrations")
	fromVer := getInstalledVersion()
	if err := migrateFromPackageSeeds(filepath.Join(root, "json")); err != nil {
		return err
	}
	if err := runSchemaMigrations(fromVer); err != nil {
		return err
	}

	updateJob.set("applying", "Installing new binary")
	newBin := filepath.Join(base, "tarr-annunciator.new")
	if err := copyFile(binPath, newBin); err != nil {
		return err
	}
	if err := os.Chmod(newBin, 0755); err != nil {
		return err
	}
	finalBin := filepath.Join(base, "tarr-annunciator")
	if err := os.Rename(newBin, finalBin); err != nil {
		return fmt.Errorf("binary swap failed: %w", err)
	}

	pkgVer := normalizeVersion(meta.AppVersion)
	if pkgVer == "" {
		pkgVer = remote.AppVersion
	}
	if err := writeInstallVersion(pkgVer, "github-release:"+remote.TagName); err != nil {
		log.Printf("Warning: could not write install_version.json: %v", err)
	}

	updateJob.set("done", fmt.Sprintf("Installed %s (backup: %s)", pkgVer, backupDir))
	return nil
}

func downloadFileHTTP(url, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", updateUA)
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var topDirs = map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		// Prevent path traversal
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") {
			return "", fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}
		target := filepath.Join(destDir, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
			parts := strings.Split(clean, string(os.PathSeparator))
			if len(parts) > 0 && parts[0] != "" {
				topDirs[parts[0]] = true
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0644)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
			parts := strings.Split(clean, string(os.PathSeparator))
			if len(parts) > 0 && parts[0] != "" {
				topDirs[parts[0]] = true
			}
		}
	}

	// Prefer single top-level directory as package root
	if len(topDirs) == 1 {
		for name := range topDirs {
			return filepath.Join(destDir, name), nil
		}
	}
	return destDir, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err == nil {
		_ = os.Chmod(dst, info.Mode())
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

// mergeCopyDir copies files from src into dst without deleting extra files already in dst.
func mergeCopyDir(src, dst string) error {
	return copyDir(src, dst)
}
