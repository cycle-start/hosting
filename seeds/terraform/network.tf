resource "libvirt_network" "hosting" {
  name    = "hosting"
  forward = { mode = "nat" }
  domain  = { name = "hosting.local" }

  ips = [{
    address = var.gateway_ip
    netmask = cidrnetmask(var.network_cidr)
  }]

  dns = { enable = "yes" }
}
