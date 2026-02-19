#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y docker.io nginx

systemctl enable docker

# Start Docker temporarily to pull the image.
systemctl start docker
docker pull dbeaver/cloudbeaver:latest
systemctl stop docker

# Create directories for CloudBeaver config and workspace.
mkdir -p /opt/cloudbeaver/workspace /opt/cloudbeaver/conf

# Install dbadmin-proxy and CloudBeaver systemd services.
cp /tmp/dbadmin-proxy /opt/hosting/bin/dbadmin-proxy
chmod +x /opt/hosting/bin/dbadmin-proxy
cp /tmp/dbadmin-proxy.service /etc/systemd/system/dbadmin-proxy.service
cp /tmp/cloudbeaver.service /etc/systemd/system/cloudbeaver.service
systemctl daemon-reload
systemctl enable cloudbeaver dbadmin-proxy

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
