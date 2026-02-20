#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y ceph ceph-mon ceph-osd ceph-mgr ceph-mds radosgw lvm2 jq

# Vector role-specific config.
cp /tmp/vector-storage.toml /etc/vector/config.d/storage.toml

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*

# Final cloud-init clean — must be last to prevent package triggers from
# recreating /var/lib/cloud state.
cloud-init clean
