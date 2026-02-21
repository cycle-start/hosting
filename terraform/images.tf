variable "image_dir" {
  description = "Path to Packer output directory containing golden images"
  type        = string
  default     = "../packer/output"
}

resource "libvirt_pool" "hosting" {
  name   = "hosting"
  type   = "dir"
  target = { path = "/var/lib/libvirt/hosting-pool" }
}

# --- Single base image (Ansible handles role-specific software) ---

resource "libvirt_volume" "image_base" {
  name = "golden-base.qcow2"
  pool = libvirt_pool.hosting.name
  create = {
    content = { url = "${var.image_dir}/base.qcow2" }
    format  = "qcow2"
  }
}
