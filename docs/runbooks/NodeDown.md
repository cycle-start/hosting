# NodeDown

## What It Means
A node VM's node_exporter has been unreachable for 3+ minutes. The node is likely down or unreachable, meaning all tenants hosted on this node are affected and their services are unavailable.

## Severity
Critical -- Tenant services on this node are degraded or completely unavailable.

## Likely Causes
1. The VM has crashed or been shut down (libvirt/QEMU failure, host machine issue)
2. Network connectivity lost between the monitoring stack and the node (bridge misconfigured, routing broken)
3. The node_exporter process has crashed or been OOM-killed
4. A firewall rule is blocking port 9100 (node_exporter metrics port)

## Investigation Steps
1. Try to reach the node directly:
   ```bash
   ping <node-ip>
   ssh ubuntu@<node-ip> uptime
   ```
2. If SSH works, check node_exporter:
   ```bash
   ssh ubuntu@<node-ip> systemctl status node_exporter
   ssh ubuntu@<node-ip> journalctl -u node_exporter --since '10 minutes ago'
   ```
3. If the node is unreachable, check the VM state on the hypervisor:
   ```bash
   virsh list --all
   virsh dominfo <hostname>
   ```
4. Check network from the hypervisor:
   ```bash
   ip link show virbr1
   brctl show
   ```

## Remediation

### Immediate
- If the VM is shut off, start it:
  ```bash
  virsh start <hostname>
  ```
- If the VM is running but node_exporter is down:
  ```bash
  ssh ubuntu@<node-ip> sudo systemctl restart node_exporter
  ```
- If the network is broken, check the bridge and routing on the hypervisor:
  ```bash
  ip route
  ip link set virbr1 up
  ```

### Long-term
- Ensure node_exporter has a systemd restart policy (`Restart=always`)
- Add hypervisor-level monitoring for VM state
- Review resource allocation to prevent OOM kills on nodes
- Ensure firewall rules are managed via Terraform and allow port 9100

## Escalation
If the hypervisor itself is unresponsive or the VM cannot be started, escalate to the infrastructure team. If multiple nodes are down simultaneously, treat as a potential network or hypervisor failure and escalate immediately.
