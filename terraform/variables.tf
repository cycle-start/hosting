variable "libvirt_uri" {
  description = "Libvirt connection URI"
  type        = string
  default     = "qemu:///system"
}

variable "network_cidr" {
  description = "CIDR for the libvirt NAT network"
  type        = string
  default     = "10.10.10.0/24"
}

variable "gateway_ip" {
  description = "Gateway IP (WSL host) on the libvirt network"
  type        = string
  default     = "10.10.10.1"
}

variable "temporal_port" {
  description = "Temporal server port on the host"
  type        = number
  default     = 7233
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key for VM access"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

# --- Control plane ---

variable "controlplane_ip" {
  description = "IP address for the k3s control plane VM"
  type        = string
  default     = "10.10.10.2"
}

variable "base_domain" {
  description = "Base domain for control plane hostnames (e.g. admin.<base_domain>)"
  type        = string
  default     = "massive-hosting.com"
}

variable "region_id" {
  description = "Region identifier for observability labels"
  type        = string
  default     = "dev"
}

variable "cluster_id" {
  description = "Cluster identifier for observability labels"
  type        = string
  default     = "vm-cluster-1"
}

variable "controlplane_nodes" {
  description = "Control plane node definitions"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 4096)
    vcpus  = optional(number, 4)
  }))
  default = [
    { name = "controlplane-0", ip = "10.10.10.2" },
  ]
}

# --- Web nodes ---

variable "web_nodes" {
  description = "Web node definitions"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 8192)
    vcpus  = optional(number, 2)
  }))
  default = [
    { name = "web-1-node-0", ip = "10.10.10.10" },
    { name = "web-1-node-1", ip = "10.10.10.11" },
  ]
}

variable "web_shard_name" {
  description = "Shard name for web nodes"
  type        = string
  default     = "web-1"
}

variable "core_api_token" {
  description = "Bearer token for node-agent to call core-api internal endpoints (cron outcome reporting)"
  type        = string
  default     = "hst_dev_e2e_test_key_00000000"
  sensitive   = true
}

# --- DB nodes ---

variable "db_nodes" {
  description = "Database node definitions"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 1024)
    vcpus  = optional(number, 2)
  }))
  default = [
    { name = "db-1-node-0", ip = "10.10.10.20" },
    { name = "db-1-node-1", ip = "10.10.10.21" },
  ]
}

variable "db_repl_password" {
  description = "MySQL replication user password"
  type        = string
  default     = "repl"
  sensitive   = true
}

variable "db_shard_name" {
  description = "Shard name for database nodes"
  type        = string
  default     = "db-1"
}

# --- DNS nodes ---

variable "dns_nodes" {
  description = "DNS node definitions"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 4096)
    vcpus  = optional(number, 1)
  }))
  default = [
    { name = "dns-1-node-0", ip = "10.10.10.30" },
  ]
}

variable "dns_shard_name" {
  description = "Shard name for DNS nodes"
  type        = string
  default     = "dns-1"
}

# --- Valkey nodes ---

variable "valkey_nodes" {
  description = "Valkey node definitions"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 4096)
    vcpus  = optional(number, 1)
  }))
  default = [
    { name = "valkey-1-node-0", ip = "10.10.10.40" },
  ]
}

variable "valkey_shard_name" {
  description = "Shard name for valkey nodes"
  type        = string
  default     = "valkey-1"
}

# --- Email nodes ---

variable "email_nodes" {
  description = "Email node definitions (runs Stalwart + node-agent)"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 4096)
    vcpus  = optional(number, 2)
  }))
  default = [
    { name = "email-1-node-0", ip = "10.10.10.80" },
  ]
}

variable "email_shard_name" {
  description = "Shard name for email nodes"
  type        = string
  default     = "email-1"
}

variable "stalwart_admin_password" {
  description = "Stalwart fallback admin password"
  type        = string
  default     = "dev-token"
  sensitive   = true
}

# --- Storage nodes (Ceph: S3/RGW + CephFS/MDS) ---

variable "storage_nodes" {
  description = "Storage node definitions (runs Ceph mon+mgr+osd+rgw+mds + node-agent)"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 16384)
    vcpus  = optional(number, 2)
  }))
  default = [
    { name = "storage-1-node-0", ip = "10.10.10.50" },
  ]
}

variable "storage_shard_name" {
  description = "Shard name for storage nodes"
  type        = string
  default     = "storage-1"
}

# --- DB Admin nodes ---

variable "dbadmin_nodes" {
  description = "DB Admin node definitions (runs phpMyAdmin + node-agent)"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 2048)
    vcpus  = optional(number, 2)
  }))
  default = [
    { name = "dbadmin-1-node-0", ip = "10.10.10.60" },
  ]
}

variable "dbadmin_shard_name" {
  description = "Shard name for DB Admin nodes"
  type        = string
  default     = "dbadmin-1"
}

# --- LB nodes ---

variable "lb_nodes" {
  description = "LB node definitions (runs HAProxy + node-agent)"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 2048)
    vcpus  = optional(number, 2)
  }))
  default = [
    { name = "lb-1-node-0", ip = "10.10.10.70" },
  ]
}

variable "lb_shard_name" {
  description = "Shard name for LB nodes"
  type        = string
  default     = "lb-1"
}

# --- Gateway nodes ---

variable "gateway_nodes" {
  description = "List of gateway node definitions"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 2048)
    vcpus  = optional(number, 2)
  }))
  default = [
    {
      name = "gateway-1-node-0"
      ip   = "10.10.10.90"
    }
  ]
}

variable "gateway_shard_name" {
  description = "Name of the gateway shard"
  type        = string
  default     = "gateway-1"
}
