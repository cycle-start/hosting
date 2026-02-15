# NginxDown

## What It Means
Nginx is unreachable on a web node for 2+ minutes. This means all tenant websites hosted on this node are down. HAProxy health checks will mark this node as unhealthy, and traffic will shift to other nodes in the shard (if available).

## Severity
Critical -- All tenant sites on this web node are unreachable. If the shard has other healthy nodes, HAProxy will route around it, but with reduced capacity.

## Likely Causes
1. Nginx process crashed due to a configuration error
2. Port conflict (another process bound to port 80/443)
3. Disk full preventing nginx from writing logs or temp files
4. Nginx configuration syntax error after a convergence update
5. The web node VM itself is down (see [NodeDown](NodeDown.md))

## Investigation Steps
1. Check nginx status:
   ```bash
   ssh ubuntu@<web-ip> systemctl status nginx
   ```
2. Check nginx error log:
   ```bash
   ssh ubuntu@<web-ip> tail -50 /var/log/nginx/error.log
   ```
3. Validate nginx configuration:
   ```bash
   ssh ubuntu@<web-ip> sudo nginx -t
   ```
4. Check for port conflicts:
   ```bash
   ssh ubuntu@<web-ip> ss -tlnp | grep -E ':80|:443'
   ```
5. Check disk space (nginx needs to write logs and temp files):
   ```bash
   ssh ubuntu@<web-ip> df -h
   ```
6. Check node-agent convergence logs for recent config changes:
   ```bash
   ssh ubuntu@<web-ip> journalctl -u node-agent --since '30 minutes ago' | grep -i nginx
   ```

## Remediation

### Immediate
- If configuration is valid, restart nginx:
  ```bash
  ssh ubuntu@<web-ip> sudo systemctl restart nginx
  ```
- If configuration is invalid, check the last convergence changes:
  ```bash
  ssh ubuntu@<web-ip> sudo nginx -t 2>&1
  # Fix the offending config file, then:
  ssh ubuntu@<web-ip> sudo systemctl restart nginx
  ```
- If disk is full, free space (see [HighDiskUsage](HighDiskUsage.md)) then restart nginx
- Verify nginx is serving traffic:
  ```bash
  curl -s -o /dev/null -w "%{http_code}" http://<web-ip>/
  ```

### Long-term
- Implement nginx configuration validation before applying convergence changes
- Add `nginx -t` as a pre-check in the node-agent before reloading nginx
- Set up logrotate for nginx logs to prevent disk exhaustion
- Ensure nginx has `Restart=always` in its systemd unit

## Escalation
If nginx cannot be started due to a persistent configuration error from convergence, escalate to the platform team to investigate the node-agent's configuration generation. If the web node itself is down, see the [NodeDown](NodeDown.md) runbook.
