# HighDiskUsage / DiskWillFillIn4Hours

## What It Means
A filesystem on a node has exceeded 80% usage (warning) or 95% usage (critical). The predictive alert `DiskWillFillIn4Hours` fires when the current fill rate will exhaust the disk within 4 hours. A full disk can cause service crashes, failed writes, and data loss.

## Severity
Warning at 80%, Critical at 95% or when disk is predicted to fill within 4 hours. A full disk on a web node breaks tenant sites; on a database node it causes data corruption risk.

## Likely Causes
1. Log files accumulating without rotation (nginx, PHP-FPM, node-agent, systemd journal)
2. Tenant uploads consuming excessive space (web nodes with CephFS mounts)
3. Temporary files not being cleaned up (`/tmp`, build artifacts)
4. Backup files growing on database nodes (binary logs, mysqldump output)

## Investigation Steps
1. Check disk usage across all filesystems:
   ```bash
   ssh ubuntu@<node-ip> df -h
   ```
2. Find what is consuming the most space:
   ```bash
   ssh ubuntu@<node-ip> sudo du -sh /var/log/* | sort -rh | head -20
   ssh ubuntu@<node-ip> sudo du -sh /var/* | sort -rh | head -10
   ```
3. Find large files:
   ```bash
   ssh ubuntu@<node-ip> sudo find / -type f -size +100M -exec ls -lh {} \; 2>/dev/null
   ```
4. Check log rotation configuration:
   ```bash
   ssh ubuntu@<node-ip> ls -la /etc/logrotate.d/
   ssh ubuntu@<node-ip> sudo logrotate --debug /etc/logrotate.conf
   ```
5. Check journal size:
   ```bash
   ssh ubuntu@<node-ip> journalctl --disk-usage
   ```

## Remediation

### Immediate
- Clear old logs:
  ```bash
  ssh ubuntu@<node-ip> sudo journalctl --vacuum-time=2d
  ssh ubuntu@<node-ip> sudo find /var/log -name '*.gz' -mtime +7 -delete
  ```
- Remove temporary files:
  ```bash
  ssh ubuntu@<node-ip> sudo rm -rf /tmp/*
  ```
- On database nodes, purge old binary logs:
  ```bash
  ssh ubuntu@<node-ip> sudo mysql -e "PURGE BINARY LOGS BEFORE NOW() - INTERVAL 1 DAY;"
  ```

### Long-term
- Configure logrotate for all services with appropriate retention
- Set `SystemMaxUse=500M` in `/etc/systemd/journald.conf`
- Implement disk usage monitoring with automated cleanup scripts
- For web nodes, implement per-tenant disk quotas
- Ensure CephFS is used for tenant data (not local disk)
- Right-size VM disks via Terraform based on shard role

## Escalation
If the disk is at 95%+ and cannot be freed quickly, escalate immediately. If tenant data is at risk, coordinate with the platform team before deleting anything. For CephFS-related disk issues, escalate to the storage team.
