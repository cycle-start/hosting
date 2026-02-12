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
  - nginx
  - php8.3-fpm
  - php8.3-cli
  - php8.3-mysql
  - php8.3-curl
  - php8.3-mbstring
  - php8.3-xml
  - php8.3-zip
  - curl

mounts:
  - ["hostbin", "/opt/hosting/bin", "9p", "trans=virtio,version=9p2000.L,ro", "0", "0"]

write_files:
  - path: /etc/systemd/system/node-agent.service
    content: |
      [Unit]
      Description=Hosting Node Agent
      After=network-online.target
      Wants=network-online.target

      [Service]
      Type=simple
      ExecStart=/opt/hosting/bin/node-agent
      Environment=TEMPORAL_ADDRESS=${temporal_address}
      Environment=NODE_ID=${node_id}
      Environment=SHARD_NAME=${shard_name}
      Environment=NGINX_CONFIG_DIR=/etc/nginx
      Environment=WEB_STORAGE_DIR=/var/www/storage
      Environment=HOME_BASE_DIR=/home
      Environment=CERT_DIR=/etc/ssl/hosting
      Restart=always
      RestartSec=5

      [Install]
      WantedBy=multi-user.target

runcmd:
  - mkdir -p /var/www/storage /etc/ssl/hosting
  - systemctl daemon-reload
  - systemctl enable --now node-agent
