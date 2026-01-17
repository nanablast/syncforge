package updater

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	Version    = "1.1.0"
	GithubRepo = "nanablast/skeema-gui"
)

// Release represents a GitHub release
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Body    string  `json:"body"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateInfo contains update information
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseNotes   string `json:"releaseNotes"`
	DownloadURL    string `json:"downloadUrl"`
	ReleaseURL     string `json:"releaseUrl"`
	AssetName      string `json:"assetName"`
	AssetSize      int64  `json:"assetSize"`
}

// GetCurrentVersion returns the current app version
func GetCurrentVersion() string {
	return Version
}

// CheckForUpdates checks GitHub releases for updates
func CheckForUpdates() (*UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GithubRepo)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// No releases yet
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: Version,
			LatestVersion:  Version,
		}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %v", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	isNewer := compareVersions(latestVersion, Version) > 0

	info := &UpdateInfo{
		Available:      isNewer,
		CurrentVersion: Version,
		LatestVersion:  latestVersion,
		ReleaseNotes:   release.Body,
		ReleaseURL:     release.HTMLURL,
	}

	// Find the appropriate asset for current platform
	assetName := getAssetName()
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetName) {
			info.DownloadURL = asset.BrowserDownloadURL
			info.AssetName = asset.Name
			info.AssetSize = asset.Size
			break
		}
	}

	return info, nil
}

// getAssetName returns the expected asset name for current platform
func getAssetName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	default:
		return runtime.GOOS
	}
}

// compareVersions compares two version strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

// DownloadUpdate downloads the update to a temporary file
func DownloadUpdate(downloadURL string, progressChan chan<- int) (string, error) {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %v", err)
	}
	defer resp.Body.Close()

	// Determine file extension
	ext := ".zip"
	if runtime.GOOS == "windows" {
		ext = ".exe"
	} else if runtime.GOOS == "linux" {
		ext = ""
	}

	// Create temp file
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "skeema-gui-update-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// Download with progress
	totalSize := resp.ContentLength
	var downloaded int64

	buffer := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			tmpFile.Write(buffer[:n])
			downloaded += int64(n)
			if progressChan != nil && totalSize > 0 {
				progress := int(float64(downloaded) / float64(totalSize) * 100)
				select {
				case progressChan <- progress:
				default:
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("download error: %v", err)
		}
	}

	return tmpFile.Name(), nil
}

// ApplyUpdate applies the downloaded update and restarts the application
func ApplyUpdate(downloadedFile string) error {
	switch runtime.GOOS {
	case "darwin":
		return applyUpdateMacOS(downloadedFile)
	case "windows":
		return applyUpdateWindows(downloadedFile)
	case "linux":
		return applyUpdateLinux(downloadedFile)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// applyUpdateMacOS handles macOS update: unzip, replace .app, restart
func applyUpdateMacOS(zipFile string) error {
	// Get current app path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Find .app bundle (go up from Contents/MacOS/executable)
	appPath := execPath
	for i := 0; i < 3; i++ {
		appPath = filepath.Dir(appPath)
	}
	if !strings.HasSuffix(appPath, ".app") {
		return fmt.Errorf("not running from an .app bundle")
	}

	appDir := filepath.Dir(appPath)
	appName := filepath.Base(appPath)

	// Create temp directory for extraction
	tmpExtractDir, err := os.MkdirTemp("", "skeema-update-extract-")
	if err != nil {
		return fmt.Errorf("failed to create temp extract dir: %v", err)
	}

	// Unzip the downloaded file
	if err := unzip(zipFile, tmpExtractDir); err != nil {
		return fmt.Errorf("failed to unzip update: %v", err)
	}

	// Find the .app in extracted files
	var newAppPath string
	entries, err := os.ReadDir(tmpExtractDir)
	if err != nil {
		return fmt.Errorf("failed to read extract dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".app") {
			newAppPath = filepath.Join(tmpExtractDir, entry.Name())
			break
		}
	}
	if newAppPath == "" {
		return fmt.Errorf("no .app found in update package")
	}

	// Create update script
	scriptContent := fmt.Sprintf(`#!/bin/bash
sleep 2
rm -rf "%s"
mv "%s" "%s"
open "%s"
rm -rf "%s"
rm "$0"
`, appPath, newAppPath, filepath.Join(appDir, appName), filepath.Join(appDir, appName), tmpExtractDir)

	scriptPath := filepath.Join(os.TempDir(), "skeema-update.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write update script: %v", err)
	}

	// Run update script and exit
	cmd := exec.Command("bash", scriptPath)
	cmd.Start()

	// Give script time to start
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)

	return nil
}

// applyUpdateWindows handles Windows update: batch script to replace exe and restart
func applyUpdateWindows(newExe string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Create batch script
	scriptContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
del "%s"
move "%s" "%s"
start "" "%s"
del "%%~f0"
`, execPath, newExe, execPath, execPath)

	scriptPath := filepath.Join(os.TempDir(), "skeema-update.bat")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write update script: %v", err)
	}

	// Run update script and exit
	cmd := exec.Command("cmd", "/c", "start", "/b", scriptPath)
	cmd.Start()

	time.Sleep(500 * time.Millisecond)
	os.Exit(0)

	return nil
}

// applyUpdateLinux handles Linux update: shell script to replace binary and restart
func applyUpdateLinux(newBinary string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Make new binary executable
	if err := os.Chmod(newBinary, 0755); err != nil {
		return fmt.Errorf("failed to make update executable: %v", err)
	}

	// Create update script
	scriptContent := fmt.Sprintf(`#!/bin/bash
sleep 2
mv "%s" "%s"
chmod +x "%s"
"%s" &
rm "$0"
`, newBinary, execPath, execPath, execPath)

	scriptPath := filepath.Join(os.TempDir(), "skeema-update.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write update script: %v", err)
	}

	// Run update script and exit
	cmd := exec.Command("bash", scriptPath)
	cmd.Start()

	time.Sleep(500 * time.Millisecond)
	os.Exit(0)

	return nil
}

// unzip extracts a zip file to destination directory
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// OpenReleaseURL opens the release page in browser
func OpenReleaseURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

// GetDownloadsFolder returns the user's downloads folder
func GetDownloadsFolder() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(homeDir, "Downloads")
}
