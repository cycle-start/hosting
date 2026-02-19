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
  - path: /opt/hosting/setup-ceph.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      set -ex

      IP="${ip_address}"
      HOST=$(hostname -s)
      FSID="${ceph_fsid}"
      OSD_DISK="/dev/vdc"

      mkdir -p /etc/ceph /var/lib/ceph/bootstrap-osd

      # --- ceph.conf ---
      cat > /etc/ceph/ceph.conf << CONF
      [global]
      fsid = $FSID
      mon initial members = $HOST
      mon host = $IP
      public network = ${ip_address}/24
      auth cluster required = cephx
      auth service required = cephx
      auth client required = cephx
      osd pool default size = 1
      osd pool default min size = 1
      osd crush chooseleaf type = 0
%{ if s3_enabled ~}

      [client.rgw.$HOST]
      rgw frontends = beast port=7480
%{ endif ~}
      CONF

      # --- Keyrings ---
      ceph-authtool --create-keyring /tmp/ceph.mon.keyring \
        --gen-key -n mon. --cap mon 'allow *'

      ceph-authtool --create-keyring /etc/ceph/ceph.client.admin.keyring \
        --gen-key -n client.admin \
        --cap mon 'allow *' --cap osd 'allow *' --cap mds 'allow *' --cap mgr 'allow *'

      ceph-authtool --create-keyring /var/lib/ceph/bootstrap-osd/ceph.keyring \
        --gen-key -n client.bootstrap-osd \
        --cap mon 'profile bootstrap-osd' --cap mgr 'allow r'

      ceph-authtool /tmp/ceph.mon.keyring \
        --import-keyring /etc/ceph/ceph.client.admin.keyring
      ceph-authtool /tmp/ceph.mon.keyring \
        --import-keyring /var/lib/ceph/bootstrap-osd/ceph.keyring

      # --- Monitor ---
      monmaptool --create --add "$HOST" "$IP" --fsid "$FSID" /tmp/monmap

      mkdir -p "/var/lib/ceph/mon/ceph-$HOST"
      chown ceph:ceph "/var/lib/ceph/mon/ceph-$HOST"
      chown ceph:ceph /tmp/ceph.mon.keyring /tmp/monmap
      sudo -u ceph ceph-mon --mkfs -i "$HOST" \
        --monmap /tmp/monmap --keyring /tmp/ceph.mon.keyring

      systemctl enable --now "ceph-mon@$HOST"

      # Wait for monitor to be ready.
      for i in $(seq 1 30); do
        ceph -s 2>/dev/null && break
        sleep 2
      done

      # Allow single-node pool creation.
      ceph config set mon auth_allow_insecure_global_id_reclaim false || true

      # --- Manager ---
      mkdir -p "/var/lib/ceph/mgr/ceph-$HOST"
      ceph auth get-or-create "mgr.$HOST" \
        mon 'allow profile mgr' osd 'allow *' mds 'allow *' \
        > "/var/lib/ceph/mgr/ceph-$HOST/keyring"
      chown -R ceph:ceph "/var/lib/ceph/mgr/ceph-$HOST"

      systemctl enable --now "ceph-mgr@$HOST"

      # --- OSD (BlueStore on /dev/vdc via LVM) ---
      pvcreate --yes "$OSD_DISK"
      vgcreate ceph-osd-vg "$OSD_DISK"
      lvcreate --yes -n osd-data -l 100%FREE ceph-osd-vg

      ceph-volume lvm create --data ceph-osd-vg/osd-data

      # Wait for OSD to come up.
      for i in $(seq 1 30); do
        ceph osd stat 2>/dev/null | grep -q "1 up" && break
        sleep 2
      done
%{ if s3_enabled ~}

      # --- RADOS Gateway (S3 API) ---
      mkdir -p "/var/lib/ceph/radosgw/ceph-rgw.$HOST"
      ceph auth get-or-create "client.rgw.$HOST" \
        osd 'allow rwx' mon 'allow rw' \
        > "/var/lib/ceph/radosgw/ceph-rgw.$HOST/keyring"
      chown -R ceph:ceph "/var/lib/ceph/radosgw/ceph-rgw.$HOST"

      systemctl enable --now "ceph-radosgw@rgw.$HOST"

      # Wait for RGW to be ready.
      for i in $(seq 1 60); do
        curl -sf http://localhost:7480 > /dev/null 2>&1 && break
        sleep 2
      done

      # --- RGW admin user ---
      radosgw-admin user create \
        --uid=admin --display-name="Admin" --system \
        > /tmp/rgw-admin.json

      ACCESS_KEY=$(jq -r '.keys[0].access_key' /tmp/rgw-admin.json)
      SECRET_KEY=$(jq -r '.keys[0].secret_key' /tmp/rgw-admin.json)

      # Append RGW credentials to node-agent env file.
      cat >> /etc/default/node-agent << ENVFILE
      RGW_ADMIN_ACCESS_KEY=$ACCESS_KEY
      RGW_ADMIN_SECRET_KEY=$SECRET_KEY
      ENVFILE

      echo "Ceph RGW setup complete. Endpoint: http://$IP:7480"
%{ endif ~}
%{ if filestore_enabled ~}

      # --- CephFS MDS ---
      mkdir -p "/var/lib/ceph/mds/ceph-$HOST"
      ceph auth get-or-create "mds.$HOST" \
        osd 'allow rwx' mds 'allow *' mon 'allow profile mds' \
        > "/var/lib/ceph/mds/ceph-$HOST/keyring"
      chown -R ceph:ceph "/var/lib/ceph/mds/ceph-$HOST"

      systemctl enable --now "ceph-mds@$HOST"

      # Create CephFS pools and filesystem
      ceph osd pool create cephfs_data 32
      ceph osd pool create cephfs_metadata 16
      ceph fs new cephfs cephfs_metadata cephfs_data

      # Import the pre-generated web client key (shared with web nodes via Terraform).
      cat > /etc/ceph/ceph.client.web.keyring << KEYRING
[client.web]
	key = ${ceph_web_key}
	caps mon = "allow r"
	caps osd = "allow rw pool=cephfs_data"
	caps mds = "allow rw"
KEYRING
      ceph auth import -i /etc/ceph/ceph.client.web.keyring

      echo "CephFS setup complete."
%{ endif ~}

  - path: /etc/default/node-agent
    content: |
      TEMPORAL_ADDRESS=${temporal_address}
      NODE_ID=${node_id}
      SHARD_NAME=${shard_name}
      INIT_SYSTEM=systemd
      RGW_ENDPOINT=http://localhost:7480
      REGION_ID=${region_id}
      CLUSTER_ID=${cluster_id}
      NODE_ROLE=storage
      SERVICE_NAME=node-agent
      METRICS_ADDR=:9100

runcmd:
  - /opt/hosting/setup-ceph.sh
  - systemctl daemon-reload
  - systemctl start node-agent
