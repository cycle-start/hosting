#cloud-config
hostname: ${hostname}
manage_etc_hosts: true

users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ${ssh_public_key}

packages:
  - mysql-server
  - curl

mounts:
  - ["hostbin", "/opt/hosting/bin", "9p", "trans=virtio,version=9p2000.L,ro", "0", "0"]

write_files:
  - path: /etc/systemd/system/node-agent.service
    content: |
      [Unit]
      Description=Hosting Node Agent
      After=network-online.target mysql.service
      Wants=network-online.target

      [Service]
      Type=simple
      ExecStart=/opt/hosting/bin/node-agent
      Environment=TEMPORAL_ADDRESS=${temporal_address}
      Environment=NODE_ID=${node_id}
      Environment=SHARD_NAME=${shard_name}
      Environment=MYSQL_DSN=root@tcp(127.0.0.1:3306)/
      Restart=always
      RestartSec=5

      [Install]
      WantedBy=multi-user.target

runcmd:
  - systemctl daemon-reload
  - systemctl enable --now node-agent
