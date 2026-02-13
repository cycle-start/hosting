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

# Main HTTP frontend
frontend http
    bind *:80
    # Control plane routing
    use_backend backend-admin-ui    if { req.hdr(host) -i admin.${base_domain} }
    use_backend backend-core-api    if { req.hdr(host) -i api.${base_domain} }
    use_backend backend-temporal-ui if { req.hdr(host) -i temporal.${base_domain} }
    use_backend backend-dbadmin    if { req.hdr(host) -i dbadmin.${base_domain} }
    # Tenant routing via dynamic map
    use_backend %[req.hdr(host),lower,map(/var/lib/haproxy/maps/fqdn-to-shard.map,shard-default)]

# Control plane backends
backend backend-admin-ui
    server admin 127.0.0.1:3001

backend backend-core-api
    server api 127.0.0.1:8090

backend backend-temporal-ui
    server temporal 127.0.0.1:8080

backend backend-dbadmin
    server dbadmin 127.0.0.1:4180

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
