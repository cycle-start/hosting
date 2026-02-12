resource "libvirt_network" "hosting" {
  name      = "hosting"
  mode      = "nat"
  domain    = "hosting.local"
  addresses = [var.network_cidr]

  dns {
    enabled = true
  }

  dhcp {
    enabled = false
  }
}
