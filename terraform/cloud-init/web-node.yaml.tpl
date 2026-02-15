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
      CORE_API_URL=${core_api_url}
      CORE_API_TOKEN=${core_api_token}

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

  - path: /usr/local/bin/cron-outcome
    permissions: '0755'
    content: |
      #!/bin/bash
      # Called by ExecStopPost=+ (as root) after every cron job execution.
      # Reports success/failure to core-api for auto-disable tracking.
      # Exit code 75 = flock lock contention (job skipped) — don't report.
      [ "$EXIT_STATUS" = "75" ] && exit 0
      source /etc/default/node-agent 2>/dev/null
      [ -z "$CORE_API_URL" ] && exit 0
      if [ "$SERVICE_RESULT" = "success" ]; then
        SUCCESS="true"
      else
        SUCCESS="false"
      fi
      curl -sf -X POST \
        -H "Authorization: Bearer $CORE_API_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"success\":$SUCCESS}" \
        "$CORE_API_URL/internal/v1/cron-jobs/$CRON_JOB_ID/outcome" \
        >/dev/null 2>&1 || true

  - path: /etc/systemd/network/50-tenant0.netdev
    content: |
      [NetDev]
      Name=tenant0
      Kind=dummy

  - path: /etc/systemd/network/50-tenant0.network
    content: |
      [Match]
      Name=tenant0
      [Network]
      Description=Tenant ULA addresses

  - path: /etc/logrotate.d/hosting-tenant-logs
    content: |
      /var/log/hosting/*/*.log {
          daily
          rotate 2
          compress
          delaycompress
          missingok
          notifempty
          copytruncate
          maxsize 100M
      }

runcmd:
  # Fetch the CephFS client keyring from the storage node (retry up to 5 minutes).
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
  # Create tenant0 dummy interface for ULA addresses.
  - systemctl restart systemd-networkd
  - systemctl start node-agent
