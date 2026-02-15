# HighCpuUsage

## What It Means
A node's CPU usage has been sustained above 80% (warning) or 95% (critical) for a prolonged period. This degrades performance for all tenants on the node and can cause request timeouts.

## Severity
Warning at 80% sustained, Critical at 95% sustained. At critical levels, tenants experience slow page loads, timeouts, and failed requests.

## Likely Causes
1. High legitimate tenant traffic (traffic spike, viral content)
2. A runaway process consuming excessive CPU (infinite loop, stuck script)
3. Insufficient CPU allocated to the VM for its workload
4. Cryptocurrency mining or other malicious activity by a compromised tenant
5. Burst of Temporal activities on a node-agent (many convergence operations at once)

## Investigation Steps
1. Identify top CPU consumers:
   ```bash
   ssh ubuntu@<node-ip> top -bn1 | head -20
   ssh ubuntu@<node-ip> ps aux --sort=-pcpu | head -20
   ```
2. Check load average:
   ```bash
   ssh ubuntu@<node-ip> uptime
   ```
3. On web nodes, check nginx and PHP-FPM:
   ```bash
   ssh ubuntu@<node-ip> ps aux | grep -E 'nginx|php-fpm' | wc -l
   ssh ubuntu@<node-ip> tail -50 /var/log/nginx/error.log
   ```
4. Check node-agent activity:
   ```bash
   ssh ubuntu@<node-ip> journalctl -u node-agent --since '10 minutes ago' --no-pager | tail -50
   ```
5. Check for suspicious processes:
   ```bash
   ssh ubuntu@<node-ip> ps aux --sort=-pcpu | grep -v -E 'nginx|php-fpm|node-agent|mysql|systemd'
   ```

## Remediation

### Immediate
- Kill runaway or suspicious processes:
  ```bash
  ssh ubuntu@<node-ip> sudo kill -9 <pid>
  ```
- On web nodes, restart PHP-FPM to clear stuck workers:
  ```bash
  ssh ubuntu@<node-ip> sudo systemctl restart php8.5-fpm
  ```
- Temporarily limit tenant traffic at the HAProxy level if a single tenant is causing the spike

### Long-term
- Implement per-tenant CPU limits via cgroups or systemd resource controls
- Set PHP-FPM `max_execution_time` to prevent runaway scripts
- Right-size VM CPU allocation in Terraform based on shard role
- Implement auto-scaling or tenant rebalancing across shards
- Set up rate limiting at HAProxy for traffic spikes

## Escalation
If CPU is sustained at 95%+ and tenant impact is confirmed, escalate to the platform team. If suspicious processes indicate a compromised tenant, escalate to security immediately.
