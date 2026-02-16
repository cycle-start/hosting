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
  - path: /opt/stalwart-mail/etc/config.toml
    owner: stalwart-mail:stalwart-mail
    permissions: '0640'
    content: |
      [server]
      hostname = "${mail_hostname}"

      [server.listener.smtp]
      bind = ["[::]:25"]
      protocol = "smtp"

      [server.listener.submission]
      bind = ["[::]:587"]
      protocol = "smtp"

      [server.listener.imap]
      bind = ["[::]:143"]
      protocol = "imap"

      [server.listener.http]
      bind = ["[::]:8080"]
      protocol = "http"

      [server.listener.sieve]
      bind = ["[::]:4190"]
      protocol = "managesieve"

      [storage]
      data = "rocksdb"
      blob = "rocksdb"
      fts = "rocksdb"
      lookup = "rocksdb"
      directory = "internal"

      [store.rocksdb]
      type = "rocksdb"
      path = "/opt/stalwart-mail/data"

      [directory.internal]
      type = "internal"
      store = "rocksdb"

      [authentication.fallback-admin]
      user = "admin"
      secret = "${stalwart_admin_password}"

      [authentication.fallback-admin.permissions]
      role = "admin"

  - path: /etc/default/node-agent
    content: |
      TEMPORAL_ADDRESS=${temporal_address}
      NODE_ID=${node_id}
      SHARD_NAME=${shard_name}
      REGION_ID=${region_id}
      CLUSTER_ID=${cluster_id}
      NODE_ROLE=email
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

runcmd:
  - chown -R stalwart-mail:stalwart-mail /opt/stalwart-mail
  - systemctl daemon-reload
  - systemctl enable --now stalwart-mail
  - systemctl start node-agent
