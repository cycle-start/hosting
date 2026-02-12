#!/bin/bash
set -e

# Initialize MySQL data directory if needed
if [ ! -d "/var/lib/mysql/mysql" ]; then
    mysqld --initialize-insecure --user=mysql
fi

# Start node-agent in the background
/usr/local/bin/node-agent &

# Start MySQL as the main process
exec mysqld --user=mysql
