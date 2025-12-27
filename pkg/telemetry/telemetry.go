package telemetry

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type TelemetryData struct {
    ISPID          int     `json:"isp_id"`
    CacheHits      int64   `json:"cache_hits"`
    CacheMisses    int64   `json:"cache_misses"`
    BandwidthSaved int64   `json:"bandwidth_saved_mb"`
    TotalRequests  int64   `json:"total_requests"`
    CacheSizeUsed  int     `json:"cache_size_used_mb"`
    CPUUsage       float64 `json:"cpu_usage"`
    MemoryUsage    float64 `json:"memory_usage"`
}

type SiteData struct {
    ISPID          int    `json:"isp_id"`
    Domain         string `json:"domain"`
    Hits           int64  `json:"hits"`
    BandwidthSaved int64  `json:"bandwidth_saved_mb"`
}

type Response struct {
    Success bool        `json:"success"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
    Error   string      `json:"error"`
}

// Send sends telemetry data to SaaS platform
func Send(saasURL string, data TelemetryData) error {
    url := fmt.Sprintf("%s/api/telemetry", saasURL)
    
    jsonData, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("failed to marshal data: %w", err)
    }
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to send telemetry: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
        var result Response
        json.NewDecoder(resp.Body).Decode(&result)
        return fmt.Errorf("server returned error: %s", result.Error)
    }
    
    return nil
}

// SendCachedSite reports cached domain statistics
func SendCachedSite(saasURL string, data SiteData) error {
    url := fmt.Sprintf("%s/api/sites/report", saasURL)
    
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

// SendSystemLog sends a log entry to the SaaS
func SendSystemLog(saasURL, level, source, message string, metadata map[string]interface{}) error {
    url := fmt.Sprintf("%s/api/logs", saasURL)
    
    logData := map[string]interface{}{
        "level":    level,
        "source":   source,
        "message":  message,
        "metadata": metadata,
    }
    
    jsonData, _ := json.Marshal(logData)
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

// StartTelemetryLoop runs telemetry collection in a loop
func StartTelemetryLoop(saasURL string, ispID int, interval time.Duration, collectFunc func() (*TelemetryData, error)) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    // Send initial telemetry immediately
    sendTelemetry(saasURL, ispID, collectFunc)
    
    for range ticker.C {
        sendTelemetry(saasURL, ispID, collectFunc)
    }
}

// sendTelemetry collects and sends telemetry data with logging
func sendTelemetry(saasURL string, ispID int, collectFunc func() (*TelemetryData, error)) {
    data, err := collectFunc()
    if err != nil {
        // Log but don't fail - collect what we can
        data = &TelemetryData{}
    }
    
    data.ISPID = ispID
    
    // Ensure we always send something
    if err := Send(saasURL, *data); err != nil {
        // Retry once after a short delay
        time.Sleep(5 * time.Second)
        Send(saasURL, *data)
    }
}
