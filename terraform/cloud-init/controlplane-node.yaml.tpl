#cloud-config
hostname: ${hostname}
manage_etc_hosts: true

users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ${ssh_public_key}

runcmd:
  # k3s is installed and started by Ansible (k3s role), not cloud-init.
  # If k3s is already installed (e.g. baked into the image), start it.
  - test -f /usr/local/bin/k3s && systemctl enable --now k3s || true
