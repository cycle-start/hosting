variable "libvirt_uri" {
  description = "Libvirt connection URI"
  type        = string
  default     = "qemu:///system"
}

variable "network_cidr" {
  description = "CIDR for the libvirt NAT network"
  type        = string
  default     = "192.168.122.0/24"
}

variable "gateway_ip" {
  description = "Gateway IP (WSL host) on the libvirt network"
  type        = string
  default     = "192.168.122.1"
}

variable "temporal_port" {
  description = "Temporal server port on the host"
  type        = number
  default     = 7233
}

variable "base_image_url" {
  description = "URL of the Ubuntu 24.04 cloud image"
  type        = string
  default     = "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key for VM access"
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}

variable "node_agent_binary" {
  description = "Path to the node-agent binary (built for linux/amd64)"
  type        = string
  default     = "../bin/node-agent"
}

# --- Web nodes ---

variable "web_nodes" {
  description = "Web node definitions"
  type = list(object({
    name   = string
    ip     = string
    memory = optional(number, 1024)
    vcpus  = optional(number, 2)
  }))
  default = [
    { name = "web-1-node-0", ip = "192.168.122.10" },
    { name = "web-1-node-1", ip = "192.168.122.11" },
  ]
}

variable "web_shard_name" {
  description = "Shard name for web nodes"
  type        = string
  default     = "web-1"
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
    { name = "db-1-node-0", ip = "192.168.122.20" },
  ]
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
    memory = optional(number, 512)
    vcpus  = optional(number, 1)
  }))
  default = [
    { name = "dns-1-node-0", ip = "192.168.122.30" },
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
    memory = optional(number, 512)
    vcpus  = optional(number, 1)
  }))
  default = [
    { name = "valkey-1-node-0", ip = "192.168.122.40" },
  ]
}

variable "valkey_shard_name" {
  description = "Shard name for valkey nodes"
  type        = string
  default     = "valkey-1"
}
