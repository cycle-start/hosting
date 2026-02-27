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
      NODE_ROLE=dbadmin
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

  - path: /etc/default/dbadmin-proxy
    content: |
      CORE_API_URL=http://${controlplane_ip}:8090/api/v1
      CORE_API_TOKEN=${core_api_token}
      LISTEN_ADDR=127.0.0.1:4180
      COOKIE_SECRET=CHANGE_ME_generate_with_openssl_rand_base64_24
      SESSION_DIR=/tmp/dbadmin-sessions

  - path: /opt/phpmyadmin/config.inc.php
    permissions: '0644'
    content: |
      <?php
      $cfg['blowfish_secret'] = 'a2d4e6f8a12345678901234567890abc';
      $cfg['Servers'][1]['auth_type'] = 'signon';
      $cfg['Servers'][1]['SignonSession'] = 'SignonSession';
      $cfg['Servers'][1]['SignonURL'] = '/auth/unauthorized';
      $cfg['Servers'][1]['host'] = '';
      $cfg['Servers'][1]['port'] = '';
      $cfg['TempDir'] = '/tmp/phpmyadmin';

  - path: /opt/phpmyadmin/signon.php
    permissions: '0644'
    content: |
      <?php
      $token = $_COOKIE['dbadmin_token'] ?? '';
      if (!$token || !preg_match('/^[a-zA-Z0-9]+$/', $token)) {
          die('Invalid session. Please use the control panel to access DB Admin.');
      }
      $file = '/tmp/dbadmin-sessions/' . $token . '.json';
      $data = @file_get_contents($file);
      if (!$data) {
          die('Session expired. Please use the control panel to access DB Admin.');
      }
      @unlink($file);
      $creds = json_decode($data, true);
      session_name('SignonSession');
      session_start();
      $_SESSION['PMA_single_signon_user'] = $creds['username'];
      $_SESSION['PMA_single_signon_password'] = $creds['password'];
      $_SESSION['PMA_single_signon_host'] = $creds['host'];
      $_SESSION['PMA_single_signon_port'] = (string)$creds['port'];
      $_SESSION['PMA_single_signon_DBNAME'] = $creds['database'];
      setcookie('dbadmin_token', '', time() - 3600, '/', '', true, true);
      header('Location: /index.php');
      exit;

  - path: /etc/nginx/sites-available/dbadmin
    permissions: '0644'
    content: |
      server {
          listen 80;
          listen [::]:80;
          server_name _;

          root /opt/phpmyadmin;
          index index.php;

          location /auth/ {
              proxy_pass http://127.0.0.1:4180;
              proxy_set_header Host $host;
              proxy_set_header X-Real-IP $remote_addr;
              proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
              proxy_set_header X-Forwarded-Proto $scheme;
          }

          location = /signon.php {
              fastcgi_pass unix:/run/php/php-fpm.sock;
              include fastcgi_params;
              fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
          }

          location ~ \.php$ {
              fastcgi_pass unix:/run/php/php-fpm.sock;
              include fastcgi_params;
              fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
          }

          location / {
              try_files $uri $uri/ =404;
          }
      }

      server {
          listen 443 ssl;
          listen [::]:443 ssl;
          server_name _;

          ssl_certificate /etc/nginx/certs/dbadmin.pem;
          ssl_certificate_key /etc/nginx/certs/dbadmin-key.pem;

          root /opt/phpmyadmin;
          index index.php;

          location /auth/ {
              proxy_pass http://127.0.0.1:4180;
              proxy_set_header Host $host;
              proxy_set_header X-Real-IP $remote_addr;
              proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
              proxy_set_header X-Forwarded-Proto $scheme;
          }

          location = /signon.php {
              fastcgi_pass unix:/run/php/php-fpm.sock;
              include fastcgi_params;
              fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
          }

          location ~ \.php$ {
              fastcgi_pass unix:/run/php/php-fpm.sock;
              include fastcgi_params;
              fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
          }

          location / {
              try_files $uri $uri/ =404;
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
  # Create session directories.
  - mkdir -p /tmp/dbadmin-sessions /tmp/phpmyadmin
  - chmod 777 /tmp/phpmyadmin
  # Enable nginx site.
  - rm -f /etc/nginx/sites-enabled/default
  - ln -sf /etc/nginx/sites-available/dbadmin /etc/nginx/sites-enabled/dbadmin
  - systemctl daemon-reload
  - systemctl start php8.3-fpm || systemctl start php-fpm || true
  - systemctl start dbadmin-proxy
  - systemctl restart nginx
