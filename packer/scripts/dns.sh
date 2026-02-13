#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y pdns-server pdns-backend-pgsql

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
