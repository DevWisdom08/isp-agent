package nginx

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
)

type CacheStats struct {
    Hits           int64
    Misses         int64
    BytesServed    int64
    CacheSizeUsed  int64
    TotalRequests  int64
}

type SystemStats struct {
    CPUUsage    float64
    MemoryUsage float64
}

// GetCacheStats collects Nginx cache statistics from multiple sources
func GetCacheStats(accessLogPath string) (*CacheStats, error) {
    stats := &CacheStats{}
    
    // Try multiple methods to collect cache stats
    // Method 1: Parse custom log format with upstream_cache_status
    if accessLogPath == "" {
        accessLogPath = "/var/log/nginx/access.log"
    }
    
    collectFromLogWithCacheStatus(stats, accessLogPath)
    
    // Method 2: Also check lancache-specific logs
    lancacheLogs := []string{
        "/var/log/nginx/lancache-steam.log",
        "/var/log/nginx/lancache-epic.log",
        "/var/log/nginx/lancache-blizzard.log",
        "/var/log/nginx/lancache-riot.log",
        "/var/log/nginx/lancache-origin.log",
        "/var/log/nginx/cache.log",
        "/var/log/nginx/isp-cache.log",
    }
    
    for _, logFile := range lancacheLogs {
        if _, err := os.Stat(logFile); err == nil {
            collectFromLogWithCacheStatus(stats, logFile)
        }
    }
    
    // Method 3: Parse any log that might have cache status indicators
    // Look for patterns: HIT, MISS, BYPASS, EXPIRED, STALE, UPDATING, REVALIDATED
    collectFromAnyLogFormat(stats, accessLogPath)
    
    stats.TotalRequests = stats.Hits + stats.Misses
    
    // Get cache directory size from multiple possible locations
    cachePaths := []string{
        "/var/cache/nginx",
        "/var/cache/nginx/isp-cache",
        "/var/cache/nginx/lancache",
        "/data/cache",
        "/cache",
    }
    
    for _, cachePath := range cachePaths {
        if _, err := os.Stat(cachePath); err == nil {
            size := getCacheDirSize(cachePath)
            if size > stats.CacheSizeUsed {
                stats.CacheSizeUsed = size
            }
        }
    }
    
    return stats, nil
}

// collectFromLogWithCacheStatus parses logs with $upstream_cache_status format
func collectFromLogWithCacheStatus(stats *CacheStats, logPath string) {
    // Parse nginx logs looking for cache status
    // Nginx upstream_cache_status values: HIT, MISS, BYPASS, EXPIRED, STALE, UPDATING, REVALIDATED
    
    // First try: Look for cache status at end of log line (common format)
    cmd := exec.Command("sh", "-c", fmt.Sprintf(
        `tail -n 50000 "%s" 2>/dev/null | grep -oE '(HIT|MISS|BYPASS|EXPIRED|STALE|UPDATING|REVALIDATED)' | sort | uniq -c`,
        logPath,
    ))
    
    output, err := cmd.Output()
    if err != nil || len(output) == 0 {
        return
    }
    
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        parts := strings.Fields(strings.TrimSpace(line))
        if len(parts) < 2 {
            continue
        }
        
        count, _ := strconv.ParseInt(parts[0], 10, 64)
        status := strings.ToUpper(parts[1])
        
        switch status {
        case "HIT", "STALE", "UPDATING", "REVALIDATED":
            stats.Hits += count
        case "MISS", "BYPASS", "EXPIRED":
            stats.Misses += count
        }
    }
    
    // Calculate bandwidth saved from cache hits (bytes served)
    cmd = exec.Command("sh", "-c", fmt.Sprintf(
        `tail -n 50000 "%s" 2>/dev/null | grep -E 'HIT|STALE' | awk '{for(i=1;i<=NF;i++){if($i ~ /^[0-9]+$/ && $i > 100){sum+=$i}}} END {print sum+0}'`,
        logPath,
    ))
    
    output, err = cmd.Output()
    if err == nil {
        bytesStr := strings.TrimSpace(string(output))
        if bytesStr != "" && bytesStr != "0" {
            bytes, _ := strconv.ParseInt(bytesStr, 10, 64)
            stats.BytesServed += bytes
        }
    }
}

// collectFromAnyLogFormat tries to extract cache info from non-standard log formats
func collectFromAnyLogFormat(stats *CacheStats, logPath string) {
    // Look for X-Cache-Status header or similar patterns in logs
    patterns := []string{
        `X-Cache-Status:\s*(HIT|MISS)`,
        `X-Cache:\s*(HIT|MISS)`,
        `cache_status=(HIT|MISS)`,
        `"cache":\s*"(HIT|MISS)"`,
        `upstream_cache_status:\s*(HIT|MISS)`,
    }
    
    file, err := os.Open(logPath)
    if err != nil {
        return
    }
    defer file.Close()
    
    // Compile all patterns
    regexps := make([]*regexp.Regexp, len(patterns))
    for i, p := range patterns {
        regexps[i] = regexp.MustCompile(p)
    }
    
    scanner := bufio.NewScanner(file)
    lineCount := 0
    maxLines := 50000
    
    // Skip to last maxLines
    for scanner.Scan() {
        lineCount++
    }
    
    file.Seek(0, 0)
    scanner = bufio.NewScanner(file)
    
    skipLines := lineCount - maxLines
    if skipLines < 0 {
        skipLines = 0
    }
    
    currentLine := 0
    for scanner.Scan() {
        currentLine++
        if currentLine <= skipLines {
            continue
        }
        
        line := scanner.Text()
        for _, re := range regexps {
            matches := re.FindStringSubmatch(line)
            if len(matches) >= 2 {
                status := strings.ToUpper(matches[1])
                if status == "HIT" {
                    stats.Hits++
                } else if status == "MISS" {
                    stats.Misses++
                }
                break // Only count once per line
            }
        }
    }
}

// getCacheDirSize returns the size of cache directory in MB
func getCacheDirSize(cachePath string) int64 {
    var totalSize int64
    
    // Method 1: Use du command (faster for large directories)
    cmd := exec.Command("du", "-sb", cachePath)
    output, err := cmd.Output()
    if err == nil {
        parts := strings.Fields(string(output))
        if len(parts) >= 1 {
            bytes, _ := strconv.ParseInt(parts[0], 10, 64)
            return bytes / (1024 * 1024) // Convert to MB
        }
    }
    
    // Method 2: Walk directory (fallback)
    filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil
        }
        if !info.IsDir() {
            totalSize += info.Size()
        }
        return nil
    })
    
    return totalSize / (1024 * 1024) // Convert to MB
}

// GetSystemStats gets CPU and memory usage
func GetSystemStats() (*SystemStats, error) {
    stats := &SystemStats{}
    
    // Get CPU usage
    cmd := exec.Command("sh", "-c", "top -bn1 | grep 'Cpu(s)' | awk '{print $2}' | cut -d'%' -f1")
    output, err := cmd.Output()
    if err == nil {
        cpuStr := strings.TrimSpace(string(output))
        stats.CPUUsage, _ = strconv.ParseFloat(cpuStr, 64)
    }
    
    // Get memory usage
    cmd = exec.Command("sh", "-c", "free | grep Mem | awk '{print ($3/$2) * 100.0}'")
    output, err = cmd.Output()
    if err == nil {
        memStr := strings.TrimSpace(string(output))
        stats.MemoryUsage, _ = strconv.ParseFloat(memStr, 64)
    }
    
    return stats, nil
}

// ReloadNginx reloads Nginx configuration
func ReloadNginx() error {
    return exec.Command("nginx", "-s", "reload").Run()
}

// TestConfig tests Nginx configuration validity
func TestConfig() error {
    return exec.Command("nginx", "-t").Run()
}

// GetTopDomains extracts top cached domains from logs
func GetTopDomains(accessLogPath string, limit int) (map[string]int64, error) {
    if accessLogPath == "" {
        accessLogPath = "/var/log/nginx/access.log"
    }
    
    cmd := exec.Command("sh", "-c", fmt.Sprintf(
        "tail -n 50000 %s | grep 'X-Cache: HIT' | awk '{print $7}' | sort | uniq -c | sort -rn | head -%d",
        accessLogPath, limit,
    ))
    
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    
    domains := make(map[string]int64)
    re := regexp.MustCompile(`\s*(\d+)\s+(.+)`)
    
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        matches := re.FindStringSubmatch(line)
        if len(matches) == 3 {
            count, _ := strconv.ParseInt(matches[1], 10, 64)
            domain := extractDomain(matches[2])
            domains[domain] = count
        }
    }
    
    return domains, nil
}

func extractDomain(url string) string {
    // Simple domain extraction
    url = strings.TrimPrefix(url, "http://")
    url = strings.TrimPrefix(url, "https://")
    parts := strings.Split(url, "/")
    if len(parts) > 0 {
        return parts[0]
    }
    return url
}

