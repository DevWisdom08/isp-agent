package license

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"
)

type LicenseInfo struct {
    LicenseKey string   `json:"license_key"`
    ISPID      int      `json:"isp_id"`
    ExpiresAt  string   `json:"expires_at"`
    Modules    []string `json:"modules"`
    Status     string   `json:"status"`
}

type ValidateRequest struct {
    LicenseKey string `json:"license_key"`
    HWID       string `json:"hw_id"`
}

type ValidateResponse struct {
    Success bool        `json:"success"`
    Data    LicenseInfo `json:"data"`
    Error   string      `json:"error"`
}

// Validate checks license with SaaS platform
func Validate(saasURL, licenseKey, hwid string) (*LicenseInfo, error) {
    url := fmt.Sprintf("%s/api/licenses/validate", saasURL)
    
    reqData := ValidateRequest{
        LicenseKey: licenseKey,
        HWID:       hwid,
    }
    
    jsonData, _ := json.Marshal(reqData)
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to connect to SaaS: %w", err)
    }
    defer resp.Body.Close()
    
    var result ValidateResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }
    
    if !result.Success {
        return nil, fmt.Errorf("license validation failed: %s", result.Error)
    }
    
    return &result.Data, nil
}

// IsExpired checks if license is expired
func IsExpired(expiresAt string) bool {
    expiry, err := time.Parse(time.RFC3339, expiresAt)
    if err != nil {
        return true
    }
    return time.Now().After(expiry)
}

// LoadConfig loads license from config file
func LoadConfig() (string, error) {
    data, err := os.ReadFile("/etc/isp-agent/license.key")
    if err != nil {
        return "", err
    }
    return string(bytes.TrimSpace(data)), nil
}

// SaveConfig saves license to config file
func SaveConfig(licenseKey string) error {
    os.MkdirAll("/etc/isp-agent", 0755)
    return os.WriteFile("/etc/isp-agent/license.key", []byte(licenseKey), 0600)
}
