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
      NODE_ROLE=dns
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100
  - path: /etc/systemd/resolved.conf.d/no-stub.conf
    content: |
      [Resolve]
      DNSStubListener=no
  - path: /etc/powerdns/pdns.d/gpgsql.conf
    owner: "pdns:pdns"
    permissions: "0640"
    content: |
      launch+=gpgsql
      gpgsql-host=${controlplane_ip}
      gpgsql-port=5433
      gpgsql-dbname=hosting_powerdns
      gpgsql-user=hosting
      gpgsql-password=hosting
      gpgsql-dnssec=no

runcmd:
  - systemctl restart systemd-resolved
  - rm -f /etc/powerdns/pdns.d/bind.conf
  - systemctl restart pdns
  - systemctl daemon-reload
