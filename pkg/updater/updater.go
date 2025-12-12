package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const CurrentVersion = "1.0.0"

type VersionInfo struct {
	ID           int       `json:"id"`
	Version      string    `json:"version"`
	DownloadURL  string    `json:"download_url"`
	Checksum     string    `json:"checksum"`
	ReleaseNotes string    `json:"release_notes"`
	IsStable     bool      `json:"is_stable"`
	CreatedAt    time.Time `json:"created_at"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    VersionInfo `json:"data"`
	Error   string      `json:"error"`
}

// CheckForUpdates checks if a new version is available
func CheckForUpdates(saasURL string) (*VersionInfo, bool, error) {
	url := fmt.Sprintf("%s/api/agent/version/latest", saasURL)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()
	
	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, false, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if !apiResp.Success {
		return nil, false, fmt.Errorf("API error: %s", apiResp.Error)
	}
	
	// Check if update is needed
	needsUpdate := apiResp.Data.Version != CurrentVersion
	
	return &apiResp.Data, needsUpdate, nil
}

// DownloadAndInstall downloads and installs a new version
func DownloadAndInstall(version *VersionInfo) error {
	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	
	// Download new version to temporary file
	tempFile := exePath + ".new"
	
	fmt.Printf("Downloading version %s from %s...\n", version.Version, version.DownloadURL)
	
	resp, err := http.Get(version.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()
	
	out, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()
	
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	out.Close()
	
	// Make executable
	if err := os.Chmod(tempFile, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	
	// Backup old version
	backupFile := exePath + ".backup"
	if err := os.Rename(exePath, backupFile); err != nil {
		return fmt.Errorf("failed to backup old version: %w", err)
	}
	
	// Install new version
	if err := os.Rename(tempFile, exePath); err != nil {
		// Rollback on failure
		os.Rename(backupFile, exePath)
		return fmt.Errorf("failed to install new version: %w", err)
	}
	
	fmt.Printf("✓ Successfully updated to version %s\n", version.Version)
	fmt.Println("  Release notes:", version.ReleaseNotes)
	fmt.Println("  Restarting agent...")
	
	// Restart the agent
	cmd := exec.Command("systemctl", "restart", "isp-agent")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to restart service: %v\n", err)
		fmt.Println("Please manually restart with: systemctl restart isp-agent")
	}
	
	return nil
}

// StartUpdateLoop checks for updates periodically
func StartUpdateLoop(saasURL string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for range ticker.C {
		version, needsUpdate, err := CheckForUpdates(saasURL)
		if err != nil {
			fmt.Printf("Update check failed: %v\n", err)
			continue
		}
		
		if needsUpdate {
			fmt.Printf("New version available: %s (current: %s)\n", version.Version, CurrentVersion)
			
			if err := DownloadAndInstall(version); err != nil {
				fmt.Printf("Update failed: %v\n", err)
			}
		}
	}
}
