resource "libvirt_pool" "hosting" {
  name = "hosting"
  type = "dir"
  path = "/var/lib/libvirt/hosting-pool"
}

resource "libvirt_volume" "ubuntu_base" {
  name   = "ubuntu-24.04-base.qcow2"
  pool   = libvirt_pool.hosting.name
  source = var.base_image_url
  format = "qcow2"
}
