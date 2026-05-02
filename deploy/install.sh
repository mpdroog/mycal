#!/bin/bash
set -e

# MyCal installation script
#
# Build with version:
#   go build -ldflags "-X main.Version=$(git rev-parse --short HEAD)"

echo "Creating mycal user..."
useradd --system --shell /usr/sbin/nologin --home-dir /var/lib/mycal mycal 2>/dev/null || true

echo "Creating directories..."
mkdir -p /opt/mycal
mkdir -p /var/lib/mycal

echo "Copying binary and assets..."
cp mycal /opt/mycal/
cp -r templates /opt/mycal/
cp -r static /opt/mycal/

echo "Setting permissions..."
chown -R root:root /opt/mycal
chmod -R 755 /opt/mycal
chown -R mycal:mycal /var/lib/mycal
chmod 700 /var/lib/mycal

echo "Installing systemd service..."
cp deploy/mycal.service /etc/systemd/system/
systemctl daemon-reload

echo "Done! Run:"
echo "  systemctl enable mycal"
echo "  systemctl start mycal"
echo ""
echo "For hfast, copy deploy/override.toml to your site root."
