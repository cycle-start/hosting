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
      REGION_ID=${region_id}
      CLUSTER_ID=${cluster_id}
      NODE_ROLE=gateway
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

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

runcmd:
  - modprobe dummy
  - systemctl restart systemd-networkd
  - systemctl daemon-reload
