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

# --- Volumes (cloned from base image) ---

resource "libvirt_volume" "web_node" {
  count          = length(var.web_nodes)
  name           = "${var.web_nodes[count.index].name}.qcow2"
  pool           = libvirt_pool.hosting.name
  base_volume_id = libvirt_volume.ubuntu_base.id
  size           = 10737418240 # 10 GB
}

resource "libvirt_volume" "db_node" {
  count          = length(var.db_nodes)
  name           = "${var.db_nodes[count.index].name}.qcow2"
  pool           = libvirt_pool.hosting.name
  base_volume_id = libvirt_volume.ubuntu_base.id
  size           = 21474836480 # 20 GB
}

resource "libvirt_volume" "dns_node" {
  count          = length(var.dns_nodes)
  name           = "${var.dns_nodes[count.index].name}.qcow2"
  pool           = libvirt_pool.hosting.name
  base_volume_id = libvirt_volume.ubuntu_base.id
  size           = 5368709120 # 5 GB
}

resource "libvirt_volume" "valkey_node" {
  count          = length(var.valkey_nodes)
  name           = "${var.valkey_nodes[count.index].name}.qcow2"
  pool           = libvirt_pool.hosting.name
  base_volume_id = libvirt_volume.ubuntu_base.id
  size           = 5368709120 # 5 GB
}

# --- Cloud-init ISOs ---

resource "libvirt_cloudinit_disk" "web_node" {
  count = length(var.web_nodes)
  name  = "${var.web_nodes[count.index].name}-cloudinit.iso"
  pool  = libvirt_pool.hosting.name
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

resource "libvirt_cloudinit_disk" "db_node" {
  count = length(var.db_nodes)
  name  = "${var.db_nodes[count.index].name}-cloudinit.iso"
  pool  = libvirt_pool.hosting.name
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

resource "libvirt_cloudinit_disk" "dns_node" {
  count = length(var.dns_nodes)
  name  = "${var.dns_nodes[count.index].name}-cloudinit.iso"
  pool  = libvirt_pool.hosting.name
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

resource "libvirt_cloudinit_disk" "valkey_node" {
  count = length(var.valkey_nodes)
  name  = "${var.valkey_nodes[count.index].name}-cloudinit.iso"
  pool  = libvirt_pool.hosting.name
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

# --- VM Domains ---

resource "libvirt_domain" "web_node" {
  count  = length(var.web_nodes)
  name   = var.web_nodes[count.index].name
  memory = var.web_nodes[count.index].memory
  vcpu   = var.web_nodes[count.index].vcpus

  cloudinit = libvirt_cloudinit_disk.web_node[count.index].id

  network_interface {
    network_id     = libvirt_network.hosting.id
    addresses      = [var.web_nodes[count.index].ip]
    wait_for_lease = false
  }

  disk {
    volume_id = libvirt_volume.web_node[count.index].id
  }

  filesystem {
    source   = abspath("${path.module}/../bin")
    target   = "hostbin"
    readonly = true
  }

  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
  }
}

resource "libvirt_domain" "db_node" {
  count  = length(var.db_nodes)
  name   = var.db_nodes[count.index].name
  memory = var.db_nodes[count.index].memory
  vcpu   = var.db_nodes[count.index].vcpus

  cloudinit = libvirt_cloudinit_disk.db_node[count.index].id

  network_interface {
    network_id     = libvirt_network.hosting.id
    addresses      = [var.db_nodes[count.index].ip]
    wait_for_lease = false
  }

  disk {
    volume_id = libvirt_volume.db_node[count.index].id
  }

  filesystem {
    source   = abspath("${path.module}/../bin")
    target   = "hostbin"
    readonly = true
  }

  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
  }
}

resource "libvirt_domain" "dns_node" {
  count  = length(var.dns_nodes)
  name   = var.dns_nodes[count.index].name
  memory = var.dns_nodes[count.index].memory
  vcpu   = var.dns_nodes[count.index].vcpus

  cloudinit = libvirt_cloudinit_disk.dns_node[count.index].id

  network_interface {
    network_id     = libvirt_network.hosting.id
    addresses      = [var.dns_nodes[count.index].ip]
    wait_for_lease = false
  }

  disk {
    volume_id = libvirt_volume.dns_node[count.index].id
  }

  filesystem {
    source   = abspath("${path.module}/../bin")
    target   = "hostbin"
    readonly = true
  }

  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
  }
}

resource "libvirt_domain" "valkey_node" {
  count  = length(var.valkey_nodes)
  name   = var.valkey_nodes[count.index].name
  memory = var.valkey_nodes[count.index].memory
  vcpu   = var.valkey_nodes[count.index].vcpus

  cloudinit = libvirt_cloudinit_disk.valkey_node[count.index].id

  network_interface {
    network_id     = libvirt_network.hosting.id
    addresses      = [var.valkey_nodes[count.index].ip]
    wait_for_lease = false
  }

  disk {
    volume_id = libvirt_volume.valkey_node[count.index].id
  }

  filesystem {
    source   = abspath("${path.module}/../bin")
    target   = "hostbin"
    readonly = true
  }

  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
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
  )

  cluster_yaml = templatefile("${path.module}/cluster.yaml.tpl", {
    nodes      = local.all_nodes
    gateway_ip = var.gateway_ip
  })
}

resource "local_file" "cluster_yaml" {
  content  = local.cluster_yaml
  filename = "${path.module}/../clusters/vm-generated.yaml"
}
