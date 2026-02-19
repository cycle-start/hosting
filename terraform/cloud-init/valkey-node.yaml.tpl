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
      VALKEY_CONFIG_DIR=/etc/valkey
      VALKEY_DATA_DIR=/var/lib/valkey
      REGION_ID=${region_id}
      CLUSTER_ID=${cluster_id}
      NODE_ROLE=valkey
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

runcmd:
  - systemctl daemon-reload
  - systemctl start node-agent
