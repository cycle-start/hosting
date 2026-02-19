#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y software-properties-common
add-apt-repository -y ppa:ondrej/php
apt-get update
apt-get install -y \
  nginx \
  php8.3-fpm php8.3-cli php8.3-mysql php8.3-curl \
  php8.3-mbstring php8.3-xml php8.3-zip \
  php8.5-fpm php8.5-cli php8.5-mysql php8.5-curl \
  php8.5-mbstring php8.5-xml php8.5-zip \
  supervisor \
  ceph-common \
  openssh-server

# Create directories used by node-agent at runtime.
mkdir -p /var/www/storage /etc/ssl/hosting /etc/ceph /etc/ssh/sshd_config.d

# Pre-configure dummy kernel module for tenant0 ULA interface.
echo "dummy" > /etc/modules-load.d/dummy.conf

# SSH hardening config.
cp /tmp/00-hosting-base.conf /etc/ssh/sshd_config.d/00-hosting-base.conf

# Install Composer globally.
php -r "copy('https://getcomposer.org/installer', '/tmp/composer-setup.php');"
php /tmp/composer-setup.php --install-dir=/usr/local/bin --filename=composer
rm /tmp/composer-setup.php

# Enable supervisord for php-worker runtime.
systemctl enable supervisor

# Base supervisord config for hosting platform.
cp /tmp/supervisor-hosting.conf /etc/supervisor/conf.d/hosting.conf

# Hide other users' processes (applied on every boot via fstab).
echo "proc /proc proc defaults,hidepid=2 0 0" >> /etc/fstab

# Vector role-specific config.
cp /tmp/vector-web.toml /etc/vector/config.d/web.toml

# CephFS mount dependency for node-agent on web nodes.
mkdir -p /etc/systemd/system/node-agent.service.d
cat > /etc/systemd/system/node-agent.service.d/cephfs.conf << 'EOF'
[Unit]
After=var-www-storage.mount
Requires=var-www-storage.mount
EOF

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
