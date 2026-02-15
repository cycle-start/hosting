# NodeExporterDown

## What It Means
The node_exporter process on a node VM is unreachable, but the node itself may still be running. Without node_exporter, we lose visibility into the node's resource usage (CPU, memory, disk, network).

## Severity
Warning -- Monitoring is degraded for this node. Tenant services are likely still running but we cannot observe the node's health.

## Likely Causes
1. The node_exporter process was OOM-killed due to memory pressure on the node
2. The node_exporter process crashed unexpectedly
3. A port conflict is preventing node_exporter from binding to port 9100
4. The systemd unit file was modified or disabled

## Investigation Steps
1. SSH into the node and check the process:
   ```bash
   ssh ubuntu@<node-ip> systemctl status node_exporter
   ```
2. Check the journal for crash reasons:
   ```bash
   ssh ubuntu@<node-ip> journalctl -u node_exporter --since '30 minutes ago' --no-pager
   ```
3. Check if the process was OOM-killed:
   ```bash
   ssh ubuntu@<node-ip> dmesg | grep -i oom
   ```
4. Check if port 9100 is in use by something else:
   ```bash
   ssh ubuntu@<node-ip> ss -tlnp | grep 9100
   ```

## Remediation

### Immediate
- Restart node_exporter:
  ```bash
  ssh ubuntu@<node-ip> sudo systemctl restart node_exporter
  ```
- Verify it is running and serving metrics:
  ```bash
  curl http://<node-ip>:9100/metrics | head
  ```

### Long-term
- Ensure the systemd unit has `Restart=always` and `RestartSec=5`
- Set memory limits for node_exporter if OOM is recurring (it should be very lightweight)
- Add a simple TCP check or process check independent of Prometheus scraping

## Escalation
If node_exporter repeatedly crashes after restart, escalate to the platform team to investigate the root cause. If the node itself is unreachable, see the [NodeDown](NodeDown.md) runbook instead.
