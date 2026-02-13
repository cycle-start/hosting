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
              "rootURI": "/",
              "serviceURI": "/api/",
              "expireSessionAfterPeriod": 1800000,
              "develMode": false,
              "enableSecurityManager": false,
              "forwardProxy": true
          },
          "app": {
              "anonymousAccessEnabled": false,
              "supportsCustomConnections": false,
              "enableReverseProxyAuth": true,
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
              "disabledDrivers": []
          }
      }

  - path: /opt/cloudbeaver/conf/initial-data.conf
    permissions: '0644'
    content: |
      {
          "adminName": "cbadmin",
          "adminPassword": "cbadmin-dev",
          "schemaName": "",
          "roles": [
              {
                  "roleId": "admin",
                  "name": "Admin",
                  "description": "Administrative access",
                  "permissions": ["admin"]
              },
              {
                  "roleId": "user",
                  "name": "User",
                  "description": "Standard user"
              }
          ]
      }

  - path: /etc/default/node-agent
    content: |
      TEMPORAL_ADDRESS=${temporal_address}
      NODE_ID=${node_id}
      SHARD_NAME=${shard_name}
      CLOUDBEAVER_ENDPOINT=http://localhost:8978

runcmd:
  - systemctl daemon-reload
  - systemctl start cloudbeaver
  - systemctl start node-agent
