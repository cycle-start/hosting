# PHPFPMPoolExhausted

## What It Means
All PHP-FPM workers are busy and the listen queue is overflowing. New PHP requests are being queued or rejected, causing tenant websites to hang or return 502/504 errors through nginx.

## Severity
Warning when active workers approach max_children, Critical when the listen queue is overflowing. At critical levels, tenants experience failed page loads and timeouts.

## Likely Causes
1. Slow PHP scripts holding workers for extended periods (database queries, external API calls)
2. High traffic volume exceeding the configured number of workers
3. Insufficient `pm.max_children` for the workload
4. Resource exhaustion on the web node (CPU or memory) slowing all workers
5. A single tenant's script is stuck in an infinite loop or deadlock

## Investigation Steps
1. Check PHP-FPM status (if status page is enabled):
   ```bash
   ssh ubuntu@<web-ip> curl -s http://127.0.0.1/fpm-status
   ```
2. Check active PHP-FPM processes:
   ```bash
   ssh ubuntu@<web-ip> ps aux | grep php-fpm
   ssh ubuntu@<web-ip> ps aux | grep php-fpm | wc -l
   ```
3. Check PHP-FPM slow log:
   ```bash
   ssh ubuntu@<web-ip> tail -50 /var/log/php8.5-fpm-slow.log
   ```
4. Check for resource exhaustion:
   ```bash
   ssh ubuntu@<web-ip> top -bn1 | head -20
   ssh ubuntu@<web-ip> free -h
   ```
5. Check nginx error log for upstream timeouts:
   ```bash
   ssh ubuntu@<web-ip> tail -50 /var/log/nginx/error.log | grep -i "upstream\|fpm\|timeout"
   ```
6. Identify which tenants are consuming the most workers:
   ```bash
   ssh ubuntu@<web-ip> ps aux | grep php-fpm | grep -v master | awk '{print $NF}' | sort | uniq -c | sort -rn | head
   ```

## Remediation

### Immediate
- Restart PHP-FPM to clear stuck workers:
  ```bash
  ssh ubuntu@<web-ip> sudo systemctl restart php8.5-fpm
  ```
- If a specific tenant is causing the issue, identify and address their scripts
- Temporarily increase max_children if the node has headroom:
  ```bash
  # Edit the pool config:
  ssh ubuntu@<web-ip> sudo vi /etc/php/8.5/fpm/pool.d/www.conf
  # Change pm.max_children, then:
  ssh ubuntu@<web-ip> sudo systemctl reload php8.5-fpm
  ```

### Long-term
- Implement per-tenant PHP-FPM pools with individual `max_children` limits via systemd socket activation
- Set `request_terminate_timeout` to kill stuck workers after a reasonable period (e.g., 60s)
- Enable PHP-FPM slow log with `request_slowlog_timeout = 5s` to identify slow scripts
- Right-size `pm.max_children` based on available memory (each worker uses ~30-50MB)
- Implement per-tenant resource limits and rate limiting
- Add PHP-FPM pool metrics to Prometheus for proactive monitoring

## Escalation
If PHP-FPM pool exhaustion is recurring and affecting multiple tenants, escalate to the platform team to investigate per-tenant isolation and resource limits. If a single tenant is the cause, coordinate with the tenant support team.
