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
      NODE_ROLE=dbadmin
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

  - path: /etc/default/dbadmin-proxy
    content: |
      CORE_API_URL=http://${controlplane_ip}:8090/api/v1
      CORE_API_TOKEN=${core_api_token}
      LISTEN_ADDR=127.0.0.1:4180
      COOKIE_SECRET=CHANGE_ME_generate_with_openssl_rand_base64_24
      SESSION_DIR=/tmp/dbadmin-sessions


