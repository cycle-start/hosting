packer {
  required_plugins {
    qemu = {
      version = ">= 1.1.0"
      source  = "github.com/hashicorp/qemu"
    }
  }
}

# --- Variables ---

variable "node_agent_binary" {
  description = "Path to pre-built linux/amd64 node-agent binary"
  type        = string
  default     = "../bin/node-agent"
}

variable "ubuntu_image_url" {
  description = "Ubuntu 24.04 cloud image URL"
  type        = string
  default     = "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
}

variable "ubuntu_image_checksum" {
  description = "Checksum for the Ubuntu cloud image (set to 'none' to skip)"
  type        = string
  default     = "none"
}

variable "output_dir" {
  description = "Directory for output images"
  type        = string
  default     = "output"
}

# --- QEMU sources (one per role) ---

source "qemu" "web" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "10G"
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "web.qcow2"
  output_directory = "${var.output_dir}/web-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "db" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "20G"
  memory           = 2048
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "db.qcow2"
  output_directory = "${var.output_dir}/db-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "dns" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "10G"
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "dns.qcow2"
  output_directory = "${var.output_dir}/dns-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "valkey" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "10G"
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "valkey.qcow2"
  output_directory = "${var.output_dir}/valkey-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "email" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "10G"
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "email.qcow2"
  output_directory = "${var.output_dir}/email-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "storage" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "20G"
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "storage.qcow2"
  output_directory = "${var.output_dir}/storage-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "dbadmin" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "10G"
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "dbadmin.qcow2"
  output_directory = "${var.output_dir}/dbadmin-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "lb" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "10G"
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "lb.qcow2"
  output_directory = "${var.output_dir}/lb-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

source "qemu" "controlplane" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "20G"
  memory           = 2048
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "controlplane.qcow2"
  output_directory = "${var.output_dir}/controlplane-build"
  net_device       = "virtio-net"
  disk_interface   = "virtio"
  headless         = true
  ssh_username     = "ubuntu"
  ssh_password     = "ubuntu"
  ssh_timeout      = "10m"
  shutdown_command = "sudo shutdown -P now"
  cd_files         = ["http/meta-data", "http/user-data"]
  cd_label         = "cidata"
}

# --- Web image ---

build {
  name = "web"

  sources = ["source.qemu.web"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "files/00-hosting-base.conf"
    destination = "/tmp/00-hosting-base.conf"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "file" {
    source      = "../deploy/vector/web.toml"
    destination = "/tmp/vector-web.toml"
  }

  provisioner "file" {
    source      = "files/supervisor-hosting.conf"
    destination = "/tmp/supervisor-hosting.conf"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/web.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/web-build/web.qcow2 ${var.output_dir}/web.qcow2",
      "rm -rf ${var.output_dir}/web-build",
    ]
  }
}

# --- DB image ---

build {
  name = "db"

  sources = ["source.qemu.db"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "file" {
    source      = "../deploy/vector/db.toml"
    destination = "/tmp/vector-db.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/db.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/db-build/db.qcow2 ${var.output_dir}/db.qcow2",
      "rm -rf ${var.output_dir}/db-build",
    ]
  }
}

# --- DNS image ---

build {
  name = "dns"

  sources = ["source.qemu.dns"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "file" {
    source      = "../deploy/vector/dns.toml"
    destination = "/tmp/vector-dns.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/dns.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/dns-build/dns.qcow2 ${var.output_dir}/dns.qcow2",
      "rm -rf ${var.output_dir}/dns-build",
    ]
  }
}

# --- Valkey image ---

build {
  name = "valkey"

  sources = ["source.qemu.valkey"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "file" {
    source      = "../deploy/vector/valkey.toml"
    destination = "/tmp/vector-valkey.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/valkey.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/valkey-build/valkey.qcow2 ${var.output_dir}/valkey.qcow2",
      "rm -rf ${var.output_dir}/valkey-build",
    ]
  }
}

# --- Email image ---

build {
  name = "email"

  sources = ["source.qemu.email"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "file" {
    source      = "../deploy/vector/email.toml"
    destination = "/tmp/vector-email.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/email.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/email-build/email.qcow2 ${var.output_dir}/email.qcow2",
      "rm -rf ${var.output_dir}/email-build",
    ]
  }
}

# --- Storage image ---

build {
  name = "storage"

  sources = ["source.qemu.storage"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "file" {
    source      = "../deploy/vector/storage.toml"
    destination = "/tmp/vector-storage.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/storage.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/storage-build/storage.qcow2 ${var.output_dir}/storage.qcow2",
      "rm -rf ${var.output_dir}/storage-build",
    ]
  }
}

# --- DB Admin image ---

build {
  name = "dbadmin"

  sources = ["source.qemu.dbadmin"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "files/cloudbeaver.service"
    destination = "/tmp/cloudbeaver.service"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/dbadmin.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/dbadmin-build/dbadmin.qcow2 ${var.output_dir}/dbadmin.qcow2",
      "rm -rf ${var.output_dir}/dbadmin-build",
    ]
  }
}

# --- LB image ---

build {
  name = "lb"

  sources = ["source.qemu.lb"]

  provisioner "file" {
    source      = var.node_agent_binary
    destination = "/tmp/node-agent"
  }

  provisioner "file" {
    source      = "files/node-agent.service"
    destination = "/tmp/node-agent.service"
  }

  provisioner "file" {
    source      = "../deploy/vector/base.toml"
    destination = "/tmp/vector.toml"
  }

  provisioner "file" {
    source      = "../deploy/vector/lb.toml"
    destination = "/tmp/vector-lb.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/common.sh",
      "scripts/lb.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/lb-build/lb.qcow2 ${var.output_dir}/lb.qcow2",
      "rm -rf ${var.output_dir}/lb-build",
    ]
  }
}

# --- Control Plane image (k3s + Helm, no node-agent) ---

build {
  name = "controlplane"

  sources = ["source.qemu.controlplane"]

  provisioner "file" {
    source      = "../deploy/vector/controlplane.toml"
    destination = "/tmp/vector-controlplane.toml"
  }

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/controlplane.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/controlplane-build/controlplane.qcow2 ${var.output_dir}/controlplane.qcow2",
      "rm -rf ${var.output_dir}/controlplane-build",
    ]
  }
}
