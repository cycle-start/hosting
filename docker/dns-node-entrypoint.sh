#!/bin/bash
set -e

# PowerDNS requires a PostgreSQL backend. Exit if not configured.
if [ -z "$PDNS_GPGSQL_HOST" ]; then
    echo "ERROR: PDNS_GPGSQL_HOST is required" >&2
    exit 1
fi

# Write PowerDNS configuration.
cat > /etc/powerdns/pdns.conf <<CONF
launch=gpgsql
gpgsql-host=$PDNS_GPGSQL_HOST
gpgsql-port=${PDNS_GPGSQL_PORT:-5432}
gpgsql-dbname=$PDNS_GPGSQL_DBNAME
gpgsql-user=$PDNS_GPGSQL_USER
gpgsql-password=$PDNS_GPGSQL_PASSWORD
api=yes
api-key=${PDNS_API_KEY:-secret}
webserver=yes
webserver-address=0.0.0.0
webserver-port=8081
webserver-allow-from=0.0.0.0/0
CONF

# Start node-agent in the background
/usr/local/bin/node-agent &

# Start PowerDNS as the main process
exec pdns_server --daemon=no
