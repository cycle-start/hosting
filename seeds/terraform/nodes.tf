# --- Volumes (backed by golden images) ---

resource "libvirt_volume" "web_node" {
  count    = length(var.web_nodes)
  name     = "${var.web_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "db_node" {
  count    = length(var.db_nodes)
  name     = "${var.db_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "dns_node" {
  count    = length(var.dns_nodes)
  name     = "${var.dns_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "valkey_node" {
  count    = length(var.valkey_nodes)
  name     = "${var.valkey_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "email_node" {
  count    = length(var.email_nodes)
  name     = "${var.email_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "storage_node" {
  count    = length(var.storage_nodes)
  name     = "${var.storage_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

# Dedicated OSD data disk for Ceph BlueStore.
resource "libvirt_volume" "storage_node_osd" {
  count    = length(var.storage_nodes)
  name     = "${var.storage_nodes[count.index].name}-osd.raw"
  pool     = libvirt_pool.hosting.name
  capacity = 10737418240 # 10 GB
  target   = { format = { type = "raw" } }
}

# --- Cloud-init ISOs ---

resource "libvirt_cloudinit_disk" "web_node" {
  count = length(var.web_nodes)
  name  = "${var.web_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.web_nodes[count.index].name}"
    local-hostname = var.web_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.web_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.web_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "web_node_seed" {
  count = length(var.web_nodes)
  name  = "${var.web_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.web_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.web_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_cloudinit_disk" "db_node" {
  count = length(var.db_nodes)
  name  = "${var.db_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.db_nodes[count.index].name}"
    local-hostname = var.db_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.db_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.db_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "db_node_seed" {
  count = length(var.db_nodes)
  name  = "${var.db_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.db_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.db_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_cloudinit_disk" "dns_node" {
  count = length(var.dns_nodes)
  name  = "${var.dns_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.dns_nodes[count.index].name}"
    local-hostname = var.dns_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.dns_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.dns_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "dns_node_seed" {
  count = length(var.dns_nodes)
  name  = "${var.dns_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.dns_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.dns_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_cloudinit_disk" "valkey_node" {
  count = length(var.valkey_nodes)
  name  = "${var.valkey_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.valkey_nodes[count.index].name}"
    local-hostname = var.valkey_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.valkey_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.valkey_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "valkey_node_seed" {
  count = length(var.valkey_nodes)
  name  = "${var.valkey_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.valkey_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.valkey_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_cloudinit_disk" "email_node" {
  count = length(var.email_nodes)
  name  = "${var.email_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.email_nodes[count.index].name}"
    local-hostname = var.email_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.email_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.email_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "email_node_seed" {
  count = length(var.email_nodes)
  name  = "${var.email_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.email_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.email_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_cloudinit_disk" "storage_node" {
  count = length(var.storage_nodes)
  name  = "${var.storage_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.storage_nodes[count.index].name}"
    local-hostname = var.storage_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.storage_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.storage_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "storage_node_seed" {
  count = length(var.storage_nodes)
  name  = "${var.storage_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.storage_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.storage_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_volume" "dbadmin_node" {
  count    = length(var.dbadmin_nodes)
  name     = "${var.dbadmin_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_cloudinit_disk" "dbadmin_node" {
  count = length(var.dbadmin_nodes)
  name  = "${var.dbadmin_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.dbadmin_nodes[count.index].name}"
    local-hostname = var.dbadmin_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.dbadmin_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.dbadmin_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "dbadmin_node_seed" {
  count = length(var.dbadmin_nodes)
  name  = "${var.dbadmin_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.dbadmin_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.dbadmin_node[count.index]]
    ignore_changes       = [create]
  }
}

# --- VM Domains ---

resource "libvirt_domain" "web_node" {
  count       = length(var.web_nodes)
  name        = var.web_nodes[count.index].name
  memory      = var.web_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.web_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.web_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.web_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_domain" "db_node" {
  count       = length(var.db_nodes)
  name        = var.db_nodes[count.index].name
  memory      = var.db_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.db_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.db_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.db_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_domain" "dns_node" {
  count       = length(var.dns_nodes)
  name        = var.dns_nodes[count.index].name
  memory      = var.dns_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.dns_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.dns_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.dns_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_domain" "valkey_node" {
  count       = length(var.valkey_nodes)
  name        = var.valkey_nodes[count.index].name
  memory      = var.valkey_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.valkey_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.valkey_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.valkey_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_domain" "email_node" {
  count       = length(var.email_nodes)
  name        = var.email_nodes[count.index].name
  memory      = var.email_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.email_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.email_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.email_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_domain" "storage_node" {
  count       = length(var.storage_nodes)
  name        = var.storage_nodes[count.index].name
  memory      = var.storage_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.storage_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.storage_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.storage_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.storage_node_osd[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdc", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_domain" "dbadmin_node" {
  count       = length(var.dbadmin_nodes)
  name        = var.dbadmin_nodes[count.index].name
  memory      = var.dbadmin_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.dbadmin_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.dbadmin_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.dbadmin_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_volume" "lb_node" {
  count    = length(var.lb_nodes)
  name     = "${var.lb_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_cloudinit_disk" "lb_node" {
  count = length(var.lb_nodes)
  name  = "${var.lb_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.lb_nodes[count.index].name}"
    local-hostname = var.lb_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.lb_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.lb_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "lb_node_seed" {
  count = length(var.lb_nodes)
  name  = "${var.lb_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.lb_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.lb_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_domain" "lb_node" {
  count       = length(var.lb_nodes)
  name        = var.lb_nodes[count.index].name
  memory      = var.lb_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.lb_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.lb_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.lb_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_volume" "gateway_node" {
  count    = length(var.gateway_nodes)
  name     = "${var.gateway_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_cloudinit_disk" "gateway_node" {
  count = length(var.gateway_nodes)
  name  = "${var.gateway_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.gateway_nodes[count.index].name}"
    local-hostname = var.gateway_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.gateway_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.gateway_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "gateway_node_seed" {
  count = length(var.gateway_nodes)
  name  = "${var.gateway_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.gateway_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.gateway_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_domain" "gateway_node" {
  count       = length(var.gateway_nodes)
  name        = var.gateway_nodes[count.index].name
  memory      = var.gateway_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.gateway_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.gateway_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.gateway_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

# --- Control Plane VM (k3s, not a hosting shard) ---

resource "libvirt_volume" "controlplane_node" {
  count    = length(var.controlplane_nodes)
  name     = "${var.controlplane_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_cloudinit_disk" "controlplane_node" {
  count = length(var.controlplane_nodes)
  name  = "${var.controlplane_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.controlplane_nodes[count.index].name}"
    local-hostname = var.controlplane_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.controlplane_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.controlplane_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "controlplane_node_seed" {
  count = length(var.controlplane_nodes)
  name  = "${var.controlplane_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.controlplane_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.controlplane_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_domain" "controlplane_node" {
  count       = length(var.controlplane_nodes)
  name        = var.controlplane_nodes[count.index].name
  memory      = var.controlplane_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.controlplane_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.controlplane_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.controlplane_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

# --- Single-node (all-in-one) VM ---

resource "libvirt_volume" "single_node" {
  count    = length(var.single_nodes)
  name     = "${var.single_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.image_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "single_node_osd" {
  count    = length(var.single_nodes)
  name     = "${var.single_nodes[count.index].name}-osd.raw"
  pool     = libvirt_pool.hosting.name
  capacity = 10737418240 # 10 GB
  target   = { format = { type = "raw" } }
}

resource "libvirt_cloudinit_disk" "single_node" {
  count = length(var.single_nodes)
  name  = "${var.single_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.single_nodes[count.index].name}"
    local-hostname = var.single_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/node.yaml.tpl", {
    hostname       = var.single_nodes[count.index].name
    ssh_public_key = file(pathexpand(var.ssh_public_key_path))
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.single_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "single_node_seed" {
  count = length(var.single_nodes)
  name  = "${var.single_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.single_node[count.index].path }
  }
  lifecycle {
    replace_triggered_by = [libvirt_cloudinit_disk.single_node[count.index]]
    ignore_changes       = [create]
  }
}

resource "libvirt_domain" "single_node" {
  count       = length(var.single_nodes)
  name        = var.single_nodes[count.index].name
  memory      = var.single_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.single_nodes[count.index].vcpus
  type        = "kvm"
  running     = true
  autostart   = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.single_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.single_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.single_node_osd[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdc", bus = "virtio" }
      },
    ]

    interfaces = [{
      type   = "network"
      source = { network = { network = libvirt_network.hosting.name } }
      model  = { type = "virtio" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

# --- Cluster YAML generation ---

locals {
  all_nodes = concat(
    [for i, n in var.web_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.web_shard_name
      role       = "web"
    }],
    [for i, n in var.db_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.db_shard_name
      role       = "database"
    }],
    [for i, n in var.dns_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.dns_shard_name
      role       = "dns"
    }],
    [for i, n in var.valkey_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.valkey_shard_name
      role       = "valkey"
    }],
    [for i, n in var.email_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.email_shard_name
      role       = "email"
    }],
    [for i, n in var.storage_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.storage_shard_name
      role       = "storage"
    }],
    [for i, n in var.dbadmin_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.dbadmin_shard_name
      role       = "dbadmin"
    }],
    [for i, n in var.lb_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.lb_shard_name
      role       = "lb"
    }],
    [for i, n in var.gateway_nodes : {
      id         = n.name
      name       = n.name
      ip         = n.ip
      shard_name = var.gateway_shard_name
      role       = "gateway"
    }],
  )

  cluster_yaml = templatefile("${path.module}/cluster.yaml.tpl", {
    nodes           = local.all_nodes
    gateway_ip      = var.gateway_ip
    controlplane_ip = var.controlplane_ip
    base_domain     = var.base_domain
    email_node_ip   = var.email_nodes[0].ip
  })

  # Group web nodes by shard for HAProxy backend generation.
  shard_backends = {
    for shard_name, nodes in {
      for node in local.all_nodes : node.shard_name => node... if node.role == "web"
    } : shard_name => [
      for node in nodes : {
        name = node.name
        ip   = node.ip
      }
    ]
  }
}

resource "local_file" "cluster_yaml" {
  content  = local.cluster_yaml
  filename = "${path.module}/../clusters/vm-generated.yaml"
}

resource "local_file" "haproxy_cfg" {
  content = templatefile("${path.module}/templates/haproxy.cfg.tpl", {
    shard_backends = local.shard_backends
    base_domain    = var.base_domain
  })
  filename = "${path.module}/../docker/haproxy/haproxy.cfg"
}
