package hwid

import (
    "crypto/md5"
    "fmt"
    "os"
    "os/exec"
    "strings"
)

// Generate creates a unique hardware ID for this server
func Generate() (string, error) {
    // Get machine-id (Linux)
    machineID, err := readFile("/etc/machine-id")
    if err != nil {
        machineID, err = readFile("/var/lib/dbus/machine-id")
        if err != nil {
            // Fallback to hostname
            machineID, _ = os.Hostname()
        }
    }
    
    // Get primary network interface MAC
    mac, err := getMACAddress()
    if err != nil {
        mac = "unknown"
    }
    
    // Combine and hash
    combined := fmt.Sprintf("%s-%s", machineID, mac)
    hash := md5.Sum([]byte(combined))
    hwid := fmt.Sprintf("ISP-%X", hash)
    
    return hwid, nil
}

func readFile(path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(data)), nil
}

func getMACAddress() (string, error) {
    // Try to get MAC from ip command
    cmd := exec.Command("sh", "-c", "ip link show | grep 'link/ether' | head -1 | awk '{print $2}'")
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    
    mac := strings.TrimSpace(string(output))
    if mac == "" {
        return "00:00:00:00:00:00", nil
    }
    
    return mac, nil
}

// Save stores the HWID to a local file
func Save(hwid string) error {
    return os.WriteFile("/etc/isp-agent/hwid", []byte(hwid), 0644)
}

// Load retrieves the stored HWID
func Load() (string, error) {
    data, err := os.ReadFile("/etc/isp-agent/hwid")
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(data)), nil
}

// GetOrCreate gets existing HWID or creates new one
func GetOrCreate() (string, error) {
    // Try to load existing
    hwid, err := Load()
    if err == nil && hwid != "" {
        return hwid, nil
    }
    
    // Generate new
    hwid, err = Generate()
    if err != nil {
        return "", err
    }
    
    // Create directory if needed
    os.MkdirAll("/etc/isp-agent", 0755)
    
    // Save for future use
    if err := Save(hwid); err != nil {
        return hwid, err // Return HWID even if save fails
    }
    
    return hwid, nil
}
