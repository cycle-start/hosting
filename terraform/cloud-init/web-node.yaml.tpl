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

runcmd:
  # Fetch CephFS client keyring and config from storage node.
  - scp -o StrictHostKeyChecking=no ubuntu@${storage_node_ip}:/etc/ceph/ceph.client.web.keyring /etc/ceph/
  - scp -o StrictHostKeyChecking=no ubuntu@${storage_node_ip}:/etc/ceph/ceph.conf /etc/ceph/
  # Mount CephFS for shared web storage.
  - mount -t ceph ${storage_node_ip}:/ /var/www/storage -o name=web,secretfile=/etc/ceph/ceph.client.web.keyring
  - echo "${storage_node_ip}:/ /var/www/storage ceph name=web,secretfile=/etc/ceph/ceph.client.web.keyring,noatime 0 0" >> /etc/fstab
  - systemctl daemon-reload
  - systemctl start node-agent
