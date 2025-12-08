package main

import (
    "encoding/json"
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
)

const Version = "1.0.0"

type Config struct {
    SaasURL      string `json:"saas_url"`
    LicenseKey   string `json:"license_key"`
    ISPName      string `json:"isp_name"`
    ServerIP     string `json:"server_ip"`
    HWID         string `json:"hwid"`
    ISPID        int    `json:"isp_id"`
    TelemetryInt int    `json:"telemetry_interval_seconds"`
}

var config Config

func main() {
    log.Printf("ISP Cache Agent v%s starting...", Version)
    
    // Parse flags
    configPath := flag.String("config", "/etc/isp-agent/config.json", "Path to config file")
    install := flag.Bool("install", false, "Install and register agent")
    flag.Parse()
    
    // Load or create config
    if *install {
        if err := installAgent(); err != nil {
            log.Fatalf("Installation failed: %v", err)
        }
        log.Println("Agent installed successfully!")
        return
    }
    
    // Load existing config
    if err := loadConfig(*configPath); err != nil {
        log.Fatalf("Failed to load config: %v. Run with -install flag to install.", err)
    }
    
    // Validate license
    log.Println("Validating license...")
    licenseInfo, err := license.Validate(config.SaasURL, config.LicenseKey, config.HWID)
    if err != nil {
        log.Fatalf("License validation failed: %v", err)
    }
    
    if license.IsExpired(licenseInfo.ExpiresAt) {
        log.Fatalf("License has expired: %s", licenseInfo.ExpiresAt)
    }
    
    log.Printf("License valid until: %s", licenseInfo.ExpiresAt)
    log.Printf("Enabled modules: %v", licenseInfo.Modules)
    config.ISPID = licenseInfo.ISPID
    
    // Start telemetry loop
    log.Printf("Starting telemetry collection (every %d seconds)...", config.TelemetryInt)
    
    stopChan := make(chan os.Signal, 1)
    signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
    
    ticker := time.NewTicker(time.Duration(config.TelemetryInt) * time.Second)
    defer ticker.Stop()
    
    // Send initial telemetry
    collectAndSend()
    
    // Main loop
    for {
        select {
        case <-ticker.C:
            collectAndSend()
        case <-stopChan:
            log.Println("Shutting down gracefully...")
            return
        }
    }
}

func collectAndSend() {
    // Collect cache stats
    cacheStats, err := nginx.GetCacheStats("")
    if err != nil {
        log.Printf("Failed to collect cache stats: %v", err)
        return
    }
    
    // Collect system stats
    sysStats, err := nginx.GetSystemStats()
    if err != nil {
        log.Printf("Failed to collect system stats: %v", err)
        sysStats = &nginx.SystemStats{}
    }
    
    // Estimate bandwidth saved (hits * average file size)
    avgFileSize := int64(2) // 2 MB average
    bandwidthSaved := (cacheStats.Hits * avgFileSize)
    
    // Send telemetry
    data := telemetry.TelemetryData{
        ISPID:          config.ISPID,
        CacheHits:      cacheStats.Hits,
        CacheMisses:    cacheStats.Misses,
        BandwidthSaved: bandwidthSaved,
        TotalRequests:  cacheStats.TotalRequests,
        CacheSizeUsed:  int(cacheStats.CacheSizeUsed),
        CPUUsage:       sysStats.CPUUsage,
        MemoryUsage:    sysStats.MemoryUsage,
    }
    
    if err := telemetry.Send(config.SaasURL, data); err != nil {
        log.Printf("Failed to send telemetry: %v", err)
        return
    }
    
    log.Printf("Telemetry sent: Hits=%d, Misses=%d, HitRate=%.2f%%, CacheSize=%dMB",
        data.CacheHits, data.CacheMisses,
        float64(data.CacheHits)/float64(data.TotalRequests)*100,
        data.CacheSizeUsed)
    
    // Collect and send top domains
    domains, err := nginx.GetTopDomains("", 20)
    if err == nil {
        for domain, hits := range domains {
            siteData := telemetry.SiteData{
                ISPID:          config.ISPID,
                Domain:         domain,
                Hits:           hits,
                BandwidthSaved: hits * avgFileSize,
            }
            telemetry.SendCachedSite(config.SaasURL, siteData)
        }
    }
}

func installAgent() error {
    var saasURL, licenseKey, ispName, serverIP string
    
    fmt.Print("Enter SaaS Platform URL (e.g., http://64.23.151.140): ")
    fmt.Scanln(&saasURL)
    
    fmt.Print("Enter License Key: ")
    fmt.Scanln(&licenseKey)
    
    fmt.Print("Enter ISP Name: ")
    fmt.Scanln(&ispName)
    
    fmt.Print("Enter Server IP: ")
    fmt.Scanln(&serverIP)
    
    // Generate HWID
    hwid, err := hwid.GetOrCreate()
    if err != nil {
        return fmt.Errorf("failed to generate HWID: %w", err)
    }
    
    log.Printf("Hardware ID: %s", hwid)
    
    // Validate license
    log.Println("Validating license...")
    licenseInfo, err := license.Validate(saasURL, licenseKey, hwid)
    if err != nil {
        return fmt.Errorf("license validation failed: %w", err)
    }
    
    log.Printf("License validated! ISP ID: %d", licenseInfo.ISPID)
    
    // Save config
    config = Config{
        SaasURL:      saasURL,
        LicenseKey:   licenseKey,
        ISPName:      ispName,
        ServerIP:     serverIP,
        HWID:         hwid,
        ISPID:        licenseInfo.ISPID,
        TelemetryInt: 60, // 1 minute
    }
    
    os.MkdirAll("/etc/isp-agent", 0755)
    
    configJSON, _ := json.MarshalIndent(config, "", "  ")
    if err := os.WriteFile("/etc/isp-agent/config.json", configJSON, 0644); err != nil {
        return fmt.Errorf("failed to save config: %w", err)
    }
    
    license.SaveConfig(licenseKey)
    
    return nil
}

func loadConfig(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    
    return json.Unmarshal(data, &config)
}
