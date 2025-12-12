package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"isp-agent/pkg/hwid"
	"isp-agent/pkg/license"
	"isp-agent/pkg/nginx"
	"isp-agent/pkg/telemetry"
	"isp-agent/pkg/updater"
)

const VERSION = "1.0.0"
const SAAS_URL = "http://64.23.151.140:8080"

func main() {
	// Command-line flags
	installFlag := flag.Bool("install", false, "Run initial installation and registration")
	hwidFlag := flag.Bool("hwid", false, "Generate and display hardware ID only")
	versionFlag := flag.Bool("version", false, "Display version information")
	checkUpdateFlag := flag.Bool("check-update", false, "Check for available updates")
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("ISP SaaS Agent v%s\n", VERSION)
		os.Exit(0)
	}

	// Handle HWID flag
	if *hwidFlag {
		id, err := hwid.Generate()
		if err != nil {
			log.Fatalf("Failed to generate hardware ID: %v", err)
		}
		fmt.Println(id)
		os.Exit(0)
	}

	// Handle check-update flag
	if *checkUpdateFlag {
		version, needsUpdate, err := updater.CheckForUpdates(SAAS_URL)
		if err != nil {
			log.Fatalf("Update check failed: %v", err)
		}
		
		fmt.Printf("Current version: %s\n", VERSION)
		fmt.Printf("Latest version: %s\n", version.Version)
		
		if needsUpdate {
			fmt.Println("✓ Update available!")
			fmt.Printf("  Release notes: %s\n", version.ReleaseNotes)
			fmt.Printf("\nTo update, run: sudo systemctl stop isp-agent && wget %s -O /opt/isp-agent/isp-agent && sudo systemctl start isp-agent\n", version.DownloadURL)
		} else {
			fmt.Println("✓ You are running the latest version")
		}
		os.Exit(0)
	}

	// Load license key from config
	licenseKey, err := license.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load license config: %v. Run with -install flag first.", err)
	}

	// Get hardware ID
	hardwareID, err := hwid.GetOrCreate()
	if err != nil {
		log.Fatalf("Failed to get hardware ID: %v", err)
	}

	// Installation mode
	if *installFlag {
		fmt.Println("=== ISP Agent Installation ===")
		fmt.Printf("Hardware ID: %s\n", hardwareID)
		fmt.Printf("License Key: %s\n", licenseKey)

		// Validate license
		licenseInfo, err := license.Validate(SAAS_URL, licenseKey, hardwareID)
		if err != nil {
			log.Fatalf("License validation failed: %v", err)
		}

		if licenseInfo.Status != "active" {
			log.Fatal("License is not active")
		}

		fmt.Println("✓ License validated successfully")
		fmt.Printf("✓ ISP ID: %d\n", licenseInfo.ISPID)
		fmt.Printf("✓ Expires: %s\n", licenseInfo.ExpiresAt)
		fmt.Println("✓ Installation complete")
		fmt.Println("\nStart the agent with: systemctl start isp-agent")
		os.Exit(0)
	}

	// Normal operation mode
	log.Printf("ISP SaaS Agent v%s starting...", VERSION)
	log.Printf("Hardware ID: %s", hardwareID)
	log.Printf("License Key: %s", licenseKey)

	// Validate license at startup
	licenseInfo, err := license.Validate(SAAS_URL, licenseKey, hardwareID)
	if err != nil {
		log.Fatalf("License validation failed: %v", err)
	}

	if licenseInfo.Status != "active" {
		log.Fatal("License is not active")
	}

	log.Printf("License validated successfully (ISP ID: %d)", licenseInfo.ISPID)

	// Check for updates on startup
	go func() {
		time.Sleep(30 * time.Second) // Wait 30s after startup
		version, needsUpdate, err := updater.CheckForUpdates(SAAS_URL)
		if err != nil {
			log.Printf("Update check failed: %v", err)
			return
		}
		
		if needsUpdate {
			log.Printf("New version available: %s (current: %s)", version.Version, VERSION)
			log.Printf("Update will be installed automatically")
			
			if err := updater.DownloadAndInstall(version); err != nil {
				log.Printf("Auto-update failed: %v", err)
			}
		}
	}()

	// Start auto-update checker (every 24 hours)
	go updater.StartUpdateLoop(SAAS_URL, 24*time.Hour)

	// Start telemetry loop in background
	collectStats := func() (*telemetry.TelemetryData, error) {
		cacheStats, err := nginx.GetCacheStats("/var/log/nginx/access.log")
		if err != nil {
			return nil, err
		}
		
		systemStats, err := nginx.GetSystemStats()
		if err != nil {
			return nil, err
		}

		return &telemetry.TelemetryData{
			ISPID:          licenseInfo.ISPID,
			CacheHits:      cacheStats.Hits,
			CacheMisses:    cacheStats.Misses,
			BandwidthSaved: cacheStats.BytesServed / (1024 * 1024), // Convert to MB
			TotalRequests:  cacheStats.TotalRequests,
			CacheSizeUsed:  int(cacheStats.CacheSizeUsed / (1024 * 1024)), // Convert to MB
			CPUUsage:       systemStats.CPUUsage,
			MemoryUsage:    systemStats.MemoryUsage,
		}, nil
	}

	go telemetry.StartTelemetryLoop(licenseKey, licenseInfo.ISPID, 5*time.Minute, collectStats)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Agent running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down gracefully...")
	time.Sleep(2 * time.Second)
	log.Println("Agent stopped")
}
