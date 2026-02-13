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

# --- Golden images (one per role, built by Packer) ---

resource "libvirt_volume" "image_web" {
  name = "golden-web.qcow2"
  pool = libvirt_pool.hosting.name
  create = {
    content = { url = "${var.image_dir}/web.qcow2" }
    format  = "qcow2"
  }
}

resource "libvirt_volume" "image_db" {
  name = "golden-db.qcow2"
  pool = libvirt_pool.hosting.name
  create = {
    content = { url = "${var.image_dir}/db.qcow2" }
    format  = "qcow2"
  }
}

resource "libvirt_volume" "image_dns" {
  name = "golden-dns.qcow2"
  pool = libvirt_pool.hosting.name
  create = {
    content = { url = "${var.image_dir}/dns.qcow2" }
    format  = "qcow2"
  }
}

resource "libvirt_volume" "image_valkey" {
  name = "golden-valkey.qcow2"
  pool = libvirt_pool.hosting.name
  create = {
    content = { url = "${var.image_dir}/valkey.qcow2" }
    format  = "qcow2"
  }
}

resource "libvirt_volume" "image_storage" {
  name = "golden-storage.qcow2"
  pool = libvirt_pool.hosting.name
  create = {
    content = { url = "${var.image_dir}/storage.qcow2" }
    format  = "qcow2"
  }
}

resource "libvirt_volume" "image_dbadmin" {
  name = "golden-dbadmin.qcow2"
  pool = libvirt_pool.hosting.name
  create = {
    content = { url = "${var.image_dir}/dbadmin.qcow2" }
    format  = "qcow2"
  }
}
