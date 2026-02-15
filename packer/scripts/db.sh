#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y mysql-server

# Configure MySQL root for passwordless TCP access (required by node-agent).
systemctl start mysql
mysql -u root -e "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY ''; FLUSH PRIVILEGES;"
systemctl stop mysql

# Enable GTID-based replication on all DB nodes.
cat > /etc/mysql/mysql.conf.d/replication.cnf << 'REPL_EOF'
[mysqld]
# --- GTID Replication ---
gtid_mode                = ON
enforce_gtid_consistency = ON
log_bin                  = /var/lib/mysql/binlog
binlog_format            = ROW
binlog_row_image         = FULL
log_slave_updates        = ON
relay_log                = /var/lib/mysql/relay-bin
relay_log_recovery       = ON

# Server ID is set at runtime via cloud-init (unique per node).
# server-id = <set by cloud-init>

# Crash-safe replication
sync_binlog              = 1
innodb_flush_log_at_trx_commit = 1

# Binary log expiration (7 days)
binlog_expire_logs_seconds = 604800

# Parallel replication
replica_parallel_workers  = 4
replica_parallel_type     = LOGICAL_CLOCK
replica_preserve_commit_order = ON

# Performance
binlog_transaction_dependency_tracking = WRITESET
transaction_write_set_extraction       = XXHASH64
REPL_EOF

# Vector role-specific config.
cp /tmp/vector-db.toml /etc/vector/config.d/db.toml

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
