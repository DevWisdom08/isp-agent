# ISP Cache Agent

Local agent for ISP cache servers. Connects to ISP SaaS Platform.

## Features

- ✅ Automatic registration
- ✅ License validation
- ✅ Real-time telemetry reporting
- ✅ Nginx cache statistics
- ✅ System monitoring (CPU, Memory)
- ✅ Top domains tracking
- ✅ Auto-restart on failure

## Installation

### One-Line Install:

curl -sSL http://64.23.151.140/static/install.sh | sudo bash### Manual Install:

1. Download the agent:
wget http://64.23.151.140/static/isp-agent
chmod +x isp-agent
sudo mv isp-agent /usr/local/bin/2. Run installation:
sudo /usr/local/bin/isp-agent -installYou'll be prompted for:
- SaaS Platform URL
- License Key
- ISP Name
- Server IP

3. Start the service:
sudo systemctl start isp-agent
sudo systemctl enable isp-agent## Configuration

Config file: `/etc/isp-agent/config.json`

{
  "saas_url": "http://64.23.151.140",
  "license_key": "ISP-XXXXXXXXXXXXXXXX",
  "isp_name": "My ISP",
  "server_ip": "192.168.1.100",
  "hwid": "ISP-XXXXXXXXXXXX",
  "isp_id": 1,
  "telemetry_interval_seconds": 60
}## Usage

# Check status
sudo systemctl status isp-agent

# View live logs
sudo journalctl -u isp-agent -f

# Restart agent
sudo systemctl restart isp-agent

# Stop agent
sudo systemctl stop isp-agent## What It Does

1. **Every 60 seconds:**
   - Collects Nginx cache stats (hits, misses)
   - Collects system stats (CPU, memory)
   - Reports to SaaS platform

2. **Every 5 minutes:**
   - Validates license
   - Checks for configuration updates

3. **Monitors:**
   - Cache hit rate
   - Bandwidth saved
   - Top cached domains
   - System health

## Requirements

- Ubuntu 20.04+ or Debian 10+
- Nginx installed
- Internet connection to SaaS platform

## License

Commercial - ISP SaaS Platform
