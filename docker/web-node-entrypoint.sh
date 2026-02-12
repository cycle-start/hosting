#!/bin/bash
set -e

# Remove default nginx site if present
rm -f /etc/nginx/sites-enabled/default

# Start PHP-FPM in the background
php-fpm8.5 --nodaemonize &

# Start node-agent in the background
/usr/local/bin/node-agent &

# Start nginx as the main process
exec nginx -g 'daemon off;'
