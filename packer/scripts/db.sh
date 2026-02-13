#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y mysql-server

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
