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
      NODE_ROLE=lb
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

  # HAProxy config with shard backends from Terraform.
  # Written to a staging path; the Ansible haproxy role copies it into place
  # after installing HAProxy and setting up directories/certs.
  - path: /usr/local/etc/haproxy/haproxy.cfg
    permissions: '0644'
    content: |
      global
          log stdout format raw local0
          maxconn 4096
          stats socket /var/run/haproxy/admin.sock mode 660 level admin expose-fd listeners
          stats socket ipv4@:9999 level admin
          stats timeout 30s

      defaults
          log     global
          mode    http
          option  httplog
          option  dontlognull
          timeout connect 5000ms
          timeout client  50000ms
          timeout server  50000ms

      # Stats UI
      frontend stats
          bind *:8404
          stats enable
          stats uri /stats
          stats refresh 10s

      # Main HTTP/HTTPS frontend (tenant traffic only)
      frontend http
          bind *:80
          bind *:443 ssl crt /etc/haproxy/certs/hosting.pem alpn http/1.1
          # Tenant routing via dynamic map
          use_backend %[req.hdr(host),lower,map(/var/lib/haproxy/maps/fqdn-to-shard.map,shard-default)]

      # Default backend (returns 503 for unmapped FQDNs)
      backend shard-default
          mode http
          http-request deny deny_status 503

      # Shard backends with real VM IPs
%{ for shard_name, servers in shard_backends ~}
      backend shard-${shard_name}
          balance hdr(Host)
          hash-type consistent
%{ for server in servers ~}
          server ${server.name} ${server.ip}:80 check
%{ endfor ~}

%{ endfor ~}
