#!/bin/bash
set -e

# Start node-agent in the background
/usr/local/bin/node-agent &

# Start Valkey as the main process
exec valkey-server --bind 0.0.0.0 --protected-mode no --dir /var/lib/valkey
