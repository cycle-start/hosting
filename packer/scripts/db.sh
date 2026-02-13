#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y mysql-server

# Configure MySQL root for passwordless TCP access (required by node-agent).
systemctl start mysql
mysql -u root -e "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY ''; FLUSH PRIVILEGES;"
systemctl stop mysql

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
