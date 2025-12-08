#!/bin/bash
# ISP Cache Agent - Installation Script
# Usage: curl -sSL http://your-domain.com/install.sh | sudo bash

set -e

echo "================================================"
echo "   ISP Cache Agent - Installation"
echo "   Version: 1.0.0"
echo "================================================"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "❌ Please run as root (use sudo)"
  exit 1
fi

# Detect OS
if [ ! -f /etc/os-release ]; then
  echo "❌ Unsupported operating system"
  exit 1
fi

source /etc/os-release
if [ "$ID" != "ubuntu" ] && [ "$ID" != "debian" ]; then
  echo "⚠️  Warning: This script is optimized for Ubuntu/Debian"
fi

echo "✅ OS: $PRETTY_NAME"
echo ""

# Install dependencies
echo "📦 Installing dependencies..."
apt-get update -qq
apt-get install -y curl wget nginx -qq

echo "✅ Dependencies installed"
echo ""

# Download agent binary
echo "⬇️  Downloading ISP Agent..."
AGENT_URL="http://64.23.151.140/static/isp-agent"
wget -q -O /tmp/isp-agent "$AGENT_URL"
chmod +x /tmp/isp-agent
mv /tmp/isp-agent /usr/local/bin/isp-agent

echo "✅ Agent downloaded"
echo ""

# Run installation
echo "🔧 Configuring agent..."
/usr/local/bin/isp-agent -install

# Install systemd service
echo "⚙️  Installing system service..."
cat > /etc/systemd/system/isp-agent.service << 'SERVICE'
[Unit]
Description=ISP Cache Agent
After=network.target nginx.service

[Service]
Type=simple
ExecStart=/usr/local/bin/isp-agent
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
SERVICE

# Enable and start service
systemctl daemon-reload
systemctl enable isp-agent
systemctl start isp-agent

echo ""
echo "================================================"
echo "   ✅ Installation Complete!"
echo "================================================"
echo ""
echo "Agent Status:"
systemctl status isp-agent --no-pager -l
echo ""
echo "Commands:"
echo "  • Check status: systemctl status isp-agent"
echo "  • View logs:    journalctl -u isp-agent -f"
echo "  • Restart:      systemctl restart isp-agent"
echo ""
