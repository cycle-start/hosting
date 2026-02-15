# DatabaseReplicationLag

## What It Means
MySQL replication lag on a database shard has exceeded 30 seconds (warning) or 300 seconds (critical), or the replication thread has stopped entirely. This means the replica is behind the primary, and a failover would result in data loss. Tenants reading from the replica may see stale data.

## Severity
Warning at 30s lag, Critical at 300s lag or stopped replication thread. A stopped replication thread requires immediate attention to prevent data divergence.

## Likely Causes
1. Heavy write load on the primary exceeding the replica's apply rate
2. Slow replica due to insufficient disk I/O or CPU
3. Network latency or instability between primary and replica
4. Disk I/O bottleneck on the replica (local SSD degradation)
5. Large transactions or DDL operations being replicated
6. Replication thread stopped due to an error (duplicate key, missing table)

## Investigation Steps
1. Check replication status on the replica:
   ```bash
   ssh ubuntu@<replica-ip> sudo mysql -e "SHOW SLAVE STATUS\G" | grep -E 'Seconds_Behind|Slave_IO|Slave_SQL|Last_Error'
   ```
2. Check disk I/O on the replica:
   ```bash
   ssh ubuntu@<replica-ip> iostat -x 1 5
   ```
3. Check network latency:
   ```bash
   ssh ubuntu@<replica-ip> ping -c 5 <primary-ip>
   ```
4. Check binary log position difference:
   ```bash
   # On primary:
   ssh ubuntu@<primary-ip> sudo mysql -e "SHOW MASTER STATUS\G"
   # On replica:
   ssh ubuntu@<replica-ip> sudo mysql -e "SHOW SLAVE STATUS\G" | grep -E 'Master_Log_File|Read_Master_Log_Pos|Relay_Master_Log_File|Exec_Master_Log_Pos'
   ```
5. Check for large transactions:
   ```bash
   ssh ubuntu@<primary-ip> sudo mysql -e "SHOW PROCESSLIST\G" | grep -A 5 "State: Sending"
   ```

## Remediation

### Immediate
- If the replication thread is stopped due to an error:
  ```bash
  # Check the error first
  ssh ubuntu@<replica-ip> sudo mysql -e "SHOW SLAVE STATUS\G" | grep Last_Error
  # If safe to skip (e.g., duplicate key on a known operation):
  ssh ubuntu@<replica-ip> sudo mysql -e "STOP SLAVE; SET GLOBAL SQL_SLAVE_SKIP_COUNTER = 1; START SLAVE;"
  ```
- If lag is due to heavy writes, wait for the replica to catch up while monitoring:
  ```bash
  watch -n 5 'ssh ubuntu@<replica-ip> sudo mysql -e "SHOW SLAVE STATUS\G" | grep Seconds_Behind'
  ```
- If disk I/O is the bottleneck, reduce write load on the primary temporarily

### Long-term
- Tune replication settings: `slave_parallel_workers`, `slave_parallel_type=LOGICAL_CLOCK`
- Ensure replicas have equivalent or better I/O performance than primaries
- Set `innodb_buffer_pool_size` appropriately on replicas
- Implement write throttling for bulk operations
- Add binary log retention policies on the primary
- Monitor replication lag continuously with lower warning thresholds

## Escalation
If replication lag exceeds 300s or the replication thread cannot be restarted, escalate to the database team immediately. If a failover is being considered, coordinate with the platform team to assess data loss risk.
