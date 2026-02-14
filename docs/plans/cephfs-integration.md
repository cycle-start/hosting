# CephFS Integration Plan for Web Nodes

## Status: Draft

## Overview

Web shards consist of 2-3 nodes that must all serve identical tenant files. CephFS provides a POSIX-compliant shared filesystem so every node in a web shard sees the same directory tree at `/var/www/storage`. This eliminates the need to replicate files between web nodes and allows the HAProxy load balancer to route any request to any node in the shard.

The storage shard already runs a single-node Ceph cluster (mon + mgr + osd + rgw + mds) with both S3 (RADOS Gateway) and CephFS enabled. Web nodes already install `ceph-common` in Packer and the cloud-init template already copies the keyring from the storage node and mounts CephFS. This plan formalizes the current approach, addresses its gaps, and documents the production-ready target state.

---

## 1. Ceph Cluster Topology

### Recommendation: Shared Ceph cluster with separate pools

The existing storage node (`storage-1-node-0`, 10.10.10.50) already runs both RGW for S3 and MDS for CephFS on the same single-node Ceph cluster. This is the correct approach and should remain the architecture.

**Rationale:**

- A separate Ceph cluster per web shard would waste resources. Ceph's pool-level isolation provides sufficient separation between S3 (RGW pools) and CephFS (`cephfs_data`, `cephfs_metadata`) workloads.
- A single cluster means one set of monitors, one OSD fleet, and one management surface. At scale, you add OSDs and MDS standby nodes to the same cluster, not new clusters.
- CephFS and RGW use different RADOS pools, so I/O from one does not contend with the other at the pool/PG level (assuming sufficient OSDs).

**Current pool layout** (created in `storage-node.yaml.tpl`):

| Pool              | PGs | Purpose                      |
|-------------------|-----|------------------------------|
| `cephfs_data`     | 32  | CephFS file data             |
| `cephfs_metadata` | 16  | CephFS metadata (inodes, dentries) |
| `default.rgw.*`   | auto | RGW pools (S3 objects)      |

**Production scaling path:**

- Add more OSDs (additional storage nodes or disks) to the same cluster.
- Add MDS standby-replay daemons for HA (the current single MDS is a SPOF).
- Consider separate CRUSH rules to pin CephFS pools to SSD-backed OSDs and RGW pools to HDD-backed OSDs if I/O profiles diverge.

### Future: Multi-shard considerations

When multiple web shards exist, they all mount the same CephFS filesystem. Tenant isolation is enforced at the directory level (permissions + CephFS quotas), not at the filesystem level. If hard multi-tenancy between web shards becomes a requirement, CephFS subvolumes or multiple CephFS filesystems on the same cluster can provide that.

---

## 2. Client Setup

### 2.1 Packages (Packer)

The web Packer image (`packer/scripts/web.sh`) already installs `ceph-common`:

```bash
apt-get install -y ceph-common
```

This provides the CephFS kernel client (`mount.ceph`), `ceph` CLI, and the FUSE client (`ceph-fuse`) as a fallback. No changes needed here.

### 2.2 ceph.conf distribution

**Current approach** (cloud-init `runcmd` in `web-node.yaml.tpl`):

```bash
scp -o StrictHostKeyChecking=no ubuntu@${storage_node_ip}:/etc/ceph/ceph.conf /etc/ceph/
```

**Problems:**

1. SCP with `StrictHostKeyChecking=no` is insecure and fragile (requires the storage node to be fully booted and SSH-accessible before web nodes run cloud-init).
2. If the storage node reboots or changes its host key, subsequent SCPs fail.
3. No retry logic if the storage node is not yet ready.

**Target approach:**

Replace the SCP-based distribution with Terraform-generated config injected via cloud-init `write_files`. The storage node's `ceph.conf` values (FSID, mon host) are deterministic at Terraform plan time.

```yaml
# In web-node.yaml.tpl
write_files:
  - path: /etc/ceph/ceph.conf
    content: |
      [global]
      fsid = ${ceph_fsid}
      mon host = ${storage_node_ip}
      auth cluster required = cephx
      auth service required = cephx
      auth client required = cephx
```

This requires making the Ceph FSID a Terraform variable (or a `random_uuid` resource) rather than generating it inside the storage node's cloud-init script. This is a prerequisite change.

**Terraform changes:**

```hcl
# In variables.tf or a dedicated ceph.tf
resource "random_uuid" "ceph_fsid" {}

# Pass to storage-node.yaml.tpl
ceph_fsid = random_uuid.ceph_fsid.result

# Pass to web-node.yaml.tpl
ceph_fsid       = random_uuid.ceph_fsid.result
storage_node_ip = var.storage_nodes[0].ip
```

### 2.3 Keyring distribution

**Current approach:**

```bash
scp ubuntu@${storage_node_ip}:/etc/ceph/ceph.client.web.keyring /etc/ceph/
```

The `client.web` keyring is created on the storage node during its setup:

```bash
ceph auth get-or-create client.web \
  mon 'allow r' osd 'allow rw pool=cephfs_data' mds 'allow rw' \
  > /etc/ceph/ceph.client.web.keyring
```

**Problem:** Same SCP fragility as above. Also, the keyring file format for kernel mount is different from the full keyring format. The kernel `mount.ceph` expects the `secretfile` to contain just the base64 secret, not the full keyring.

**Target approach:**

1. Generate a deterministic secret for `client.web` at Terraform time and inject it into both the storage node (to register with Ceph auth) and the web nodes (as a secret file).
2. Alternatively (simpler): Keep the keyring generated on the storage node, but use a cloud-init `bootcmd` with retry logic to fetch it, and convert it to the kernel-mount format.

**Recommended: Two-phase approach.**

Phase 1 (immediate fix): Keep SCP but add retry logic and a proper secret file:

```yaml
runcmd:
  # Wait for storage node to be ready (CephFS keyring exists).
  - |
    for i in $(seq 1 60); do
      scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 \
        ubuntu@${storage_node_ip}:/etc/ceph/ceph.client.web.keyring /etc/ceph/ && break
      sleep 5
    done
  # Extract the secret key for kernel mount.
  - grep 'key = ' /etc/ceph/ceph.client.web.keyring | awk '{print $3}' > /etc/ceph/ceph.client.web.secret
  - chmod 600 /etc/ceph/ceph.client.web.secret
```

Phase 2 (production): Move FSID and web client secret to Terraform-managed variables. Inject via cloud-init `write_files` on both storage and web nodes. No SCP dependency.

---

## 3. Mount Management

### 3.1 Mount point

All web node code already uses `/var/www/storage` as the CephFS root. This is configured via the `WEB_STORAGE_DIR=/var/www/storage` environment variable in the node-agent config. The directory is created in the Packer build (`packer/scripts/web.sh`):

```bash
mkdir -p /var/www/storage
```

### 3.2 Current mount command

The cloud-init `runcmd` currently does:

```bash
mount -t ceph ${storage_node_ip}:/ /var/www/storage \
  -o name=web,secretfile=/etc/ceph/ceph.client.web.keyring,noatime
```

**Problems:**

1. `secretfile` should point to a file containing just the base64 secret, not the full keyring. The kernel CephFS client expects the raw key.
2. No `_netdev` option, which tells systemd the mount requires network.
3. The fstab entry is appended in runcmd, meaning multiple cloud-init runs could duplicate it.

### 3.3 Target: systemd mount unit

Replace the fstab entry and ad-hoc mount command with a proper systemd `.mount` unit. This gives us:

- Automatic dependency ordering (after network)
- Mount health visibility via `systemctl status`
- Automatic remount on failure via `RestartSec`
- Clean integration with the node-agent service dependency chain

**File: `/etc/systemd/system/var-www-storage.mount`**

```ini
[Unit]
Description=CephFS Web Storage
After=network-online.target
Wants=network-online.target
# If the mount fails, don't block the rest of boot.
# The node-agent will detect the missing mount and report unhealthy.

[Mount]
What=${storage_node_ip}:/
Where=/var/www/storage
Type=ceph
Options=name=web,secretfile=/etc/ceph/ceph.client.web.secret,noatime,_netdev
TimeoutSec=30

[Install]
WantedBy=multi-user.target
```

**File: `/etc/systemd/system/var-www-storage.automount`** (optional, for lazy mount):

Not needed. Web storage must be available before the node-agent starts serving traffic.

**Dependency chain:**

```
network-online.target
  -> var-www-storage.mount
    -> node-agent.service
```

Update the node-agent systemd unit to require the mount:

```ini
# In node-agent.service
[Unit]
After=var-www-storage.mount
Requires=var-www-storage.mount
```

### 3.4 Mount options

| Option       | Purpose                                                  |
|-------------|----------------------------------------------------------|
| `name=web`  | CephX client name (`client.web`)                         |
| `secretfile=...` | Path to file containing the base64 CephX secret    |
| `noatime`   | Avoid metadata writes on every read (performance)        |
| `_netdev`   | Tells systemd this is a network filesystem               |
| `rw`        | Read-write (default, but explicit for clarity)            |

### 3.5 Mount health checks

The node-agent should verify the CephFS mount is healthy before performing any webroot operations. Two levels of checking:

**Level 1: Mount presence check (fast, every operation).**

Before any `WebrootManager` or `TenantManager` operation, verify the mount exists:

```go
func (m *WebrootManager) checkMount() error {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(m.webStorageDir, &stat); err != nil {
        return status.Errorf(codes.Unavailable,
            "CephFS not mounted at %s: %v", m.webStorageDir, err)
    }
    // CephFS magic number: 0x00C36400
    if stat.Type != 0x00C36400 {
        return status.Errorf(codes.Unavailable,
            "unexpected filesystem at %s: type=0x%X (expected CephFS 0x00C36400)",
            m.webStorageDir, stat.Type)
    }
    return nil
}
```

**Level 2: Write probe (periodic, for health reporting).**

A periodic goroutine in the node-agent writes and reads a sentinel file to confirm CephFS is functional:

```go
func (m *WebrootManager) probeMount(ctx context.Context) error {
    probePath := filepath.Join(m.webStorageDir, ".node-probe-"+nodeID)
    if err := os.WriteFile(probePath, []byte(time.Now().String()), 0600); err != nil {
        return fmt.Errorf("CephFS write probe failed: %w", err)
    }
    if err := os.Remove(probePath); err != nil {
        return fmt.Errorf("CephFS delete probe failed: %w", err)
    }
    return nil
}
```

This probe result feeds into the node-agent's `/healthz` endpoint so the platform can detect degraded nodes.

### 3.6 Delivery via cloud-init

The systemd mount unit file should be written via cloud-init `write_files` and enabled in `runcmd`:

```yaml
write_files:
  - path: /etc/systemd/system/var-www-storage.mount
    content: |
      [Unit]
      Description=CephFS Web Storage
      After=network-online.target
      Wants=network-online.target

      [Mount]
      What=${storage_node_ip}:/
      Where=/var/www/storage
      Type=ceph
      Options=name=web,secretfile=/etc/ceph/ceph.client.web.secret,noatime,_netdev
      TimeoutSec=30

      [Install]
      WantedBy=multi-user.target

runcmd:
  # (keyring fetch + secret extraction as described in section 2.3)
  - systemctl daemon-reload
  - systemctl enable --now var-www-storage.mount
  # Wait for mount to be active before starting node-agent.
  - systemctl is-active var-www-storage.mount
  - systemctl start node-agent
```

---

## 4. Node Agent Integration

### 4.1 Mount verification before operations

Every mutating webroot/tenant operation must verify the CephFS mount is present. This is a guard, not a retry mechanism -- if CephFS is down, the activity should fail and Temporal's retry policy handles the retry.

**Add to `WebrootManager`:**

```go
func (m *WebrootManager) Create(ctx context.Context, info *runtime.WebrootInfo) error {
    if err := m.checkMount(); err != nil {
        return err
    }
    // ... existing logic
}
```

Same pattern for `Update`, `Delete`. Same for `TenantManager.Create`, `TenantManager.Delete`.

### 4.2 Health reporting

The node-agent currently has no health endpoint. Add one that reports CephFS mount status:

```go
type HealthStatus struct {
    CephFSMounted  bool   `json:"cephfs_mounted"`
    CephFSWritable bool   `json:"cephfs_writable"`
    LastProbeAt    string `json:"last_probe_at"`
    LastError      string `json:"last_error,omitempty"`
}
```

This feeds into the Temporal heartbeat mechanism: if the node's CephFS mount is degraded, the workflow layer can avoid routing new webroot operations to that node (graceful degradation).

### 4.3 No changes to WebStorageDir

The `WebStorageDir` config (`/var/www/storage`) remains unchanged. The node-agent does not need to know it is CephFS -- it just operates on a directory. The mount verification is an additional safety layer, not a fundamental change to the storage abstraction.

### 4.4 Activity-level error handling

When the mount check fails, the activity returns `codes.Unavailable`. Temporal's retry policy will retry the activity on the same node. If CephFS recovers within the retry window, the operation succeeds. If not, the workflow fails with a clear error message indicating CephFS is unavailable.

The `convergeWebShard` workflow already collects per-node errors without stopping, so a single node's CephFS failure does not block convergence on other nodes.

---

## 5. Tenant Isolation

### 5.1 Directory structure

The existing structure (from `TenantManager.Create`) is correct and already lives on CephFS:

```
/var/www/storage/                     (CephFS root, mounted)
  {tenant}/                           root:root 0755 (ChrootDirectory)
    home/                             tenant:tenant 0700
    webroots/                         tenant:tenant 0750
      {webroot}/                      tenant:tenant 0755
        public/                       tenant:tenant 0755  (optional)
    logs/                             tenant:tenant 0750
    tmp/                              tenant:tenant 1777
```

### 5.2 POSIX permissions

CephFS fully supports POSIX permissions, including uid/gid ownership. Since all web nodes in a shard create the same Linux users with the same UIDs (the `uid` field comes from the core database via `TenantManager.Create`), ownership is consistent across nodes. This is a key correctness requirement: **tenant UIDs must be globally unique and deterministic**, which the current schema enforces via the `tenants.uid` column.

### 5.3 CephFS quotas

CephFS supports directory-level quotas via extended attributes. This should be used to enforce per-tenant disk usage limits.

**Setting a quota:**

```bash
# Set a 10GB quota on a tenant directory
setfattr -n ceph.quota.max_bytes -v 10737418240 /var/www/storage/{tenant}

# Set a file count limit (optional, protects against inode exhaustion)
setfattr -n ceph.quota.max_files -v 100000 /var/www/storage/{tenant}
```

**Integration with the platform:**

1. Add a `disk_quota_bytes` column to the `tenants` table (bigint, default from platform config).
2. The `CreateTenant` activity sets the quota xattr after creating the directory structure.
3. The `UpdateTenant` activity can modify the quota.
4. Quota enforcement is done by CephFS in the kernel -- no node-agent polling needed.

**Implementation in TenantManager:**

```go
func (m *TenantManager) setQuota(ctx context.Context, tenantDir string, quotaBytes int64) error {
    cmd := exec.CommandContext(ctx, "setfattr",
        "-n", "ceph.quota.max_bytes",
        "-v", strconv.FormatInt(quotaBytes, 10),
        tenantDir,
    )
    if output, err := cmd.CombinedOutput(); err != nil {
        return status.Errorf(codes.Internal,
            "set CephFS quota on %s: %s: %v", tenantDir, string(output), err)
    }
    return nil
}
```

**Quota reporting:**

```bash
# Read current usage
getfattr -n ceph.dir.rbytes /var/www/storage/{tenant}
# Read quota limit
getfattr -n ceph.quota.max_bytes /var/www/storage/{tenant}
```

This can be exposed via a `GetTenantDiskUsage` activity that the API layer queries for usage reporting.

### 5.4 No kernel-level user namespace isolation

CephFS does not provide user-namespace isolation. Tenants are isolated by:

1. Linux user accounts with distinct UIDs.
2. POSIX directory permissions (0755 on chroot root, 0750 on subdirs).
3. OpenSSH ChrootDirectory for SSH/SFTP access.
4. PHP-FPM running as the tenant's UID.
5. CephFS quotas limiting storage consumption.

A tenant process cannot read another tenant's files because the directory permissions prevent it. A compromised web process running as tenant A cannot traverse to tenant B's directory.

---

## 6. Failover and Degradation

### 6.1 Ceph cluster degrades (single MDS)

The current setup has a single MDS. If it goes down:

- Existing CephFS mounts on web nodes will hang on metadata operations (ls, stat, open of new files).
- In-flight HTTP requests that need to open files will timeout.
- Nginx serves cached/open file descriptors until they expire.
- Nginx `sendfile()` on already-open FDs continues working briefly.

**Mitigation (immediate):**

- MDS standby: Add a standby MDS daemon to the storage node (or a second storage node). CephFS automatically promotes the standby if the active MDS fails.
- Kernel client recovery: The CephFS kernel client has a configurable `mount_timeout` (default 300s). After this, stale operations fail with EIO. Nginx returns 502/504 for affected requests.

**Mitigation (production):**

- Run 2+ MDS daemons (1 active + 1 standby-replay) on separate hosts.
- Monitor MDS health via `ceph mds stat` in a cluster-level health check.

### 6.2 OSD failure (single OSD)

The current setup has `osd pool default size = 1` (no replication). An OSD failure means data loss.

**Production requirement:**

- Minimum 3 OSDs across 2+ hosts with `osd pool default size = 2` (or 3).
- This is an infrastructure concern, not a node-agent concern.

### 6.3 Network partition between web node and storage node

If a web node loses connectivity to the storage node's Ceph mon/OSD:

- The kernel CephFS client enters a `recovering` state.
- File operations block for up to `mount_timeout` seconds.
- After timeout, operations fail with EIO.

**Node-agent behavior:**

- The mount health probe (section 3.5) detects the failure.
- The node reports itself as unhealthy.
- New Temporal activities targeting this node will fail with `codes.Unavailable`.
- The HAProxy health check (if implemented) removes the node from the backend pool.

**HAProxy integration:**

Each web node should expose an HTTP health endpoint that checks CephFS mount status. HAProxy uses this for its server health checks:

```
backend web-1
    option httpchk GET /healthz
    server web-1-node-0 10.10.10.10:80 check
    server web-1-node-1 10.10.10.11:80 check
```

The health endpoint on the node-agent (or a lightweight HTTP server on each web node) returns 200 only when CephFS is mounted and writable. This ensures HAProxy stops routing traffic to a node whose CephFS mount is degraded.

### 6.4 Mount recovery

The kernel CephFS client automatically reconnects when the Ceph cluster recovers. No manual intervention is needed. The systemd mount unit's `TimeoutSec=30` applies only to the initial mount, not to runtime recovery.

If the mount enters an unrecoverable state (e.g., the secret key was revoked), a manual `umount -f /var/www/storage && systemctl start var-www-storage.mount` is needed. This can be triggered via a Temporal activity on the node.

---

## 7. Cloud-init / Packer Changes

### 7.1 Packer (`packer/scripts/web.sh`) -- No changes needed

The web image already installs `ceph-common` and creates `/var/www/storage` and `/etc/ceph`. Current state is correct:

```bash
apt-get install -y ceph-common
mkdir -p /var/www/storage /etc/ceph
```

### 7.2 Packer (`packer/scripts/storage.sh`) -- No changes needed

The storage image already installs `ceph ceph-mon ceph-osd ceph-mgr ceph-mds radosgw`. Current state is correct.

### 7.3 Terraform (`terraform/variables.tf`) -- Add Ceph FSID variable

```hcl
resource "random_uuid" "ceph_fsid" {}
```

This FSID is passed to both the storage-node and web-node cloud-init templates, removing the runtime dependency on SCP.

### 7.4 Cloud-init (`terraform/cloud-init/storage-node.yaml.tpl`) -- Accept FSID

Change the setup-ceph.sh script to use the Terraform-provided FSID instead of generating one:

```diff
- FSID=$(cat /proc/sys/kernel/random/uuid)
+ FSID="${ceph_fsid}"
```

After this change, the storage node must be reprovisioned (destroy + recreate) since the FSID is baked into the monitor at creation time. This is acceptable because this is pre-release software and the migration policy is to wipe and restart.

### 7.5 Cloud-init (`terraform/cloud-init/web-node.yaml.tpl`) -- Full rewrite

Replace the current SCP-based approach with a robust setup:

```yaml
#cloud-config
hostname: ${hostname}
manage_etc_hosts: true

users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ${ssh_public_key}

write_files:
  - path: /etc/default/node-agent
    content: |
      TEMPORAL_ADDRESS=${temporal_address}
      NODE_ID=${node_id}
      SHARD_NAME=${shard_name}
      NGINX_CONFIG_DIR=/etc/nginx
      WEB_STORAGE_DIR=/var/www/storage
      CERT_DIR=/etc/ssl/hosting
      SSH_CONFIG_DIR=/etc/ssh/sshd_config.d
      REGION_ID=${region_id}
      CLUSTER_ID=${cluster_id}
      NODE_ROLE=web
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

  - path: /etc/ceph/ceph.conf
    content: |
      [global]
      fsid = ${ceph_fsid}
      mon host = ${storage_node_ip}
      auth cluster required = cephx
      auth service required = cephx
      auth client required = cephx

  - path: /etc/systemd/system/var-www-storage.mount
    content: |
      [Unit]
      Description=CephFS Web Storage
      After=network-online.target
      Wants=network-online.target

      [Mount]
      What=${storage_node_ip}:/
      Where=/var/www/storage
      Type=ceph
      Options=name=web,secretfile=/etc/ceph/ceph.client.web.secret,noatime,_netdev
      TimeoutSec=30

      [Install]
      WantedBy=multi-user.target

runcmd:
  # Fetch the CephFS client keyring from the storage node.
  # Retry up to 5 minutes in case the storage node is still bootstrapping.
  - |
    for i in $(seq 1 60); do
      scp -o StrictHostKeyChecking=no -o ConnectTimeout=5 \
        ubuntu@${storage_node_ip}:/etc/ceph/ceph.client.web.keyring \
        /etc/ceph/ceph.client.web.keyring && break
      echo "Waiting for storage node CephFS keyring (attempt $i/60)..."
      sleep 5
    done
  # Extract the base64 secret for the kernel CephFS client.
  - grep 'key = ' /etc/ceph/ceph.client.web.keyring | awk '{print $3}' > /etc/ceph/ceph.client.web.secret
  - chmod 600 /etc/ceph/ceph.client.web.secret /etc/ceph/ceph.client.web.keyring
  # Mount CephFS via systemd.
  - systemctl daemon-reload
  - systemctl enable --now var-www-storage.mount
  # Verify the mount is active before starting the node-agent.
  - mountpoint -q /var/www/storage || (echo "FATAL: CephFS not mounted" && exit 1)
  - systemctl start node-agent
```

### 7.6 Node-agent systemd unit update

Update `packer/files/node-agent.service` to declare a dependency on the CephFS mount for web nodes. Since the same service file is used across all node roles, the dependency should be conditional or the web Packer image should install a drop-in:

**Option A: Packer drop-in for web nodes** (preferred):

Add to `packer/scripts/web.sh`:

```bash
mkdir -p /etc/systemd/system/node-agent.service.d
cat > /etc/systemd/system/node-agent.service.d/cephfs.conf << 'EOF'
[Unit]
After=var-www-storage.mount
Requires=var-www-storage.mount
EOF
```

This way the base `node-agent.service` remains shared, but on web nodes it automatically gains the CephFS dependency.

---

## 8. Implementation Order

### Phase 1: Fix the mount (immediate, small diff)

1. Add `random_uuid.ceph_fsid` to Terraform.
2. Update `storage-node.yaml.tpl` to accept `ceph_fsid` from Terraform.
3. Rewrite `web-node.yaml.tpl` with retry logic, secret extraction, and systemd mount unit.
4. Add the `node-agent.service.d/cephfs.conf` drop-in to `packer/scripts/web.sh`.
5. Rebuild Packer images and redeploy (`terraform destroy && terraform apply`).

### Phase 2: Node-agent mount verification (code change)

1. Add `checkMount()` to `WebrootManager` and `TenantManager`.
2. Call `checkMount()` at the start of `Create`, `Update`, `Delete` in both managers.
3. Add the periodic mount probe goroutine to the node-agent.
4. Expose mount health via the node-agent's `/healthz` endpoint.
5. Write unit tests (mock the `Statfs` call).

### Phase 3: CephFS quotas (schema + code change)

1. Add `disk_quota_bytes` to the `tenants` table.
2. Add `setQuota()` to `TenantManager`.
3. Call `setQuota()` in the `CreateTenant` and `UpdateTenant` activities.
4. Add a `GetTenantDiskUsage` activity that reads `ceph.dir.rbytes`.
5. Expose disk usage via the tenant API.

### Phase 4: Production hardening

1. Add MDS standby daemon configuration.
2. Increase OSD replication factor.
3. Add HAProxy health check integration.
4. Add alerting on CephFS mount failures (Vector -> Loki -> alert rules).

---

## 9. Files Modified

| File | Change |
|------|--------|
| `terraform/variables.tf` | Add description update for storage_nodes (already has MDS mention) |
| `terraform/nodes.tf` | Add `random_uuid.ceph_fsid`, pass to cloud-init templates |
| `terraform/cloud-init/storage-node.yaml.tpl` | Accept `ceph_fsid` variable, use it instead of random UUID |
| `terraform/cloud-init/web-node.yaml.tpl` | Full rewrite: ceph.conf via write_files, systemd mount unit, retry logic |
| `packer/scripts/web.sh` | Add systemd drop-in for CephFS mount dependency |
| `packer/files/node-agent.service` | No change (drop-in handles it) |
| `internal/agent/webroot.go` | Add `checkMount()`, call before Create/Update/Delete |
| `internal/agent/tenant.go` | Add `checkMount()`, call before Create/Delete |
| `internal/agent/server.go` | No change (WebStorageDir already in Config) |
| `internal/config/config.go` | No change |
| `internal/workflow/converge_shard.go` | No change (error handling already captures per-node failures) |

---

## 10. Risks and Open Questions

1. **Single MDS is a SPOF.** The current single-node Ceph cluster has no MDS redundancy. An MDS crash takes down all web storage until recovery. This is acceptable for dev but must be addressed before production.

2. **Single OSD means no replication.** Data loss risk. Production requires 3+ OSDs with replication.

3. **Keyring rotation.** If the `client.web` secret is compromised, it must be rotated on all web nodes. Currently this requires re-running cloud-init or SSH-ing to each node. Consider a Temporal workflow that pushes new keyring material and remounts.

4. **CephFS kernel client vs FUSE.** The kernel client is faster but less forgiving of cluster issues. The FUSE client (`ceph-fuse`) is more resilient to transient failures but has higher latency. The kernel client is the right choice for web hosting where latency matters.

5. **Tenant directory creation race.** When a new tenant is created, the `CreateTenant` activity runs on all web nodes in the shard. Since CephFS is shared, only one node actually creates the directories; the others are idempotent no-ops (`os.MkdirAll`). The `chown` calls are also idempotent. No race condition exists because the operations converge to the same state regardless of execution order.

6. **Log files on CephFS.** The nginx config writes access/error logs to `/var/www/storage/{tenant}/logs/`. On CephFS, concurrent writes from multiple web nodes to the same log file would interleave unpredictably. However, each nginx instance writes to the same filename, and CephFS provides atomic appends for O_APPEND writes, so log lines will not be corrupted but may be interleaved. If per-node log isolation is needed, the log filename should include the hostname (already available via `$hostname` in nginx). This is a minor concern since Vector ships logs to Loki anyway.
