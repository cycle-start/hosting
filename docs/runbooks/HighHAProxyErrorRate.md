# HighHAProxyErrorRate

## What It Means
HAProxy is returning 5xx responses for more than 5% of requests. This indicates that tenant HTTP requests are failing, either because backend servers are unhealthy, requests are timing out, or routing is misconfigured.

## Severity
Warning at 5% error rate, Critical at higher sustained rates. Tenants are actively experiencing failed HTTP requests.

## Likely Causes
1. All or most backend servers are unhealthy, forcing HAProxy to return 503
2. Backend servers are timing out under load, causing 504 responses
3. Misconfigured FQDN-to-shard map file causing routing failures
4. Backend web nodes returning 5xx errors themselves (nginx or PHP-FPM issues)
5. HAProxy connection limits reached

## Investigation Steps
1. Check HAProxy stats for backend health:
   ```bash
   echo "show stat" | nc 10.10.10.70 9999
   ```
2. Check error breakdown:
   ```bash
   echo "show stat" | nc 10.10.10.70 9999 | awk -F, '{print $1, $2, $8, $9, $10}'
   ```
3. Check HAProxy logs for error patterns:
   ```bash
   ssh ubuntu@10.10.10.70 journalctl -u haproxy --since '10 minutes ago' | grep -E '5[0-9]{2}' | tail -30
   ```
4. Check the FQDN map file:
   ```bash
   ssh ubuntu@10.10.10.70 cat /etc/haproxy/fqdn-map.txt | head -20
   ```
5. Check backend web nodes:
   ```bash
   ssh ubuntu@<web-node-ip> systemctl status nginx
   ssh ubuntu@<web-node-ip> tail -20 /var/log/nginx/error.log
   ```

## Remediation

### Immediate
- If backends are unhealthy, fix them (see [HAProxyBackendDown](HAProxyBackendDown.md))
- If timeouts are the issue, check backend load and consider temporarily increasing HAProxy timeout:
  ```bash
  # Check current timeout settings
  ssh ubuntu@10.10.10.70 grep timeout /etc/haproxy/haproxy.cfg
  ```
- If the FQDN map is corrupted, reload it:
  ```bash
  echo "set map /etc/haproxy/fqdn-map.txt <fqdn> <backend>" | nc 10.10.10.70 9999
  ```
- Reload HAProxy if config changes are needed:
  ```bash
  ssh ubuntu@10.10.10.70 sudo systemctl reload haproxy
  ```

### Long-term
- Implement better health checks with appropriate thresholds
- Add circuit breaking for unhealthy backends to fail fast
- Tune timeout values based on observed P99 latencies
- Implement FQDN map validation before updates
- Add HAProxy error rate dashboards to Grafana

## Escalation
If the error rate is above 10% and backends appear healthy, escalate to the platform team for investigation. If related to backend node failures, coordinate with the infrastructure team.
