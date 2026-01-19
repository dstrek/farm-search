#!/bin/bash
# Server setup script for Farm Search
# Run this once on a fresh Ubuntu server as root

set -euo pipefail

DOMAIN="farms.dstrek.com"
APP_USER="farm-search"
APP_DIR="/opt/farm-search"

echo "==> Setting up Farm Search server..."

# Update system
echo "==> Updating system packages..."
apt-get update
apt-get upgrade -y

# Install dependencies
echo "==> Installing dependencies..."
apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl

# Install Caddy
echo "==> Installing Caddy..."
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt-get update
apt-get install -y caddy

# Create application user
echo "==> Creating application user..."
if ! id "$APP_USER" &>/dev/null; then
    useradd --system --home "$APP_DIR" --shell /bin/false "$APP_USER"
fi

# Create application directory structure
echo "==> Creating application directories..."
mkdir -p "$APP_DIR"/{bin,data,web/static,web/templates}
chown -R "$APP_USER:$APP_USER" "$APP_DIR"

# Create systemd service with restart protection
echo "==> Creating systemd service..."
cat > /etc/systemd/system/farm-search.service << 'EOF'
[Unit]
Description=Farm Search Web Application
After=network.target
StartLimitIntervalSec=300
StartLimitBurst=10

[Service]
Type=simple
User=farm-search
Group=farm-search
WorkingDirectory=/opt/farm-search
ExecStart=/opt/farm-search/bin/server -port 8080 -db /opt/farm-search/data/farm-search.db
Restart=always
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/farm-search/data
PrivateTmp=true

# Environment
Environment=GIN_MODE=release
Environment=MAPBOX_TOKEN=

[Install]
WantedBy=multi-user.target
EOF

# Configure Caddy as reverse proxy with automatic HTTPS
echo "==> Configuring Caddy..."
cat > /etc/caddy/Caddyfile << EOF
${DOMAIN} {
    reverse_proxy localhost:8080
    encode gzip

    # Security headers
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
    }

    # Logging
    log {
        output file /var/log/caddy/farm-search.log
    }
}
EOF

# Create log directory for Caddy
mkdir -p /var/log/caddy
chown caddy:caddy /var/log/caddy

# Reload systemd and enable services
echo "==> Enabling services..."
systemctl daemon-reload
systemctl enable farm-search
systemctl enable caddy
systemctl restart caddy

echo "==> Server setup complete!"
echo ""
echo "Next steps:"
echo "  1. Deploy the application using: make deploy"
echo "  2. The app will be available at: https://${DOMAIN}"
echo ""
echo "Service management:"
echo "  systemctl status farm-search  - Check app status"
echo "  systemctl restart farm-search - Restart app"
echo "  journalctl -u farm-search -f  - View app logs"
