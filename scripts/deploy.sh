#!/bin/bash
# Deploy script for Farm Search
# Builds the application locally and deploys to production server

set -euo pipefail

SERVER="root@107.191.56.246"
APP_DIR="/opt/farm-search"
APP_USER="farm-search"
MAPBOX_TOKEN="${MAPBOX_TOKEN:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() { echo -e "${GREEN}==>${NC} $1"; }
warn() { echo -e "${YELLOW}==>${NC} $1"; }
error() { echo -e "${RED}==>${NC} $1" >&2; }

# Change to project root
cd "$(dirname "$0")/.."

# Check if server is reachable
log "Checking server connectivity..."
if ! ssh -o BatchMode=yes -o ConnectTimeout=5 "$SERVER" "true" 2>/dev/null; then
    error "Cannot connect to $SERVER"
    exit 1
fi

# Build for Linux
log "Building for Linux amd64..."
mkdir -p bin
GOOS=linux GOARCH=amd64 go build -o bin/server-linux ./cmd/server
GOOS=linux GOARCH=amd64 go build -o bin/tools-linux ./cmd/tools

# Create a tarball of static files and scripts
log "Packaging static files..."
tar -czf bin/web.tar.gz web/ scripts/

# Upload files
log "Uploading files to server..."
scp -q bin/server-linux "$SERVER:$APP_DIR/bin/server.new"
scp -q bin/tools-linux "$SERVER:$APP_DIR/bin/tools"
scp -q bin/web.tar.gz "$SERVER:$APP_DIR/"

# Deploy on server
log "Deploying on server..."
ssh "$SERVER" bash << REMOTE_SCRIPT
set -euo pipefail

APP_DIR="/opt/farm-search"
APP_USER="farm-search"

cd "$APP_DIR"

# Extract web files
tar -xzf web.tar.gz
rm web.tar.gz

# Set permissions
chown -R "$APP_USER:$APP_USER" "$APP_DIR"
chmod +x bin/server.new bin/tools

# Atomic swap of binary
mv bin/server.new bin/server

# Initialize database if it doesn't exist
if [ ! -f "$APP_DIR/data/farm-search.db" ]; then
    echo "Initializing database..."
    sudo -u "$APP_USER" "$APP_DIR/bin/tools" seed
fi

# Update MAPBOX_TOKEN in systemd service if provided
MAPBOX_TOKEN="$MAPBOX_TOKEN"
if [ -n "$MAPBOX_TOKEN" ]; then
    if grep -q "Environment=MAPBOX_TOKEN=" /etc/systemd/system/farm-search.service; then
        sed -i "s|Environment=MAPBOX_TOKEN=.*|Environment=MAPBOX_TOKEN=$MAPBOX_TOKEN|" /etc/systemd/system/farm-search.service
    else
        sed -i "/\[Service\]/a Environment=MAPBOX_TOKEN=$MAPBOX_TOKEN" /etc/systemd/system/farm-search.service
    fi
    systemctl daemon-reload
fi

# Restart the service
echo "Restarting service..."
systemctl restart farm-search

# Wait and check status
sleep 2
if systemctl is-active --quiet farm-search; then
    echo "Deployment successful! Service is running."
else
    echo "Warning: Service may not have started correctly"
    systemctl status farm-search --no-pager || true
    exit 1
fi
REMOTE_SCRIPT

# Cleanup local build artifacts
log "Cleaning up..."
rm -f bin/server-linux bin/tools-linux bin/web.tar.gz

log "Deployment complete!"
echo ""
echo "Application deployed to: https://farms.dstrek.com"
echo ""
echo "Useful commands:"
echo "  ssh $SERVER 'systemctl status farm-search'   - Check status"
echo "  ssh $SERVER 'journalctl -u farm-search -f'   - View logs"
