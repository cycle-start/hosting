version: 2
ethernets:
  ens3:
    addresses:
      - ${ip_address}/24
    gateway4: ${gateway}
    nameservers:
      addresses:
        - ${gateway}
        - 8.8.8.8
