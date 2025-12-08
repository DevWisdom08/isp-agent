package nginx

import (
    "fmt"
    
    "os/exec"
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

// GetCacheStats parses Nginx cache stats from access.log
func GetCacheStats(accessLogPath string) (*CacheStats, error) {
    if accessLogPath == "" {
        accessLogPath = "/var/log/nginx/access.log"
    }
    
    stats := &CacheStats{}
    
    // Count cache hits and misses from X-Cache header in logs
    cmd := exec.Command("sh", "-c", fmt.Sprintf(
        "tail -n 10000 %s | grep -oE 'X-Cache: (HIT|MISS)' | sort | uniq -c",
        accessLogPath,
    ))
    
    output, err := cmd.Output()
    if err != nil {
        // If log doesn't exist or empty, return zeros
        return stats, nil
    }
    
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if strings.Contains(line, "HIT") {
            parts := strings.Fields(line)
            if len(parts) >= 1 {
                stats.Hits, _ = strconv.ParseInt(parts[0], 10, 64)
            }
        } else if strings.Contains(line, "MISS") {
            parts := strings.Fields(line)
            if len(parts) >= 1 {
                stats.Misses, _ = strconv.ParseInt(parts[0], 10, 64)
            }
        }
    }
    
    stats.TotalRequests = stats.Hits + stats.Misses
    
    // Get cache directory size
    cachePath := "/var/cache/nginx"
    cmd = exec.Command("du", "-sb", cachePath)
    output, err = cmd.Output()
    if err == nil {
        parts := strings.Fields(string(output))
        if len(parts) >= 1 {
            bytes, _ := strconv.ParseInt(parts[0], 10, 64)
            stats.CacheSizeUsed = bytes / 1024 / 1024 // Convert to MB
        }
    }
    
    return stats, nil
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
