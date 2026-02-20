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
      INIT_SYSTEM=systemd
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

  - path: /etc/ceph/ceph.client.web.secret
    permissions: '0600'
    content: "${ceph_web_key}"

  - path: /etc/systemd/system/var-www-storage.mount
    content: |
      [Unit]
      Description=CephFS Web Storage
      After=network-online.target
      Wants=network-online.target

      [Mount]
      What=web@${ceph_fsid}.cephfs=/
      Where=/var/www/storage
      Type=ceph
      Options=secretfile=/etc/ceph/ceph.client.web.secret,noatime,_netdev
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

  - path: /etc/modules-load.d/dummy.conf
    content: |
      dummy

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

  - path: /etc/nginx/conf.d/hosting-log-format.conf
    content: |
      log_format hosting_json escape=json
        '{'
          '"time":"$time_iso8601",'
          '"remote_addr":"$remote_addr",'
          '"method":"$request_method",'
          '"uri":"$request_uri",'
          '"status":$status,'
          '"bytes_sent":$bytes_sent,'
          '"request_time":$request_time,'
          '"upstream_time":"$upstream_response_time",'
          '"http_referer":"$http_referer",'
          '"http_user_agent":"$http_user_agent",'
          '"host":"$host",'
          '"server_name":"$server_name"'
        '}';

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
  # Wait for the CephFS mount to succeed (storage node needs time to boot and
  # create the CephFS filesystem). Retry mount for up to 5 minutes.
  - systemctl daemon-reload
  - |
    for i in $(seq 1 60); do
      systemctl start var-www-storage.mount && mountpoint -q /var/www/storage && break
      echo "Waiting for CephFS mount (attempt $i/60)..."
      sleep 5
    done
  - systemctl enable var-www-storage.mount
  # Verify mount succeeded.
  - "mountpoint -q /var/www/storage || { echo 'FATAL: CephFS not mounted'; exit 1; }"
  # Ensure tenant0 dummy interface is up (module loaded by systemd-modules-load
  # from /etc/modules-load.d/dummy.conf, interface created by systemd-networkd
  # from /etc/systemd/network/50-tenant0.netdev).
  - modprobe dummy
  - systemctl restart systemd-networkd
  - systemctl start node-agent
