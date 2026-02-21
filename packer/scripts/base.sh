#!/bin/bash
set -ex

# Wait for cloud-init to finish (Packer boot cloud-init).
# Exit code 2 = "recoverable errors" which is fine for our minimal cloud-init.
cloud-init status --wait || true

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get upgrade -y
apt-get install -y curl python3

# Install HWE kernel for Ceph Squid (19.x) msgr2 compatibility and newer
# hardware support. The stock 6.8 kernel's ceph module can't parse Squid
# OSD maps via msgr2.
apt-get install -y linux-generic-hwe-24.04

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*

# Final cloud-init clean — must be last.
cloud-init clean
