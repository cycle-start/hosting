#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y ceph ceph-mon ceph-osd ceph-mgr ceph-mds radosgw lvm2 jq

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
