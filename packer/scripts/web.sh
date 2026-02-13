#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y \
  nginx \
  php8.3-fpm php8.3-cli php8.3-mysql php8.3-curl \
  php8.3-mbstring php8.3-xml php8.3-zip \
  ceph-common \
  openssh-server

# Create directories used by node-agent at runtime.
mkdir -p /var/www/storage /etc/ssl/hosting /etc/ceph /etc/ssh/sshd_config.d

# SSH hardening config.
cp /tmp/00-hosting-base.conf /etc/ssh/sshd_config.d/00-hosting-base.conf

# Hide other users' processes (applied on every boot via fstab).
echo "proc /proc proc defaults,hidepid=2 0 0" >> /etc/fstab

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
