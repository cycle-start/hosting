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

# Install CloudBeaver systemd service.
cp /tmp/cloudbeaver.service /etc/systemd/system/cloudbeaver.service
systemctl daemon-reload
systemctl enable cloudbeaver

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
