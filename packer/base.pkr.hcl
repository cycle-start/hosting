packer {
  required_plugins {
    qemu = {
      version = ">= 1.1.0"
      source  = "github.com/hashicorp/qemu"
    }
  }
}

# --- Variables ---

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

# --- Single base image (Ansible handles role-specific software) ---

source "qemu" "base" {
  iso_url          = var.ubuntu_image_url
  iso_checksum     = var.ubuntu_image_checksum
  disk_image       = true
  disk_size        = "20G"
  memory           = 2048
  format           = "qcow2"
  accelerator      = "kvm"
  vm_name          = "base.qcow2"
  output_directory = "${var.output_dir}/base-build"
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

build {
  name = "base"

  sources = ["source.qemu.base"]

  provisioner "shell" {
    execute_command = "sudo sh -c '{{ .Vars }} {{ .Path }}'"
    scripts = [
      "scripts/base.sh",
    ]
  }

  post-processor "shell-local" {
    inline = [
      "cp ${var.output_dir}/base-build/base.qcow2 ${var.output_dir}/base.qcow2",
      "rm -rf ${var.output_dir}/base-build",
    ]
  }
}
