# CephHealthDegraded

## What It Means
The Ceph cluster health status is not `HEALTH_OK`. It may be `HEALTH_WARN` (degraded, non-critical issues) or `HEALTH_ERR` (critical issues requiring immediate attention). This affects S3 object storage and potentially CephFS shared storage for web nodes.

## Severity
Warning for `HEALTH_WARN` (degraded but functional), Critical for `HEALTH_ERR` (data at risk or service disrupted). At critical levels, S3 operations may fail and tenant data could be at risk.

## Likely Causes
1. An OSD (Object Storage Daemon) is down or unreachable
2. Placement group (PG) inconsistency or degraded PGs
3. Disk failure on the storage node
4. Network connectivity issues between Ceph components
5. Clock skew between monitor nodes
6. Insufficient disk space on OSDs

## Investigation Steps
1. Check overall Ceph health:
   ```bash
   ssh ubuntu@<storage-ip> sudo ceph -s
   ```
2. Get detailed health information:
   ```bash
   ssh ubuntu@<storage-ip> sudo ceph health detail
   ```
3. Check OSD status:
   ```bash
   ssh ubuntu@<storage-ip> sudo ceph osd tree
   ssh ubuntu@<storage-ip> sudo ceph osd stat
   ```
4. Check for degraded PGs:
   ```bash
   ssh ubuntu@<storage-ip> sudo ceph pg stat
   ssh ubuntu@<storage-ip> sudo ceph pg dump_stuck unclean
   ```
5. Check disk health on the storage node:
   ```bash
   ssh ubuntu@<storage-ip> sudo smartctl -a /dev/vdc
   ssh ubuntu@<storage-ip> df -h
   ```
6. Check RGW (RADOS Gateway) status:
   ```bash
   ssh ubuntu@<storage-ip> sudo systemctl status ceph-radosgw@rgw.$(hostname)
   ```

## Remediation

### Immediate
- If an OSD is down, restart it:
  ```bash
  ssh ubuntu@<storage-ip> sudo systemctl restart ceph-osd@0
  ```
- If PGs are inconsistent, initiate a repair:
  ```bash
  ssh ubuntu@<storage-ip> sudo ceph pg repair <pg-id>
  ```
- If disk space is low, check and clean up:
  ```bash
  ssh ubuntu@<storage-ip> sudo ceph df
  ```
- If the RGW is down:
  ```bash
  ssh ubuntu@<storage-ip> sudo systemctl restart ceph-radosgw@rgw.$(hostname)
  ```

### Long-term
- Implement disk health monitoring with SMART data
- Set OSD nearfull and full ratios appropriately
- Ensure Ceph has adequate redundancy (replication factor)
- Plan disk replacement procedures before failures occur
- Add Ceph dashboard to Grafana for continuous visibility
- For single-node Ceph (dev), understand that there is no redundancy -- plan for multi-node in production

## Escalation
If Ceph is in `HEALTH_ERR` state or data integrity is at risk, escalate to the storage/infrastructure team immediately. If S3 operations are failing for tenants, coordinate with the platform team for tenant communication.
