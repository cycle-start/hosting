version: 2
ethernets:
  eth0:
    match:
      driver: virtio_net
    addresses:
      - ${ip_address}/24
    routes:
      - to: default
        via: ${gateway}
    nameservers:
      addresses:
        - ${gateway}
        - 8.8.8.8
