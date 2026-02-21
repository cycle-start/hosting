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
  - path: /opt/cloudbeaver/conf/cloudbeaver.conf
    permissions: '0644'
    content: |
      {
          "server": {
              "serverPort": 8978,
              "serverName": "CloudBeaver DB Admin",
              "contentRoot": "web",
              "driversLocation": "drivers",
              "rootURI": "/",
              "serviceURI": "/api/",
              "expireSessionAfterPeriod": 1800000,
              "develMode": false,
              "enableSecurityManager": false,
              "forwardProxy": true,
              "database": {
                  "driver": "h2_embedded_v2",
                  "url": "jdbc:h2:$${workspace}/.data/cb.h2v2.dat",
                  "initialDataConfiguration": "conf/initial-data.conf",
                  "pool": {
                      "minIdleConnections": 4,
                      "maxIdleConnections": 10,
                      "maxConnections": 100,
                      "validationQuery": "SELECT 1"
                  },
                  "backupEnabled": true
              }
          },
          "app": {
              "anonymousAccessEnabled": true,
              "supportsCustomConnections": true,
              "forwardProxy": true,
              "publicCredentialsSaveEnabled": false,
              "adminCredentialsSaveEnabled": true,
              "resourceManagerEnabled": true,
              "showReadOnlyConnectionInfo": false,
              "grantConnectionsAccessToAnonymousTeam": false,
              "resourceQuotas": {
                  "dataExportFileSizeLimit": 10000000,
                  "resourceManagerFileSizeLimit": 500000,
                  "sqlMaxRunningQueries": 100,
                  "sqlResultSetRowsLimit": 100000,
                  "sqlResultSetMemoryLimit": 2000000,
                  "sqlTextPreviewMaxLength": 4096,
                  "sqlBinaryPreviewMaxLength": 261120
              },
              "enabledAuthProviders": [
                  "local"
              ],
              "disabledDrivers": []
          }
      }

  - path: /opt/cloudbeaver/conf/initial-data.conf
    permissions: '0644'
    content: |
      {
          "adminName": "cbadmin",
          "adminPassword": "cbadmin-dev",
          "teams": [
              {
                  "subjectId": "admin",
                  "teamName": "Admin",
                  "description": "Administrative access. Has all permissions.",
                  "permissions": ["admin"]
              },
              {
                  "subjectId": "user",
                  "teamName": "User",
                  "description": "All users, including anonymous.",
                  "permissions": []
              }
          ]
      }

  - path: /etc/default/node-agent
    content: |
      TEMPORAL_ADDRESS=${temporal_address}
      NODE_ID=${node_id}
      SHARD_NAME=${shard_name}
      INIT_SYSTEM=systemd
      CLOUDBEAVER_ENDPOINT=http://localhost:8978
      REGION_ID=${region_id}
      CLUSTER_ID=${cluster_id}
      NODE_ROLE=dbadmin
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

  - path: /etc/default/dbadmin-proxy
    content: |
      CORE_API_URL=http://${controlplane_ip}:8090/api/v1
      CORE_API_TOKEN=${core_api_token}
      CLOUDBEAVER_URL=http://127.0.0.1:8978
      LISTEN_ADDR=127.0.0.1:4180
      COOKIE_SECRET=CHANGE_ME_generate_with_openssl_rand_base64_24

  - path: /etc/nginx/sites-available/dbadmin
    permissions: '0644'
    content: |
      server {
          listen 80;
          listen [::]:80;
          server_name _;

          location / {
              proxy_pass http://127.0.0.1:4180;
              proxy_set_header Host $host;
              proxy_set_header X-Real-IP $remote_addr;
              proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
              proxy_set_header X-Forwarded-Proto $scheme;
              proxy_http_version 1.1;
              proxy_set_header Upgrade $http_upgrade;
              proxy_set_header Connection "upgrade";
          }
      }

      server {
          listen 443 ssl;
          listen [::]:443 ssl;
          server_name _;

          ssl_certificate /etc/nginx/certs/dbadmin.pem;
          ssl_certificate_key /etc/nginx/certs/dbadmin-key.pem;

          location / {
              proxy_pass http://127.0.0.1:4180;
              proxy_set_header Host $host;
              proxy_set_header X-Real-IP $remote_addr;
              proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
              proxy_set_header X-Forwarded-Proto $scheme;
              proxy_http_version 1.1;
              proxy_set_header Upgrade $http_upgrade;
              proxy_set_header Connection "upgrade";
          }
      }

runcmd:
  # Generate self-signed cert for nginx.
  - mkdir -p /etc/nginx/certs
  - |
    openssl req -x509 -newkey rsa:2048 \
      -keyout /etc/nginx/certs/dbadmin-key.pem -out /etc/nginx/certs/dbadmin.pem \
      -days 365 -nodes -subj '/CN=dbadmin.${base_domain}' \
      -addext 'subjectAltName=DNS:dbadmin.${base_domain}' 2>/dev/null
  # Enable nginx site.
  - rm -f /etc/nginx/sites-enabled/default
  - ln -sf /etc/nginx/sites-available/dbadmin /etc/nginx/sites-enabled/dbadmin
  - systemctl daemon-reload
  - systemctl start cloudbeaver
  - systemctl start dbadmin-proxy
  - systemctl restart nginx
