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
  # Start k3s with traefik and servicelb disabled.
  - systemctl enable --now k3s
  # Wait for k3s to be ready (kubeconfig file + node responding).
  - until [ -f /etc/rancher/k3s/k3s.yaml ] && k3s kubectl get node 2>/dev/null; do sleep 2; done
  # Make kubeconfig accessible for the ubuntu user.
  - mkdir -p /home/ubuntu/.kube
  - cp /etc/rancher/k3s/k3s.yaml /home/ubuntu/.kube/config
  - chmod 644 /etc/rancher/k3s/k3s.yaml
  - chown -R ubuntu:ubuntu /home/ubuntu/.kube
