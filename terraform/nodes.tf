# --- Random UUIDs for node IDs ---

resource "random_uuid" "web_node_id" {
  count = length(var.web_nodes)
}

resource "random_uuid" "db_node_id" {
  count = length(var.db_nodes)
}

resource "random_uuid" "dns_node_id" {
  count = length(var.dns_nodes)
}

resource "random_uuid" "valkey_node_id" {
  count = length(var.valkey_nodes)
}

resource "random_uuid" "s3_node_id" {
  count = length(var.s3_nodes)
}

# --- Volumes (backed by base image) ---

resource "libvirt_volume" "web_node" {
  count    = length(var.web_nodes)
  name     = "${var.web_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 10737418240 # 10 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.ubuntu_base.path
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
    path   = libvirt_volume.ubuntu_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "dns_node" {
  count    = length(var.dns_nodes)
  name     = "${var.dns_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 5368709120 # 5 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.ubuntu_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "valkey_node" {
  count    = length(var.valkey_nodes)
  name     = "${var.valkey_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 5368709120 # 5 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.ubuntu_base.path
    format = { type = "qcow2" }
  }
}

resource "libvirt_volume" "s3_node" {
  count    = length(var.s3_nodes)
  name     = "${var.s3_nodes[count.index].name}.qcow2"
  pool     = libvirt_pool.hosting.name
  capacity = 21474836480 # 20 GB
  target   = { format = { type = "qcow2" } }
  backing_store = {
    path   = libvirt_volume.ubuntu_base.path
    format = { type = "qcow2" }
  }
}

# Dedicated OSD data disk for Ceph BlueStore.
resource "libvirt_volume" "s3_node_osd" {
  count    = length(var.s3_nodes)
  name     = "${var.s3_nodes[count.index].name}-osd.raw"
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
  user_data = templatefile("${path.module}/cloud-init/web-node.yaml.tpl", {
    hostname         = var.web_nodes[count.index].name
    node_id          = random_uuid.web_node_id[count.index].result
    shard_name       = var.web_shard_name
    temporal_address = "${var.gateway_ip}:${var.temporal_port}"
    ssh_public_key   = file(pathexpand(var.ssh_public_key_path))
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
}

resource "libvirt_cloudinit_disk" "db_node" {
  count = length(var.db_nodes)
  name  = "${var.db_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.db_nodes[count.index].name}"
    local-hostname = var.db_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/db-node.yaml.tpl", {
    hostname         = var.db_nodes[count.index].name
    node_id          = random_uuid.db_node_id[count.index].result
    shard_name       = var.db_shard_name
    temporal_address = "${var.gateway_ip}:${var.temporal_port}"
    ssh_public_key   = file(pathexpand(var.ssh_public_key_path))
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
}

resource "libvirt_cloudinit_disk" "dns_node" {
  count = length(var.dns_nodes)
  name  = "${var.dns_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.dns_nodes[count.index].name}"
    local-hostname = var.dns_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/dns-node.yaml.tpl", {
    hostname         = var.dns_nodes[count.index].name
    node_id          = random_uuid.dns_node_id[count.index].result
    shard_name       = var.dns_shard_name
    temporal_address = "${var.gateway_ip}:${var.temporal_port}"
    ssh_public_key   = file(pathexpand(var.ssh_public_key_path))
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
}

resource "libvirt_cloudinit_disk" "valkey_node" {
  count = length(var.valkey_nodes)
  name  = "${var.valkey_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.valkey_nodes[count.index].name}"
    local-hostname = var.valkey_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/valkey-node.yaml.tpl", {
    hostname         = var.valkey_nodes[count.index].name
    node_id          = random_uuid.valkey_node_id[count.index].result
    shard_name       = var.valkey_shard_name
    temporal_address = "${var.gateway_ip}:${var.temporal_port}"
    ssh_public_key   = file(pathexpand(var.ssh_public_key_path))
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
}

resource "libvirt_cloudinit_disk" "s3_node" {
  count = length(var.s3_nodes)
  name  = "${var.s3_nodes[count.index].name}-cloudinit.iso"
  meta_data = yamlencode({
    instance-id    = "i-${var.s3_nodes[count.index].name}"
    local-hostname = var.s3_nodes[count.index].name
  })
  user_data = templatefile("${path.module}/cloud-init/s3-node.yaml.tpl", {
    hostname         = var.s3_nodes[count.index].name
    node_id          = random_uuid.s3_node_id[count.index].result
    shard_name       = var.s3_shard_name
    temporal_address = "${var.gateway_ip}:${var.temporal_port}"
    ssh_public_key   = file(pathexpand(var.ssh_public_key_path))
    ip_address       = var.s3_nodes[count.index].ip
  })
  network_config = templatefile("${path.module}/cloud-init/network.yaml.tpl", {
    ip_address = var.s3_nodes[count.index].ip
    gateway    = var.gateway_ip
  })
}

resource "libvirt_volume" "s3_node_seed" {
  count = length(var.s3_nodes)
  name  = "${var.s3_nodes[count.index].name}-seed.iso"
  pool  = libvirt_pool.hosting.name
  create = {
    content = { url = libvirt_cloudinit_disk.s3_node[count.index].path }
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

    filesystems = [{
      type       = "mount"
      accessmode = "passthrough"
      source     = { mount = { dir = abspath("${path.module}/../bin") } }
      target     = { dir = "hostbin" }
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

    filesystems = [{
      type       = "mount"
      accessmode = "passthrough"
      source     = { mount = { dir = abspath("${path.module}/../bin") } }
      target     = { dir = "hostbin" }
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

    filesystems = [{
      type       = "mount"
      accessmode = "passthrough"
      source     = { mount = { dir = abspath("${path.module}/../bin") } }
      target     = { dir = "hostbin" }
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

    filesystems = [{
      type       = "mount"
      accessmode = "passthrough"
      source     = { mount = { dir = abspath("${path.module}/../bin") } }
      target     = { dir = "hostbin" }
    }]

    consoles = [{
      type   = "pty"
      target = { type = "serial", port = "0" }
    }]
  }
}

resource "libvirt_domain" "s3_node" {
  count       = length(var.s3_nodes)
  name        = var.s3_nodes[count.index].name
  memory      = var.s3_nodes[count.index].memory
  memory_unit = "MiB"
  vcpu        = var.s3_nodes[count.index].vcpus
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
            volume = libvirt_volume.s3_node[count.index].name
          }
        }
        driver = { type = "qcow2" }
        target = { dev = "vda", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.s3_node_seed[count.index].name
          }
        }
        driver = { type = "raw" }
        target = { dev = "vdb", bus = "virtio" }
      },
      {
        source = {
          volume = {
            pool   = libvirt_pool.hosting.name
            volume = libvirt_volume.s3_node_osd[count.index].name
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

    filesystems = [{
      type       = "mount"
      accessmode = "passthrough"
      source     = { mount = { dir = abspath("${path.module}/../bin") } }
      target     = { dir = "hostbin" }
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
      id         = random_uuid.web_node_id[i].result
      name       = n.name
      ip         = n.ip
      shard_name = var.web_shard_name
      role       = "web"
    }],
    [for i, n in var.db_nodes : {
      id         = random_uuid.db_node_id[i].result
      name       = n.name
      ip         = n.ip
      shard_name = var.db_shard_name
      role       = "database"
    }],
    [for i, n in var.dns_nodes : {
      id         = random_uuid.dns_node_id[i].result
      name       = n.name
      ip         = n.ip
      shard_name = var.dns_shard_name
      role       = "dns"
    }],
    [for i, n in var.valkey_nodes : {
      id         = random_uuid.valkey_node_id[i].result
      name       = n.name
      ip         = n.ip
      shard_name = var.valkey_shard_name
      role       = "valkey"
    }],
    [for i, n in var.s3_nodes : {
      id         = random_uuid.s3_node_id[i].result
      name       = n.name
      ip         = n.ip
      shard_name = var.s3_shard_name
      role       = "s3"
    }],
  )

  cluster_yaml = templatefile("${path.module}/cluster.yaml.tpl", {
    nodes      = local.all_nodes
    gateway_ip = var.gateway_ip
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
  })
  filename = "${path.module}/../docker/haproxy/haproxy.cfg"
}
