package updater

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	Version    = "2.0.0"
	GithubRepo = "nanablast/syncforge"
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
	client := &http.Client{Timeout: 15 * time.Second}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GithubRepo)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: Version,
			LatestVersion:  Version,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
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

func validateDownloadURL(rawURL string, allowInsecure bool) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid download URL: missing host")
	}
	if u.Scheme == "https" {
		return nil
	}
	if allowInsecure && u.Scheme == "http" {
		return nil
	}
	return fmt.Errorf("refusing non-HTTPS download URL scheme: %s", u.Scheme)
}

func validateReleaseURL(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid release URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	return u.String(), nil
}

// DownloadUpdate downloads the update to a temporary file
func DownloadUpdate(downloadURL string, progressChan chan<- int) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	return downloadUpdateWithClient(client, downloadURL, progressChan, false)
}

func downloadUpdateWithClient(client *http.Client, downloadURL string, progressChan chan<- int, allowInsecure bool) (string, error) {
	if err := validateDownloadURL(downloadURL, allowInsecure); err != nil {
		return "", err
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	ext := ".zip"
	if runtime.GOOS == "windows" {
		ext = ".exe"
	} else if runtime.GOOS == "linux" {
		ext = ""
	}

	tmpFile, err := os.CreateTemp("", "syncforge-update-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()

	cleanupOnError := true
	defer func() {
		_ = tmpFile.Close()
		if cleanupOnError {
			_ = os.Remove(tmpPath)
		}
	}()

	totalSize := resp.ContentLength
	var downloaded int64

	buffer := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			written, writeErr := tmpFile.Write(buffer[:n])
			if writeErr != nil {
				return "", fmt.Errorf("failed to write temp file: %v", writeErr)
			}
			if written != n {
				return "", io.ErrShortWrite
			}
			downloaded += int64(written)
			if progressChan != nil && totalSize > 0 {
				progress := int(float64(downloaded) / float64(totalSize) * 100)
				select {
				case progressChan <- progress:
				default:
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("download error: %v", readErr)
		}
	}

	if downloaded == 0 {
		return "", fmt.Errorf("downloaded update file is empty")
	}
	if totalSize > 0 && downloaded != totalSize {
		return "", fmt.Errorf("incomplete download: got %d bytes, expected %d", downloaded, totalSize)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %v", err)
	}
	cleanupOnError = false
	return tmpPath, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
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

func applyUpdateMacOS(zipFile string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	appPath := execPath
	for i := 0; i < 3; i++ {
		appPath = filepath.Dir(appPath)
	}
	if !strings.HasSuffix(appPath, ".app") {
		return fmt.Errorf("not running from an .app bundle")
	}

	appDir := filepath.Dir(appPath)
	appName := filepath.Base(appPath)

	tmpExtractDir, err := os.MkdirTemp("", "syncforge-update-extract-")
	if err != nil {
		return fmt.Errorf("failed to create temp extract dir: %v", err)
	}

	if err := unzip(zipFile, tmpExtractDir); err != nil {
		_ = os.RemoveAll(tmpExtractDir)
		return fmt.Errorf("failed to unzip update: %v", err)
	}

	var newAppPath string
	entries, err := os.ReadDir(tmpExtractDir)
	if err != nil {
		_ = os.RemoveAll(tmpExtractDir)
		return fmt.Errorf("failed to read extract dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".app") {
			newAppPath = filepath.Join(tmpExtractDir, entry.Name())
			break
		}
	}
	if newAppPath == "" {
		_ = os.RemoveAll(tmpExtractDir)
		return fmt.Errorf("no .app found in update package")
	}

	targetAppPath := filepath.Join(appDir, appName)
	scriptContent := fmt.Sprintf(`#!/bin/bash
set -e
sleep 2
rm -rf %s
mv %s %s
xattr -dr com.apple.quarantine %s 2>/dev/null || true
open %s
rm -rf %s
rm -f -- "$0"
`, shellQuote(appPath), shellQuote(newAppPath), shellQuote(targetAppPath), shellQuote(targetAppPath), shellQuote(targetAppPath), shellQuote(tmpExtractDir))

	scriptFile, err := os.CreateTemp("", "syncforge-update-*.sh")
	if err != nil {
		return fmt.Errorf("failed to create update script: %v", err)
	}
	scriptPath := scriptFile.Name()
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		_ = scriptFile.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("failed to write update script: %v", err)
	}
	_ = scriptFile.Chmod(0700)
	_ = scriptFile.Close()

	cmd := exec.Command("bash", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update script: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	os.Exit(0)

	return nil
}

func applyUpdateWindows(newExe string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	scriptContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
del "%s"
move "%s" "%s"
start "" "%s"
del "%%~f0"
`, execPath, newExe, execPath, execPath)

	scriptFile, err := os.CreateTemp("", "syncforge-update-*.bat")
	if err != nil {
		return fmt.Errorf("failed to create update script: %v", err)
	}
	scriptPath := scriptFile.Name()
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		_ = scriptFile.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("failed to write update script: %v", err)
	}
	_ = scriptFile.Close()

	cmd := exec.Command("cmd", "/c", "start", "/b", "", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update script: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	os.Exit(0)

	return nil
}

func applyUpdateLinux(newBinary string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	if err := os.Chmod(newBinary, 0755); err != nil {
		return fmt.Errorf("failed to make update executable: %v", err)
	}

	scriptContent := fmt.Sprintf(`#!/bin/bash
set -e
sleep 2
mv %s %s
chmod +x %s
%s &
rm -f -- "$0"
`, shellQuote(newBinary), shellQuote(execPath), shellQuote(execPath), shellQuote(execPath))

	scriptFile, err := os.CreateTemp("", "syncforge-update-*.sh")
	if err != nil {
		return fmt.Errorf("failed to create update script: %v", err)
	}
	scriptPath := scriptFile.Name()
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		_ = scriptFile.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("failed to write update script: %v", err)
	}
	_ = scriptFile.Chmod(0700)
	_ = scriptFile.Close()

	cmd := exec.Command("bash", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update script: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	os.Exit(0)

	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		closeErr := outFile.Close()
		_ = rc.Close()

		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}

	return nil
}

// OpenReleaseURL opens the release page in browser
func OpenReleaseURL(rawURL string) error {
	safeURL, err := validateReleaseURL(rawURL)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", safeURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", safeURL)
	case "linux":
		cmd = exec.Command("xdg-open", safeURL)
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
