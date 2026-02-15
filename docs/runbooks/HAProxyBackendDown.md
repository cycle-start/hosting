# HAProxyBackendDown

## What It Means
An HAProxy backend has zero healthy servers (critical) or has lost some servers from its pool (warning/degraded). With no healthy backends, all tenant HTTP requests routed through this backend will fail. With reduced backends, remaining servers take more load and tenants may experience degraded performance.

## Severity
Critical when zero healthy servers (total outage for affected tenants), Warning when some servers are lost (degraded performance, reduced redundancy).

## Likely Causes
1. All backend web nodes are down or unreachable
2. Nginx on the web nodes is not responding to health checks
3. Network partition between HAProxy and backend nodes
4. Misconfigured health check endpoint or parameters
5. Backend nodes are overloaded and failing health checks

## Investigation Steps
1. Check HAProxy stats:
   ```bash
   echo "show stat" | nc 10.10.10.70 9999 | tr ',' '\n' | head -40
   echo "show stat" | nc 10.10.10.70 9999 | grep -E 'svname|status|check_status'
   ```
2. Check HAProxy server states:
   ```bash
   echo "show servers state" | nc 10.10.10.70 9999
   ```
3. Check web node health directly:
   ```bash
   curl -s -o /dev/null -w "%{http_code}" http://<web-node-ip>:80/
   ssh ubuntu@<web-node-ip> systemctl status nginx
   ```
4. Check HAProxy logs:
   ```bash
   ssh ubuntu@10.10.10.70 journalctl -u haproxy --since '10 minutes ago' --no-pager | tail -50
   ```
5. Check network connectivity:
   ```bash
   ssh ubuntu@10.10.10.70 ping -c 3 <web-node-ip>
   ```

## Remediation

### Immediate
- If web nodes are down, start them:
  ```bash
  virsh start <web-node-hostname>
  ```
- If nginx is down on the web nodes:
  ```bash
  ssh ubuntu@<web-node-ip> sudo systemctl restart nginx
  ```
- If a network issue, check routing and bridges on the hypervisor:
  ```bash
  ip route
  brctl show
  ```
- Force HAProxy to re-check backends:
  ```bash
  echo "enable server <backend>/<server>" | nc 10.10.10.70 9999
  ```

### Long-term
- Ensure at least 2-3 web nodes per shard for redundancy
- Configure HAProxy health checks with appropriate intervals and thresholds
- Implement auto-recovery for web nodes via Temporal convergence workflows
- Add HAProxy stats dashboard to Grafana for visibility

## Escalation
If all backends are down and cannot be restored within 5 minutes, escalate to the platform team immediately. This is a full tenant-facing outage for the affected shard.
