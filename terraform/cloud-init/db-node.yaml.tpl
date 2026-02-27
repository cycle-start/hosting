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
      MYSQL_DSN=root@tcp(127.0.0.1:3306)/
      REGION_ID=${region_id}
      CLUSTER_ID=${cluster_id}
      NODE_ROLE=database
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

  # Staged MySQL config — the mysql Ansible role copies this into place
  # after installing MySQL and creating the config directory.
  - path: /usr/local/etc/mysql/server-id.cnf
    content: |
      [mysqld]
      server-id = ${server_id}
      bind-address = *
      event_scheduler = ON
