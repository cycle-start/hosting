output "web_node_ips" {
  description = "IP addresses of web nodes"
  value       = { for k, v in var.web_nodes : v.name => v.ip }
}

output "db_node_ips" {
  description = "IP addresses of database nodes"
  value       = { for k, v in var.db_nodes : v.name => v.ip }
}

output "dns_node_ips" {
  description = "IP addresses of DNS nodes"
  value       = { for k, v in var.dns_nodes : v.name => v.ip }
}

output "valkey_node_ips" {
  description = "IP addresses of Valkey nodes"
  value       = { for k, v in var.valkey_nodes : v.name => v.ip }
}

output "web_node_ids" {
  description = "Generated UUIDs for web nodes"
  value       = { for k, v in random_uuid.web_node_id : var.web_nodes[k].name => v.result }
}

output "db_node_ids" {
  description = "Generated UUIDs for DB nodes"
  value       = { for k, v in random_uuid.db_node_id : var.db_nodes[k].name => v.result }
}

output "dns_node_ids" {
  description = "Generated UUIDs for DNS nodes"
  value       = { for k, v in random_uuid.dns_node_id : var.dns_nodes[k].name => v.result }
}

output "valkey_node_ids" {
  description = "Generated UUIDs for Valkey nodes"
  value       = { for k, v in random_uuid.valkey_node_id : var.valkey_nodes[k].name => v.result }
}

output "storage_node_ips" {
  description = "IP addresses of storage/Ceph nodes"
  value       = { for k, v in var.storage_nodes : v.name => v.ip }
}

output "storage_node_ids" {
  description = "Generated UUIDs for storage nodes"
  value       = { for k, v in random_uuid.storage_node_id : var.storage_nodes[k].name => v.result }
}

output "dbadmin_node_ips" {
  description = "IP addresses of DB Admin nodes"
  value       = { for k, v in var.dbadmin_nodes : v.name => v.ip }
}

output "dbadmin_node_ids" {
  description = "Generated UUIDs for DB Admin nodes"
  value       = { for k, v in random_uuid.dbadmin_node_id : var.dbadmin_nodes[k].name => v.result }
}

output "lb_node_ips" {
  description = "IP addresses of LB nodes"
  value       = { for k, v in var.lb_nodes : v.name => v.ip }
}

output "lb_node_ids" {
  description = "Generated UUIDs for LB nodes"
  value       = { for k, v in random_uuid.lb_node_id : var.lb_nodes[k].name => v.result }
}

output "email_node_ips" {
  description = "IP addresses of email nodes"
  value       = { for k, v in var.email_nodes : v.name => v.ip }
}

output "email_node_ids" {
  description = "Generated UUIDs for email nodes"
  value       = { for k, v in random_uuid.email_node_id : var.email_nodes[k].name => v.result }
}

output "gateway_node_ips" {
  description = "IP addresses of gateway nodes"
  value       = { for k, v in var.gateway_nodes : v.name => v.ip }
}

output "gateway_node_ids" {
  description = "Generated UUIDs for gateway nodes"
  value       = { for k, v in random_uuid.gateway_node_id : var.gateway_nodes[k].name => v.result }
}

output "controlplane_ip" {
  description = "IP address of the k3s control plane VM"
  value       = var.controlplane_ip
}

output "cluster_yaml" {
  description = "Generated cluster YAML for hostctl cluster apply"
  value       = local.cluster_yaml
}
